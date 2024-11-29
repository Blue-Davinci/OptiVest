package data

import (
	"context"
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
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	RecoveryCodes []string  `json:"recovery_codes"`
	Used          bool      `json:"used,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
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
		ID:            codeDetail.ID,
		UserID:        userID,
		RecoveryCodes: recoveryCodes.RecoveryCodes,
		Used:          false,
		CreatedAt:     codeDetail.CreatedAt.Time,
	}
	// we are good to go
	return recoveryCodeDetails, nil
}
