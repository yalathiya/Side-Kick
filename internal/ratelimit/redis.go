package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// luaTokenBucket is the Lua script for atomic token bucket rate limiting in Redis.
// Returns 1 if allowed, 0 if denied.
const luaTokenBucket = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local data = redis.call("HMGET", key, "tokens", "last_check")
local tokens = tonumber(data[1])
local last_check = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_check = now
end

-- Refill tokens based on elapsed time
local elapsed = now - last_check
tokens = math.min(tokens + (elapsed * refill_rate), capacity)

if tokens >= 1 then
    tokens = tokens - 1
    redis.call("HMSET", key, "tokens", tokens, "last_check", now)
    redis.call("EXPIRE", key, math.ceil(capacity / refill_rate) + 10)
    return 1
else
    redis.call("HMSET", key, "tokens", tokens, "last_check", now)
    redis.call("EXPIRE", key, math.ceil(capacity / refill_rate) + 10)
    return 0
end
`

// RedisLimiter implements distributed rate limiting using Redis.
type RedisLimiter struct {
	client   *redis.Client
	script   *redis.Script
	rate     float64
	burst    int
	fallback Limiter // in-memory fallback when Redis is unavailable
}

// NewRedis creates a Redis-backed rate limiter.
// Falls back to the provided in-memory limiter if Redis is unavailable.
func NewRedis(redisURL string, redisDB int, rate float64, burst int, fallback Limiter) (*RedisLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	opts.DB = redisDB

	client := redis.NewClient(opts)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisLimiter{
		client:   client,
		script:   redis.NewScript(luaTokenBucket),
		rate:     rate,
		burst:    burst,
		fallback: fallback,
	}, nil
}

// Allow checks if a request from the given key is allowed.
// Falls back to in-memory limiter if Redis is unreachable.
func (rl *RedisLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	redisKey := "sidekick:ratelimit:" + key
	now := float64(time.Now().UnixMilli()) / 1000.0

	result, err := rl.script.Run(ctx, rl.client, []string{redisKey},
		rl.burst, rl.rate, now,
	).Int()

	if err != nil {
		// Redis unavailable — gracefully fall back to in-memory
		if rl.fallback != nil {
			return rl.fallback.Allow(key)
		}
		return true // fail open if no fallback
	}

	return result == 1
}

// Close shuts down the Redis connection.
func (rl *RedisLimiter) Close() error {
	return rl.client.Close()
}
