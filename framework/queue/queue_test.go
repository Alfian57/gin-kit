package queue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type emailPayload struct {
	To string `json:"to"`
}

func TestSyncDriverDispatchRoundTrip(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	var handled []string
	Register(q, "emails.welcome", func(_ context.Context, payload emailPayload) error {
		handled = append(handled, payload.To)
		return nil
	})
	if err := Dispatch(context.Background(), q, "emails.welcome", emailPayload{To: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	if len(handled) != 1 || handled[0] != "a@b.c" {
		t.Fatalf("job not handled inline: %v", handled)
	}
}

func TestSyncDriverPropagatesHandlerError(t *testing.T) {
	q, err := New(Options{Driver: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("smtp offline")
	Register(q, "emails.welcome", func(context.Context, emailPayload) error { return sentinel })
	if err := Dispatch(context.Background(), q, "emails.welcome", emailPayload{}); !errors.Is(err, sentinel) {
		t.Fatalf("handler error not propagated: %v", err)
	}
}

func TestSyncDriverRejectsUnknownJob(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	err = Dispatch(context.Background(), q, "missing.job", emailPayload{})
	if err == nil || !strings.Contains(err.Error(), "no handler registered") {
		t.Fatalf("unknown job accepted: %v", err)
	}
}

func TestRegisterRejectsMalformedPayload(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	Register(q, "typed", func(context.Context, emailPayload) error { return nil })
	if err := q.driver.enqueue(context.Background(), "typed", []byte("{broken"), jobOptions{queue: "default"}); err == nil {
		t.Fatal("malformed payload accepted")
	}
}

func TestSyncDriverIgnoresDispatchOptions(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	var handled bool
	Register(q, "job", func(context.Context, emailPayload) error {
		handled = true
		return nil
	})
	err = Dispatch(context.Background(), q, "job", emailPayload{},
		Delay(time.Hour), OnQueue("high"), MaxRetry(0))
	if err != nil || !handled {
		t.Fatalf("options should be ignored inline: handled=%v err=%v", handled, err)
	}
}

func TestStartBlocksUntilCancelAndCloseIsIdempotent(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- q.Start(ctx) }()
	select {
	case err := <-finished:
		t.Fatalf("Start returned before cancel: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewValidatesOptions(t *testing.T) {
	for _, test := range []struct {
		name    string
		options Options
	}{
		{"unknown driver", Options{Driver: "kafka"}},
		{"redis without url", Options{Driver: "redis"}},
		{"redis with bad url", Options{Driver: "redis", RedisURL: "http://wrong"}},
		{"negative concurrency", Options{Concurrency: -1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.options); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
}

func TestDispatchOptionsPopulateJobOptions(t *testing.T) {
	options := jobOptions{queue: "default"}
	for _, opt := range []Option{Delay(5 * time.Second), OnQueue("high"), MaxRetry(3)} {
		opt(&options)
	}
	if options.delay != 5*time.Second || options.queue != "high" || options.maxRetry == nil || *options.maxRetry != 3 {
		t.Fatalf("job options not populated: %+v", options)
	}
}
