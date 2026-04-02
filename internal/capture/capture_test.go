package capture

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_AddAndEntries(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Add(Entry{RequestID: "1", Timestamp: time.Now()})
	rb.Add(Entry{RequestID: "2", Timestamp: time.Now()})

	entries := rb.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].RequestID != "1" || entries[1].RequestID != "2" {
		t.Error("entries not in chronological order")
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	rb := NewRingBuffer(3)

	// Add 5 entries — should only keep last 3
	for i := 1; i <= 5; i++ {
		rb.Add(Entry{RequestID: fmt.Sprintf("%d", i)})
	}

	entries := rb.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be 3, 4, 5 (oldest first)
	if entries[0].RequestID != "3" {
		t.Errorf("expected oldest=3, got %s", entries[0].RequestID)
	}
	if entries[2].RequestID != "5" {
		t.Errorf("expected newest=5, got %s", entries[2].RequestID)
	}
}

func TestRingBuffer_Recent(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 1; i <= 5; i++ {
		rb.Add(Entry{RequestID: fmt.Sprintf("%d", i)})
	}

	recent := rb.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent, got %d", len(recent))
	}
	// Newest first: 5, 4, 3
	if recent[0].RequestID != "5" {
		t.Errorf("expected newest=5, got %s", recent[0].RequestID)
	}
	if recent[2].RequestID != "3" {
		t.Errorf("expected third=3, got %s", recent[2].RequestID)
	}
}

func TestRingBuffer_RecentMoreThanAvailable(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Add(Entry{RequestID: "only"})

	recent := rb.Recent(5)
	if len(recent) != 1 {
		t.Errorf("expected 1, got %d", len(recent))
	}
}

func TestRingBuffer_Len(t *testing.T) {
	rb := NewRingBuffer(3)
	if rb.Len() != 0 {
		t.Error("expected 0 initially")
	}

	rb.Add(Entry{})
	rb.Add(Entry{})
	if rb.Len() != 2 {
		t.Errorf("expected 2, got %d", rb.Len())
	}

	rb.Add(Entry{})
	rb.Add(Entry{}) // wraps
	if rb.Len() != 3 {
		t.Errorf("expected max 3, got %d", rb.Len())
	}
}

func TestRingBuffer_FindByRequestID(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Add(Entry{RequestID: "abc", Method: "GET", Path: "/test"})
	rb.Add(Entry{RequestID: "def", Method: "POST", Path: "/other"})

	found := rb.FindByRequestID("abc")
	if found == nil {
		t.Fatal("expected to find entry")
	}
	if found.Method != "GET" || found.Path != "/test" {
		t.Errorf("unexpected entry: %+v", found)
	}

	if rb.FindByRequestID("nonexistent") != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestRingBuffer_DefaultSize(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.size != 100 {
		t.Errorf("expected default size 100, got %d", rb.size)
	}

	rb = NewRingBuffer(-5)
	if rb.size != 100 {
		t.Errorf("expected default size 100 for negative, got %d", rb.size)
	}
}

func TestRingBuffer_ConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(50)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				rb.Add(Entry{RequestID: fmt.Sprintf("%d-%d", id, j)})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				rb.Entries()
				rb.Len()
				rb.Recent(10)
			}
		}()
	}

	wg.Wait()

	// Should have exactly 50 (buffer size)
	if rb.Len() != 50 {
		t.Errorf("expected 50 after concurrent writes, got %d", rb.Len())
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(10)
	if len(rb.Entries()) != 0 {
		t.Error("expected empty entries")
	}
	if len(rb.Recent(5)) != 0 {
		t.Error("expected empty recent")
	}
}

// --- Query / Filter tests ---

func seedBuffer() *RingBuffer {
	rb := NewRingBuffer(100)
	rb.Add(Entry{RequestID: "1", Method: "GET", Path: "/api/users", Status: 200, DurationMs: 10})
	rb.Add(Entry{RequestID: "2", Method: "POST", Path: "/api/users", Status: 201, DurationMs: 50})
	rb.Add(Entry{RequestID: "3", Method: "GET", Path: "/api/orders", Status: 404, DurationMs: 5})
	rb.Add(Entry{RequestID: "4", Method: "GET", Path: "/api/users/1", Status: 500, DurationMs: 200})
	rb.Add(Entry{RequestID: "5", Method: "DELETE", Path: "/api/users/1", Status: 401, DurationMs: 2})
	rb.Add(Entry{RequestID: "6", Method: "GET", Path: "/api/orders", Status: 200, DurationMs: 30})
	rb.Add(Entry{RequestID: "7", Method: "PUT", Path: "/api/users/1", Status: 403, DurationMs: 3})
	rb.Add(Entry{RequestID: "8", Method: "GET", Path: "/health", Status: 200, DurationMs: 1})
	return rb
}

func TestQuery_NoFilter_NewestFirst(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{})
	if len(results) != 8 {
		t.Fatalf("expected 8, got %d", len(results))
	}
	if results[0].RequestID != "8" {
		t.Errorf("expected newest first (8), got %s", results[0].RequestID)
	}
}

func TestQuery_FilterByStatusGroup_Errors(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{StatusGroups: []string{"4xx", "5xx"}})

	for _, e := range results {
		if e.Status < 400 {
			t.Errorf("expected only 4xx/5xx, got status %d", e.Status)
		}
	}
	// 404, 500, 401, 403 = 4 entries
	if len(results) != 4 {
		t.Errorf("expected 4 error entries, got %d", len(results))
	}
}

func TestQuery_FilterByStatusGroup_Success(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{StatusGroups: []string{"2xx"}})
	if len(results) != 4 {
		t.Errorf("expected 4 success entries, got %d", len(results))
	}
	for _, e := range results {
		if e.Status < 200 || e.Status >= 300 {
			t.Errorf("expected only 2xx, got status %d", e.Status)
		}
	}
}

func TestQuery_FilterByMethod(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{Methods: []string{"POST", "DELETE"}})
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
	for _, e := range results {
		if e.Method != "POST" && e.Method != "DELETE" {
			t.Errorf("unexpected method %s", e.Method)
		}
	}
}

func TestQuery_FilterByRoute(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{Route: "/api/users"})
	// /api/users, /api/users, /api/users/1, /api/users/1, /api/users/1 = 5
	if len(results) != 5 {
		t.Errorf("expected 5 entries with /api/users prefix, got %d", len(results))
	}
	for _, e := range results {
		if e.Path != "/api/users" && e.Path != "/api/users/1" {
			t.Errorf("unexpected path %s", e.Path)
		}
	}
}

func TestQuery_SortByDuration_SlowestFirst(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{Sort: "duration"})
	if len(results) < 2 {
		t.Fatal("expected results")
	}
	if results[0].DurationMs != 200 {
		t.Errorf("expected slowest (200ms) first, got %.1fms", results[0].DurationMs)
	}
	// Verify descending order
	for i := 1; i < len(results); i++ {
		if results[i].DurationMs > results[i-1].DurationMs {
			t.Errorf("not sorted: %.1fms > %.1fms at index %d", results[i].DurationMs, results[i-1].DurationMs, i)
		}
	}
}

func TestQuery_SortByDuration_WithStatusFilter(t *testing.T) {
	rb := seedBuffer()
	// Slowest errors
	results := rb.Query(Filter{StatusGroups: []string{"4xx", "5xx"}, Sort: "duration", Limit: 2})
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	// 500 at 200ms should be first, then 404 at 5ms or 403 at 3ms
	if results[0].Status != 500 {
		t.Errorf("expected 500 (slowest error) first, got %d", results[0].Status)
	}
}

func TestQuery_Limit(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{Limit: 3})
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}
}

func TestQuery_CombinedFilters(t *testing.T) {
	rb := seedBuffer()
	// GET requests that returned errors, sorted by duration
	results := rb.Query(Filter{
		Methods:      []string{"GET"},
		StatusGroups: []string{"4xx", "5xx"},
		Sort:         "duration",
	})
	// GET 404 /api/orders (5ms), GET 500 /api/users/1 (200ms) — sorted: 500 first
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if results[0].Status != 500 {
		t.Errorf("expected 500 first (slowest), got %d", results[0].Status)
	}
	if results[1].Status != 404 {
		t.Errorf("expected 404 second, got %d", results[1].Status)
	}
}

func TestQuery_UnauthorizedRequests(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{StatusGroups: []string{"4xx"}})
	// 404, 401, 403 = 3 entries
	if len(results) != 3 {
		t.Errorf("expected 3 unauthorized/client errors, got %d", len(results))
	}
}

func TestQuery_EmptyResult(t *testing.T) {
	rb := seedBuffer()
	results := rb.Query(Filter{Methods: []string{"PATCH"}})
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
