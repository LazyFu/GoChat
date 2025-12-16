package crypto

import (
	"crypto/rand"
	"errors"
)

// GenerateRandomKey returns a cryptographically secure random key of the given length.
// For AES-256, use length=32.
func GenerateRandomKey(length int) ([]byte, error) {
	if length <= 0 {
		return nil, errors.New("invalid key length")
	}
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
