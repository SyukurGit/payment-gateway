package util

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// NewOrderID generates format: ORD-YYYYMMDD-XXXXXXXXXX (5 random bytes = 1.1 trillion per day)
func NewOrderID() string {
	dateStr := time.Now().Format("20060102")
	b := make([]byte, 5)
	rand.Read(b)
	randHex := hex.EncodeToString(b)
	return fmt.Sprintf("ORD-%s-%s", dateStr, randHex)
}

// NewAdminSessionToken generates a 32-byte secure random hex session token
func NewAdminSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NewSecretToken generates format: sec_ + 24 random hex characters
func NewSecretToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "sec_" + hex.EncodeToString(b)
}

// NewProductID generates format: prod_ + 8 random hex characters
func NewProductID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "prod_" + hex.EncodeToString(b)
}

// VerifyHMAC verifies a payload against a hex signature using HMAC-SHA256
func VerifyHMAC(secret, payload, hexSig string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedMAC), []byte(hexSig))
}
