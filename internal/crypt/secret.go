package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const Prefix = "enc:v1:"

type Cipher struct {
	aead cipher.AEAD
}

func New(secret string) (*Cipher, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("secret tidak boleh kosong")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return Prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) Decrypt(value string) (string, error) {
	if value == "" || !IsEncrypted(value) {
		return value, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, Prefix))
	if err != nil {
		return "", err
	}
	if len(raw) < c.aead.NonceSize() {
		return "", errors.New("ciphertext tidak valid")
	}
	nonce, ciphertext := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("gagal decrypt router password: secret tidak cocok atau data rusak")
	}
	return string(plaintext), nil
}
