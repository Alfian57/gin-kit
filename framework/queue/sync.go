package queue

import (
	"context"
	"fmt"
)

// syncDriver executes jobs inline at dispatch time, for development and tests.
type syncDriver struct {
	queue *Queue
}

func (d *syncDriver) enqueue(ctx context.Context, name string, payload []byte, _ jobOptions) error {
	handler, ok := d.queue.handlers[name]
	if !ok {
		return fmt.Errorf("queue: no handler registered for %q", name)
	}
	return handler(ctx, payload)
}

func (d *syncDriver) run(ctx context.Context, _ map[string]HandlerFunc) error {
	<-ctx.Done()
	return nil
}

func (d *syncDriver) close() error { return nil }
