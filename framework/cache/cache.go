// Package cache provides a Laravel-style cache with in-memory and Redis
// drivers behind one small interface.
package cache

import (
	"context"
	"encoding/json"
	"time"
)

// Store is the cache contract shared by every driver.
type Store interface {
	// Get returns the value for key; ok reports whether the key existed.
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	// Set stores value under key. A ttl of zero or less stores it without
	// expiry.
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// Forget removes key; deleting a missing key is not an error.
	Forget(ctx context.Context, key string) error
	// Increment adds by to the integer stored under key, starting from zero
	// when the key is missing, and returns the new value. A non-integer value
	// is an error.
	Increment(ctx context.Context, key string, by int64) (int64, error)
	Close() error
}

// Remember returns the cached JSON-decoded value for key, computing and
// storing it on a miss. A stored value that no longer decodes into T is
// treated as a miss and recomputed, so shape changes self-heal.
func Remember[T any](ctx context.Context, store Store, key string, ttl time.Duration, compute func(context.Context) (T, error)) (T, error) {
	var value T
	raw, ok, err := store.Get(ctx, key)
	if err != nil {
		return value, err
	}
	if ok && json.Unmarshal([]byte(raw), &value) == nil {
		return value, nil
	}
	value, err = compute(ctx)
	if err != nil {
		return value, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	if err := store.Set(ctx, key, string(encoded), ttl); err != nil {
		return value, err
	}
	return value, nil
}
