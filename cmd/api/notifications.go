package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// wsHandler() is a method that will handle the WebSocket connections for our application.
// It will upgrade the connection to a WebSocket connection and then authenticate the user
// that is trying to connect to the WebSocket. If the user is authenticated, we will then
// add the user to the Clients map and then listen for Pub/Sub messages and handle incoming
// WebSocket messages.
// We will also load any pending notifications for the user and send them to the user via the
// WebSocket connection.
func (app *application) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := app.WebSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		app.logger.Error("WebSocket upgrade error", zap.Error(err))
		// Call the error response function without the connection
		app.wsWebSocketUpgradeError(nil) // Pass nil since the connection upgrade failed
		return
	}
	// Check if we have reached the maximum number of connections
	if app.limitWsConnections() {
		app.wsMaxConnectionsResponse(conn)
		return
	}
	// Authenticate the user
	user, err := app.authenticateWSUser(r)
	if err != nil || user == nil {
		app.wsInvalidAuthenticationResponse(conn)
		return
	}
	// Get the user ID
	userID := user.ID
	//defer conn.Close()
	app.logger.Info("WebSocket connection opened", zap.Int64("userID", userID))
	// Register WebSocket connection
	app.addClient(userID, conn)

	// Load pending notifications for the user
	app.loadAndSendPendingNotifications(userID)

	// Listen for Pub/Sub messages
	go app.listenForPubSub(userID, conn)

	// Handle incoming WebSocket messages (for marking notifications as read)
	go app.listenForMessages(conn, userID)

	// simulate goal completion notifications
	go app.simulateGoalCompletionNotifications()
}

// limitWsConnections() will limit the number of WebSocket connections that can be made to the
// server. We will use a mutex to lock the Clients map and check the number of connections
// that have been made. If the number of connections is greater than the limit, we will return
// true, else we will return false.
func (app *application) limitWsConnections() bool {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()
	return len(app.Clients) >= app.config.ws.MaxConcurrentConnections
}

// addClient() will add a client to the Clients map.
func (app *application) addClient(userID int64, conn *websocket.Conn) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()
	app.Clients[userID] = conn
}

// removeClient() will remove a client from the Clients map.
func (app *application) removeClient(userID int64) {
	app.Mutex.Lock()
	defer app.Mutex.Unlock()
	delete(app.Clients, userID)
}

// listenForMessages() will listen for messages from the user and mark the notification as read
// in REDIS and the database.
func (app *application) listenForMessages(conn *websocket.Conn, userID int64) {
	pendingKey := fmt.Sprintf("pending:%d", userID)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// write closing message
			app.wsServerErrorResponse(conn, "We could not process your request")
			app.logger.Info("WebSocket error", zap.Error(err))
			conn.Close()
			// Remove user connection on error
			app.removeClient(userID)
			return
		}

		// Expect the message from the user to be the notification ID
		notificationID := string(message)
		app.logger.Info("Notification ID received", zap.String("notificationID", notificationID))

		// Mark as read in Redis and DB
		err = app.RedisDB.HDel(context.Background(), pendingKey, notificationID).Err()
		if err != nil {
			app.logger.Info("Error deleting notification from REDIS", zap.Error(err))
			continue
		}
		// update notification status in the database to read
		notificationIDInt, err := strconv.ParseInt(notificationID, 10, 64)
		if err != nil {
			app.logger.Info("Error converting notification ID to int", zap.Error(err))
			continue
		}
		err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
			notificationIDInt,
			sql.NullTime{Time: time.Now(), Valid: true},
			data.NotificationStatusTypeRead,
		)
		if err != nil {
			app.logger.Info("Error converting notification ID to int", zap.Error(err))
			continue
		}
	}
}

// listenForPubSub() will listen for Pub/Sub messages for the user and send them to the user
// via the WebSocket connection.
func (app *application) listenForPubSub(userID int64, conn *websocket.Conn) {
	app.logger.Info("Listening for Pub/Sub messages", zap.Int64("userID", userID))
	pubSub := app.RedisDB.Subscribe(context.Background(), fmt.Sprintf("notifications:%d", userID))
	defer pubSub.Close()

	for msg := range pubSub.Channel() {
		app.logger.Info("Received Pub/Sub message", zap.String("message", msg.Payload))
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		if err != nil {
			// write closing message
			app.wsServerErrorResponse(conn, "An error occurred and we could not respond to your request")
			app.logger.Info("WebSocket error", zap.Error(err))
			conn.Close()
			return
		}
	}
}

func (app *application) loadAndProcessRedisData(ctx context.Context, userID int64, pendingKey string) error {
	pendingRedisNotifications, err := app.RedisDB.HGetAll(ctx, pendingKey).Result()
	if err != nil {
		app.logger.Error("Failed to get pending notifications from REDIS", zap.Error(err))
		return err
	}
	// check length
	if len(pendingRedisNotifications) == 0 {
		app.logger.Info("No pending notifications found", zap.Int64("userID", userID))
		return nil
	}

	// batch send pending notifications
	if err := app.Clients[userID].WriteJSON(pendingRedisNotifications); err != nil {
		app.logger.Error("Failed to send notification via WebSocket", zap.Error(err))
		return err
	}
	// if succesdful, remove from REDIS
	if err := app.RedisDB.Del(ctx, pendingKey).Err(); err != nil {
		app.logger.Error("Failed to delete pending notifications from REDIS", zap.Error(err))
		return err
	}
	// loop through the pending notifications and send them to the user
	for notificationID, notificationJSON := range pendingRedisNotifications {
		var notificationContent data.NotificationContent
		err := json.Unmarshal([]byte(notificationJSON), &notificationContent)
		if err != nil {
			app.logger.Info("Error unmarshalling notification content", zap.Error(err))
			continue
		}
		// Convert notificationID from string to int64
		notificationID, err := strconv.ParseInt(notificationID, 10, 64)
		if err != nil {
			app.logger.Error("Failed to convert notificationID to int64", zap.Error(err))
			continue
		}

		// Mark as delivered in the database
		err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
			notificationID,
			sql.NullTime{Time: time.Time{}, Valid: false},
			data.NotificationStatusTypeDelivered,
		)
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

func (app *application) loadAndProcessDBData(userID int64) error {
	pendingNotifications, err := app.models.NotificationManager.GetUnreadNotifications(userID)
	if err != nil {
		return err
	}
	// check length
	if len(pendingNotifications) == 0 {
		app.logger.Info("No pending notifications found", zap.Int64("userID", userID))
		return data.ErrGeneralRecordNotFound
	}
	app.logger.Info("Pending notifications found", zap.Int("count", len(pendingNotifications)))
	for _, notification := range pendingNotifications {
		app.logger.Info("Pending notifications sent from Database", zap.Int64("Notification ID", notification.ID))
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
			Meta:           notificationMeta,
		}
		app.logger.Info("Pending notifications sent from Database", zap.Int64("Notification ID", notification.ID))
		// Publish to the pub/sub system (assuming you have a PublishNotification function)
		if err := app.Clients[userID].WriteJSON(notificationContent); err != nil {
			app.logger.Error("Failed to send notification via WebSocket", zap.Error(err))
			continue
		}
		// Mark as delivered in the database
		err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
			notification.ID,
			sql.NullTime{Time: time.Time{}, Valid: false},
			data.NotificationStatusTypeDelivered,
		)
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

// loadAndSendPendingNotifications() will first load any pending notifications for the user
// in REDIS and then send them to the user via the websocket connection.
// We also load any pending notifications from the DB and also send them to the user.
func (app *application) loadAndSendPendingNotifications(userID int64) {
	app.logger.Info("Loading and sending pending notifications", zap.Int64("userID", userID))
	ctx := context.Background()
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, userID)
	// get the pending notifications from the REDIS
	err := app.loadAndProcessRedisData(ctx, userID, pendingKey)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.logger.Info("Edit conflict while processing pending notifications from REDIS", zap.String("no update could be made as such notification does not exist", "error"))
		default:
			app.logger.Error("Failed to load and process pending notifications from REDIS", zap.Error(err))
		}
	}

	app.logger.Info("Starting Loading and sending pending notifications from Database", zap.Int64("userID", userID))
	err = app.loadAndProcessDBData(userID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.logger.Info("No pending notifications found in the database", zap.Int64("userID", userID))
		default:
			app.logger.Error("Failed to load and process pending notifications from the database", zap.Error(err))
		}
	}
}

// publishNotification() will publish the notification to the user via the WebSocket connection
// if the user is online. If the user is offline, the notification will be saved to REDIS.
// We will also update the notification status in the database.
func (app *application) publishNotification(userID int64, notificationContent data.NotificationContent) error {
	app.logger.Info("Publishing notification", zap.Int64("userID", userID), zap.Int64("notificationID", notificationContent.NotificationID))
	// context with timeout using DefaultNotificationTimeout
	ctx, cancel := context.WithTimeout(context.Background(), data.DefaultNotificationTimeout)
	defer cancel()
	// Convert NotificationContent struct to JSON
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, userID)
	notificationData, err := json.Marshal(notificationContent)
	if err != nil {
		return fmt.Errorf("error marshalling notification content: %v", err)
	}
	channel := fmt.Sprintf("notifications:%d", userID)
	app.Mutex.Lock()
	_, ok := app.Clients[userID]
	app.Mutex.Unlock()
	// check if user is online
	if ok {
		app.logger.Info("User is online, sending notification via WebSocket", zap.Int64("userID", userID))
		// update notification o delivered
		err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
			notificationContent.NotificationID,
			sql.NullTime{Time: time.Time{}, Valid: false},
			data.NotificationStatusTypeDelivered,
		)
		if err != nil {
			message := err.Error()
			app.logger.Info("Failed to update the notification status", zap.String("Message:", message))
			//return fmt.Errorf("error updating notification status: %v", err)
		}
		return app.RedisDB.Publish(context.Background(), channel, notificationData).Err()
	} else {
		// marshal the notification content to JSON
		notificationData, err := json.Marshal(notificationContent)
		if err != nil {
			return fmt.Errorf("error marshalling notification content to JSON: %v", err)
		}
		// user is offline, save to REDIS
		err = app.RedisDB.HSet(ctx, pendingKey, strconv.FormatInt(notificationContent.NotificationID, 10), notificationData).Err()
		if err != nil {
			return fmt.Errorf("error saving notification to REDIS: %v", err)
		}

		// Set the TTL for the pending notification
		err = app.RedisDB.Expire(ctx, pendingKey, data.DefaultRedisNotificationTTLDuration).Err()
		if err != nil {
			return fmt.Errorf("error setting TTL for pending notification: %v", err)
		}
		// update notification to pending
		err = app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
			notificationContent.NotificationID,
			sql.NullTime{Time: time.Time{}, Valid: false},
			data.NotificationStatusTypePending,
		)
		if err != nil {
			return fmt.Errorf("error updating notification status: %v", err)
		}
	}
	return nil
}

// validateWSUser is a method that we will use  for our websocket routes to validate the user
// that is trying to connect to the websocket. This will be used in the wsHandler method.
// It will use the same logic as the authenticate middleware. but will be used for the websocket
// routes.
// We will return a user and an error if there is an issue taking in a http.Request
func (app *application) authenticateWSUser(r *http.Request) (*data.User, error) {
	user, err := app.aunthenticatorHelper(r)
	if err != nil {
		return nil, err
	}
	if user == data.AnonymousUser {
		app.logger.Info("Anonymous user detected")
		return nil, data.ErrGeneralRecordNotFound
	}
	//app.logger.Info("Obtained a user with this ID", zap.Int64("Connected ID", user.ID))
	return user, nil
}

func (app *application) simulateGoalCompletionNotifications() {
	pendingKey := fmt.Sprintf("%s:%d", data.RedisNotManPendingNotificationKey, 1)
	for {
		time.Sleep(5 * time.Second) // Simulate goal completion
		// send notification vial pubishNotification
		notificationContent := data.NotificationContent{
			NotificationID: 1,
			Message:        "Congratulations! You have completed your goal.",
			Meta: data.NotificationMeta{
				Url:      "https://example.com/goals/1",
				ImageUrl: "https://example.com/images/goal.png",
				Tags:     "goal,completed",
			},
		}
		err := app.publishNotification(1, notificationContent)
		if err != nil {
			app.logger.Error("Error publishing notification", zap.Error(err))
		}
		// Marshal the notification content to JSON
		notificationJSON, err := json.Marshal(notificationContent)
		if err != nil {
			app.logger.Error("Error marshalling notification content to JSON", zap.Error(err))
			continue
		}
		// save to redis
		err = app.RedisDB.HSet(context.Background(), pendingKey, strconv.FormatInt(notificationContent.NotificationID, 10), notificationJSON).Err()
		if err != nil {
			app.logger.Error("Error saving notification to REDIS", zap.Error(err))
		}
		err = app.RedisDB.HSet(context.Background(), pendingKey, strconv.FormatInt(3, 10), notificationJSON).Err()
		if err != nil {
			app.logger.Error("Error saving notification to REDIS", zap.Error(err))
		}
	}
}
