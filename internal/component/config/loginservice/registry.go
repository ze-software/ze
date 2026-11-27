// Design: ai/patterns/registration.md -- login-service name registry
// Related: ../redistribute/registry.go -- sibling leaf registry of the same shape

// Package loginservice is the stdlib-only leaf registry of login service names
// offered by this build. Each component that offers an operator login registers
// its own service name at init() so compiling out a surface removes its name
// from config validation and editor completion.
//
// Safe for concurrent use.
package loginservice

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
)

// ErrConflict is returned when a login service name is re-registered with a
// different owner or description.
var ErrConflict = errors.New("loginservice: registration conflict")

// ErrInvalidEntry is returned when a registration omits its service name or
// owner.
var ErrInvalidEntry = errors.New("loginservice: invalid registration")

// Entry describes one registered login service.
type Entry struct {
	Name        string // wire/config identifier: "ssh", "web", "rest", "grpc", "mcp", ...
	Owner       string // owning package path, e.g. "internal/component/ssh"
	Description string // human-readable summary of the login door
}

var (
	mu      sync.RWMutex
	entries = map[string]Entry{}
)

// Register adds entry to the login-service registry. Re-registration with an
// identical name, owner, and description is accepted idempotently; a
// conflicting owner or description returns an error wrapping ErrConflict.
func Register(entry Entry) error {
	if entry.Name == "" {
		return fmt.Errorf("%w: empty service name", ErrInvalidEntry)
	}
	if entry.Owner == "" {
		return fmt.Errorf("%w: %q has empty owner", ErrInvalidEntry, entry.Name)
	}

	mu.Lock()
	defer mu.Unlock()

	if existing, ok := entries[entry.Name]; ok {
		if existing.Owner != entry.Owner {
			return fmt.Errorf("%w: %q already registered by %q, got %q",
				ErrConflict, entry.Name, existing.Owner, entry.Owner)
		}
		if existing.Description != entry.Description {
			return fmt.Errorf("%w: %q already registered by %q with a different description",
				ErrConflict, entry.Name, existing.Owner)
		}
		slog.Debug("login service already registered", "name", entry.Name, "owner", entry.Owner)
		return nil
	}
	entries[entry.Name] = entry
	return nil
}

// MustRegister registers entry and panics on error. Intended for package init()
// registrations where a conflict or empty field is a compile-time Ze defect.
func MustRegister(entry Entry) {
	if err := Register(entry); err != nil {
		panic("BUG: loginservice: " + err.Error())
	}
}

// Names returns all registered login-service names in sorted order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Registered reports whether name is currently registered as a login service.
func Registered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, ok := entries[name]
	return ok
}

// Lookup returns the registered Entry for name, if present.
func Lookup(name string) (Entry, bool) {
	mu.RLock()
	defer mu.RUnlock()

	entry, ok := entries[name]
	return entry, ok
}

// Snapshot returns a copy of the current registry contents for test isolation.
func Snapshot() map[string]Entry {
	mu.RLock()
	defer mu.RUnlock()

	return maps.Clone(entries)
}

// Restore replaces the registry contents with a prior Snapshot. Passing nil
// clears the registry. Intended for test cleanup.
func Restore(snapshot map[string]Entry) {
	mu.Lock()
	defer mu.Unlock()

	if snapshot == nil {
		entries = map[string]Entry{}
		return
	}
	entries = maps.Clone(snapshot)
}
