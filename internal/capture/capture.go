package capture

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Filter defines criteria for querying captured requests.
type Filter struct {
	StatusGroups []string // "2xx", "4xx", "5xx" etc.
	Methods      []string // "GET", "POST" etc.
	Route        string   // path prefix match
	Sort         string   // "duration" (slowest first), "status", or "" (newest first)
	Limit        int      // max results (0 = default 100)
}

// Entry holds captured data for a single request/response pair.
type Entry struct {
	RequestID  string            `json:"request_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Status     int               `json:"status"`
	DurationMs float64           `json:"duration_ms"`
	ClientIP   string            `json:"client_ip"`
	UserID     string            `json:"user_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	ReqBody    string            `json:"req_body,omitempty"`
	RespBody   string            `json:"resp_body,omitempty"`
}

// RingBuffer is a fixed-size circular buffer for request log entries.
// Thread-safe for concurrent writes and reads.
type RingBuffer struct {
	mu      sync.RWMutex
	entries []Entry
	size    int
	pos     int    // next write position
	count   int    // total entries written (for distinguishing empty vs full)
}

// NewRingBuffer creates a buffer that holds at most `size` entries.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 100
	}
	return &RingBuffer{
		entries: make([]Entry, size),
		size:    size,
	}
}

// Add writes an entry to the ring buffer, overwriting the oldest if full.
func (rb *RingBuffer) Add(e Entry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.pos] = e
	rb.pos = (rb.pos + 1) % rb.size
	rb.count++
}

// Entries returns all stored entries in chronological order (oldest first).
func (rb *RingBuffer) Entries() []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	n := rb.size
	if rb.count < rb.size {
		n = rb.count
	}

	result := make([]Entry, 0, n)
	if rb.count < rb.size {
		// Buffer not yet full — entries 0..pos-1
		result = append(result, rb.entries[:rb.pos]...)
	} else {
		// Buffer wrapped — oldest is at pos, read pos..end then 0..pos-1
		result = append(result, rb.entries[rb.pos:]...)
		result = append(result, rb.entries[:rb.pos]...)
	}
	return result
}

// Recent returns the last `n` entries in reverse chronological order (newest first).
func (rb *RingBuffer) Recent(n int) []Entry {
	all := rb.Entries()
	if n > len(all) {
		n = len(all)
	}
	// Reverse the last n entries
	result := make([]Entry, n)
	for i := 0; i < n; i++ {
		result[i] = all[len(all)-1-i]
	}
	return result
}

// Len returns the number of entries currently stored.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.count < rb.size {
		return rb.count
	}
	return rb.size
}

// Query returns entries matching the given filter criteria.
func (rb *RingBuffer) Query(f Filter) []Entry {
	all := rb.Entries()

	// Apply filters
	var filtered []Entry
	for _, e := range all {
		if !matchFilter(e, f) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Sort
	switch f.Sort {
	case "duration":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].DurationMs > filtered[j].DurationMs
		})
	case "status":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Status > filtered[j].Status
		})
	default:
		// Newest first (reverse chronological)
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}

	// Limit
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > len(filtered) {
		limit = len(filtered)
	}
	return filtered[:limit]
}

func matchFilter(e Entry, f Filter) bool {
	// Status group filter
	if len(f.StatusGroups) > 0 {
		group := statusGroup(e.Status)
		matched := false
		for _, g := range f.StatusGroups {
			if g == group {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Method filter
	if len(f.Methods) > 0 {
		matched := false
		for _, m := range f.Methods {
			if strings.EqualFold(m, e.Method) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Route prefix filter
	if f.Route != "" && !strings.HasPrefix(e.Path, f.Route) {
		return false
	}

	return true
}

func statusGroup(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}

// FindByRequestID returns the entry with the given request ID, or nil.
func (rb *RingBuffer) FindByRequestID(id string) *Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	for i := range rb.entries {
		if rb.entries[i].RequestID == id {
			e := rb.entries[i]
			return &e
		}
	}
	return nil
}
