package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// Sessions manages JWT issuance and Redis-backed session liveness.
// Redis stores jti → userID so sessions can be revoked (e.g. logout).
type Sessions struct {
	secret []byte
	ttl    time.Duration
	rdb    *redis.Client
}

// NewSessions returns a Sessions manager.
func NewSessions(secret []byte, ttl time.Duration, rdb *redis.Client) *Sessions {
	return &Sessions{secret: secret, ttl: ttl, rdb: rdb}
}

// authClaims is the JWT payload.
type authClaims struct {
	Phone string `json:"phone,omitempty"`
	jwt.RegisteredClaims
}

// Issue creates a signed JWT and stores the session in Redis.
// Returns the signed token string and its expiry time.
func (s *Sessions) Issue(ctx context.Context, userID, phone string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	// jti is user-scoped and nanosecond-timestamped to avoid collisions.
	jti := fmt.Sprintf("%s:%d", userID, now.UnixNano())

	c := authClaims{
		Phone: phone,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("session: sign: %w", err)
	}

	// Redis key: session:{jti} → userID, TTL mirrors the JWT expiry.
	if err := s.rdb.Set(ctx, "session:"+jti, userID, s.ttl).Err(); err != nil {
		return "", time.Time{}, fmt.Errorf("session: redis store: %w", err)
	}

	return signed, exp, nil
}

// Verify parses and validates a JWT, then checks Redis for session liveness.
// Returns the userID on success.
func (s *Sessions) Verify(ctx context.Context, tokenStr string) (string, error) {
	c := &authClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("session: unexpected signing method %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return "", fmt.Errorf("session: invalid token")
	}

	// Confirm session is still live in Redis (revocation / logout support).
	if err := s.rdb.Get(ctx, "session:"+c.ID).Err(); err != nil {
		return "", fmt.Errorf("session: not found or expired")
	}

	return c.Subject, nil
}

// Revoke deletes the session from Redis (logout).
func (s *Sessions) Revoke(ctx context.Context, tokenStr string) error {
	c := &authClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return fmt.Errorf("session: parse for revoke: %w", err)
	}
	return s.rdb.Del(ctx, "session:"+c.ID).Err()
}
