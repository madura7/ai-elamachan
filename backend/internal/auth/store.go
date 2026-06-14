package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned by Store operations.
var (
	ErrChallengeNotFound  = errors.New("auth: challenge not found")
	ErrChallengeExpired   = errors.New("auth: challenge expired")
	ErrChallengeConsumed  = errors.New("auth: challenge already consumed")
	ErrMaxAttemptsReached = errors.New("auth: max verification attempts reached")
)

// Challenge holds OTP challenge data fetched from otp_challenges.
type Challenge struct {
	ID           string
	PhoneE164    string
	CodeHash     string
	Purpose      string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	AttemptCount int
}

// User holds user data fetched from the users table.
type User struct {
	ID                string
	PhoneE164         string
	DisplayName       string
	PreferredLanguage string
}

// Store wraps auth-related database operations.
type Store struct {
	db          *sql.DB
	maxAttempts int
}

// NewStore creates a Store backed by db.
func NewStore(db *sql.DB, maxAttempts int) *Store {
	return &Store{db: db, maxAttempts: maxAttempts}
}

// CreateChallenge inserts a new OTP challenge row and returns it.
func (s *Store) CreateChallenge(ctx context.Context, phone, codeHash, purpose string, expiresAt time.Time) (*Challenge, error) {
	const q = `
		INSERT INTO otp_challenges (phone_e164, code_hash, purpose, expires_at)
		VALUES ($1, $2, $3::otp_purpose, $4)
		RETURNING id, phone_e164, code_hash, purpose, expires_at, consumed_at, attempt_count`

	c := &Challenge{}
	err := s.db.QueryRowContext(ctx, q, phone, codeHash, purpose, expiresAt).
		Scan(&c.ID, &c.PhoneE164, &c.CodeHash, &c.Purpose, &c.ExpiresAt, &c.ConsumedAt, &c.AttemptCount)
	if err != nil {
		return nil, fmt.Errorf("auth: create challenge: %w", err)
	}
	return c, nil
}

// GetChallenge fetches the challenge by UUID and validates it is live.
// Returns sentinel errors for not-found, expired, consumed, and over-attempt states.
func (s *Store) GetChallenge(ctx context.Context, id string) (*Challenge, error) {
	const q = `
		SELECT id, phone_e164, code_hash, purpose, expires_at, consumed_at, attempt_count
		FROM otp_challenges WHERE id = $1`

	c := &Challenge{}
	err := s.db.QueryRowContext(ctx, q, id).
		Scan(&c.ID, &c.PhoneE164, &c.CodeHash, &c.Purpose, &c.ExpiresAt, &c.ConsumedAt, &c.AttemptCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChallengeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get challenge: %w", err)
	}
	if c.ConsumedAt != nil {
		return nil, ErrChallengeConsumed
	}
	if time.Now().After(c.ExpiresAt) {
		return nil, ErrChallengeExpired
	}
	if c.AttemptCount >= s.maxAttempts {
		return nil, ErrMaxAttemptsReached
	}
	return c, nil
}

// IncrAttempt increments attempt_count for a challenge, returning the new count.
// Called before comparing the code so brute-force burns attempts even on timing paths.
func (s *Store) IncrAttempt(ctx context.Context, id string) (int, error) {
	const q = `UPDATE otp_challenges SET attempt_count = attempt_count + 1 WHERE id = $1 RETURNING attempt_count`
	var count int
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&count); err != nil {
		return 0, fmt.Errorf("auth: incr attempt: %w", err)
	}
	return count, nil
}

// ConsumeChallenge marks a challenge consumed (single-use enforcement).
// Returns ErrChallengeConsumed if it was already consumed (concurrent verify race).
func (s *Store) ConsumeChallenge(ctx context.Context, id string) error {
	const q = `UPDATE otp_challenges SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: consume challenge: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrChallengeConsumed
	}
	return nil
}

// FindOrCreateUser returns the user for phoneE164, creating a new user if absent.
// New users get a default display_name of "User_{last4}" and preferredLang.
func (s *Store) FindOrCreateUser(ctx context.Context, phoneE164, preferredLang string) (*User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	const findQ = `SELECT id, phone_e164, display_name, preferred_language FROM users WHERE phone_e164 = $1`
	u := &User{}
	err = tx.QueryRowContext(ctx, findQ, phoneE164).
		Scan(&u.ID, &u.PhoneE164, &u.DisplayName, &u.PreferredLanguage)
	if err == nil {
		_ = tx.Commit()
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("auth: find user: %w", err)
	}

	// New user: derive a default display name from the last 4 digits.
	suffix := phoneE164
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	displayName := "User_" + suffix

	const createQ = `
		INSERT INTO users (phone_e164, phone_verified_at, display_name, preferred_language)
		VALUES ($1, now(), $2, $3)
		RETURNING id, phone_e164, display_name, preferred_language`
	err = tx.QueryRowContext(ctx, createQ, phoneE164, displayName, preferredLang).
		Scan(&u.ID, &u.PhoneE164, &u.DisplayName, &u.PreferredLanguage)
	if err != nil {
		return nil, fmt.Errorf("auth: create user: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("auth: commit user: %w", err)
	}
	return u, nil
}
