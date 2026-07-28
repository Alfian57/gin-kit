package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// testRedisStore performs this package operation.
func testRedisStore(t *testing.T, prefix string) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	store, err := NewRedis(RedisOptions{Client: client, Prefix: prefix})
	if err != nil {
		t.Fatal(err)
	}
	return store, server
}

func TestNewRedisRequiresClient(t *testing.T) {
	if _, err := NewRedis(RedisOptions{}); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestRedisStore(t *testing.T) {
	ctx := context.Background()
	store, server := testRedisStore(t, "")

	if _, ok, err := store.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("miss expected: ok=%v err=%v", ok, err)
	}
	if err := store.Set(ctx, "key", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if value, ok, _ := store.Get(ctx, "key"); !ok || value != "value" {
		t.Fatalf("hit expected: %q %v", value, ok)
	}
	server.FastForward(2 * time.Minute)
	if _, ok, _ := store.Get(ctx, "key"); ok {
		t.Fatal("TTL not applied")
	}

	if value, err := store.Increment(ctx, "counter", 5); err != nil || value != 5 {
		t.Fatalf("increment: %d %v", value, err)
	}
	if err := store.Forget(ctx, "counter"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.Get(ctx, "counter"); ok {
		t.Fatal("forgotten key still present")
	}
	if err := store.Close(); err != nil {
		t.Fatal("redis store close must be a no-op nil")
	}
}

func TestRedisPrefixIsolation(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	first, err := NewRedis(RedisOptions{Client: client, Prefix: "one:"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRedis(RedisOptions{Client: client, Prefix: "two:"})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Set(ctx, "key", "from-one", 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := second.Get(ctx, "key"); ok {
		t.Fatal("prefixes are not isolated")
	}
	if value, ok, _ := first.Get(ctx, "key"); !ok || value != "from-one" {
		t.Fatalf("prefixed read failed: %q %v", value, ok)
	}
}
