package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/nyaruka/phonenumbers"
	"golang.org/x/crypto/bcrypt"
)

// constants for general module usage
const (
	DefaultUserDBContextTimeout = 5 * time.Second
	DefaulRedistUserMFATTLS     = 5 * time.Minute
)

// constants for the MFA status enumeration
const (
	MFAEnumerationAccepted = database.MfaStatusTypeAccepted
	MFAEnumerationPending  = database.MfaStatusTypePending
	MFAEnumerationRejected = database.MfaStatusTypeRejected
)

// constants for tags to be used during REDIS operations
const (
	RedisMFASetupPendingPrefix = "mfa_setup_pending"
	RedisMFALoginPendingPrefix = "mfa_login_pending"
)

type UserModel struct {
	DB *database.Queries
}

// User represents a user in the system.
type User struct {
	ID               int64                  `json:"id"`                 // Unique user ID
	FirstName        string                 `json:"first_name"`         // First name
	LastName         string                 `json:"last_name"`          // Last name
	Email            string                 `json:"email"`              // Case-insensitive email, must be unique
	ProfileAvatarURL string                 `json:"profile_avatar_url"` // URL to user's profile picture
	Password         password               `json:"-"`                  // Securely stored password hash (bcrypt)
	UserRole         string                 `json:"user_role"`          // User Role
	PhoneNumber      string                 `json:"phone_number"`       // Phone number for multi-factor authentication (MFA)
	Activated        bool                   `json:"activated"`          // Account activation status (email confirmation, etc.)
	Version          int32                  `json:"version"`            // Record versioning for optimistic locking
	CreatedAt        time.Time              `json:"created_at"`         // Timestamp of account creation
	UpdatedAt        time.Time              `json:"updated_at"`         // Timestamp for last update (e.g., profile changes)
	LastLogin        time.Time              `json:"last_login"`         // Track the user's last login time
	ProfileCompleted bool                   `json:"profile_completed"`  // Whether the user completed full profile
	DOB              time.Time              `json:"dob"`                // Date of Birth (for financial regulations)
	Address          string                 `json:"address,omitempty"`  // Optional address for KYC requirements
	CountryCode      string                 `json:"country_code"`       // Two-letter ISO country code for region-specific financial services
	CurrencyCode     string                 `json:"currency_code"`      // Default currency (ISO 4217) for transactions and accounts
	MFAEnabled       bool                   `json:"mfa_enabled"`        // Multi-factor authentication (MFA) enabled
	MFASecret        string                 `json:"mfa_secret"`         // Secret key for TOTP-based MFA
	MFAStatus        database.MfaStatusType `json:"mfa_status"`         // Status of MFA (pending, accepted, rejected)
	MFALastChecked   *time.Time             `json:"mfa_last_checked"`   // Timestamp of the last MFA check, can be NULL
}

// Define a custom ErrDuplicateEmail error.
var (
	ErrDuplicateEmail = errors.New("duplicate email")
	ErrEditConflict   = errors.New("edit conflict")
)

// Define Default Image:
const DefaultProfileImage = "https://res.cloudinary.com/djg9a13ka/image/upload/v1721808901/avatars/avatar_1721808896310.png"

// Declare a new AnonymousUser variable.
var AnonymousUser = &User{}

// Check if a User instance is the AnonymousUser.
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

// Create a custom password type which is a struct containing the plaintext and hashed
// versions of the password for a user.
type password struct {
	plaintext *string
	hash      []byte
}

// set() calculates the bcrypt hash of a plaintext password, and stores both
// the hash and the plaintext versions in the struct.
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plaintextPassword
	p.hash = hash
	return nil
}

// The Matches() method checks whether the provided plaintext password matches the
// hashed password stored in the struct, returning true if it matches and false
// otherwise.
func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		//fmt.Printf(">>>>> Plain text: %s\nHash: %v\n", plaintextPassword, p.hash)
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}
func ValidateImageURL(v *validator.Validator, image_url string) {
	v.Check(image_url != "", "image", "must be provided")
}
func ValidateName(v *validator.Validator, name string) {
	v.Check(name != "", "name", "must be provided")
	v.Check(len(name) <= 500, "name", "must not be more than 500 bytes long")
}
func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}
func ValidatePhoneNumber(v *validator.Validator, phone_number, region string) {
	parsedNumber, err := phonenumbers.Parse(phone_number, region)
	if err != nil {
		v.AddError("phone_number", "is invalid")
		return
	}
	if !phonenumbers.IsValidNumber(parsedNumber) {
		v.AddError("phone_number", "is invalid")
		return
	}

}

func ValidateUser(v *validator.Validator, user *User) {
	// Call the standalone ValidateName() helper.
	ValidateName(v, user.FirstName)
	ValidateName(v, user.LastName)
	// Call the standalone ValidateEmail() helper.
	ValidateEmail(v, user.Email)
	// Call the standalone ValidatePhoneNumber() helper.
	ValidatePhoneNumber(v, user.PhoneNumber, user.CountryCode)
	// Validate Image
	ValidateImageURL(v, user.ProfileAvatarURL)
	// If the plaintext password is not nil, call the standalone
	// ValidatePasswordPlaintext() helper.
	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}
	// If the password hash is ever nil, this will be due to a logic error in our
	// codebase. So rather than adding an error to the validation map we
	// raise a panic instead.
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

// CreateNewUser() creates a new user in the database. The function takes a pointer to a User struct
// and an encryption key as input. We decrypt the key, use it to encrypt necessary items before we save
// it back to the DB.
func (m UserModel) CreateNewUser(user *User, encryption_key string) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultUserDBContextTimeout)
	defer cancel()
	// decrypt our hex
	decodedKey, err := DecodeEncryptionKey(encryption_key)
	if err != nil {
		return err
	}
	// encrypt and set the password
	encryptedPhoneNumber, err := EncryptData(user.PhoneNumber, decodedKey)
	if err != nil {
		return err
	}
	// perform an insert
	createdUser, err := m.DB.CreateNewUser(ctx, database.CreateNewUserParams{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		ProfileAvatarUrl: user.ProfileAvatarURL,
		Password:         user.Password.hash,
		PhoneNumber:      encryptedPhoneNumber,
		ProfileCompleted: user.ProfileCompleted,
		Dob:              user.DOB,
		Address:          sql.NullString{String: user.Address, Valid: user.Address != ""},
		CountryCode:      sql.NullString{String: user.CountryCode, Valid: user.CountryCode != ""},
		CurrencyCode:     sql.NullString{String: user.CurrencyCode, Valid: user.CurrencyCode != ""},
	})
	// check for any error and if it is a constraint violation
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	// fill in retruned data
	user.ID = createdUser.ID
	user.CreatedAt = createdUser.CreatedAt
	user.UserRole = createdUser.RoleLevel
	user.UpdatedAt = createdUser.UpdatedAt
	user.Version = createdUser.Version
	user.MFAEnabled = createdUser.MfaEnabled
	user.MFASecret = createdUser.MfaSecret.String
	user.MFAStatus = createdUser.MfaStatus.MfaStatusType
	user.MFALastChecked = &createdUser.MfaLastChecked.Time
	// we are good
	return nil
}

// GetForToken() retrieves the details of a user based on a token, scope, and encryption key.
func (m UserModel) GetForToken(tokenScope, tokenPlaintext, encryption_key string) (*User, error) {
	// decrypt our hex
	decodedKey, err := DecodeEncryptionKey(encryption_key)
	if err != nil {
		return nil, err
	}
	// Calculate sha256 hash of plaintext
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))
	ctx, cancel := contextGenerator(context.Background(), DefaultUserDBContextTimeout)
	defer cancel()
	// get the user
	user, err := m.DB.GetForToken(ctx, database.GetForTokenParams{
		Hash:   tokenHash[:],
		Scope:  tokenScope,
		Expiry: time.Now(),
	})
	// check for any error
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// decrypt the phone number
	decryptedNumber, err := DecryptData(user.PhoneNumber, decodedKey)
	if err != nil {
		return nil, err
	}
	// make a user
	tokenuser := populateUser(user, decryptedNumber)
	// fill in the user data
	return tokenuser, nil
}

// GetByEmail() retrieves the details of a user based on an email
// An Encryption key is also passed to decrypt any data that was encrypted
// Such as the phone number or the MFA secret.
func (m UserModel) GetByEmail(email, encryption_key string) (*User, error) {
	// decrypt our hex
	decodedKey, err := DecodeEncryptionKey(encryption_key)
	if err != nil {
		return nil, err
	}
	// Get our context
	ctx, cancel := contextGenerator(context.Background(), DefaultUserDBContextTimeout)
	defer cancel()
	// Get the user
	user, err := m.DB.GetUserByEmail(ctx, email)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// decrypt the phone number
	decryptedNumber, err := DecryptData(user.PhoneNumber, decodedKey)
	if err != nil {
		return nil, err
	}
	// make a user
	emailUser := populateUser(user, decryptedNumber)
	// return
	return emailUser, nil
}

// UpdateUser() updates the details of a user in the database.
// The function takes a pointer to a User struct and an encryption key as input.
// We decode the key, use it to encrypt necessary items before we save it back to the DB
func (m UserModel) UpdateUser(user *User, encryption_key string) error {
	// decrypt our hex
	decodedKey, err := DecodeEncryptionKey(encryption_key)
	if err != nil {
		return err
	}
	// get context
	ctx, cancel := contextGenerator(context.Background(), DefaultUserDBContextTimeout)
	defer cancel()
	// encrypt and set the phone number
	encryptedPhoneNumber, err := EncryptData(user.PhoneNumber, decodedKey)
	if err != nil {
		return err
	}
	// perform the update
	updatedUser, err := m.DB.UpdateUser(ctx, database.UpdateUserParams{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Email:            user.Email,
		ProfileAvatarUrl: user.ProfileAvatarURL,
		Password:         user.Password.hash,
		RoleLevel:        user.UserRole,
		PhoneNumber:      encryptedPhoneNumber,
		Activated:        user.Activated,
		LastLogin:        user.LastLogin,
		ProfileCompleted: user.ProfileCompleted,
		Dob:              user.DOB,
		Address:          sql.NullString{String: user.Address, Valid: user.Address != ""},
		CountryCode:      sql.NullString{String: user.CountryCode, Valid: user.CountryCode != ""},
		CurrencyCode:     sql.NullString{String: user.CurrencyCode, Valid: user.CurrencyCode != ""},
		MfaEnabled:       user.MFAEnabled,
		MfaSecret:        sql.NullString{String: user.MFASecret, Valid: user.MFASecret != ""},
		MfaStatus:        database.NullMfaStatusType{MfaStatusType: database.MfaStatusTypeAccepted, Valid: true},
		MfaLastChecked:   sql.NullTime{Time: *user.MFALastChecked, Valid: user.MFALastChecked != nil},
		ID:               user.ID,
		Version:          int32(user.Version),
	})
	// check for any error
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// fill in the version and update time as well
	user.Version = updatedUser.Version
	user.UpdatedAt = updatedUser.UpdatedAt
	// we are good
	return nil
}

// populateUser() is a helper function that takes a database.User struct and returns a
// pointer to a User struct containing the same data. It also decrypts the phone number
// using the provided encryption key.
// populateUser takes in a SQLC-generated row (userRow) and a decrypted phone number,
// and populates a User struct based on the type of userRow.
// The function currently supports two types: database.GetForTokenRow and database.User.
// If a new row type is introduced, this function can be extended to handle it.

func populateUser(userRow interface{}, decryptedNumber string) *User {
	switch user := userRow.(type) {
	// Case for database.GetForTokenRow: Populates a User object with fields specific to the GetForTokenRow
	// Case for database.User: Populates a User object with fields specific to the User type.
	case database.User:
		// Create a new password struct instance for the user.
		userPassword := password{
			hash: user.Password,
		}
		return &User{
			ID:               user.ID,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			Email:            user.Email,
			ProfileAvatarURL: user.ProfileAvatarUrl,
			Password:         userPassword,
			UserRole:         user.RoleLevel,
			PhoneNumber:      decryptedNumber,
			Activated:        user.Activated,
			Version:          user.Version,
			CreatedAt:        user.CreatedAt,
			UpdatedAt:        user.UpdatedAt,
			LastLogin:        user.LastLogin,
			ProfileCompleted: user.ProfileCompleted,
			DOB:              user.Dob,
			Address:          user.Address.String,
			CountryCode:      user.CountryCode.String,
			CurrencyCode:     user.CurrencyCode.String,
			MFAEnabled:       user.MfaEnabled,
			MFASecret:        user.MfaSecret.String,
			MFAStatus:        user.MfaStatus.MfaStatusType,
			MFALastChecked:   &user.MfaLastChecked.Time,
		}

	// Default case: Returns nil if the input type does not match any supported types.
	default:
		return nil
	}
}
