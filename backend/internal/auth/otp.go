package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// GenerateOTP returns a cryptographically random 6-digit code (zero-padded).
func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("otp: generate: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP returns a bcrypt hash of the 6-digit code.
// The hash is stored in otp_challenges.code_hash; the plaintext is never persisted.
func HashOTP(code string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("otp: hash: %w", err)
	}
	return string(h), nil
}

// VerifyOTP returns true if code matches the bcrypt hash.
func VerifyOTP(hash, code string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}
