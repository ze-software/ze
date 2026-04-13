// Design: docs/architecture/core-design.md -- Firewall backend abstraction
// Related: model.go -- Data model types consumed by Backend

package firewall

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// loggerPtr is the package-level logger, disabled by default.
// Updated by the component's register.go when the plugin starts.
var loggerPtr atomic.Pointer[slog.Logger]

func init() { //nolint:gochecknoinits // logger bootstrap only
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

// Backend defines the operations that a firewall management backend must
// implement. The firewall component dispatches all kernel-specific work
// through this interface. Implementations are registered via RegisterBackend
// and selected by the "backend" config leaf (default: "nft").
//
// Apply receives the full desired state. The backend reconciles against the
// kernel: create new ze_* tables, replace changed ones, delete orphans.
// Non-ze_* tables MUST NOT be touched.
//
// Caller MUST call CloseBackend when done.
type Backend interface {
	// Apply receives full desired state and reconciles against kernel.
	// Only ze_* tables are touched. Non-ze_* tables are never modified.
	Apply(desired []Table) error

	// ListTables returns current ze_* tables from the kernel. For CLI show.
	ListTables() ([]Table, error)

	// GetCounters returns per-term packet/byte counter values. For CLI show counters.
	GetCounters(tableName string) ([]ChainCounters, error)

	// Close releases resources held by the backend.
	Close() error
}

// backendsMu protects the backends map and activeBackend.
var backendsMu sync.Mutex

// backends maps backend names to factory functions. Populated by
// RegisterBackend calls in init() from backend packages.
var backends = map[string]func() (Backend, error){}

// activeBackend is the currently loaded backend. Set by LoadBackend
// during OnConfigure. Nil until a backend is loaded.
var activeBackend Backend

// RegisterBackend registers a backend factory under the given name.
// Called from init() in backend packages (e.g., firewallnft).
// MUST be called before LoadBackend. Duplicate names are rejected.
func RegisterBackend(name string, factory func() (Backend, error)) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, exists := backends[name]; exists {
		return fmt.Errorf("firewall: backend %q already registered", name)
	}
	backends[name] = factory
	return nil
}

// LoadBackend creates and activates the named backend. Called by the firewall
// component during OnConfigure. Returns an error if the name is not registered.
// Caller MUST call CloseBackend when done.
func LoadBackend(name string) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if activeBackend != nil {
		if closeErr := activeBackend.Close(); closeErr != nil {
			loggerPtr.Load().Warn("firewall: close previous backend", "err", closeErr)
		}
		activeBackend = nil
	}

	factory, ok := backends[name]
	if !ok {
		registered := make([]string, 0, len(backends))
		for k := range backends {
			registered = append(registered, k)
		}
		return fmt.Errorf("firewall: unknown backend %q (registered: %v)", name, registered)
	}

	b, err := factory()
	if err != nil {
		return fmt.Errorf("firewall: backend %q init: %w", name, err)
	}
	activeBackend = b
	return nil
}

// GetBackend returns the active backend, or nil if none loaded.
func GetBackend() Backend {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	return activeBackend
}

// CloseBackend shuts down the active backend and clears it.
func CloseBackend() error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if activeBackend == nil {
		return nil
	}
	err := activeBackend.Close()
	activeBackend = nil
	return err
}
