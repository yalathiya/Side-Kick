package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a per-key token bucket rate limiter.
type TokenBucket struct {
	rate    float64 // tokens added per second
	burst   int     // max tokens (bucket capacity)
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// New creates a new TokenBucket rate limiter.
// rate: tokens per second, burst: maximum bucket size.
func New(rate float64, burst int) *TokenBucket {
	tb := &TokenBucket{
		rate:    rate,
		burst:   burst,
		buckets: make(map[string]*bucket),
	}
	go tb.cleanup()
	return tb
}

// Allow checks if a request from the given key is allowed.
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, exists := tb.buckets[key]
	if !exists {
		tb.buckets[key] = &bucket{
			tokens:    float64(tb.burst) - 1,
			lastCheck: now,
		}
		return true
	}

	// Add tokens based on elapsed time
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > float64(tb.burst) {
		b.tokens = float64(tb.burst)
	}
	b.lastCheck = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// cleanup periodically removes stale entries to prevent memory leaks.
func (tb *TokenBucket) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tb.mu.Lock()
		now := time.Now()
		for key, b := range tb.buckets {
			// Remove buckets that have been idle and are full
			if now.Sub(b.lastCheck) > 10*time.Minute {
				delete(tb.buckets, key)
			}
		}
		tb.mu.Unlock()
	}
}
