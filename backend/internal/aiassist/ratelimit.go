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

// defaultSpendWindow is the refill window for the default workspace spend cap.
// The cap is "at most N billable model calls per window"; it refills each window
// rather than being a lifetime counter, so a burst of usage cannot permanently
// disable the endpoint for everyone.
const defaultSpendWindow = time.Hour

// spendCap is an in-process, windowed SpendGuard: it allows at most maxCalls
// billable model calls per window, refilling at the start of each window. It is
// a hook point — production should replace it with a guard backed by the real
// workspace spend ledger / Secret Manager budget. maxCalls <= 0 means "no cap".
type spendCap struct {
	maxCalls int
	window   time.Duration
	now      func() time.Time

	mu          sync.Mutex
	windowStart time.Time
	used        int
}

// NewSpendCap returns a windowed SpendGuard allowing at most maxCalls model
// calls per window. maxCalls <= 0 disables the cap; window <= 0 uses
// defaultSpendWindow.
func NewSpendCap(maxCalls int, window time.Duration) SpendGuard {
	if window <= 0 {
		window = defaultSpendWindow
	}
	return &spendCap{maxCalls: maxCalls, window: window, now: time.Now}
}

func (s *spendCap) Allow() bool {
	if s.maxCalls <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if s.windowStart.IsZero() || now.Sub(s.windowStart) >= s.window {
		s.windowStart = now
		s.used = 0
	}
	if s.used >= s.maxCalls {
		return false
	}
	s.used++
	return true
}
