package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

const (
	DefaultGenManDBContextTimeout = 5 * time.Second
)

var (
	ErrInvalidContactUsStatus = errors.New("invalid contact us status")
)

const (
	ContactUsStatusPending  = database.ContactUsStatusPending
	ContactUsStatusResolved = database.ContactUsStatusResolved
	ContactUsStatusClosed   = database.ContactUsStatusInprogress
)

type GeneralManagerModel struct {
	DB *database.Queries
}

type ContactUs struct {
	ID        int64                    `json:"id"`
	UserID    int64                    `json:"user_id"`
	Name      string                   `json:"name"`
	Email     string                   `json:"email"`
	Subject   string                   `json:"subject"`
	Message   string                   `json:"message"`
	Status    database.ContactUsStatus `json:"status"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	Version   int                      `json:"version,omitempty"`
}

// MapContactUsToConstant maps the contact us status to a constant
func (m GeneralManagerModel) MapContactUsToConstant(status string) (database.ContactUsStatus, error) {
	switch status {
	case "pending":
		return ContactUsStatusPending, nil
	case "resolved":
		return ContactUsStatusResolved, nil
	case "in progress":
		return ContactUsStatusClosed, nil
	default:
		return "", ErrInvalidContactUsStatus
	}
}

// ValidteContactUs validates the contact us struct
func ValidateContactUs(v *validator.Validator, contactUs *ContactUs) {
	ValidateName(v, contactUs.Name, "name")
	ValidateEmail(v, contactUs.Email)
	ValidateName(v, contactUs.Subject, "subject")
	ValidateName(v, contactUs.Message, "message")
}

// CreateContactUs adds a contact Us request for a user with a specific email
// We recieve a userID, if any, and a contact us struct
func (m GeneralManagerModel) CreateContactUs(userID int64, contactUs *ContactUs) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultGenManDBContextTimeout)
	defer cancel()
	// create a contact us request
	updatedContactUs, err := m.DB.CreateContactUs(ctx, database.CreateContactUsParams{
		UserID:  sql.NullInt64{Int64: userID, Valid: true},
		Name:    contactUs.Name,
		Email:   contactUs.Email,
		Subject: contactUs.Subject,
		Message: contactUs.Message,
	})
	if err != nil {
		return err
	}
	// fill the contactUs struct with the updated contactUs
	contactUs.ID = updatedContactUs.ID
	contactUs.UserID = userID
	contactUs.Status = updatedContactUs.Status.ContactUsStatus
	contactUs.CreatedAt = updatedContactUs.CreatedAt.Time
	contactUs.UpdatedAt = updatedContactUs.UpdatedAt.Time
	// return nil if no error
	return nil
}
