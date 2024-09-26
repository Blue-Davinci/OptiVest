package data

import (
	"errors"

	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

type MFAToken struct {
	TOTPCode  string `json:"totp_code"`
	TOTPToken string `json:"totp_token"`
	Email     string `json:"email"`
}

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
