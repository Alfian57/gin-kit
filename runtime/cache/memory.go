package cache

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MemoryOptions defines an implementation type used by this package.
type MemoryOptions struct {
	// CleanupInterval between janitor sweeps of expired entries; defaults to
	// one minute. A negative interval disables the janitor.
	CleanupInterval time.Duration
}

// Memory is an in-process Store suitable for development and single-instance
// deployments. It is unbounded; use the Redis driver when eviction or shared
// state is needed.
type Memory struct {
	// mu store data used by this type.
	mu sync.RWMutex
	// entries store data used by this type.
	entries map[string]memoryEntry
	// now store data used by this type.
	now func() time.Time
	// stop store data used by this type.
	stop chan struct{}
	// closeOnce store data used by this type.
	closeOnce sync.Once
}

// memoryEntry defines an implementation type used by this package.
type memoryEntry struct {
	// value store data used by this type.
	value     string
	expiresAt time.Time // zero means never
}

// NewMemory performs this package operation.
func NewMemory(options MemoryOptions) *Memory {
	interval := options.CleanupInterval
	if interval == 0 {
		interval = time.Minute
	}
	memory := &Memory{
		entries: make(map[string]memoryEntry),
		now:     time.Now,
		stop:    make(chan struct{}),
	}
	if interval > 0 {
		go memory.janitor(interval)
	}
	return memory
}

// Get performs this package operation.
func (m *Memory) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok || m.expired(entry) {
		return "", false, nil
	}
	return entry.value, true, nil
}

// Set performs this package operation.
func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) error {
	entry := memoryEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = m.now().Add(ttl)
	}
	m.mu.Lock()
	m.entries[key] = entry
	m.mu.Unlock()
	return nil
}

// Forget performs this package operation.
func (m *Memory) Forget(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
	return nil
}

// Increment performs this package operation.
func (m *Memory) Increment(_ context.Context, key string, by int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[key]
	if !ok || m.expired(entry) {
		entry = memoryEntry{}
	}
	current := int64(0)
	if entry.value != "" {
		parsed, err := strconv.ParseInt(entry.value, 10, 64)
		if err != nil {
			return 0, err
		}
		current = parsed
	}
	current += by
	entry.value = strconv.FormatInt(current, 10)
	m.entries[key] = entry
	return current, nil
}

// Close stops the janitor. It is idempotent.
func (m *Memory) Close() error {
	m.closeOnce.Do(func() { close(m.stop) })
	return nil
}

// expired performs this package operation.
func (m *Memory) expired(entry memoryEntry) bool {
	return !entry.expiresAt.IsZero() && m.now().After(entry.expiresAt)
}

// janitor performs this package operation.
func (m *Memory) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			for key, entry := range m.entries {
				if m.expired(entry) {
					delete(m.entries, key)
				}
			}
			m.mu.Unlock()
		}
	}
}
