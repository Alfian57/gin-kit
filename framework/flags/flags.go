// Package flags provides a small, in-memory set of boolean feature flags.
//
// Flags are explicit application state: create a Set during startup, pass it
// to the components that need it, and update it at runtime with Set. The
// package does not provide global state or persistence.
package flags

import (
	"os"
	"sort"
	"strings"
	"sync"
)

// Set is a concurrency-safe collection of boolean feature flags.
//
// The zero value is ready to use. A nil *Set behaves like an empty set when
// queried with Enabled.
type Set struct {
	mu     sync.RWMutex
	values map[string]bool
}

// New returns a Set with names enabled. Empty names are ignored and
// surrounding whitespace is trimmed.
func New(names ...string) *Set {
	set := &Set{}
	for _, name := range names {
		set.Set(name, true)
	}
	return set
}

// Parse returns a Set containing the enabled names in a comma-separated
// string. Empty items are ignored and surrounding whitespace is trimmed.
func Parse(csv string) *Set {
	return New(strings.Split(csv, ",")...)
}

// FromEnv parses the FLAGS environment variable.
func FromEnv() *Set {
	return Parse(os.Getenv("FLAGS"))
}

// Enabled reports whether name is enabled. It returns false for an empty name
// or a nil receiver.
func (s *Set) Enabled(name string) bool {
	if s == nil {
		return false
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[name]
}

// Set enables or disables name. Empty names are ignored.
func (s *Set) Set(name string, on bool) {
	if s == nil {
		return
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		if s.values == nil {
			s.values = make(map[string]bool)
		}
		s.values[name] = true
		return
	}
	delete(s.values, name)
}

// Names returns the enabled flag names in sorted order.
func (s *Set) Names() []string {
	if s == nil {
		return []string{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.values))
	for name, enabled := range s.values {
		if enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
