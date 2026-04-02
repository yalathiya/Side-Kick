package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func redisAvailable() bool {
	url := os.Getenv("SIDEKICK_TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return false
	}
	client := redis.NewClient(opts)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return client.Ping(ctx).Err() == nil
}

func TestNewRedis_InvalidURL(t *testing.T) {
	_, err := NewRedis("not-a-valid-url", 0, 10, 20, nil)
	if err == nil {
		t.Error("expected error for invalid Redis URL")
	}
}

func TestNewRedis_ConnectionFailed(t *testing.T) {
	// Try to connect to a port that's almost certainly not running Redis
	_, err := NewRedis("redis://localhost:59999", 0, 10, 20, nil)
	if err == nil {
		t.Error("expected error when Redis is unreachable")
	}
}

func TestRedisLimiter_FallbackOnError(t *testing.T) {
	// Create a limiter with an invalid Redis that will fail on every call
	fallback := New(10, 20)

	// We can't create a RedisLimiter via NewRedis (it pings),
	// so construct directly to test fallback behavior
	rl := &RedisLimiter{
		client:   redis.NewClient(&redis.Options{Addr: "localhost:59999"}),
		script:   redis.NewScript(luaTokenBucket),
		rate:     10,
		burst:    20,
		fallback: fallback,
	}
	defer rl.Close()

	// Should fall back to in-memory and allow
	if !rl.Allow("test-ip") {
		t.Error("expected Allow to succeed via fallback")
	}
}

func TestRedisLimiter_FallbackNilFailsOpen(t *testing.T) {
	rl := &RedisLimiter{
		client:   redis.NewClient(&redis.Options{Addr: "localhost:59999"}),
		script:   redis.NewScript(luaTokenBucket),
		rate:     10,
		burst:    20,
		fallback: nil, // no fallback
	}
	defer rl.Close()

	// Should fail open (allow) when no fallback
	if !rl.Allow("test-ip") {
		t.Error("expected Allow to fail open with no fallback")
	}
}

func TestRedisLimiter_LimiterInterface(t *testing.T) {
	// Verify RedisLimiter satisfies the Limiter interface
	var _ Limiter = (*RedisLimiter)(nil)
}

func TestTokenBucket_LimiterInterface(t *testing.T) {
	// Verify TokenBucket satisfies the Limiter interface
	var _ Limiter = (*TokenBucket)(nil)
}

// Integration tests — only run when Redis is available
func TestRedisLimiter_Integration(t *testing.T) {
	if !redisAvailable() {
		t.Skip("Redis not available, skipping integration tests")
	}

	url := os.Getenv("SIDEKICK_TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}

	t.Run("AllowBurst", func(t *testing.T) {
		rl, err := NewRedis(url, 0, 10, 5, nil)
		if err != nil {
			t.Fatalf("failed to create redis limiter: %v", err)
		}
		defer rl.Close()

		key := "test:burst:" + t.Name()

		// Flush the test key
		rl.client.Del(context.Background(), "sidekick:ratelimit:"+key)

		// Should allow burst of 5
		for i := 0; i < 5; i++ {
			if !rl.Allow(key) {
				t.Errorf("expected allow on request %d", i+1)
			}
		}

		// 6th should be denied
		if rl.Allow(key) {
			t.Error("expected deny after burst exceeded")
		}
	})

	t.Run("PerKeyIsolation", func(t *testing.T) {
		rl, err := NewRedis(url, 0, 10, 2, nil)
		if err != nil {
			t.Fatalf("failed to create redis limiter: %v", err)
		}
		defer rl.Close()

		keyA := "test:iso:a:" + t.Name()
		keyB := "test:iso:b:" + t.Name()

		rl.client.Del(context.Background(), "sidekick:ratelimit:"+keyA)
		rl.client.Del(context.Background(), "sidekick:ratelimit:"+keyB)

		// Exhaust keyA
		rl.Allow(keyA)
		rl.Allow(keyA)

		// keyB should still work
		if !rl.Allow(keyB) {
			t.Error("expected keyB to be allowed (isolated from keyA)")
		}
	})
}
