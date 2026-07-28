package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type profile struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

func TestRememberComputesOnMissAndSkipsOnHit(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()

	computed := 0
	compute := func(context.Context) (profile, error) {
		computed++
		return profile{Name: "gin-kit", Score: 10}, nil
	}
	first, err := Remember(ctx, store, "profile", time.Minute, compute)
	if err != nil || first.Name != "gin-kit" || computed != 1 {
		t.Fatalf("miss path: %+v computed=%d err=%v", first, computed, err)
	}
	second, err := Remember(ctx, store, "profile", time.Minute, compute)
	if err != nil || second.Score != 10 || computed != 1 {
		t.Fatalf("hit path recomputed: computed=%d err=%v", computed, err)
	}
}

func TestRememberPropagatesComputeErrorWithoutStoring(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()

	sentinel := errors.New("db offline")
	if _, err := Remember(ctx, store, "key", time.Minute, func(context.Context) (int, error) {
		return 0, sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("compute error not propagated: %v", err)
	}
	if _, ok, _ := store.Get(ctx, "key"); ok {
		t.Fatal("failed computation was stored")
	}
}

func TestRememberRecomputesCorruptValues(t *testing.T) {
	ctx := context.Background()
	store := NewMemory(MemoryOptions{CleanupInterval: -1})
	defer store.Close()

	if err := store.Set(ctx, "profile", "{not-json", time.Minute); err != nil {
		t.Fatal(err)
	}
	value, err := Remember(ctx, store, "profile", time.Minute, func(context.Context) (profile, error) {
		return profile{Name: "fresh"}, nil
	})
	if err != nil || value.Name != "fresh" {
		t.Fatalf("corrupt value not recomputed: %+v err=%v", value, err)
	}
	raw, ok, _ := store.Get(ctx, "profile")
	if !ok || raw == "{not-json" {
		t.Fatalf("corrupt value not overwritten: %q", raw)
	}
}
