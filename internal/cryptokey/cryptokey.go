package cryptokey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func Seal(master []byte, plaintext string) ([]byte, error) {
	gcm, err := newGCM(master)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func Open(master []byte, blob []byte) (string, error) {
	gcm, err := newGCM(master)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := gcm.Open(nil, blob[:nonceSize], blob[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func newGCM(master []byte) (cipher.AEAD, error) {
	if len(master) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(master)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
