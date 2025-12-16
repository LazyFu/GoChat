package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// GenerateNonce returns a random 12-byte nonce suitable for AES-GCM.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// EncryptAESGCM encrypts plaintext using AES-GCM.
func EncryptAESGCM(plaintext, key []byte, associatedData []byte) (nonce []byte, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = GenerateNonce()
	if err != nil {
		return nil, nil, err
	}
	// Seal appends the output to the first argument; passing nil returns a new slice.
	ciphertext = gcm.Seal(nil, nonce, plaintext, associatedData)
	return nonce, ciphertext, nil
}

// DecryptAESGCM decrypts ciphertext using AES-GCM.
func DecryptAESGCM(nonce, ciphertext, key []byte, associatedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
