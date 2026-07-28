package cache

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()
	current := time.Unix(1000, 0)
	store.now = func() time.Time { return current }

	if _, ok, err := store.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("miss expected: ok=%v err=%v", ok, err)
	}
	if err := store.Set(ctx, "key", "value", 0); err != nil {
		t.Fatal(err)
	}
	if value, ok, _ := store.Get(ctx, "key"); !ok || value != "value" {
		t.Fatalf("hit expected: %q %v", value, ok)
	}
	if err := store.Set(ctx, "key", "updated", 0); err != nil {
		t.Fatal(err)
	}
	if value, _, _ := store.Get(ctx, "key"); value != "updated" {
		t.Fatalf("overwrite failed: %q", value)
	}

	if err := store.Set(ctx, "expiring", "gone", time.Minute); err != nil {
		t.Fatal(err)
	}
	current = current.Add(2 * time.Minute)
	if _, ok, _ := store.Get(ctx, "expiring"); ok {
		t.Fatal("expired entry still returned")
	}

	if err := store.Forget(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if err := store.Forget(ctx, "key"); err != nil {
		t.Fatal("forget must be idempotent")
	}
	if _, ok, _ := store.Get(ctx, "key"); ok {
		t.Fatal("forgotten entry still returned")
	}
}

func TestMemoryIncrement(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()

	value, err := store.Increment(ctx, "counter", 2)
	if err != nil || value != 2 {
		t.Fatalf("new counter: %d %v", value, err)
	}
	value, err = store.Increment(ctx, "counter", 3)
	if err != nil || value != 5 {
		t.Fatalf("existing counter: %d %v", value, err)
	}
	if err := store.Set(ctx, "text", "not-a-number", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Increment(ctx, "text", 1); err == nil {
		t.Fatal("expected error incrementing non-integer value")
	}
}

func TestMemoryIncrementPreservesExpiry(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()
	current := time.Unix(1000, 0)
	store.now = func() time.Time { return current }

	if err := store.Set(ctx, "counter", "1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Increment(ctx, "counter", 1); err != nil {
		t.Fatal(err)
	}
	current = current.Add(2 * time.Minute)
	if _, ok, _ := store.Get(ctx, "counter"); ok {
		t.Fatal("expiry not preserved across increments")
	}
}

func TestMemoryJanitorSweepsExpiredEntries(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: 5 * time.Millisecond})
	defer store.Close()
	if err := store.Set(ctx, "short", "value", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.RLock()
		_, exists := store.entries["short"]
		store.mu.RUnlock()
		if !exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("janitor did not sweep the expired entry")
}

func TestMemoryCloseIsIdempotent(t *testing.T) {
	store := NewMemory(MemoryOptions{})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: time.Millisecond})
	defer store.Close()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			key := "worker-" + strconv.Itoa(worker%2)
			for i := 0; i < 100; i++ {
				_ = store.Set(ctx, key, "v", time.Millisecond)
				_, _, _ = store.Get(ctx, key)
				_, _ = store.Increment(ctx, "counter", 1)
				_ = store.Forget(ctx, key)
			}
		}(worker)
	}
	wg.Wait()
}
