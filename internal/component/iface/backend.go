// Design: docs/features/interfaces.md -- Interface backend abstraction
// Overview: iface.go -- shared types and topic constants
// Related: tunnel.go -- TunnelSpec and tunnel kind enum

package iface

import (
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Backend defines the operations that an interface management backend must
// implement. The iface component dispatches all OS-specific work through
// this interface. Implementations are registered via RegisterBackend and
// selected by the "backend" config leaf (default: "netlink").
//
// All methods that take an interface name MUST validate it before use.
// Implementations on unsupported platforms MUST return descriptive errors.
type Backend interface {
	// Interface lifecycle.
	CreateDummy(name string) error
	CreateVeth(name, peerName string) error
	CreateBridge(name string) error
	CreateVLAN(parentName string, vlanID int) error
	// CreateTunnel creates an L3 or L2 tunnel netdev for one of the kinds in
	// TunnelKind. The TunnelSpec carries kind-specific parameters; fields not
	// applicable to a kind are ignored. See tunnel.go for the spec shape.
	CreateTunnel(spec TunnelSpec) error
	DeleteInterface(name string) error

	// Address management.
	AddAddress(ifaceName, cidr string) error
	RemoveAddress(ifaceName, cidr string) error
	// ReplaceAddressWithLifetime adds or replaces an address with explicit
	// valid and preferred lifetimes (seconds). Used by DHCP for lease-aware
	// address installation. validLft=0 or preferredLft=0 means kernel default.
	ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error

	// Link state.
	SetAdminUp(ifaceName string) error
	SetAdminDown(ifaceName string) error

	// Interface properties.
	SetMTU(ifaceName string, mtu int) error
	SetMACAddress(ifaceName, mac string) error
	GetMACAddress(ifaceName string) (string, error)
	GetStats(ifaceName string) (*InterfaceStats, error)

	// Query.
	ListInterfaces() ([]InterfaceInfo, error)
	GetInterface(name string) (*InterfaceInfo, error)

	// Bridge operations.
	BridgeAddPort(bridgeName, portName string) error
	BridgeDelPort(portName string) error
	BridgeSetSTP(bridgeName string, enabled bool) error

	// Sysctl (per-interface kernel tunables).
	SetIPv4Forwarding(ifaceName string, enabled bool) error
	SetIPv4ArpFilter(ifaceName string, enabled bool) error
	SetIPv4ArpAccept(ifaceName string, enabled bool) error
	SetIPv4ProxyARP(ifaceName string, enabled bool) error
	SetIPv4ArpAnnounce(ifaceName string, level int) error
	SetIPv4ArpIgnore(ifaceName string, level int) error
	SetIPv4RPFilter(ifaceName string, level int) error
	SetIPv6Autoconf(ifaceName string, enabled bool) error
	SetIPv6AcceptRA(ifaceName string, level int) error
	SetIPv6Forwarding(ifaceName string, enabled bool) error

	// Traffic mirroring.
	SetupMirror(srcIface, dstIface string, ingress, egress bool) error
	RemoveMirror(srcIface string) error

	// Monitoring. StartMonitor begins watching OS interface events and
	// emitting them on the EventBus. StopMonitor halts monitoring and waits
	// for the monitor goroutine to exit.
	StartMonitor(eb ze.EventBus) error
	StopMonitor()

	// Close releases any resources held by the backend.
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
// Called from init() in backend packages (e.g., ifacenetlink).
// MUST be called before LoadBackend. Duplicate names are rejected.
func RegisterBackend(name string, factory func() (Backend, error)) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, exists := backends[name]; exists {
		return fmt.Errorf("iface: backend %q already registered", name)
	}
	backends[name] = factory
	return nil
}

// LoadBackend creates and activates the named backend. Called by the iface
// component during OnConfigure. Returns an error if the name is not registered.
// Caller MUST call CloseBackend when done.
func LoadBackend(name string) error {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	// Close previous backend to avoid leaking resources (e.g., monitor goroutines).
	if activeBackend != nil {
		if closeErr := activeBackend.Close(); closeErr != nil {
			loggerPtr.Load().Warn("iface: close previous backend", "err", closeErr)
		}
		activeBackend = nil
	}

	factory, ok := backends[name]
	if !ok {
		registered := make([]string, 0, len(backends))
		for k := range backends {
			registered = append(registered, k)
		}
		return fmt.Errorf("iface: unknown backend %q (registered: %v)", name, registered)
	}

	b, err := factory()
	if err != nil {
		return fmt.Errorf("iface: backend %q init: %w", name, err)
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
