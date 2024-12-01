package data

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

type MFAManager struct {
	DB *database.Queries
}

type MFAToken struct {
	TOTPCode  string `json:"totp_code"`
	TOTPToken string `json:"totp_token"`
	Email     string `json:"email"`
}

type RecoveryCodeDetail struct {
	ID            int64          `json:"id"`
	UserID        int64          `json:"user_id"`
	RecoveryCodes *RecoveryCodes `json:"recovery_codes"`
	Used          bool           `json:"used,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at,omitempty"`
}

// Matches() compares the concatenated recovery codes with the provided plaintext password
// We generate the SHA-256 hash of the concatenated string
func (p *RecoveryCodeDetail) Matches(concatenatedPlainTextStr string) (bool, error) {
	// Generate the SHA-256 hash of the concatenated string
	generatedHash := sha256.Sum256([]byte(concatenatedPlainTextStr))

	// Use subtle.ConstantTimeCompare for a timing-attack-resistant comparison
	if subtle.ConstantTimeCompare(generatedHash[:], p.RecoveryCodes.CodeHash) == 1 {
		return true, nil
	}

	// If the hashes do not match, return false
	return false, nil
}

type RecoveryCodes struct {
	RecoveryCodes []string `json:"recovery_codes"`
	CodeHash      []byte   `json:"-"`
}

// MFASession is a struct that holds the email and status of a user's MFA session
type MFASession struct {
	Email string `json:"email"`
	Value string `json:"status"`
}

const (
	MFAStatusPending     = "pending"
	DefaultMFAManTimeout = 5 * time.Second
)

var (
	ErrMFANotEnabled            = errors.New("mfa is not enabled for this user")
	ErrInvalidTOTPCode          = errors.New("TOTP code is invalid")
	ErrRedisMFAKeyNotFound      = errors.New("key not found in Redis")
	ErrRedisMFAKeyAlreadyExists = errors.New("user already has a pending mfa session. please complete the session before starting a new one")
)

func ValidateTOTPCode(v *validator.Validator, mfaToken *MFAToken) {
	//feed name
	v.Check(mfaToken.TOTPCode != "", "code", "must be provided")
	v.Check(len(mfaToken.TOTPCode) == 6, "code", "must be a valid code")
}

func ValidateMFAToken(v *validator.Validator, mfaToken *MFAToken) {
	//feed name
	v.Check(mfaToken.TOTPToken != "", "code", "must be provided")
	v.Check(len(mfaToken.TOTPToken) < 6, "code", "must be a valid code")
}

func ValidateFullMFA(v *validator.Validator, mfaToken *MFAToken) {
	ValidateTOTPCode(v, mfaToken)
	ValidateMFAToken(v, mfaToken)
	ValidateEmail(v, mfaToken.Email)
}

// CreateNewRecoveryCode() creates recovery codes for a user after a successful MFA opt-IN
// We use generateRecoveryCodea() to create 5 recovery codes which we will receive
// We shall proceed to save the recovery codes to the database
// We shall return the *RecoveryCodeDetail and error
func (m MFAManager) CreateNewRecoveryCode(userID int64) (*RecoveryCodeDetail, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultMFAManTimeout)
	defer cancel()
	// Generate the recovery codes
	recoveryCodes, err := generateRecoveryCodes(5)
	if err != nil {
		return nil, err
	}
	// Save the recovery codes to the database
	codeDetail, err := m.DB.CreateNewRecoveryCode(ctx, database.CreateNewRecoveryCodeParams{
		UserID:   userID,
		CodeHash: recoveryCodes.CodeHash,
	})
	if err != nil {
		return nil, err
	}
	// Make the recovery code detail struct
	recoveryCodeDetails := &RecoveryCodeDetail{
		ID:     codeDetail.ID,
		UserID: userID,
		RecoveryCodes: &RecoveryCodes{
			RecoveryCodes: recoveryCodes.RecoveryCodes,
		},
		Used:      false,
		CreatedAt: codeDetail.CreatedAt.Time,
	}
	// we are good to go
	return recoveryCodeDetails, nil
}

// GetRecoveryCodesByUserID() fetches the recovery codes for a user by their user ID
// We shall return the *RecoveryCodeDetail and error
func (m MFAManager) GetRecoveryCodesByUserID(userID int64) (*RecoveryCodeDetail, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultMFAManTimeout)
	defer cancel()
	// Fetch the recovery codes from the database
	codeDetail, err := m.DB.GetRecoveryCodesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Make the recovery code detail struct
	recoveryCodeDetails := &RecoveryCodeDetail{
		ID:     codeDetail.ID,
		UserID: userID,
		RecoveryCodes: &RecoveryCodes{
			CodeHash: codeDetail.CodeHash,
		},
		Used:      codeDetail.Used.Bool,
		CreatedAt: codeDetail.CreatedAt.Time,
		UpdatedAt: codeDetail.UpdatedAt.Time,
	}
	// we are good to go
	return recoveryCodeDetails, nil
}

// MarkRecoveryCodeAsUsed() marks a recovery code as used by its ID and user ID
// We also return an editconflict error if the recovery code has already been used
// We shall return the error
func (m MFAManager) MarkRecoveryCodeAsUsed(id, userID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultMFAManTimeout)
	defer cancel()
	// Mark the recovery code as used in the database
	_, err := m.DB.MarkRecoveryCodeAsUsed(ctx, database.MarkRecoveryCodeAsUsedParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrGeneralRecordNotFound
		default:
			return err
		}
	}
	// we are good to go
	return nil
}

// DeleteRecoveryCodeByID() deletes a recovery code by its ID
// We shall return the error
func (m MFAManager) DeleteRecoveryCodeByID(id, userID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultMFAManTimeout)
	defer cancel()
	// Delete the recovery code from the database
	_, err := m.DB.DeleteRecoveryCodeByID(ctx, database.DeleteRecoveryCodeByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrGeneralRecordNotFound
		default:
			return err
		}
	}
	// we are good to go
	return nil
}
