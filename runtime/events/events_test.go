package events

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// userRegistered defines an implementation type used by this package.
type userRegistered struct {
	// Email store data used by this type.
	Email string
}

// orderPlaced defines an implementation type used by this package.
type orderPlaced struct {
	// ID store data used by this type.
	ID string
}

func TestEmitDeliversToTypedSubscribersInOrder(t *testing.T) {
	bus := NewBus()
	var order []string
	On(bus, func(_ context.Context, event userRegistered) error {
		order = append(order, "first:"+event.Email)
		return nil
	})
	On(bus, func(_ context.Context, event userRegistered) error {
		order = append(order, "second:"+event.Email)
		return nil
	})
	On(bus, func(_ context.Context, event orderPlaced) error {
		order = append(order, "wrong-type")
		return nil
	})
	if err := Emit(context.Background(), bus, userRegistered{Email: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "first:a@b.c" || order[1] != "second:a@b.c" {
		t.Fatalf("delivery order wrong: %v", order)
	}
}

func TestEmitJoinsErrorsAndRecoversPanics(t *testing.T) {
	bus := NewBus()
	first := errors.New("first failed")
	On(bus, func(context.Context, userRegistered) error { return first })
	On(bus, func(context.Context, userRegistered) error { panic("second exploded") })
	var thirdRan bool
	On(bus, func(context.Context, userRegistered) error {
		thirdRan = true
		return nil
	})
	err := Emit(context.Background(), bus, userRegistered{})
	if !errors.Is(err, first) || !strings.Contains(err.Error(), "second exploded") {
		t.Fatalf("errors not joined: %v", err)
	}
	if !thirdRan {
		t.Fatal("handler after panic did not run")
	}
}

func TestUnsubscribeStopsDeliveryAndIsIdempotent(t *testing.T) {
	bus := NewBus()
	var calls int
	unsubscribe := On(bus, func(context.Context, userRegistered) error {
		calls++
		return nil
	})
	_ = Emit(context.Background(), bus, userRegistered{})
	unsubscribe()
	unsubscribe()
	_ = Emit(context.Background(), bus, userRegistered{})
	if calls != 1 {
		t.Fatalf("unsubscribe did not stop delivery: calls=%d", calls)
	}
}

func TestEmitWithoutSubscribersIsNil(t *testing.T) {
	if err := Emit(context.Background(), NewBus(), orderPlaced{ID: "1"}); err != nil {
		t.Fatal(err)
	}
}

func TestPointerAndValueTypesAreIsolated(t *testing.T) {
	bus := NewBus()
	var valueCalls, pointerCalls int
	On(bus, func(context.Context, userRegistered) error {
		valueCalls++
		return nil
	})
	On(bus, func(context.Context, *userRegistered) error {
		pointerCalls++
		return nil
	})
	_ = Emit(context.Background(), bus, userRegistered{})
	_ = Emit(context.Background(), bus, &userRegistered{})
	if valueCalls != 1 || pointerCalls != 1 {
		t.Fatalf("type isolation broken: value=%d pointer=%d", valueCalls, pointerCalls)
	}
}

func TestConcurrentSubscribeEmitUnsubscribe(t *testing.T) {
	bus := NewBus()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				unsubscribe := On(bus, func(context.Context, userRegistered) error { return nil })
				_ = Emit(context.Background(), bus, userRegistered{})
				unsubscribe()
			}
		}()
	}
	wg.Wait()
}
