package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/hibiken/asynq"
)

// asynqDriver backs the queue with Redis through asynq: persistent jobs,
// exponential-backoff retries, delayed execution, and graceful drain.
type asynqDriver struct {
	// client store data used by this type.
	client *asynq.Client
	// redis store data used by this type.
	redis asynq.RedisConnOpt
	// concurrency store data used by this type.
	concurrency int
	// queues store data used by this type.
	queues map[string]int
	// shutdownTimeout store data used by this type.
	shutdownTimeout time.Duration
	// logger store data used by this type.
	logger *slog.Logger
	// inspectorOnce store data used by this type.
	inspectorOnce sync.Once
	// inspector store data used by this type.
	inspector *asynq.Inspector
}

// newAsynqDriver performs this package operation.
func newAsynqDriver(options Options) (*asynqDriver, error) {
	redisOpt, err := asynq.ParseRedisURI(options.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("queue: invalid Redis URL: %w", err)
	}
	return &asynqDriver{
		client:          asynq.NewClient(redisOpt),
		redis:           redisOpt,
		concurrency:     options.Concurrency,
		queues:          options.Queues,
		shutdownTimeout: options.ShutdownTimeout,
		logger:          options.Logger,
	}, nil
}

// enqueue performs this package operation.
func (d *asynqDriver) enqueue(ctx context.Context, name string, payload []byte, options jobOptions) error {
	if _, ok := d.queues[options.queue]; !ok {
		return fmt.Errorf("queue: %q is not a configured queue", options.queue)
	}
	taskOptions := []asynq.Option{asynq.Queue(options.queue)}
	if options.delay > 0 {
		taskOptions = append(taskOptions, asynq.ProcessIn(options.delay))
	}
	if options.maxRetry != nil {
		taskOptions = append(taskOptions, asynq.MaxRetry(*options.maxRetry))
	}
	_, err := d.client.EnqueueContext(ctx, asynq.NewTask(name, payload), taskOptions...)
	return err
}

// run performs this package operation.
func (d *asynqDriver) run(ctx context.Context, handlers map[string]HandlerFunc) error {
	mux := asynq.NewServeMux()
	for name, handler := range handlers {
		mux.HandleFunc(name, func(ctx context.Context, task *asynq.Task) error {
			return handler(ctx, task.Payload())
		})
	}
	server := asynq.NewServer(d.redis, asynq.Config{
		Concurrency:     d.concurrency,
		Queues:          d.queues,
		ShutdownTimeout: d.shutdownTimeout,
		Logger:          &slogAsynqAdapter{logger: d.logger},
	})
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("queue: start worker: %w", err)
	}
	<-ctx.Done()
	server.Shutdown()
	return nil
}

// stats inspects the broker. The inspector opens its own Redis connection,
// so it is created lazily on the first call and closed with the driver.
func (d *asynqDriver) stats(ctx context.Context) (Stats, error) {
	d.inspectorOnce.Do(func() { d.inspector = asynq.NewInspector(d.redis) })
	names, err := d.inspector.Queues()
	if err != nil {
		return Stats{}, fmt.Errorf("queue: list queues: %w", err)
	}
	sort.Strings(names)
	result := Stats{Driver: "redis"}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
		info, err := d.inspector.GetQueueInfo(name)
		if err != nil {
			return Stats{}, fmt.Errorf("queue: inspect queue %q: %w", name, err)
		}
		result.Queues = append(result.Queues, QueueStats{
			Name:      name,
			Pending:   info.Pending,
			Active:    info.Active,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
		})
	}
	return result, nil
}

// close performs this package operation.
func (d *asynqDriver) close() error {
	err := d.client.Close()
	// Synchronize with a concurrent lazy creation before reading the field.
	d.inspectorOnce.Do(func() {})
	if d.inspector != nil {
		err = errors.Join(err, d.inspector.Close())
	}
	return err
}

// slogAsynqAdapter keeps asynq's logs inside the application's structured
// logging discipline.
type slogAsynqAdapter struct {
	// logger store data used by this type.
	logger *slog.Logger
}

// Debug performs this package operation.
func (a *slogAsynqAdapter) Debug(args ...any) { a.logger.Debug(fmt.Sprint(args...)) }

// Info performs this package operation.
func (a *slogAsynqAdapter) Info(args ...any) { a.logger.Info(fmt.Sprint(args...)) }

// Warn performs this package operation.
func (a *slogAsynqAdapter) Warn(args ...any) { a.logger.Warn(fmt.Sprint(args...)) }

// Error performs this package operation.
func (a *slogAsynqAdapter) Error(args ...any) { a.logger.Error(fmt.Sprint(args...)) }

// Fatal performs this package operation.
func (a *slogAsynqAdapter) Fatal(args ...any) { a.logger.Error(fmt.Sprint(args...)) }
