// Design: docs/architecture/core-design.md -- core-leaf support bundle module registry

// Package supportreg provides a pure standard-library registry for support
// bundle collector modules. Static collectors in internal/component/support and
// live daemon collectors in internal/component/plugin/server both register here
// without introducing any component-to-component import.
package supportreg

import (
	"slices"
	"sync"
	"time"
)

// CollectOptions carries operator-selected filters for a support bundle run.
type CollectOptions struct {
	ConfigPath string
	Since      string
	Sensitive  bool
	SinceTime  time.Time
}

// Collector produces the structured payload for one named support bundle module.
// Implementations MUST be safe for concurrent use and MUST bound their output.
type Collector func(opts *CollectOptions) (any, error)

type entry struct {
	id uint64
	fn Collector
}

var (
	mu         sync.RWMutex
	nextID     uint64
	collectors = make(map[string]entry)
)

// Register adds a named support bundle collector and returns an idempotent
// unregister closure that removes only that specific registration.
// Panics on empty name, nil fn, or a duplicate live module name.
// Safe for concurrent use.
func Register(name string, fn Collector) func() {
	if name == "" {
		panic("BUG: supportreg.Register called with empty module name")
	}
	if fn == nil {
		panic("BUG: supportreg.Register called with nil collector")
	}

	mu.Lock()
	if _, exists := collectors[name]; exists {
		mu.Unlock()
		panic("BUG: supportreg.Register duplicate module: " + name)
	}
	nextID++
	id := nextID
	collectors[name] = entry{id: id, fn: fn}
	mu.Unlock()

	return func() {
		mu.Lock()
		defer mu.Unlock()
		cur, ok := collectors[name]
		if !ok {
			return
		}
		if cur.id != id {
			return
		}
		delete(collectors, name)
	}
}

// Names returns all currently registered support module names sorted ascending.
// Safe for concurrent use.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(collectors))
	for name := range collectors {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Snapshot returns a shallow copy of the registered module map so callers can
// iterate and invoke collectors outside the registry lock.
// Safe for concurrent use.
func Snapshot() map[string]Collector {
	mu.RLock()
	defer mu.RUnlock()

	out := make(map[string]Collector, len(collectors))
	for name, e := range collectors {
		out[name] = e.fn
	}
	return out
}
