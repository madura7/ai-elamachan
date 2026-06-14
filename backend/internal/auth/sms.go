package auth

import (
	"context"
	"log"
)

// Sender dispatches an OTP code to a phone number.
// The dev stub logs the code to stdout; real delivery needs VER-44 (provider).
type Sender interface {
	Send(ctx context.Context, phoneE164, code string) error
}

// DevStubSender satisfies Sender by logging the OTP to the server log.
// NEVER use in production — the code appears in plaintext server logs.
type DevStubSender struct{}

func (DevStubSender) Send(_ context.Context, phone, code string) error {
	log.Printf("[AUTH DEV STUB] OTP for %s → %s (dev-only, VER-44 needed for real SMS)", phone, code)
	return nil
}
