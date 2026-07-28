package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisOptions defines an implementation type used by this package.
type RedisOptions struct {
	// Client is required. The caller owns it and closes it; the runtime
	// shares one client between the cache and other Redis consumers.
	Client *redis.Client
	// Prefix is prepended to every key, e.g. "myapp:".
	Prefix string
}

// Redis is a Store backed by a shared Redis client.
type Redis struct {
	// client store data used by this type.
	client *redis.Client
	// prefix store data used by this type.
	prefix string
}

// NewRedis performs this package operation.
func NewRedis(options RedisOptions) (*Redis, error) {
	if options.Client == nil {
		return nil, errors.New("cache: redis store requires a client")
	}
	return &Redis{client: options.Client, prefix: options.Prefix}, nil
}

// Get performs this package operation.
func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := r.client.Get(ctx, r.prefix+key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Set performs this package operation.
func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl < 0 {
		ttl = 0
	}
	return r.client.Set(ctx, r.prefix+key, value, ttl).Err()
}

// Forget performs this package operation.
func (r *Redis) Forget(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.prefix+key).Err()
}

// Increment performs this package operation.
func (r *Redis) Increment(ctx context.Context, key string, by int64) (int64, error) {
	return r.client.IncrBy(ctx, r.prefix+key, by).Result()
}

// Close is a no-op: the underlying client is owned by the application.
func (r *Redis) Close() error { return nil }
