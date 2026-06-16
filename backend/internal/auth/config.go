package auth

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the auth subsystem.
type Config struct {
	JWTSecret       []byte
	SessionTTL      time.Duration
	OTPExpiryTTL    time.Duration
	MaxOTPAttempts  int
	PhoneRateLimit  int
	PhoneRateWindow time.Duration
	IPRateLimit     int
	IPRateWindow    time.Duration
	RedisURL        string
	DatabaseURL     string
	// SMSMode: "dev" logs the OTP; real provider requires VER-44.
	SMSMode string
	// DevOTPBypass: when true AND SMS_MODE=="dev", seeded test phones accept OTP "000000".
	// Structurally impossible in prod: boot fails if this flag is on with a real SMS provider.
	DevOTPBypass bool
}

// NewConfigFromEnv reads auth configuration from environment variables.
// Required: JWT_SECRET, DATABASE_URL.
// Optional: REDIS_URL (default redis://localhost:6379/0), SMS_MODE (default dev),
//
//	OTP_EXPIRY_MINUTES (default 10).
func NewConfigFromEnv() (Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return Config{}, fmt.Errorf("auth: JWT_SECRET is required")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("auth: DATABASE_URL is required")
	}
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/0")

	smsMode := envOr("SMS_MODE", "dev")
	devOTPBypass := os.Getenv("DEV_OTP_BYPASS") == "true"

	// Fail fast: the bypass is structurally impossible with a real SMS provider.
	// This ensures it can never accidentally activate in production.
	if devOTPBypass && smsMode != "dev" {
		return Config{}, fmt.Errorf("auth: DEV_OTP_BYPASS=true is not allowed when SMS_MODE=%q (only permitted with SMS_MODE=dev)", smsMode)
	}

	c := Config{
		JWTSecret:       []byte(secret),
		SessionTTL:      30 * 24 * time.Hour,
		OTPExpiryTTL:    10 * time.Minute,
		MaxOTPAttempts:  5,
		PhoneRateLimit:  5,
		PhoneRateWindow: 10 * time.Minute,
		IPRateLimit:     20,
		IPRateWindow:    10 * time.Minute,
		RedisURL:        redisURL,
		DatabaseURL:     dbURL,
		SMSMode:         smsMode,
		DevOTPBypass:    devOTPBypass,
	}

	if v := os.Getenv("OTP_EXPIRY_MINUTES"); v != "" {
		mins, err := strconv.Atoi(v)
		if err != nil || mins <= 0 {
			return Config{}, fmt.Errorf("auth: OTP_EXPIRY_MINUTES must be a positive integer")
		}
		c.OTPExpiryTTL = time.Duration(mins) * time.Minute
	}

	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
