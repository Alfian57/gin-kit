package queue

import (
	"context"
	"fmt"
)

// syncDriver executes jobs inline at dispatch time, for development and tests.
type syncDriver struct {
	// queue store data used by this type.
	queue *Queue
}

// enqueue performs this package operation.
func (d *syncDriver) enqueue(ctx context.Context, name string, payload []byte, _ jobOptions) error {
	handler, ok := d.queue.handlers[name]
	if !ok {
		return fmt.Errorf("queue: no handler registered for %q", name)
	}
	return handler(ctx, payload)
}

// run performs this package operation.
func (d *syncDriver) run(ctx context.Context, _ map[string]HandlerFunc) error {
	<-ctx.Done()
	return nil
}

// stats performs this package operation.
func (d *syncDriver) stats(context.Context) (Stats, error) { return Stats{Driver: "sync"}, nil }

// close performs this package operation.
func (d *syncDriver) close() error { return nil }
