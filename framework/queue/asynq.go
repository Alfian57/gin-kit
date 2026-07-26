package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// asynqDriver backs the queue with Redis through asynq: persistent jobs,
// exponential-backoff retries, delayed execution, and graceful drain.
type asynqDriver struct {
	client          *asynq.Client
	redis           asynq.RedisConnOpt
	concurrency     int
	queues          map[string]int
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

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

func (d *asynqDriver) close() error { return d.client.Close() }

// slogAsynqAdapter keeps asynq's logs inside the application's structured
// logging discipline.
type slogAsynqAdapter struct {
	logger *slog.Logger
}

func (a *slogAsynqAdapter) Debug(args ...any) { a.logger.Debug(fmt.Sprint(args...)) }
func (a *slogAsynqAdapter) Info(args ...any)  { a.logger.Info(fmt.Sprint(args...)) }
func (a *slogAsynqAdapter) Warn(args ...any)  { a.logger.Warn(fmt.Sprint(args...)) }
func (a *slogAsynqAdapter) Error(args ...any) { a.logger.Error(fmt.Sprint(args...)) }
func (a *slogAsynqAdapter) Fatal(args ...any) { a.logger.Error(fmt.Sprint(args...)) }
