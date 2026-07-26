package queue

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestAsynqIntegration exercises the redis driver end to end. It requires a
// real Redis server because asynq relies on Lua scripting and blocking
// semantics that in-memory fakes do not faithfully emulate.
func TestAsynqIntegration(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("set REDIS_TEST_URL (e.g. redis://127.0.0.1:6379/9) to run redis integration tests")
	}
	q, err := New(Options{Driver: "redis", RedisURL: redisURL, Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	handled := make(chan string, 1)
	Register(q, "integration.ping", func(_ context.Context, payload emailPayload) error {
		handled <- payload.To
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- q.Start(ctx) }()

	if err := Dispatch(context.Background(), q, "integration.ping", emailPayload{To: "worker@example.com"}); err != nil {
		t.Fatal(err)
	}
	select {
	case to := <-handled:
		if to != "worker@example.com" {
			t.Fatalf("wrong payload: %q", to)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("job was not processed in time")
	}
	cancel()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestAsynqEnqueueRejectsUnknownQueue(t *testing.T) {
	q, err := New(Options{Driver: "redis", RedisURL: "redis://127.0.0.1:1/0"})
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	err = Dispatch(context.Background(), q, "job", emailPayload{}, OnQueue("missing"))
	if err == nil {
		t.Fatal("dispatch to unconfigured queue accepted")
	}
}
