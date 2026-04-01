package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_FirstRequestAlwaysAllowed(t *testing.T) {
	rl := New(10, 10)
	if !rl.Allow("192.168.1.1") {
		t.Error("first request should always be allowed")
	}
}

func TestAllow_BurstLimit(t *testing.T) {
	burst := 5
	rl := New(1, burst) // 1 token/sec, burst of 5

	for i := 0; i < burst; i++ {
		if !rl.Allow("client-a") {
			t.Errorf("request %d should be allowed within burst limit", i+1)
		}
	}

	// Next request should be rejected (burst exhausted)
	if rl.Allow("client-a") {
		t.Error("request beyond burst should be rejected")
	}
}

func TestAllow_PerKeyIsolation(t *testing.T) {
	rl := New(1, 2) // burst of 2

	// Exhaust client-a's bucket
	rl.Allow("client-a")
	rl.Allow("client-a")
	if rl.Allow("client-a") {
		t.Error("client-a should be rate limited")
	}

	// client-b should still have tokens
	if !rl.Allow("client-b") {
		t.Error("client-b should not be affected by client-a's rate limit")
	}
}

func TestAllow_TokenRefill(t *testing.T) {
	rl := New(100, 1) // 100 tokens/sec, burst of 1

	// Use the only token
	if !rl.Allow("client") {
		t.Error("first request should be allowed")
	}
	if rl.Allow("client") {
		t.Error("second immediate request should be rejected (burst=1)")
	}

	// Wait for refill
	time.Sleep(20 * time.Millisecond)

	// Should have refilled by now (100 tokens/sec * 0.02s = 2 tokens, capped at burst=1)
	if !rl.Allow("client") {
		t.Error("request after refill should be allowed")
	}
}

func TestAllow_TokensCappedAtBurst(t *testing.T) {
	rl := New(1000, 3) // fast refill, burst of 3

	// Wait to ensure bucket is full
	time.Sleep(10 * time.Millisecond)

	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow("client") {
			allowed++
		}
	}

	if allowed != 3 {
		t.Errorf("expected exactly %d allowed (burst cap), got %d", 3, allowed)
	}
}

func TestAllow_BurstOfOne(t *testing.T) {
	rl := New(0, 1) // rate=0 (no refill), burst of 1

	// First request allowed (uses the 1 token)
	if !rl.Allow("client") {
		t.Error("first request with burst=1 should be allowed")
	}

	// All subsequent requests rejected (no refill)
	for i := 0; i < 5; i++ {
		if rl.Allow("client") {
			t.Errorf("request %d should be rejected (no refill)", i+2)
		}
	}
}

func TestAllow_ConcurrentAccess(t *testing.T) {
	rl := New(1000, 50)

	var wg sync.WaitGroup
	allowed := make(chan bool, 200)

	// 200 concurrent requests from same key
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- rl.Allow("concurrent-client")
		}()
	}

	wg.Wait()
	close(allowed)

	totalAllowed := 0
	for a := range allowed {
		if a {
			totalAllowed++
		}
	}

	// Should allow approximately burst amount (50), not more
	if totalAllowed > 55 { // small margin for timing
		t.Errorf("expected ~50 allowed (burst), got %d", totalAllowed)
	}
	if totalAllowed < 40 {
		t.Errorf("expected ~50 allowed, got too few: %d", totalAllowed)
	}
}

func TestAllow_MultipleKeys(t *testing.T) {
	rl := New(10, 5)

	keys := []string{"ip-1", "ip-2", "ip-3", "ip-4", "ip-5"}
	for _, key := range keys {
		if !rl.Allow(key) {
			t.Errorf("first request from %s should be allowed", key)
		}
	}

	// Verify buckets were created for each key
	rl.mu.Lock()
	if len(rl.buckets) != len(keys) {
		t.Errorf("expected %d buckets, got %d", len(keys), len(rl.buckets))
	}
	rl.mu.Unlock()
}

func TestAllow_RapidRequests(t *testing.T) {
	rl := New(2, 2) // 2 tokens/sec, burst of 2

	// Exhaust burst
	rl.Allow("rapid")
	rl.Allow("rapid")

	// Rapid fire — all should be rejected
	rejected := 0
	for i := 0; i < 10; i++ {
		if !rl.Allow("rapid") {
			rejected++
		}
	}

	if rejected != 10 {
		t.Errorf("expected all 10 rapid requests rejected, got %d rejected", rejected)
	}
}

func TestCleanup_DoesNotPanic(t *testing.T) {
	rl := New(10, 10)

	// Add some buckets
	rl.Allow("test1")
	rl.Allow("test2")
	rl.Allow("test3")

	// Verify buckets were created
	rl.mu.Lock()
	initialCount := len(rl.buckets)
	rl.mu.Unlock()

	if initialCount != 3 {
		t.Errorf("expected 3 buckets, got %d", initialCount)
	}

	// Wait a bit to ensure cleanup goroutine runs without panicking
	// (cleanup runs every 5 minutes, but we can't wait that long)
	// This test mainly ensures the goroutine doesn't crash immediately
	time.Sleep(100 * time.Millisecond)
}

func TestCleanup_BucketCreation(t *testing.T) {
	rl := New(10, 10)

	// Create buckets
	rl.Allow("ip1")
	rl.Allow("ip2")

	rl.mu.Lock()
	count := len(rl.buckets)
	rl.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 buckets created, got %d", count)
	}
}
