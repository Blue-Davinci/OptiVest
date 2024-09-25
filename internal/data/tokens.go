package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

// Define the TokenModel type.
type TokenModel struct {
	DB *database.Queries
}

// Timeout constants for our module
const (
	DefaultTokenExpiryTime       = 72 * time.Hour
	DefaultTokenDBContextTimeout = 5 * time.Second
)

// Define constants for the token scope.
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopePasswordReset  = "password-reset"
)

// Define a Token struct to hold the data for an individual token. This includes the
// plaintext and hashed versions of the token, associated user ID, expiry time and
// scope.
type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

// Check that the plaintext token has been provided and is exactly 26 bytes long.
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// Create a Token instance containing the user ID, expiry, and scope information.
// We add the provided ttl (time-to-live) duration parameter to the
// current time to get the expiry time
func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}
	// Initialize a zero-valued byte slice with a length of 16 bytes.
	randomBytes := make([]byte, 16)
	// Use the Read() function from the crypto/rand package to fill the byte slice random bytes
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}
	// Encode the byte slice to a base-32-encoded string and assign it to the token
	// Plaintext field.
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	// Generate a SHA-256 hash of the plaintext token string. This will be the value
	// that we store in the `hash` field of our database table.
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]
	return token, nil
}

func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	api_key, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("API Key: %v\n || User ID: %d", api_key, userID)
	// insert the api key into the database
	err = m.Insert(api_key)
	return api_key, err
}

func (m TokenModel) Insert(api_key *Token) error {
	// create our timeout context. All of them will just be 5 seconds
	ctx, cancel := contextGenerator(context.Background(), DefaultTokenDBContextTimeout)
	defer cancel()
	_, err := m.DB.CreateNewToken(ctx, database.CreateNewTokenParams{
		Hash:   api_key.Hash,
		UserID: api_key.UserID,
		Expiry: api_key.Expiry,
		Scope:  api_key.Scope,
	})
	return err
}

// DeleteAllForUser() deletes all tokens for a specific user and scope.
func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	// create our timeout context. All of them will just be 5 seconds
	ctx, cancel := contextGenerator(context.Background(), DefaultTokenDBContextTimeout)
	defer cancel()
	err := m.DB.DeletAllTokensForUser(ctx, database.DeletAllTokensForUserParams{
		UserID: userID,
		Scope:  scope,
	})
	return err
}
