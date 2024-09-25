package data

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	KeyLength16 = 16
	KeyLength24 = 24
	KeyLength32 = 32
)

var (
	ErrInvalidEncryptionKeyLength = errors.New("invalid key length, must be 16, 24, or 32 bytes")
)

// contextGenerator is a helper function that generates a new context.Context from a
// context.Context and a timeout duration. This is useful for creating new contexts with
// deadlines for outgoing requests in our data layer.
func contextGenerator(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// generateSecurityKey() generates a cryptographically secure AES key of the given length
func generateSecurityKey(keyLength int) ([]byte, error) {
	if keyLength != KeyLength16 && keyLength != KeyLength24 && keyLength != KeyLength32 {
		return nil, ErrInvalidEncryptionKeyLength
	}
	key := make([]byte, keyLength)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func DecodeEncryptionKey(encryptionkey string) ([]byte, error) {
	if encryptionkey == "" {
		return nil, fmt.Errorf("encryption key cannot be empty")
	}
	decodedKey, err := hex.DecodeString(encryptionkey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}
	return decodedKey, nil
}

// encryptData encrypts a given piece of data using AES-GCM
func EncryptData(data string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	encrypted := gcm.Seal(nonce, nonce, []byte(data), nil)
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

// DecryptData function decrypts AES-GCM encrypted data.
func DecryptData(encryptedData string, key []byte) (string, error) {
	if encryptedData == "" {
		return "", fmt.Errorf("encrypted data cannot be empty")
	}

	data, err := base64.URLEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("invalid encrypted data: insufficient length")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(decrypted), nil
}
