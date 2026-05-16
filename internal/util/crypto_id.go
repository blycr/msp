package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
)

// IDCodec encrypts and decrypts media path IDs using AES-GCM with a
// deterministic nonce derived from the path via HMAC-SHA256.
// A nil key causes EncodeID to fall back to plain base64.
type IDCodec struct {
	key []byte
}

// NewIDCodec creates a new IDCodec. A nil key causes EncodeID to fall back
// to plain base64.
func NewIDCodec(key []byte) *IDCodec {
	return &IDCodec{key: key}
}

// LoadOrCreateKey loads an existing 32-byte key from path, or generates
// and persists a new one with 0600 permissions.
func LoadOrCreateKey(path string) ([]byte, error) {
	//nolint:gosec // path is a fixed key file path (msp.key), not user input
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

// EncodeID encrypts an absolute path into a URL-safe base64 string.
// Falls back to plain base64 if no key is configured.
func (c *IDCodec) EncodeID(path string) string {
	if c == nil || c.key == nil {
		return base64.RawURLEncoding.EncodeToString([]byte(path))
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(path))
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(path))
	}
	nonce := make([]byte, gcm.NonceSize())
	// Derive deterministic nonce from path using HMAC-SHA256.
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(path))
	sum := mac.Sum(nil)
	copy(nonce, sum[:gcm.NonceSize()])
	ciphertext := gcm.Seal(nonce, nonce, []byte(path), nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext)
}

// DecodeID decrypts a URL-safe base64 ID back to the original path.
// Falls back to plain base64 if no key is configured.
func (c *IDCodec) DecodeID(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	if c == nil || c.key == nil {
		return string(b), nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(b) < nonceSize {
		return "", errors.New("bad id")
	}
	nonce, ciphertext := b[:nonceSize], b[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("bad id")
	}
	return string(plaintext), nil
}
