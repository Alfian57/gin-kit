// Package events provides a dependency-free, in-process, typed event bus.
// Subscriptions are keyed by the event's Go type; there is no scanning,
// injection, or implicit wiring — handlers are registered explicitly with On.
//
// Dispatch is synchronous. For asynchronous handling, subscribe a handler
// that dispatches a queue job and return immediately.
package events

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// subscription defines an implementation type used by this package.
type subscription struct {
	// id store data used by this type.
	id uint64
	// call store data used by this type.
	call func(context.Context, any) error
}

// Bus routes emitted events to subscribers of the same event type.
type Bus struct {
	// mu store data used by this type.
	mu sync.RWMutex
	// nextID store data used by this type.
	nextID uint64
	// handlers store data used by this type.
	handlers map[reflect.Type][]subscription
}

// NewBus performs this package operation.
func NewBus() *Bus {
	return &Bus{handlers: make(map[reflect.Type][]subscription)}
}

// On subscribes handler to events of type T and returns an idempotent
// unsubscribe function. Handlers run in registration order.
func On[T any](bus *Bus, handler func(context.Context, T) error) (unsubscribe func()) {
	key := reflect.TypeOf((*T)(nil)).Elem()
	bus.mu.Lock()
	bus.nextID++
	id := bus.nextID
	bus.handlers[key] = append(bus.handlers[key], subscription{
		id: id,
		call: func(ctx context.Context, event any) error {
			return handler(ctx, event.(T))
		},
	})
	bus.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			bus.mu.Lock()
			defer bus.mu.Unlock()
			current := bus.handlers[key]
			for index, item := range current {
				if item.id == id {
					bus.handlers[key] = append(current[:index:index], current[index+1:]...)
					break
				}
			}
		})
	}
}

// Emit dispatches event synchronously to every subscriber of exactly type T,
// in registration order, and joins their errors. Handler panics become
// errors, and later handlers still run. Emitting with no subscribers is not
// an error.
func Emit[T any](ctx context.Context, bus *Bus, event T) error {
	key := reflect.TypeOf((*T)(nil)).Elem()
	bus.mu.RLock()
	snapshot := append([]subscription(nil), bus.handlers[key]...)
	bus.mu.RUnlock()

	var result error
	for _, item := range snapshot {
		result = errors.Join(result, safeCall(ctx, item, event))
	}
	return result
}

// safeCall performs this package operation.
func safeCall[T any](ctx context.Context, item subscription, event T) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("events: handler panic: %v", recovered)
		}
	}()
	return item.call(ctx, event)
}
