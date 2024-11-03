package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"math/rand"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// ServeSSE streams data to a single client
func (app *application) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// Get user ID from request context
	userID := app.contextGetUser(r).ID

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Register client
	app.AddClient(userID, w)
	defer app.RemoveClient(userID)

	// Load and send pending notifications, This is safe as it is a one-time operation
	// no matter how many times the client reconnects, the pending notifications will only be sent once
	go app.loadAndSendPendingNotifications(userID)

	app.logger.Info("SSE client connected", zap.Int64("userID", userID))

	// Stream messages to client
	for {
		select {
		case msg, ok := <-app.Clients[userID]:
			if !ok {
				return // Exit if the channel is closed
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush() // Flush the buffer, send message to client
		case <-r.Context().Done(): // Stop if client disconnects
			return
		}
	}
}

// loadAndSendPendingNotifications loads and sends pending notifications from Redis and the database
func (app *application) loadAndSendPendingNotifications(userID int64) {
	app.logger.Info("Loading and sending pending notifications for user:", zap.Int64("userID", userID))
	ctx := context.Background()

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

// AddClient adds a new client to the Clients map with userID
func (app *application) AddClient(userID int64, w http.ResponseWriter) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()

	// Check if the user already has an active connection
	if _, exists := app.Clients[userID]; exists {
		app.RemoveClient(userID) // Close existing connection to avoid duplicates
	}

	// Initialize user-specific listeners if they do not already exist
	// this is to prevent multiple goroutines listening to the same user or channel
	// this will also prevent duplicate notifications from being sent to the user
	if _, listening := app.ListeningUsers[userID]; !listening {
		go app.ListenForRedisPubSubUserMessages(userID)
		go app.listenToAwardNotifications()
		// Simulate data with Redis pub/sub
		go app.SimulateDataWithRedisPubSub(userID)
		app.ListeningUsers[userID] = true
	}

	// Create a new channel for the user
	app.Clients[userID] = make(chan string)
}

// RemoveClient removes the client from Clients map and closes their channel
func (app *application) RemoveClient(userID int64) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()
	if ch, exists := app.Clients[userID]; exists {
		close(ch)
		delete(app.Clients, userID)
	}
}

// PublishNotification publishes a message to a specific user's SSE channel if they are online.
// If the user is offline, it stores the notification in Redis for future delivery.
func (app *application) PublishNotification(userID int64, notification data.NotificationContent) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()

	// Check if the user has an active connection
	if ch, exists := app.Clients[userID]; exists {
		// set the time to now with seconds precision
		notification.SentAt = time.Now()
		// Marshal the notification to JSON
		notificationJSON, err := json.Marshal(notification)
		if err != nil {
			app.logger.Error("Failed to marshal notification content", zap.Error(err))
			return
		}

		// Send notification directly to the user's SSE channel
		ch <- string(notificationJSON)
		// update database notification status
		err = app.updateDatabaseNotificationStatus(notification.NotificationID, data.NotificationStatusTypeDelivered)
		if err != nil {
			app.logger.Error("Error updating notification status in database", zap.Error(err))
			return
		}
	} else {
		// If the user is offline, save the notification to Redis for future delivery
		err := app.storeNotificationInRedis(userID, notification)
		if err != nil {
			app.logger.Error("Error storing notification in Redis", zap.Error(err))
			// do not return here, proceed and try to update the database notification status
		}
		err = app.updateDatabaseNotificationStatus(notification.NotificationID, data.NotificationStatusTypePending)
		if err != nil {
			app.logger.Error("Error updating notification status in database", zap.Error(err))
			return
		}
		app.logger.Info("User is offline; notification stored in Redis", zap.Int64("userID", userID))
	}
}

// storeNotificationInRedis saves the notification to Redis for delivery when the user reconnects.
func (app *application) storeNotificationInRedis(userID int64, notification data.NotificationContent) error {
	ctx := context.Background()
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
		NotificationType: data.NotificationTypeDefault,
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
	// we are going to publish the notification to the user's Redis channel
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	// publish the notification to the user's Redis channel
	err = app.RedisDB.Publish(context.Background(), channel, string(notificationJSON)).Err()
	if err != nil {
		return err
	}
	return nil
}

// ListenForUserMessages listens to Redis pub/sub and sends messages to the specific user's SSE channel
func (app *application) ListenForRedisPubSubUserMessages(userID int64) {
	ctx := context.Background()
	pubsub := app.RedisDB.Subscribe(ctx, fmt.Sprintf("%s:%d", data.RedisNotManNotificationKey, userID))

	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		var notification data.NotificationContent
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			app.logger.Error("Failed to unmarshal Redis message", zap.Error(err))
			continue
		}
		app.PublishNotification(userID, notification)
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
					award, err := app.models.AwardManager.GetAwardByAwardID(payload.AwardID)
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

// BroadcastMessage sends a message to all connected clients
func (app *application) BroadcastNotification(notification data.NotificationContent) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		app.logger.Error("Failed to marshal notification content", zap.Error(err))
		return
	}

	for _, ch := range app.Clients {
		ch <- string(notificationJSON)
	}
}

// SimulateDataWithRedisPubSub simulates data and publishes messages to a user-specific Redis pub/sub channel
func (app *application) SimulateDataWithRedisPubSub(userID int64) {
	for {
		// Sleep to simulate a delay between notifications
		time.Sleep(10 * time.Second)

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
