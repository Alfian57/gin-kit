package devtools

import (
	"context"
	"sync"
	"time"

	"github.com/Alfian57/gin-kit/runtime/mail"
)

// MailEntry is one message captured by the devtools outbox, successful or
// not. The embedded envelope carries metadata and bodies; attachment content
// is never captured.
type MailEntry struct {
	ID     int64     `json:"id"`
	Time   time.Time `json:"time"`
	Status string    `json:"status"` // "sent" or "failed"
	Error  string    `json:"error,omitempty"`
	mail.Envelope
}

// mailRing is a fixed-capacity concurrent outbox that keeps the newest
// entries and assigns each one a monotonically increasing ID.
type mailRing struct {
	mu      sync.RWMutex
	entries []MailEntry
	next    int64
}

func newMailRing(capacity int) *mailRing {
	return &mailRing{entries: make([]MailEntry, capacity)}
}

func (r *mailRing) Add(entry MailEntry) {
	r.mu.Lock()
	r.next++
	entry.ID = r.next
	r.entries[int((r.next-1)%int64(len(r.entries)))] = entry
	r.mu.Unlock()
}

// Snapshot copies the retained entries, newest first.
func (r *mailRing) Snapshot() []MailEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := int(min(r.next, int64(len(r.entries))))
	snapshot := make([]MailEntry, 0, count)
	for i := range count {
		snapshot = append(snapshot, r.entries[int((r.next-1-int64(i))%int64(len(r.entries)))])
	}
	return snapshot
}

// Find returns the retained entry with the given ID.
func (r *mailRing) Find(id int64) (MailEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id < 1 || id > r.next || id <= r.next-int64(len(r.entries)) {
		return MailEntry{}, false
	}
	return r.entries[int((id-1)%int64(len(r.entries)))], true
}

// recordingMailer decorates a Mailer, recording every send — success and
// failure alike — into the devtools outbox before returning the underlying
// result unchanged.
type recordingMailer struct {
	next   mail.Mailer
	outbox *mailRing
}

func (m *recordingMailer) Send(ctx context.Context, message *mail.Message) error {
	err := m.next.Send(ctx, message)
	entry := MailEntry{Time: time.Now(), Status: "sent", Envelope: message.Envelope()}
	if err != nil {
		entry.Status = "failed"
		entry.Error = err.Error()
	}
	m.outbox.Add(entry)
	return err
}

func (m *recordingMailer) Close() error { return m.next.Close() }
