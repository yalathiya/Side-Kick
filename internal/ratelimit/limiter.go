package ratelimit

// Limiter is the interface for rate limit backends.
// Both in-memory (TokenBucket) and Redis implementations satisfy this.
type Limiter interface {
	Allow(key string) bool
}
