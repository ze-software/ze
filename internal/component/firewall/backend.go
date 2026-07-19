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
// kernel: create new ze_* tables, replace changed ones, delete owned orphans.
// Non-ze_* tables MUST NOT be touched.
//
// Caller MUST call CloseBackend when done.
type Backend interface {
	// Apply receives full desired state and reconciles against kernel.
	// Only ze_* tables are touched. Non-ze_* tables are never modified.
	//
	// Single-writer contract: the registry serializes Apply behind reconcileMu
	// (registry.go, ApplyAll), so an implementation is NEVER called
	// concurrently. It may therefore keep un-synchronized per-backend state
	// (e.g. an applied-tables map) without its own mutex. Do NOT rely on being
	// called from a single goroutine identity -- rely only on the guarantee
	// that no two Apply calls overlap.
	//
	// Apply MUST NOT block indefinitely. Because reconcileMu is the outermost
	// firewall lock and is held for the whole reconcile, a kernel or netlink
	// round-trip that never returns would hold reconcileMu forever and stall
	// every firewall owner -- and any command handler that also needs a lock
	// the calling plugin holds across ApplyAll. Bound every kernel call with a
	// deadline and surface a timeout as an error rather than blocking.
	Apply(desired []Table) error

	// ListTables returns current ze_* tables from the kernel. For CLI show.
	ListTables() ([]Table, error)

	// GetCounters returns per-term packet/byte counter values. For CLI show counters.
	GetCounters(tableName string) ([]ChainCounters, error)

	// Close releases resources held by the backend.
	Close() error
}

// Verifier is a stateless pre-apply check specific to a backend. It is
// called during OnConfigVerify with the parsed desired state; returning
// an error rejects the commit before the backend is loaded. Verifiers are
// optional: a backend without a verifier accepts any config that passed
// YANG-level validation.
//
// This exists alongside the schema-level ze:backend YANG gate because
// the gate annotates LEAVES (a single type), not individual expression
// types. Rejecting "MatchConnMark but accepting MatchSourceAddress"
// under the vpp backend requires per-expression logic, which lives in
// a Verifier.
type Verifier func(desired []Table) error

// backendsMu protects the backends map, activeBackend, and verifiers.
// Lock order: backendsMu sits UNDER reconcileMu (registry.go) -- ApplyAll holds
// reconcileMu across its backendsMu section -- and is never held across a
// backend kernel call (ApplyAll releases it before b.Apply). It nests below no
// other firewall lock.
var backendsMu sync.Mutex

// backends maps backend names to factory functions. Populated by
// RegisterBackend calls in init() from backend packages.
var backends = map[string]func() (Backend, error){}

// verifiers maps backend names to verifier functions. Populated by
// RegisterVerifier calls in init(). Missing names mean "no extra checks".
var verifiers = map[string]Verifier{}

// activeBackend is the currently loaded backend. Set by LoadBackend
// during OnConfigure. Nil until a backend is loaded.
var activeBackend Backend

// RegisterVerifier registers an optional commit-time verifier for a backend.
// Called from init() in backend packages that need to reject configs which
// reference unsupported expression types before Apply runs. Duplicate
// registrations are rejected.
func RegisterVerifier(name string, v Verifier) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, exists := verifiers[name]; exists {
		return fmt.Errorf("firewall: verifier for backend %q already registered", name)
	}
	verifiers[name] = v
	return nil
}

// RunVerifier invokes the registered verifier for backendName against the
// parsed desired state. Returns nil if no verifier is registered (i.e. the
// backend accepts anything the YANG schema already allowed) or if the
// verifier reports no issue.
func RunVerifier(backendName string, desired []Table) error {
	backendsMu.Lock()
	v, ok := verifiers[backendName]
	backendsMu.Unlock()
	if !ok {
		return nil
	}
	return v(desired)
}

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
// The previous backend is kept alive until the new one is successfully created.
// On failure, the previous backend remains active.
// Caller MUST call CloseBackend when done.
//
// Invariant: after any outcome, `activeBackend != nil` iff
// `ActiveBackendName() != ""`. Every failure path (factory lookup miss,
// factory error) preserves the previous backend and name so CLI handlers
// can trust that GetBackend() and ActiveBackendName() stay consistent.
func LoadBackend(name string) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	return loadBackendLocked(name)
}

// loadBackendLocked creates and activates the named backend. Callers MUST
// hold backendsMu. LoadBackend is the exported, self-locking entry point;
// ApplyAll calls this directly because it already holds backendsMu when it
// loads the OS-default backend on demand for plugin-owned tables.
func loadBackendLocked(name string) error {
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

	prev := activeBackend
	activeBackend = b
	if prev != nil {
		if closeErr := prev.Close(); closeErr != nil {
			loggerPtr.Load().Warn("firewall: close previous backend", "err", closeErr)
		}
	}
	setActiveBackendName(name)
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
	setActiveBackendName("")
	StoreLastApplied(nil)
	return err
}
