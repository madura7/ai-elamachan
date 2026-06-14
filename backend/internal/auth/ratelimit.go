package auth

import (
	"sync"
	"time"
)

// RateLimiter gates requests by caller key.
type RateLimiter interface {
	Allow(key string) bool
}

// windowLimiter is an in-memory fixed-window rate limiter.
// Adequate for a single backend instance; swap for a Redis-backed implementation
// in multi-replica deployments.
type windowLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*windowBucket
}

type windowBucket struct {
	start time.Time
	count int
}

// NewWindowLimiter returns a rate limiter allowing at most limit requests per
// key within each window.
func NewWindowLimiter(limit int, window time.Duration) RateLimiter {
	return &windowLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*windowBucket),
	}
}

func (l *windowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &windowBucket{start: now, count: 1}
		return l.limit >= 1
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
