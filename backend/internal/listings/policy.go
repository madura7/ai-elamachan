package listings

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"time"
)

// Decision is the outcome of a CheckCanPost call.
type Decision struct {
	Allowed    bool
	Code       string
	Message    string
	RetryAfter int64 // Unix timestamp seconds; 0 when not applicable.
}

// PostingPolicy gates whether a given user may create a new listing.
type PostingPolicy interface {
	CheckCanPost(ctx context.Context, userID string) Decision
}

// AllowAllPolicy always permits posting.
type AllowAllPolicy struct{}

func (AllowAllPolicy) CheckCanPost(_ context.Context, _ string) Decision {
	return Decision{Allowed: true}
}

// DailyLimitPolicy denies posts after MaxPerDay listings in the trailing 24 h.
type DailyLimitPolicy struct {
	db        *sql.DB
	MaxPerDay int
}

// policyFromEnv reads POSTING_MAX_PER_DAY and returns the appropriate policy.
// A value of 0 (default) means unlimited.
func policyFromEnv(db *sql.DB) PostingPolicy {
	v := os.Getenv("POSTING_MAX_PER_DAY")
	if v == "" {
		return AllowAllPolicy{}
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return AllowAllPolicy{}
	}
	return &DailyLimitPolicy{db: db, MaxPerDay: n}
}

func (p *DailyLimitPolicy) CheckCanPost(ctx context.Context, userID string) Decision {
	var count int
	err := p.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listings WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'`,
		userID,
	).Scan(&count)
	if err != nil {
		// Fail open on DB errors.
		return Decision{Allowed: true}
	}
	if count >= p.MaxPerDay {
		return Decision{
			Allowed:    false,
			Code:       "posting_limit_exceeded",
			Message:    "daily posting limit reached",
			RetryAfter: time.Now().Add(24 * time.Hour).Unix(),
		}
	}
	return Decision{Allowed: true}
}
