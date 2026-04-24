// Design: docs/architecture/core-design.md -- Traffic control backend abstraction
// Related: model.go -- Data model types consumed by Backend

package traffic

import (
	"context"
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

// Backend defines the operations that a traffic control backend must
// implement. The traffic component dispatches all kernel-specific work
// through this interface. Implementations are registered via RegisterBackend
// and selected by the "backend" config leaf (default: "tc").
//
// Apply receives the full desired state keyed by interface name. The backend
// reconciles qdiscs, classes, and filters on each interface.
//
// Caller MUST call CloseBackend when done.
type Backend interface {
	// Apply receives full desired state and reconciles qdiscs/classes/filters.
	// The ctx is propagated from the component's plugin lifecycle: backends that
	// can interrupt long-running kernel/IPC calls MUST honor cancellation so a
	// daemon SIGTERM does not block on an unreachable service (e.g. VPP).
	// Backends whose underlying library has no ctx-aware API may ignore it.
	//
	// ctx MUST NOT be nil. Callers pass context.Background() as the floor.
	Apply(ctx context.Context, desired map[string]InterfaceQoS) error

	// ListQdiscs returns current tc state for an interface. For CLI show.
	ListQdiscs(ifaceName string) (InterfaceQoS, error)

	// Close releases resources held by the backend.
	Close() error
}

// DefaultBackendName returns the backend name used when the config does not
// specify one. It is the exported view of the package-private
// defaultBackendName constant, selected at build time via
// default_linux.go / default_other.go. `ze config validate` consults this
// so the offline CLI diagnoses the same rejection as the daemon on a
// config that omits the backend leaf.
func DefaultBackendName() string { return defaultBackendName }

// Verifier is a stateless pre-apply check specific to a backend. It is
// called during OnConfigVerify with the parsed desired state; returning
// an error rejects the commit before the backend is loaded. Verifiers are
// optional -- a backend without a verifier accepts any config that passed
// YANG-level validation.
//
// This exists alongside the schema-level `ze:backend` YANG gate because
// the gate annotates LEAVES (a single type), not individual ENUM VALUES.
// Rejecting "qdisc hfsc but accepting qdisc htb" under the vpp backend
// requires per-value logic, which lives in a Verifier.
type Verifier func(desired map[string]InterfaceQoS) error

// backendsMu protects the backends map, activeBackend, and verifiers.
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

// RegisterBackend registers a backend factory under the given name.
// Called from init() in backend packages (e.g., trafficnetlink).
// MUST be called before LoadBackend. Duplicate names are rejected.
func RegisterBackend(name string, factory func() (Backend, error)) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, exists := backends[name]; exists {
		return fmt.Errorf("traffic: backend %q already registered", name)
	}
	backends[name] = factory
	return nil
}

// RegisterVerifier registers an optional commit-time verifier for a backend.
// Called from init() in backend packages that need to reject configs which
// reference unsupported qdisc or filter types before Apply runs. Duplicate
// registrations are rejected.
func RegisterVerifier(name string, v Verifier) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, exists := verifiers[name]; exists {
		return fmt.Errorf("traffic: verifier for backend %q already registered", name)
	}
	verifiers[name] = v
	return nil
}

// RunVerifier invokes the registered verifier for backendName against the
// parsed desired state. Returns nil if no verifier is registered (i.e. the
// backend accepts anything the YANG schema already allowed) or if the
// verifier reports no issue.
func RunVerifier(backendName string, desired map[string]InterfaceQoS) error {
	backendsMu.Lock()
	v, ok := verifiers[backendName]
	backendsMu.Unlock()
	if !ok {
		return nil
	}
	return v(desired)
}

// LoadBackend creates and activates the named backend. Called by the traffic
// component during OnConfigure. Returns an error if the name is not registered.
// The previous backend is kept alive until the new one is successfully created.
// On failure, the previous backend remains active.
// Caller MUST call CloseBackend when done.
func LoadBackend(name string) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	factory, ok := backends[name]
	if !ok {
		registered := make([]string, 0, len(backends))
		for k := range backends {
			registered = append(registered, k)
		}
		return fmt.Errorf("traffic: unknown backend %q (registered: %v)", name, registered)
	}

	b, err := factory()
	if err != nil {
		return fmt.Errorf("traffic: backend %q init: %w", name, err)
	}

	prev := activeBackend
	activeBackend = b
	if prev != nil {
		if closeErr := prev.Close(); closeErr != nil {
			loggerPtr.Load().Warn("traffic: close previous backend", "err", closeErr)
		}
	}
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
