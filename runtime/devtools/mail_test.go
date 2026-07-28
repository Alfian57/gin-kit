package devtools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/mail"
)

// stubMailer succeeds every send without touching a transport.
type stubMailer struct{}

// Send performs this package operation.
func (stubMailer) Send(context.Context, *mail.Message) error { return nil }

// Close performs this package operation.
func (stubMailer) Close() error { return nil }

// failingMailer fails every send with a fixed error.
type failingMailer struct {
	// err is returned by each Send call.
	err error
}

// Send performs this package operation.
func (m failingMailer) Send(context.Context, *mail.Message) error { return m.err }

// Close performs this package operation.
func (failingMailer) Close() error { return nil }

func TestWrapMailerRecordsSentMessages(t *testing.T) {
	d := New(Options{})
	mailer := d.WrapMailer(stubMailer{})
	message := mail.NewMessage().
		To("to@example.com").Cc("cc@example.com").
		Subject("Welcome").Text("hello").
		Attach("notes.txt", "text/plain", strings.NewReader("attached content"))
	if err := mailer.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	entries := d.outbox.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("outbox length = %d", len(entries))
	}
	entry := entries[0]
	if entry.Status != "sent" || entry.Error != "" || entry.ID != 1 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Subject != "Welcome" || entry.To[0] != "to@example.com" || entry.Cc[0] != "cc@example.com" {
		t.Fatalf("envelope not captured: %+v", entry.Envelope)
	}
	if len(entry.Attachments) != 1 || entry.Attachments[0].Filename != "notes.txt" || entry.Attachments[0].Size != len("attached content") {
		t.Fatalf("attachment metadata wrong: %+v", entry.Attachments)
	}
}

func TestWrapMailerRecordsFailuresWithTheError(t *testing.T) {
	d := New(Options{})
	sentinel := errors.New("smtp offline")
	mailer := d.WrapMailer(failingMailer{err: sentinel})
	err := mailer.Send(context.Background(), mail.NewMessage().To("to@example.com").Text("hi"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("underlying error not returned: %v", err)
	}
	entries := d.outbox.Snapshot()
	if len(entries) != 1 || entries[0].Status != "failed" || entries[0].Error != "smtp offline" {
		t.Fatalf("failure not recorded: %+v", entries)
	}
}

func TestWrapMailerRespectsTheOutboxCap(t *testing.T) {
	d := New(Options{MaxMails: 5})
	mailer := d.WrapMailer(stubMailer{})
	for index := 1; index <= 8; index++ {
		message := mail.NewMessage().To("to@example.com").Subject(fmt.Sprintf("m%d", index)).Text("x")
		if err := mailer.Send(context.Background(), message); err != nil {
			t.Fatal(err)
		}
	}
	entries := d.outbox.Snapshot()
	if len(entries) != 5 {
		t.Fatalf("outbox length = %d, want 5", len(entries))
	}
	if entries[0].Subject != "m8" || entries[4].Subject != "m4" {
		t.Fatalf("outbox not newest-first: first=%s last=%s", entries[0].Subject, entries[4].Subject)
	}
	if _, found := d.outbox.Find(3); found {
		t.Fatal("evicted entry still findable")
	}
	if entry, found := d.outbox.Find(8); !found || entry.Subject != "m8" {
		t.Fatalf("newest entry not findable: %+v found=%v", entry, found)
	}
}
