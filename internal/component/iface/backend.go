// Design: docs/features/interfaces.md -- Interface backend abstraction
// Overview: iface.go -- shared types and topic constants
// Related: tunnel.go -- TunnelSpec and tunnel kind enum

package iface

import (
	"errors"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// ErrBackendNotReady signals that a backend method was called before the
// underlying transport is usable. Callers that can defer their work (for
// example, applyConfig's reconciliation phase) should detect this sentinel
// with errors.Is and retry when the transport becomes ready.
//
// Produced by ifacevpp.ensureChannel when the vpp component has not yet
// completed its GoVPP handshake. The netlink backend never returns it.
var ErrBackendNotReady = errors.New("iface: backend not ready")

// ErrCountersNotResettable indicates the backend cannot physically zero
// RX/TX counters in the kernel (Linux has no generic reset; only a few
// drivers support ETHTOOL_* resets). The iface dispatch layer catches
// this sentinel from Backend.ResetCounters and falls back to a
// baseline-delta model: the current counter values are captured as a
// per-interface baseline, and GetStats/GetInterface subsequently
// report `current - baseline` so the operator sees "since last clear"
// values. Backends that CAN reset counters (VPP's
// sw_interface_clear_stats) return nil on success and this sentinel
// is not used.
var ErrCountersNotResettable = errors.New("iface: backend cannot reset counters (use baseline-delta)")

// vppBackendName is the string key ifacevpp registers under via
// iface.RegisterBackend. Exposed as a named constant so the
// reconcileOnVPPReady handler can gate on "is the active backend vpp?"
// without relying on a string literal scattered across the package.
const vppBackendName = "vpp"

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
	// CreateWireguardDevice creates a WireGuard netdev with the given name.
	// Only the netdev is created here; the private key, listen port, fwmark,
	// and peer set are configured by ConfigureWireguardDevice via wgctrl
	// because rtnetlink does not expose those fields. On kernels without
	// the wireguard module the netlink layer returns an error.
	CreateWireguardDevice(name string) error
	// ConfigureWireguardDevice applies the given spec as the complete
	// desired state of the named wireguard netdev: private key, listen
	// port, firewall mark, and full peer set. The current implementation
	// uses ReplacePeers: true under the hood, so any peers present in the
	// kernel but absent from the spec are removed. Requires an already-
	// existing netdev created via CreateWireguardDevice.
	ConfigureWireguardDevice(spec WireguardSpec) error
	// GetWireguardDevice reads the kernel's current state for the named
	// wireguard netdev and returns it as a WireguardSpec. Used by
	// reconciliation and by `ze init` discovery of pre-existing netdevs.
	// Keys are copied verbatim; callers must not log the returned Spec
	// unless they have already redacted sensitive fields.
	GetWireguardDevice(name string) (WireguardSpec, error)
	DeleteInterface(name string) error

	// Address management.
	AddAddress(ifaceName, cidr string) error
	RemoveAddress(ifaceName, cidr string) error
	// ReplaceAddressWithLifetime adds or replaces an address with explicit
	// valid and preferred lifetimes (seconds). Used by DHCP for lease-aware
	// address installation. validLft=0 or preferredLft=0 means kernel default.
	ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error

	// AddAddressP2P installs a point-to-point address on a virtual
	// interface: IFA_LOCAL holds the local side, IFA_ADDRESS holds the
	// remote (peer) side. Used by PPP NCPs (IPCP, IPv6CP) and any other
	// tunnel that needs /32 (/128) addressing with an explicit peer.
	// Both arguments are CIDR strings; the prefix length is what the
	// kernel stores and what `ip -d addr show` reports. The address
	// pair (local, peer) may be unrelated subnets -- this is how PPP
	// links typically work. Returns an error if the interface does not
	// exist or the kernel rejects the add.
	AddAddressP2P(ifaceName, localCIDR, peerCIDR string) error

	// Route management. Used by DHCP to install/remove default gateway.
	// destCIDR is the destination (e.g., "0.0.0.0/0"), gateway is the
	// next-hop IP (e.g., "192.168.1.1"), ifaceName scopes the route.
	// metric is the route priority (lower = preferred); 0 = kernel default.
	// On Linux, route identity is (dst, gw, link, metric), so both
	// AddRoute and RemoveRoute require metric to target the correct entry.
	AddRoute(ifaceName, destCIDR, gateway string, metric int) error
	RemoveRoute(ifaceName, destCIDR, gateway string, metric int) error
	// ListRoutes returns all routes matching the given destination CIDR on
	// the named interface. Used by IPv6 RA default route management to
	// clean up stale kernel-installed routes after suppressing accept_ra_defrtr.
	ListRoutes(ifaceName, destCIDR string) ([]RouteInfo, error)

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
	// ListNeighbors returns the kernel neighbor table (IPv4 ARP + IPv6 ND).
	// family is one of NeighborFamilyAny / NeighborFamilyIPv4 / NeighborFamilyIPv6
	// declared in iface.go; backends translate to their native constants.
	ListNeighbors(family int) ([]NeighborInfo, error)

	// ListKernelRoutes returns up to `limit` entries from the kernel's
	// routing table. filterPrefix, when non-empty, restricts the result
	// to the exact CIDR match (e.g. "10.0.0.0/8"). Empty returns
	// everything. limit == 0 means unbounded; positive values cap the
	// Go-side slice so a full-DFZ dump on a busy daemon cannot turn a
	// single read into a multi-hundred-megabyte allocation.
	// VPP backends should reject under exact-or-reject rather than return
	// kernel routes (the VPP fastpath FIB is authoritative on that backend).
	ListKernelRoutes(filterPrefix string, limit int) ([]KernelRoute, error)

	// ResetCounters zeros RX/TX counters for the named interface, or for
	// every managed interface when name == "". Linux netlink has no
	// generic counter-reset syscall and MUST reject under exact-or-reject;
	// VPP implements this via sw_interface_clear_stats (pending wiring).
	ResetCounters(name string) error

	// Bridge operations.
	BridgeAddPort(bridgeName, portName string) error
	BridgeDelPort(portName string) error
	BridgeSetSTP(bridgeName string, enabled bool) error

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

// DefaultBackendName returns the backend name used when the config does
// not specify one. It is the exported view of the package-private
// defaultBackendName constant, selected at build time via
// default_linux.go / default_other.go. `ze config validate` consults this
// so the offline CLI diagnoses the same rejection as the daemon on a
// config that omits the backend leaf.
func DefaultBackendName() string { return defaultBackendName }

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
