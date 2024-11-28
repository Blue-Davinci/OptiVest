package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/sqlc-dev/pqtype"
)

type NotificationManagerModel struct {
	DB *database.Queries
}

const (
	RedisNotManPendingNotificationKey = "pending_notifications"
	RedisNotManNotificationKey        = "notifications"
)

const (
	DefualtNotManContextTimeout         = 3 * time.Second
	DefaultNotificationTimeout          = 30 * time.Second
	DefaultRedisNotificationTTLDuration = 10 * time.Minute
)

const (
	NotificationTypeDefault             = "default"
	NotificationTypeUserRegistration    = "user_registration"
	NotificationTypeUserWelcome         = "user_welcome"
	NotificationTypeAccount             = "account_update"
	NotificationTypeFeeds               = "feeds"
	NotificationTypeFinancialTracking   = "financial_tracking"
	NotificationTypeFinancialManagement = "financial_management"
	NotificationTypeBudget              = "budget"
	NotificationTypeAward               = "award"
	NotificationTypeGroupInvite         = "group_invite"
)

const (
	NotificationStatusTypeDelivered = database.NotificationStatusDelivered
	NotificationStatusTypePending   = database.NotificationStatusPending
	NotificationStatusTypeRead      = database.NotificationStatusRead
	NotificationStatusTypeExpired   = database.NotificationStatusExpired
)

// Notification represents a notification in the system.
type Notification struct {
	ID               int64                       `json:"id"`
	UserID           int64                       `json:"user_id"`
	Message          string                      `json:"message"`
	NotificationType string                      `json:"notification_type"`
	Status           database.NotificationStatus `json:"status"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
	ReadAt           *time.Time                  `json:"read_at,omitempty"` // Nullable
	ExpiresAt        *time.Time                  `json:"-"`                 // Nullable
	Meta             json.RawMessage             `json:"meta,omitempty"`    // Can be used for JSONB
	RedisKey         *string                     `json:"-"`                 // Nullable, not exposed to the client
}

// Struct to hold the notification information
type NotificationContent struct {
	NotificationID   int64                       `json:"notification_id"`
	Message          string                      `json:"message"`
	NotificationType string                      `json:"notification_type"`
	Status           database.NotificationStatus `json:"status"`
	Meta             NotificationMeta            `json:"meta"`
	SentAt           time.Time                   `json:"sent_at"`
}

type NotificationMeta struct {
	Url      string `json:"url,omitempty"`
	ImageUrl string `json:"image_url,omitempty"`
	Tags     string `json:"tags,omitempty"`
}

// MapNotificationStatusTypeToConst maps the notification status type to the database notification status type.
func MapNotificationStatusTypeToConst(statusType string) (database.NotificationStatus, error) {
	switch statusType {
	case "delivered":
		return NotificationStatusTypeDelivered, nil
	case "pending":
		return NotificationStatusTypePending, nil
	case "read":
		return NotificationStatusTypeRead, nil
	case "expired":
		return NotificationStatusTypeExpired, nil
	default:
		return NotificationStatusTypePending, ErrInvalidStatusType
	}
}

// CreateNewNotification() creates a new notification in the system.
// we take in a user id, and a pointer to a notification.
// We return an error if there was an issue creating the notification.
func (m NotificationManagerModel) CreateNewNotification(userID int64, mynotification *Notification) error {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Create a new notification in the database
	notificationDetail, err := m.DB.CreateNewNotification(ctx, database.CreateNewNotificationParams{
		UserID:           userID,
		Message:          mynotification.Message,
		NotificationType: mynotification.NotificationType,
		Status:           mynotification.Status,
		ExpiresAt:        sql.NullTime{Time: time.Time{}, Valid: false},
		Meta:             pqtype.NullRawMessage{RawMessage: mynotification.Meta, Valid: true},
		RedisKey:         sql.NullString{String: *mynotification.RedisKey, Valid: false},
	})
	if err != nil {
		return err
	}
	// fill in the notification struct with the information from the database
	mynotification.ID = notificationDetail.ID
	mynotification.UserID = userID
	mynotification.CreatedAt = notificationDetail.CreatedAt
	mynotification.UpdatedAt = notificationDetail.UpdatedAt
	// return nil if there was no error
	return nil
}

// UpdateNotificationReadAtAndStatus() updates a notification by updating
// the read at and status of a notification.
// We take in a notification id, a read at time, and a status.
// We return an error if there was an issue updating the notification.
func (m NotificationManagerModel) UpdateNotificationReadAtAndStatus(notificationID int64, readAt sql.NullTime, status database.NotificationStatus) error {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Update the notification in the database
	updatedAt, err := m.DB.UpdateNotificationReadAtAndStatus(ctx, database.UpdateNotificationReadAtAndStatusParams{
		ID:     notificationID,
		ReadAt: readAt,
		Status: status,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return ErrEditConflict
		default:
			return err
		}
	}
	fmt.Println("Notification: ", notificationID, ", was updated at: ", updatedAt)
	// return nil if there was no error
	return nil
}

// GetAllNotificationsByUserId() gets all the notifications for a user.
// This method supports both pagination as a notification_type search.
// We take in a user id, notification type and a filter and return a slice of notifications and an error if there was an issue.
func (m NotificationManagerModel) GetAllNotificationsByUserId(userID int64, notificationType string, filters Filters) ([]*Notification, Metadata, error) {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Get all the notifications from the database
	notificationsRows, err := m.DB.GetAllNotificationsByUserId(ctx, database.GetAllNotificationsByUserIdParams{
		UserID:  userID,
		Column2: notificationType,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	// check for empty notifications and return
	if len(notificationsRows) == 0 {
		//fmt.Println("No notifications found for user: ", userID)
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}

	// create a slice of notifications
	notifications := []*Notification{}
	totalNotifications := 0
	// loop through using the populate function to fill in the notification struct
	for _, notification := range notificationsRows {
		totalNotifications = int(notification.TotalCount)
		notifications = append(notifications, populateNotification(notification))
	}
	// make metadata struct
	metadata := calculateMetadata(totalNotifications, filters.Page, filters.PageSize)
	// return the notifications if there was no error
	return notifications, metadata, nil
}

// GetUnreadNotifications() gets all the unread notifications for a user i.e
// all notifications that are marked as pending and also whose expired at time
// is greater than the now.
// We take in a user id and return a slice of notifications and an error if there was an issue.
func (m NotificationManagerModel) GetUnreadNotifications(userID int64) ([]*Notification, error) {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Get all the unread notifications from the database
	notificationsRows, err := m.DB.GetUnreadNotifications(ctx, userID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// check for empty notifications and return
	if len(notificationsRows) == 0 {
		//fmt.Println("No notifications found for user: ", userID)
		return nil, ErrGeneralRecordNotFound
	}

	// create a slice of notifications
	notifications := []*Notification{}
	// loop through using the populate function to fill in the notification struct
	for _, notification := range notificationsRows {
		notifications = append(notifications, populateNotification(notification))
	}
	// return the notifications if there was no error
	return notifications, nil
}

// GetAllExpiredNotifications() gets all the expired notifications for a user i.e
// all notifications that are marked as pending and also whose expired at time
// is less than the now.
// We take in a filter and return a slice of notifications and an error if there was an issue.
func (m NotificationManagerModel) GetAllExpiredNotifications(filters Filters) ([]*Notification, Metadata, error) {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Get all the expired notifications from the database
	notificationsRows, err := m.DB.GetAllExpiredNotifications(ctx, database.GetAllExpiredNotificationsParams{
		Limit:  int32(filters.limit()),
		Offset: int32(filters.offset()),
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	// check for empty notifications and return
	if len(notificationsRows) == 0 {
		//fmt.Println("No notifications found")
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}

	// create a slice of notifications
	notifications := []*Notification{}
	totalNotifications := 0
	// loop through using the populate function to fill in the notification struct
	for _, notification := range notificationsRows {
		totalNotifications = int(notification.TotalCount)
		notifications = append(notifications, populateNotification(notification))
	}
	// make metadata struct
	metadata := calculateMetadata(totalNotifications, filters.Page, filters.PageSize)
	// return the notifications if there was no error
	return notifications, metadata, nil
}

// DeleteNotificationById() deletes a notification by id.
// We take in a notification id and a userID and return an error if there was an issue deleting the notification.
func (m NotificationManagerModel) DeleteNotificationById(notificationID int64, userID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Delete the notification from the database
	_, err := m.DB.DeleteNotificationById(ctx, database.DeleteNotificationByIdParams{
		ID:     notificationID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return ErrGeneralRecordNotFound
		default:
			return err
		}
	}
	// return nil if there was no error
	return nil
}

// DeleteAllNotificationsByUserId() deletes all notifications for a user.
// We take in a user id and return an error if there was an issue deleting the notifications.
func (m NotificationManagerModel) DeleteAllNotificationsByUserId(userID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefualtNotManContextTimeout)
	defer cancel()
	// Delete all notifications from the database
	err := m.DB.DeleteAllNotificationsByUserId(ctx, userID)
	if err != nil {
		return err
	}
	// return nil if there was no error
	return nil
}

func populateNotification(notificationRow interface{}) *Notification {
	switch notification := notificationRow.(type) {
	case database.Notification:
		return &Notification{
			ID:               notification.ID,
			UserID:           notification.UserID,
			Message:          notification.Message,
			NotificationType: notification.NotificationType,
			Status:           notification.Status,
			CreatedAt:        notification.CreatedAt,
			UpdatedAt:        notification.UpdatedAt,
			ReadAt:           &notification.ReadAt.Time,
			ExpiresAt:        &notification.ExpiresAt.Time,
			Meta:             notification.Meta.RawMessage,
			RedisKey:         &notification.RedisKey.String,
		}
	case database.GetAllExpiredNotificationsRow:
		return &Notification{
			ID:               notification.ID,
			UserID:           notification.UserID,
			Message:          notification.Message,
			NotificationType: notification.NotificationType,
			Status:           notification.Status,
			CreatedAt:        notification.CreatedAt,
			UpdatedAt:        notification.UpdatedAt,
			ReadAt:           &notification.ReadAt.Time,
			ExpiresAt:        &notification.ExpiresAt.Time,
			Meta:             notification.Meta.RawMessage,
			RedisKey:         &notification.RedisKey.String,
		}
	case database.GetAllNotificationsByUserIdRow:
		return &Notification{
			ID:               notification.ID,
			UserID:           notification.UserID,
			Message:          notification.Message,
			NotificationType: notification.NotificationType,
			Status:           notification.Status,
			CreatedAt:        notification.CreatedAt,
			UpdatedAt:        notification.UpdatedAt,
			ReadAt:           &notification.ReadAt.Time,
			ExpiresAt:        &notification.ExpiresAt.Time,
			Meta:             notification.Meta.RawMessage,
			RedisKey:         &notification.RedisKey.String,
		}
	default:
		return nil
	}
}
