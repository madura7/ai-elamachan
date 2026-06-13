package aiassist

import (
	"sync"
	"time"
)

// defaultWindow is the fixed window used by the default per-user rate limit.
const defaultWindow = time.Minute

// windowLimiter is a simple in-memory fixed-window rate limiter keyed by caller.
// It is adequate for a single backend instance; a multi-replica deployment
// should swap in a Redis-backed RateLimiter so the budget is shared.
type windowLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	buckets map[string]*windowBucket
}

type windowBucket struct {
	windowStart time.Time
	count       int
}

// NewWindowLimiter returns a RateLimiter that allows at most limit requests per
// key within each window.
func NewWindowLimiter(limit int, window time.Duration) RateLimiter {
	return &windowLimiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		buckets: make(map[string]*windowBucket),
	}
}

func (l *windowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.windowStart) >= l.window {
		l.buckets[key] = &windowBucket{windowStart: now, count: 1}
		return l.limit >= 1
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

// spendCap is a simple in-process SpendGuard that allows a bounded number of
// model calls before tripping. It is a hook point: production should replace it
// with a guard that reads the real workspace spend ledger / Secret Manager
// budget. maxCalls <= 0 means "no cap".
type spendCap struct {
	mu       sync.Mutex
	maxCalls int
	used     int
}

// NewSpendCap returns a SpendGuard that trips after maxCalls model calls.
// maxCalls <= 0 disables the cap.
func NewSpendCap(maxCalls int) SpendGuard {
	return &spendCap{maxCalls: maxCalls}
}

func (s *spendCap) Allow() bool {
	if s.maxCalls <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used >= s.maxCalls {
		return false
	}
	s.used++
	return true
}
