package vkid

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func randomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func NewState() string {
	return randomString()
}

func NewVerifier() string {
	return randomString()
}

func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
