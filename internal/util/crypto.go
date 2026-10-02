package util

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func randomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func NewID() string {
	return "ord_" + randomString(10)
}

func NewAPIKey() string {
	return "ak_" + randomString(24)
}

func NewSecret() string {
	return randomString(32)
}

func HMACSign(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func HMACVerify(secret string, payload []byte, signature string) bool {
	expected := HMACSign(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
