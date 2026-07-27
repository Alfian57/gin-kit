// Package queue provides explicit background jobs with typed handler
// registration, an inline sync driver for development, and a Redis (asynq)
// driver for production with retries, delays, and graceful drain.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// HandlerFunc processes one job payload.
type HandlerFunc func(ctx context.Context, payload []byte) error

type Options struct {
	// Driver selects the backend: "sync" (default, executes jobs inline) or
	// "redis" (asynq worker with retries and graceful drain).
	Driver string
	// RedisURL configures the redis driver, e.g. redis://localhost:6379/0.
	RedisURL string
	// Concurrency is the redis worker goroutine count, defaulting to 10.
	Concurrency int
	// Queues maps queue names to priorities; defaults to {"default": 1}.
	Queues map[string]int
	// ShutdownTimeout bounds the in-flight drain on stop, defaulting to 8s.
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

type jobOptions struct {
	delay    time.Duration
	queue    string
	maxRetry *int
}

// Option customizes a single dispatch.
type Option func(*jobOptions)

// Delay schedules the job to run after d. Ignored by the sync driver.
func Delay(d time.Duration) Option { return func(o *jobOptions) { o.delay = d } }

// OnQueue routes the job to a named queue listed in Options.Queues.
func OnQueue(name string) Option { return func(o *jobOptions) { o.queue = name } }

// MaxRetry caps redis-driver retries for the job. Ignored by the sync driver.
func MaxRetry(n int) Option { return func(o *jobOptions) { o.maxRetry = &n } }

type driver interface {
	enqueue(ctx context.Context, name string, payload []byte, options jobOptions) error
	run(ctx context.Context, handlers map[string]HandlerFunc) error
	stats(ctx context.Context) (Stats, error)
	close() error
}

// QueueStats reports the task counts of one named queue.
type QueueStats struct {
	Name      string `json:"name"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
}

// Stats reports the driver backing the queue and per-queue task counts. The
// sync driver executes jobs inline, so it reports no queues.
type Stats struct {
	Driver string       `json:"driver"`
	Queues []QueueStats `json:"queues,omitempty"`
}

// Queue dispatches and processes background jobs.
type Queue struct {
	driver   driver
	handlers map[string]HandlerFunc
	logger   *slog.Logger
}

func New(options Options) (*Queue, error) {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Concurrency == 0 {
		options.Concurrency = 10
	}
	if options.Concurrency < 1 {
		return nil, fmt.Errorf("queue: concurrency must be positive, got %d", options.Concurrency)
	}
	if len(options.Queues) == 0 {
		options.Queues = map[string]int{"default": 1}
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 8 * time.Second
	}
	q := &Queue{handlers: make(map[string]HandlerFunc), logger: options.Logger}
	switch options.Driver {
	case "", "sync":
		q.driver = &syncDriver{queue: q}
	case "redis":
		if options.RedisURL == "" {
			return nil, fmt.Errorf("queue: the redis driver requires a Redis URL")
		}
		redisDriver, err := newAsynqDriver(options)
		if err != nil {
			return nil, err
		}
		q.driver = redisDriver
	default:
		return nil, fmt.Errorf("queue: unknown driver %q", options.Driver)
	}
	return q, nil
}

// Handle registers a raw payload handler for the named job. Register every
// handler before Start; later registrations are not picked up by a running
// worker.
func (q *Queue) Handle(name string, handler HandlerFunc) {
	if name == "" || handler == nil {
		return
	}
	q.handlers[name] = handler
}

// Register binds a typed handler; payloads are JSON-decoded into T before the
// handler runs, and undecodable payloads fail the job.
func Register[T any](q *Queue, name string, handler func(context.Context, T) error) {
	q.Handle(name, func(ctx context.Context, payload []byte) error {
		var value T
		if err := json.Unmarshal(payload, &value); err != nil {
			return fmt.Errorf("queue: decode %s payload: %w", name, err)
		}
		return handler(ctx, value)
	})
}

// Dispatch JSON-encodes payload and enqueues the named job. The sync driver
// executes the handler inline and returns its error directly.
func Dispatch[T any](ctx context.Context, q *Queue, name string, payload T, opts ...Option) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: encode %s payload: %w", name, err)
	}
	options := jobOptions{queue: "default"}
	for _, opt := range opts {
		opt(&options)
	}
	return q.driver.enqueue(ctx, name, encoded, options)
}

// Start runs the worker until ctx is canceled, then drains in-flight jobs.
// It is safe to use as an application runner (app.Go) or from a dedicated
// worker binary. The sync driver simply blocks until cancellation.
func (q *Queue) Start(ctx context.Context) error {
	handlers := make(map[string]HandlerFunc, len(q.handlers))
	for name, handler := range q.handlers {
		handlers[name] = handler
	}
	return q.driver.run(ctx, handlers)
}

// Stats reports the queue backend and its per-queue task counts. The redis
// driver inspects the broker; the sync driver reports only its name.
func (q *Queue) Stats(ctx context.Context) (Stats, error) { return q.driver.stats(ctx) }

// Close releases driver resources. It is safe to call more than once.
func (q *Queue) Close() error { return q.driver.close() }
