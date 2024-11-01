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

	// Listen for Redis messages for the specific user
	go app.ListenForRedisPubSubUserMessages(userID)

	// Load and send pending notifications
	go app.loadAndSendPendingNotifications(userID)

	// Simulate data
	go app.SimulateData(1)

	// Simulate data with Redis pub/sub
	go app.SimulateDataWithRedisPubSub(3)

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

	// Load from Redis
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, userID)
	err := app.loadAndProcessRedisData(ctx, userID, pendingKey)
	if err != nil {
		app.logger.Info("Error loading notifications from Redis:", zap.Error(err))
	}

	// Load from Database if needed
	err = app.loadAndProcessDBData(userID)
	if err != nil {
		app.logger.Info("Error loading notifications from database:", zap.Error(err))
	}
}

// loadAndProcessRedisData processes pending notifications from Redis
func (app *application) loadAndProcessRedisData(ctx context.Context, userID int64, pendingKey string) error {
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
		app.PublishNotification(userID, notification)
	}
	return app.RedisDB.Del(ctx, pendingKey).Err()
}

// loadAndProcessDBData loads and processes pending notifications from the database
func (app *application) loadAndProcessDBData(userID int64) error {
	pendingNotifications := []data.NotificationContent{
		{NotificationID: 1, Message: "Database notification 1", Meta: data.NotificationMeta{Url: "https://example.com/1"}},
		{NotificationID: 2, Message: "Database notification 2", Meta: data.NotificationMeta{Url: "https://example.com/2"}},
	}

	if len(pendingNotifications) == 0 {
		app.logger.Info("No pending notifications in DB for user:", zap.Int64("userID", userID))
		return errors.New("no pending notifications found")
	}

	for _, notification := range pendingNotifications {
		app.PublishNotification(userID, notification)
	}
	return nil
}

// AddClient adds a new client to the Clients map with userID
func (app *application) AddClient(userID int64, w http.ResponseWriter) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()
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
		// Marshal the notification to JSON
		notificationJSON, err := json.Marshal(notification)
		if err != nil {
			app.logger.Error("Failed to marshal notification content", zap.Error(err))
			return
		}

		// Send notification directly to the user's SSE channel
		ch <- string(notificationJSON)
		app.logger.Info("Notification sent to user via SSE", zap.Int64("userID", userID))
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
		return fmt.Errorf("error updating notification status: %v", err)
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

// SimulateData simulates data and publishes messages to a user-specific channel in Redis
func (app *application) SimulateData(userID int64) {
	channel := fmt.Sprintf("%s:%d", data.RedisNotManNotificationKey, userID)
	for {
		time.Sleep(7 * time.Second)

		notification := data.NotificationContent{
			NotificationID: rand.Int63(),
			Message:        fmt.Sprintf("Simulated data: %d", rand.Intn(100)),
			Meta:           data.NotificationMeta{Url: "https://example.com", Tags: "simulation"},
		}

		notificationJSON, err := json.Marshal(notification)
		if err != nil {
			app.logger.Error("Failed to marshal simulated notification", zap.Error(err))
			continue
		}

		err = app.RedisDB.Publish(context.Background(), channel, string(notificationJSON)).Err()
		if err != nil {
			app.logger.Info("Error publishing simulated data:", zap.Error(err))
		}
	}
}

// SimulateDataWithRedisPubSub simulates data and publishes messages to a user-specific Redis pub/sub channel
func (app *application) SimulateDataWithRedisPubSub(userID int64) {

	for {
		// Sleep to simulate a delay between notifications
		time.Sleep(10 * time.Second)

		// Create a simulated notification
		notification := data.NotificationContent{
			NotificationID: rand.Int63(),
			Message:        fmt.Sprintf("Redis Simulation data: %d", rand.Intn(100)),
			Meta:           data.NotificationMeta{Url: "https://example.com", Tags: "simulation"},
		}

		err := app.PublishNotificationToRedis(userID, data.NotificationTypeDefault, notification)
		if err != nil {
			app.logger.Error("Error publishing simulated data to Redis", zap.Error(err))
		}
	}
}
