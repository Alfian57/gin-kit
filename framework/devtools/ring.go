package devtools

import (
	"sync"
	"time"
)

// RequestEntry is one completed request in the devtools log. It deliberately
// stores no request or response bodies, no headers beyond the user agent,
// and never the query string.
type RequestEntry struct {
	ID         int64     `json:"id"`
	Time       time.Time `json:"time"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	RequestID  string    `json:"request_id"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	ErrorCode  string    `json:"error_code,omitempty"`
}

// requestRing is a fixed-capacity concurrent buffer that keeps the newest
// entries and assigns each one a monotonically increasing ID.
type requestRing struct {
	mu      sync.RWMutex
	entries []RequestEntry
	next    int64
}

func newRequestRing(capacity int) *requestRing {
	return &requestRing{entries: make([]RequestEntry, capacity)}
}

func (r *requestRing) Add(entry RequestEntry) {
	r.mu.Lock()
	r.next++
	entry.ID = r.next
	r.entries[int((r.next-1)%int64(len(r.entries)))] = entry
	r.mu.Unlock()
}

// Snapshot copies the retained entries, newest first.
func (r *requestRing) Snapshot() []RequestEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := int(min(r.next, int64(len(r.entries))))
	snapshot := make([]RequestEntry, 0, count)
	for i := range count {
		snapshot = append(snapshot, r.entries[int((r.next-1-int64(i))%int64(len(r.entries)))])
	}
	return snapshot
}
