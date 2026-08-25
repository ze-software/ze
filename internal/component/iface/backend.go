// Design: docs/features/interfaces.md -- Interface backend abstraction
// Overview: iface.go -- shared types and topic constants
// Related: tunnel.go -- TunnelSpec and tunnel kind enum

package iface

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/pkg/ze"
)

// ErrBackendNotReady signals that a backend method was called before the
// underlying transport is usable. Callers that can defer their work (for
// example, applyConfig's reconciliation phase) should detect this sentinel
// with errors.Is and retry when the transport becomes ready.
//
// Produced by ifacevpp.ensureChannel when the vpp component has not yet
// completed its GoVPP handshake. The netlink backend never returns it.
var ErrBackendNotReady = errors.New("iface: backend not ready")

// ErrInterfaceExists signals that a create step found the name already bound
// to a live interface, so it kept that interface and issued no create. The
// step created nothing, and applyConfig reads the sentinel with errors.Is to
// keep its undo from deleting a device an earlier apply made.
//
// A backend that cannot answer "does this name exist" without a create is not
// required to produce it: the netlink backend returns the kernel's EEXIST and
// applyConfig falls back to Backend.GetInterface for that case. Produced by
// ifacevpp.CreateDummy, whose create message (create_loopback) carries no name
// and allocates a fresh interface on every call.
var ErrInterfaceExists = errors.New("iface: interface already exists")

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

// VLANSpec carries the parameters for an 802.1Q sub-interface. The QoS maps
// translate between the 3-bit 802.1p PCP field of the tag header and the
// internal packet priority (IEEE 802.1Q: PCP and priority are 0-7):
// IngressQoSMap maps received PCP to internal priority, EgressQoSMap maps
// internal priority to transmitted PCP. nil maps mean unconfigured -- the
// backend MUST NOT emit an empty mapping attribute for them.
type VLANSpec struct {
	Parent        string
	VLANID        int
	IngressQoSMap map[uint32]uint32 // received PCP -> internal priority
	EgressQoSMap  map[uint32]uint32 // internal priority -> transmitted PCP
}

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
	// CreateVLAN creates an 802.1Q sub-interface named "<Parent>.<VLANID>".
	// The optional QoS maps translate between 802.1p PCP bits and internal
	// priority; nil maps mean no mapping is configured.
	CreateVLAN(spec VLANSpec) error
	// UpdateVLANQoSMap replaces the ingress and egress 802.1p QoS maps
	// on an existing VLAN sub-interface. nil maps mean "clear to defaults".
	// Used by dynamic CoS to change PCP mappings without deleting the VLAN.
	UpdateVLANQoSMap(ifaceName string, ingress, egress map[uint32]uint32) error
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
	// CreateMacvlanDevice creates a bridge-mode macvlan netdev on spec.Parent
	// carrying spec.MAC, marked with the owned-device alias spec.Alias
	// ("ze:owned:<owner>", set by the reconcile pass) and brought admin-up,
	// with its MTU inherited from the parent. The MAC and alias are set
	// atomically in the create (rtnetlink RTM_NEWLINK), so there is no
	// create-then-mark window. Deletion uses the existing DeleteInterface;
	// listing rides ListInterfaces (InterfaceInfo.Alias carries the marker).
	// Non-netlink backends (VPP, non-Linux) reject under exact-or-reject.
	CreateMacvlanDevice(spec MacvlanSpec) error
	// CreateXFRM creates an XFRM interface netdev with the given spec.
	// The IfID binds the interface to XFRM security associations; the
	// optional PhysicalDev constrains the underlay device. On kernels
	// without XFRM interface support (< 4.19) netlink returns an error.
	CreateXFRM(spec XFRMSpec) error
	// GetXFRMInfo reads the if_id from the kernel for the named XFRM
	// netdev and queries XFRM policies bound to that if_id. Used by
	// show commands to display IPsec binding details for both managed
	// and unmanaged XFRM interfaces.
	GetXFRMInfo(name string) (XFRMInfo, error)
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
	// On Linux the kernel keys a route on (dst, gw, link, metric), so both
	// AddRoute and RemoveRoute require metric to target the correct entry.
	//
	// proto is the producer that owns the route, and the delete matches on it
	// as well as on that key. AddRoute stamps it on the route it installs and
	// rejects rtproto.Any, which names no producer. RemoveRoute with
	// rtproto.Any matches a route whatever installed it, so a caller that
	// wants that must name it: it is never reached by leaving a value out.
	AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error
	RemoveRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error
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
	// LinkSpeedDuplex returns the link speed (Mbit/s) and duplex ("full"/"half")
	// for the named interface, or (0, "") when unknown. Best-effort enrichment
	// for the flow-export sFlow if_counters; on Linux it reads sysfs, other
	// backends (VPP, non-Linux stub) return (0, "").
	LinkSpeedDuplex(ifaceName string) (speedMbps int, duplex string)

	// Query.
	ListInterfaces() ([]InterfaceInfo, error)
	GetInterface(name string) (*InterfaceInfo, error)
	// ListNeighbors returns the kernel neighbor table (IPv4 ARP + IPv6 ND).
	// family is one of NeighborFamilyAny / NeighborFamilyIPv4 / NeighborFamilyIPv6
	// declared in iface.go; backends translate to their native constants.
	ListNeighbors(family int) ([]NeighborInfo, error)

	// RouteLookup performs a longest-prefix-match lookup for the given
	// destination IP in the backend's FIB. Returns the matching route as
	// a map suitable for JSON serialization with keys: destination, prefix,
	// next-hop, interface, protocol, metric, table.
	RouteLookup(dest netip.Addr) (map[string]any, error)

	// AddressIsLocal reports whether dest is an address this box terminates
	// (owned by one of its interfaces / the loopback) rather than one it forwards.
	// On Linux this is the kernel's RTN_LOCAL classification; on VPP it is a local
	// FIB entry. Used to classify a DDoS victim as local (control-plane, INPUT hook)
	// vs remote (transit, FORWARD hook). Returns an error when the backend cannot
	// answer; callers treat an error as "not local" (remote is the fail-safe).
	AddressIsLocal(dest netip.Addr) (bool, error)

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

	// ListMirrors reports every mirror the dataplane carries right now, one
	// entry per source interface. It is what lets a reconcile compare LIVE
	// state against the configuration rather than against the previous config.
	// A mirror the operator removed while ze was down appears in no previous
	// config. A mirror whose teardown was skipped was already consumed by the
	// apply that skipped it. Neither is reachable from a delta.
	//
	// A backend that cannot enumerate them MUST return an error and MUST NOT
	// return an empty slice. "No mirror is installed" and "I cannot tell" are
	// different answers. Reading the second as the first reports that the
	// dataplane matches the configuration while it copies packets to a
	// destination the operator deleted.
	ListMirrors() ([]MirrorState, error)

	// SetupLCPPair creates a Linux Control Plane pair for a VPP interface: a
	// Linux TAP that shadows the named VPP interface so kernel networking (the
	// ze BGP listener, ssh, ...) can bind on it. hostName is the desired Linux
	// TAP name. This is a VPP-only concept -- the netlink and non-Linux
	// backends reject it; the vpp backend consumes the configured lcp netns and
	// no-ops when LCP is disabled. RemoveLCPPair tears the pair down.
	SetupLCPPair(vppIface, hostName string) error
	RemoveLCPPair(vppIface string) error

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

// activeBackendName is the registered name of the currently loaded backend
// (e.g. "netlink" or "vpp"). Set by LoadBackend alongside activeBackend and
// cleared by CloseBackend. Empty when no backend is loaded. Exposed via
// ActiveBackendName so a consumer that resolves logical names for a specific
// data plane (e.g. the VPP static backend) can confirm the active iface
// backend matches its data plane before trusting a resolved index.
var activeBackendName string

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
		return fmt.Errorf("iface: unknown backend %q (registered: %v)", name, registered)
	}

	b, err := factory()
	if err != nil {
		return fmt.Errorf("iface: backend %q init: %w", name, err)
	}

	// Swap first, then close old so a failed apply still has the new backend active.
	prev := activeBackend
	activeBackend = b
	activeBackendName = name
	if prev != nil {
		if closeErr := prev.Close(); closeErr != nil {
			loggerPtr.Load().Warn("iface: close previous backend", "err", closeErr)
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

// EnsureBackend loads the build-time default backend when none is loaded, so a
// consumer that needs interface data (e.g. OSPF reading an interface's IPv4 for
// its multicast join) works even when the config has no interface{} block to
// trigger an explicit backend load. It is a no-op when a backend is already
// loaded, so an explicit `interface { backend vpp }` always wins. The default
// name is empty on platforms with no OS backend (only the stub), where it
// returns the same "no backend" error the caller would have hit anyway.
func EnsureBackend() error {
	if GetBackend() != nil {
		return nil
	}
	name := DefaultBackendName()
	if name == "" {
		return errIfaceNoBackendLoaded
	}
	return LoadBackend(name)
}

// ActiveBackendName returns the registered name of the active iface backend
// (e.g. "netlink" or "vpp"), or "" when no backend is loaded. It lets a
// data-plane-specific consumer confirm the active iface backend matches its
// data plane before trusting a resolved index: resolving a logical name
// against a netlink backend yields a KERNEL ifindex, which must never be
// programmed into VPP as a sw_if_index. Companion to DefaultBackendName (the
// build-time fallback); this reports what is actually loaded now.
func ActiveBackendName() string {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	return activeBackendName
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
	activeBackendName = ""
	return err
}
