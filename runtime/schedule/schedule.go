// Package schedule provides cron-style task scheduling on robfig/cron with
// per-job panic recovery, optional overlap skipping, and graceful stop as an
// application runner.
//
// The scheduler is single-instance in this version: running N replicas
// executes every job N times. Run it in one instance (or a dedicated binary)
// when deploying multiple replicas.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// Options defines an implementation type used by this package.
type Options struct {
	Logger   *slog.Logger   // defaults to slog.Default()
	Location *time.Location // defaults to time.Local
	// StopTimeout bounds the wait for running jobs on stop, defaulting to 10s.
	StopTimeout time.Duration
}

// jobConfig defines an implementation type used by this package.
type jobConfig struct {
	// skipIfRunning store data used by this type.
	skipIfRunning bool
}

// JobOption customizes one scheduled job.
type JobOption func(*jobConfig)

// SkipIfRunning skips a tick while the previous run of the same job is still
// executing. Overlapping runs are allowed by default.
func SkipIfRunning() JobOption { return func(c *jobConfig) { c.skipIfRunning = true } }

// Scheduler runs named jobs on cron schedules.
type Scheduler struct {
	// cron store data used by this type.
	cron *cron.Cron
	// logger store data used by this type.
	logger *slog.Logger
	// stopTimeout store data used by this type.
	stopTimeout time.Duration
	runCtx      atomic.Value // ctxHolder
}

// ctxHolder gives atomic.Value one consistent concrete type regardless of the
// stored context's implementation.
type ctxHolder struct {
	// ctx store data used by this type.
	ctx context.Context
}

// New performs this package operation.
func New(options Options) *Scheduler {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Location == nil {
		options.Location = time.Local
	}
	if options.StopTimeout <= 0 {
		options.StopTimeout = 10 * time.Second
	}
	scheduler := &Scheduler{
		logger:      options.Logger,
		stopTimeout: options.StopTimeout,
	}
	scheduler.runCtx.Store(ctxHolder{ctx: context.Background()})
	scheduler.cron = cron.New(cron.WithLocation(options.Location))
	return scheduler
}

// Cron schedules job with a five-field cron spec such as "*/5 * * * *". The
// spec is validated immediately. A failing or panicking job is logged and
// never stops the scheduler or the application.
func (s *Scheduler) Cron(spec, name string, job func(context.Context) error, opts ...JobOption) error {
	if name == "" {
		return errors.New("schedule: job name must not be empty")
	}
	parsed, err := cron.ParseStandard(spec)
	if err != nil {
		return fmt.Errorf("schedule: invalid cron spec %q: %w", spec, err)
	}
	s.schedule(parsed, name, job, opts...)
	return nil
}

// Every schedules job at a fixed interval of at least one second.
func (s *Scheduler) Every(interval time.Duration, name string, job func(context.Context) error, opts ...JobOption) error {
	if name == "" {
		return errors.New("schedule: job name must not be empty")
	}
	if interval < time.Second {
		return fmt.Errorf("schedule: interval must be at least one second, got %s", interval)
	}
	s.schedule(cron.Every(interval), name, job, opts...)
	return nil
}

// Daily schedules job at midnight in the scheduler's location.
func (s *Scheduler) Daily(name string, job func(context.Context) error, opts ...JobOption) error {
	return s.Cron("0 0 * * *", name, job, opts...)
}

// Hourly schedules job at the top of every hour.
func (s *Scheduler) Hourly(name string, job func(context.Context) error, opts ...JobOption) error {
	return s.Cron("0 * * * *", name, job, opts...)
}

// schedule performs this package operation.
func (s *Scheduler) schedule(when cron.Schedule, name string, job func(context.Context) error, opts ...JobOption) {
	config := jobConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	var wrapped cron.Job = cron.FuncJob(func() { s.runJob(name, job) })
	if config.skipIfRunning {
		wrapped = cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(wrapped)
	}
	s.cron.Schedule(when, wrapped)
}

// runJob performs this package operation.
func (s *Scheduler) runJob(name string, job func(context.Context) error) {
	ctx := s.runCtx.Load().(ctxHolder).ctx
	started := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.Error("scheduled job panicked", "job", name, "panic", recovered)
		}
	}()
	if err := job(ctx); err != nil {
		s.logger.Error("scheduled job failed", "job", name, "error", err, "duration_ms", time.Since(started).Milliseconds())
		return
	}
	s.logger.Debug("scheduled job completed", "job", name, "duration_ms", time.Since(started).Milliseconds())
}

// Run starts the scheduler and blocks until ctx is canceled, then stops
// scheduling and waits up to StopTimeout for running jobs. Intended for
// app.Go("scheduler", scheduler.Run).
func (s *Scheduler) Run(ctx context.Context) error {
	s.runCtx.Store(ctxHolder{ctx: ctx})
	s.cron.Start()
	<-ctx.Done()
	stopped := s.cron.Stop()
	select {
	case <-stopped.Done():
	case <-time.After(s.stopTimeout):
		s.logger.Warn("scheduled jobs did not finish before the stop timeout")
	}
	return nil
}
