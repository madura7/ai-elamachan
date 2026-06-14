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
		SMSMode:         envOr("SMS_MODE", "dev"),
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
