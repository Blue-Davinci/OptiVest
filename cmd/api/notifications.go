package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// sseClientChanBuffer bounds how many pending notifications a single SSE client
// can buffer before the producer drops messages. With a slow consumer the
// in-memory queue is bounded; persistence is handled by the Redis pending key.
const sseClientChanBuffer = 32

// ServeSSE streams notifications to a single connected client.
//
// The handler reads from a per-user channel returned by AddClient (snapshotted
// once on entry, no map lookups per iteration) and exits cleanly when either
// (a) the client disconnects, (b) the channel is closed by AddClient because a
// newer connection took over, or (c) the request context is cancelled by
// graceful shutdown.
func (app *application) ServeSSE(w http.ResponseWriter, r *http.Request) {
	userID := app.contextGetUser(r).ID

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Register the client and capture the channel reference up front so the hot
	// loop never indexes the Clients map (which previously raced with AddClient
	// and RemoveClient mutating it concurrently).
	ch := app.AddClient(userID)
	defer app.RemoveClient(userID)

	// Pending notifications can be loaded asynchronously; this is a one-time
	// operation per connection, idempotent across reconnects.
	go app.loadAndSendPendingNotifications(userID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	app.logger.Info("SSE client connected", zap.Int64("userID", userID))

	for {
		select {
		case msg, open := <-ch:
			if !open {
				return // channel closed (newer connection or shutdown)
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// loadAndSendPendingNotifications loads and sends pending notifications from
// Redis and the database. It is invoked from a goroutine kicked off by
// AddClient, so it is not request-scoped — we bind to the application
// lifecycle context (app.ctx) so that on graceful shutdown the in-flight
// Redis read is cancelled cleanly.
func (app *application) loadAndSendPendingNotifications(userID int64) {
	app.logger.Info("Loading and sending pending notifications for user:", zap.Int64("userID", userID))
	ctx := app.ctx

	// Track processed notifications using a map for deduplication
	processedNotifications := make(map[int64]bool)
	// Load from Redis
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, userID)
	err := app.loadAndProcessRedisData(ctx, userID, pendingKey, &processedNotifications)
	if err != nil {
		app.logger.Info("Error loading notifications from Redis:", zap.Error(err))
	}

	// Load from Database
	err = app.loadAndProcessDBData(userID, &processedNotifications)
	if err != nil {
		app.logger.Info("Error loading notifications from database:", zap.Error(err))
	}
}

// loadAndProcessRedisData processes pending notifications from Redis
func (app *application) loadAndProcessRedisData(ctx context.Context, userID int64, pendingKey string, processed *map[int64]bool) error {
	pendingNotifications, err := app.RedisDB.HGetAll(ctx, pendingKey).Result()
	if err != nil {
		return err
	}
	if len(pendingNotifications) == 0 {
		app.logger.Info("No pending notifications found in Redis for user:", zap.Int64("userID", userID))
		return nil
	}

	// Send notifications and remove from Redis if successful
	for _, notificationJSON := range pendingNotifications {
		var notification data.NotificationContent
		if err := json.Unmarshal([]byte(notificationJSON), &notification); err != nil {
			app.logger.Error("Failed to unmarshal notification from Redis", zap.Error(err))
			continue
		}
		// Deduplication check
		if _, exists := (*processed)[notification.NotificationID]; exists {
			app.logger.Info("Skipping duplicate notification from Redis", zap.Int64("notification_id", notification.NotificationID))
			continue
		}
		app.PublishNotification(userID, notification)
		(*processed)[notification.NotificationID] = true // Mark as processed
	}
	return app.RedisDB.Del(ctx, pendingKey).Err()
}

// loadAndProcessDBData loads and processes pending notifications from the database
func (app *application) loadAndProcessDBData(userID int64, processed *map[int64]bool) error {
	pendingNotifications, err := app.models.NotificationManager.GetUnreadNotifications(userID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.logger.Info("No pending notifications found in database for user:", zap.Int64("userID", userID))
			return nil // not really an error, just no notifications found
		default:
			return err
		}
	}

	for _, notification := range pendingNotifications {
		// Deduplication check
		if _, exists := (*processed)[notification.ID]; exists {
			app.logger.Info("Skipping duplicate notification from DB", zap.Int64("notification_id", notification.ID))
			continue
		}
		app.logger.Info("Pending notifications recieved from Database", zap.Int64("Notification ID", notification.ID))
		var notificationMeta data.NotificationMeta
		err := json.Unmarshal([]byte(notification.Meta), &notificationMeta)
		if err != nil {
			app.logger.Error("Failed to unmarshal notification meta", zap.Error(err))
			continue
		}
		// create our notification content
		notificationContent := data.NotificationContent{
			NotificationID: notification.ID,
			Message:        notification.Message,
			SentAt:         notification.CreatedAt,
			Meta:           notificationMeta,
		}
		app.logger.Info("Pending notifications sent from Database", zap.Int64("Notification ID", notification.ID))
		// Publish to the pub/sub system
		app.PublishNotification(userID, notificationContent)
		// update the notification status to delivered
		err = app.updateDatabaseNotificationStatus(notification.ID, data.NotificationStatusTypeDelivered)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrEditConflict):
				return data.ErrEditConflict
			default:
				return err
			}
		}
	}
	return nil
}

// AddClient registers a new SSE connection for userID and returns the
// receive-only channel the caller should read from. If the user already had
// an active connection, the previous channel is closed (so the previous
// goroutine exits cleanly) and a new one takes its place.
//
// The previous implementation held app.Mutex while calling RemoveClient,
// which deadlocked because RemoveClient also takes the mutex. Lock-protected
// state mutation is now consolidated into a single critical section that
// performs both removal and re-registration in place.
func (app *application) AddClient(userID int64) <-chan string {
	ch := make(chan string, sseClientChanBuffer)

	app.Mutex.Lock()
	// If a previous connection exists, close its channel and cancel its
	// pubsub context so the old SSE goroutine and the old Redis listener
	// terminate before we install the new ones.
	if prev, exists := app.Clients[userID]; exists {
		close(prev)
		app.logger.Info("SSE client replacing previous connection", zap.Int64("userID", userID))
	}
	if cancel, exists := app.ClientCancelFuncs[userID]; exists {
		cancel()
		delete(app.ClientCancelFuncs, userID)
	}
	delete(app.ListeningUsers, userID)

	// Install the new channel and start a fresh per-user pubsub listener.
	// The pubsub ctx derives from app.ctx, not context.Background(), so on
	// graceful shutdown every per-user Redis subscription is cancelled in
	// addition to the explicit RemoveClient calls — no goroutine leak even
	// if a client never disconnects cleanly.
	ctx, cancelFunc := context.WithCancel(app.ctx)
	app.Clients[userID] = ch
	app.ClientCancelFuncs[userID] = cancelFunc
	app.ListeningUsers[userID] = true
	app.Mutex.Unlock()

	go app.ListenForRedisPubSubUserMessages(ctx, userID)
	return ch
}

// RemoveClient closes the channel associated with userID (if any) and stops
// the per-user Redis listener. It is safe to call multiple times.
func (app *application) RemoveClient(userID int64) {
	app.Mutex.Lock()
	ch, hadChannel := app.Clients[userID]
	cancel, hadCancel := app.ClientCancelFuncs[userID]
	delete(app.Clients, userID)
	delete(app.ClientCancelFuncs, userID)
	delete(app.ListeningUsers, userID)
	app.Mutex.Unlock()

	if hadChannel {
		// Close outside the lock so any in-flight producer that is currently
		// blocked on `select { case ch <- msg: ... case <-time.After(...) }`
		// is not held back by us holding the registry mutex.
		close(ch)
		app.logger.Info("SSE client disconnected", zap.Int64("userID", userID))
	}
	if hadCancel {
		cancel()
	}
}

// sseSendTimeout bounds how long a producer waits for a slow SSE consumer
// before dropping the live message and persisting it to Redis instead.
const sseSendTimeout = 100 * time.Millisecond

// PublishNotification publishes a message to a specific user's SSE channel if
// they are online. If the user is offline, or the connection is too slow to
// keep up, the notification is persisted to Redis for future delivery on the
// user's next reconnect.
//
// Concurrency model:
//   - Producers acquire RLock and hold it for the duration of the bounded send.
//   - RemoveClient/AddClient acquire Lock to close+replace channels.
//
// Because Go's RWMutex prevents readers and writers from overlapping, the
// producer can never send into a closed channel: any close must wait for our
// RUnlock. The bounded timeout prevents a slow client from blocking writers
// indefinitely.
func (app *application) PublishNotification(userID int64, notification data.NotificationContent) {
	notification.SentAt = time.Now()
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		app.logger.Error("Failed to marshal notification content", zap.Error(err))
		return
	}
	msg := string(notificationJSON)

	delivered := false
	app.Mutex.RLock()
	ch, online := app.Clients[userID]
	if online {
		select {
		case ch <- msg:
			delivered = true
		case <-time.After(sseSendTimeout):
			app.logger.Warn("SSE buffer full; dropping live message and persisting to Redis",
				zap.Int64("userID", userID))
		}
	}
	app.Mutex.RUnlock()

	var statusErr error
	if delivered {
		statusErr = app.updateDatabaseNotificationStatus(notification.NotificationID, data.NotificationStatusTypeDelivered)
	} else {
		if storeErr := app.storeNotificationInRedis(userID, notification); storeErr != nil {
			app.logger.Error("Error storing notification in Redis", zap.Error(storeErr))
		}
		statusErr = app.updateDatabaseNotificationStatus(notification.NotificationID, data.NotificationStatusTypePending)
		if !online {
			app.logger.Info("User is offline; notification stored in Redis", zap.Int64("userID", userID))
		}
	}
	if statusErr != nil {
		app.logger.Error("Error updating notification status in database", zap.Error(statusErr))
	}
}

// storeNotificationInRedis saves the notification to Redis for delivery when
// the user reconnects. It runs as part of fan-out from PublishNotification,
// which itself is called both from request flows and background schedulers.
// We bind to app.ctx so the write completes regardless of whether the
// triggering request has gone away — the user expects the notification to
// arrive, not to be cancelled when their browser tab closes.
func (app *application) storeNotificationInRedis(userID int64, notification data.NotificationContent) error {
	ctx := app.ctx
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, userID)

	// Marshal the notification for storage in Redis
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	// Store the notification in Redis under the pending key
	err = app.RedisDB.HSet(ctx, pendingKey, notification.NotificationID, notificationJSON).Err()
	if err != nil {
		return err
	}

	// Set the TTL for the pending notification
	err = app.RedisDB.Expire(ctx, pendingKey, data.DefaultRedisNotificationTTLDuration).Err()
	if err != nil {
		app.logger.Error("error setting TTL for pending notification: %v", zap.Error(err))
		return err
	}
	return nil
}

// updateDatabaseNotificationStatus updates the status of a notification in the database
func (app *application) updateDatabaseNotificationStatus(notificationID int64, status database.NotificationStatus) error {
	err := app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
		notificationID,
		sql.NullTime{Time: time.Time{}, Valid: false},
		status,
	)
	if err != nil {
		return err
	}
	return nil
}

// PublishNotificationToRedis publishes a message to a specific user's Redis pub/sub channel
func (app *application) PublishNotificationToRedis(userID int64, notificationType string, notification data.NotificationContent) error {
	app.logger.Info("Publishing notification to Redis", zap.Int64("userID", userID))
	// redis key
	channel := fmt.Sprintf("%s:%d", data.RedisNotManNotificationKey, userID)
	// marshal the meta data to JSON
	metaJSON, err := json.Marshal(notification.Meta)
	if err != nil {
		return err
	}
	// attempt to save the notification to the database
	savedNotification := &data.Notification{
		Message:          notification.Message,
		NotificationType: notificationType,
		Status:           data.NotificationStatusTypePending,
		Meta:             metaJSON,
		RedisKey:         &channel,
	}
	err = app.models.NotificationManager.CreateNewNotification(userID, savedNotification)
	if err != nil {
		return err
	}
	// set the notification ID
	notification.NotificationID = savedNotification.ID
	// set status to pending
	notification.Status = data.NotificationStatusTypePending
	// notification type
	notification.NotificationType = notificationType
	// we are going to publish the notification to the user's Redis channel
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	// publish the notification to the user's Redis channel. Bound to app.ctx
	// rather than the originating request so notifications still get delivered
	// even if the user disconnects mid-request (the recipient — who may be a
	// different user entirely — should not pay for the originator's tab close).
	if err := app.RedisDB.Publish(app.ctx, channel, string(notificationJSON)).Err(); err != nil {
		return err
	}
	return nil
}

// ListenForUserMessages listens to Redis pub/sub and sends messages to the specific user's SSE channel
func (app *application) ListenForRedisPubSubUserMessages(ctx context.Context, userID int64) {
	pubsub := app.RedisDB.Subscribe(ctx, fmt.Sprintf("%s:%d", data.RedisNotManNotificationKey, userID))

	defer pubsub.Close()

	for {
		select {
		case <-ctx.Done(): // Exit loop when context is canceled
			app.logger.Info("Stopping Redis pub/sub listener for user", zap.Int64("userID", userID))
			return
		case msg := <-pubsub.Channel():
			var notification data.NotificationContent
			if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
				app.logger.Error("Failed to unmarshal Redis message", zap.Error(err))
				continue
			}
			app.PublishNotification(userID, notification)
		}
	}
}

func (app *application) listenToAwardNotifications() {
	app.logger.Info("Starting PostgreSQL listener on channel 'new_award'")

	// Initialize the PostgreSQL listener
	listener := pq.NewListener(app.config.db.dsn, 10*time.Second, time.Minute, func(event pq.ListenerEventType, err error) {
		if err != nil {
			app.logger.Error("PostgreSQL award listener error", zap.Error(err))
		}
	})

	// Listen to the 'new_award' channel
	err := listener.Listen("new_award")
	if err != nil {
		app.logger.Error("Error listening to PostgreSQL notifications", zap.Error(err))
		return
	}

	// Goroutine to process notifications as they arrive
	go func() {
		for {
			select {
			case notification := <-listener.Notify:
				if notification != nil {
					// Parse the JSON payload from the notification
					var payload struct {
						AwardID int32 `json:"award_id"`
						UserID  int64 `json:"user_id"`
					}
					// Unmarshal the JSON payload
					err := json.Unmarshal([]byte(notification.Extra), &payload)
					if err != nil {
						app.logger.Error("Failed to parse notification payload", zap.Error(err))
						continue
					}
					// convert the award ID to int32
					// get the award by award ID
					award, err := app.models.AwardManager.GetAwardByAwardID(app.ctx, payload.AwardID)
					if err != nil {
						app.logger.Error("Failed to get award by award ID", zap.Error(err))
						continue
					}

					// Log the received award and user IDs
					app.logger.Info(fmt.Sprintf("New award notification received: Award ID %d, User ID %d", payload.AwardID, payload.UserID))
					// Prepare the notification content
					notificationContent := data.NotificationContent{
						Message: fmt.Sprintf("A new award has been granted!<br>Award_Name: %s<br>Award_Description: %s<br>Award_Points: %d",
							award.Code, award.Description, award.Points),
						Meta: data.NotificationMeta{
							Url:      app.config.frontend.awardurl,
							ImageUrl: award.AwardImageUrl,
							Tags:     "award",
						},
					}

					// Publish the notification to Redis for the user
					err = app.PublishNotificationToRedis(payload.UserID, "new_award", notificationContent)
					if err != nil {
						app.logger.Error("Error publishing award notification to Redis", zap.Error(err))
					}
				}
			case <-time.After(90 * time.Second): // Ping the listener every 90 seconds to prevent timeout
				listener.Ping()
			}
		}
	}()
}

// BroadcastNotification sends a notification to every currently-connected SSE
// client. Slow consumers are dropped after sseSendTimeout so a single stuck
// client cannot block the broadcast for the rest of the fleet.
func (app *application) BroadcastNotification(notification data.NotificationContent) {
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		app.logger.Error("Failed to marshal notification content", zap.Error(err))
		return
	}
	msg := string(notificationJSON)

	app.Mutex.RLock()
	defer app.Mutex.RUnlock()

	for userID, ch := range app.Clients {
		select {
		case ch <- msg:
		case <-time.After(sseSendTimeout):
			app.logger.Warn("dropping broadcast to slow SSE consumer",
				zap.Int64("userID", userID))
		}
	}
}

/*/ SimulateDataWithRedisPubSub simulates data and publishes messages to a user-specific Redis pub/sub channel
func (app *application) SimulateDataWithRedisPubSub(userID int64) {
	for {
		app.logger.Info("Simulating data for user:", zap.Int64("userID", userID))
		// Sleep to simulate a delay between notifications
		time.Sleep(25 * time.Second)

		// Create a simulated notification
		notification := data.NotificationContent{
			Message: fmt.Sprintf("Redis Simulation data: %d", rand.Intn(100)),
			Meta: data.NotificationMeta{
				Url:      "http://localhost:5173/dashboard/notifications",
				ImageUrl: "https://images.unsplash.com/photo-1640160186315-838b53fcabc6?q=80&w=1172&auto=format&fit=crop&ixlib=rb-4.0.3&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D",
				Tags:     "simulation",
			},
		}

		err := app.PublishNotificationToRedis(userID, data.NotificationTypeDefault, notification)
		if err != nil {
			app.logger.Error("Error publishing simulated data to Redis", zap.Error(err))
		}
	}
}
*/
