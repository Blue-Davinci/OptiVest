package data

import (
	"errors"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"golang.org/x/crypto/bcrypt"
)

type UserModel struct {
	DB *database.Queries
}

type User struct {
	ID int64 `json:"id"`
}

// Define a custom ErrDuplicateEmail error.
var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

// Define Default Image:
const DefaultImage = "https://res.cloudinary.com/djg9a13ka/image/upload/v1723392222/avatars/avatar_1723392218316.png"

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
