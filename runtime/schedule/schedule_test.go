package schedule

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// testScheduler performs this package operation.
func testScheduler() *Scheduler {
	return New(Options{Logger: slog.New(slog.DiscardHandler), StopTimeout: time.Second})
}

func TestValidationErrors(t *testing.T) {
	scheduler := testScheduler()
	if err := scheduler.Cron("not a spec", "job", func(context.Context) error { return nil }); err == nil {
		t.Fatal("invalid cron spec accepted")
	}
	if err := scheduler.Cron("* * * * *", "", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty name accepted")
	}
	if err := scheduler.Every(time.Millisecond, "fast", func(context.Context) error { return nil }); err == nil {
		t.Fatal("sub-second interval accepted")
	}
}

func TestDailyAndHourlyRegister(t *testing.T) {
	scheduler := testScheduler()
	if err := scheduler.Daily("daily", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Hourly("hourly", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if entries := len(scheduler.cron.Entries()); entries != 2 {
		t.Fatalf("expected 2 entries, got %d", entries)
	}
}

// fastSchedule fires every few milliseconds so lifecycle tests stay quick.
type fastSchedule struct {
	// interval is added to the supplied time on every Next call.
	interval time.Duration
}

// Next performs this package operation.
func (s fastSchedule) Next(t time.Time) time.Time { return t.Add(s.interval) }

func TestRunExecutesJobsAndStopsOnCancel(t *testing.T) {
	scheduler := testScheduler()
	var runs atomic.Int64
	scheduler.schedule(fastSchedule{5 * time.Millisecond}, "ticker", func(context.Context) error {
		runs.Add(1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- scheduler.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for runs.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runs.Load() < 2 {
		t.Fatal("scheduled job did not run")
	}
	cancel()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	settled := runs.Load()
	time.Sleep(30 * time.Millisecond)
	if runs.Load() != settled {
		t.Fatal("jobs kept running after stop")
	}
}

func TestPanickingJobDoesNotStopScheduler(t *testing.T) {
	scheduler := testScheduler()
	var healthyRuns atomic.Int64
	scheduler.schedule(fastSchedule{5 * time.Millisecond}, "panicky", func(context.Context) error {
		panic("boom")
	})
	scheduler.schedule(fastSchedule{5 * time.Millisecond}, "healthy", func(context.Context) error {
		healthyRuns.Add(1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- scheduler.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for healthyRuns.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-finished
	if healthyRuns.Load() < 3 {
		t.Fatal("panicking job disrupted the scheduler")
	}
}

func TestSkipIfRunningPreventsOverlap(t *testing.T) {
	scheduler := testScheduler()
	var concurrent, peak atomic.Int64
	scheduler.schedule(fastSchedule{5 * time.Millisecond}, "slow", func(context.Context) error {
		now := concurrent.Add(1)
		if now > peak.Load() {
			peak.Store(now)
		}
		time.Sleep(40 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, SkipIfRunning())
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- scheduler.Run(ctx) }()
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-finished
	if peak.Load() > 1 {
		t.Fatalf("job overlapped despite SkipIfRunning: peak=%d", peak.Load())
	}
}

func TestJobReceivesRunContext(t *testing.T) {
	scheduler := testScheduler()
	observed := make(chan struct{}, 1)
	scheduler.schedule(fastSchedule{5 * time.Millisecond}, "ctx", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			t.Error("job context was never canceled")
		}
		select {
		case observed <- struct{}{}:
		default:
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- scheduler.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-finished
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("job never observed the run context")
	}
}
