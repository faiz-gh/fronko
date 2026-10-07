// Package secrets encrypts small values (such as storage credentials) for
// storage in the database, using AES-256-GCM with a key held outside it.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeySize is the required key length: 32 bytes selects AES-256.
const KeySize = 32

// version prefixes every sealed value so the format or key can be rotated later.
const version byte = 1

var ErrDecrypt = errors.New("secrets: unable to decrypt value")

// Box seals and opens values with a single key.
type Box struct {
	aead cipher.AEAD
}

// ParseKey decodes a base64 key (e.g. from `openssl rand -base64 32`).
func ParseKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("secrets: key is not valid base64: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", KeySize, len(key))
	}
	return key, nil
}

func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext. aad (additional authenticated data) is not stored
// but must be passed unchanged to Open; binding it to the owning row stops a
// ciphertext from being copied onto another user's record.
func (b *Box) Seal(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+b.aead.Overhead())
	out = append(out, version)
	out = append(out, nonce...)
	return b.aead.Seal(out, nonce, plaintext, aad), nil
}

func (b *Box) Open(sealed, aad []byte) ([]byte, error) {
	nonceSize := b.aead.NonceSize()
	if len(sealed) < 1+nonceSize+b.aead.Overhead() || sealed[0] != version {
		return nil, ErrDecrypt
	}
	nonce := sealed[1 : 1+nonceSize]
	plaintext, err := b.aead.Open(nil, nonce, sealed[1+nonceSize:], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
