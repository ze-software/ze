// Design: docs/features/interfaces.md — Interface plugin shared types
// Detail: register.go — plugin registration and config application
// Detail: backend.go — Backend interface and registry
// Detail: dispatch.go — package-level functions delegating to backend
// Detail: config.go — config parsing and declarative application
// Detail: validators.go — interface name validation and autocomplete
// Detail: migrate_linux.go — make-before-break interface migration
// Detail: discover.go — OS interface discovery and Ze type mapping

// Package iface implements the interface monitoring and management plugin.
//
// It manages OS network interfaces through a pluggable backend architecture.
// The Backend interface defines all OS-specific operations. The netlink backend
// (internal/plugins/iface/netlink) handles Linux. DHCP is a separate plugin
// (internal/plugins/iface/dhcp). All interface types use a JunOS-style
// two-layer model: physical interface + logical units.
package iface

import "github.com/ze-software/ze/internal/component/plugin"

// Bus topic constants for interface events.
// Topics are hierarchical strings matching the Bus prefix subscription model.
const (
	// TopicPrefix is the shared prefix for all interface events.
	TopicPrefix = "interface/"

	// TopicCreated is published when an interface appears.
	TopicCreated = "interface/created"
	// TopicDeleted is published when an interface is removed.
	TopicDeleted = "interface/deleted"
	// TopicUp is published when link state transitions to up.
	TopicUp = "interface/up"
	// TopicDown is published when link state transitions to down.
	TopicDown = "interface/down"
	// TopicAddrAdded is published when an IP is assigned (DAD complete for IPv6).
	TopicAddrAdded = "interface/addr/added"
	// TopicAddrRemoved is published when an IP is removed.
	TopicAddrRemoved = "interface/addr/removed"

	// TopicDHCPLeaseAcquired is published when a DHCP lease is first obtained.
	TopicDHCPLeaseAcquired = "interface/dhcp/lease-acquired"
	// TopicDHCPLeaseRenewed is published when a DHCP lease is renewed.
	TopicDHCPLeaseRenewed = "interface/dhcp/lease-renewed"
	// TopicDHCPLeaseExpired is published when a DHCP lease expires.
	TopicDHCPLeaseExpired = "interface/dhcp/lease-expired"
)

// AddrPayload is the JSON payload for address events (addr/added, addr/removed).
// Field names use kebab-case per rules/json-format.md.
type AddrPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
	Managed      bool   `json:"managed"`
}

// LinkPayload is the JSON payload for link events (created, deleted).
type LinkPayload struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Index   int    `json:"index"`
	MTU     int    `json:"mtu"`
	Managed bool   `json:"managed"`
}

// StatePayload is the JSON payload for state events (up, down).
type StatePayload struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
}

// MirrorState is one interface's live mirror as the dataplane holds it: the
// destination device of each direction, empty when that direction copies
// nothing. It is the answer Backend.ListMirrors gives, and it is deliberately
// the same shape as the mirror a unit asks for. A reconcile therefore compares
// the two with one equality rather than a rule per direction.
//
// Interface and both destinations are OS DEVICE names, never logical ze names.
// The dataplane knows only devices, and the reconcile resolves the config's
// selectors to devices before it compares.
type MirrorState struct {
	Interface string `json:"interface"`
	Ingress   string `json:"ingress,omitempty"`
	Egress    string `json:"egress,omitempty"`
}

// MirrorDestinationUnresolved is the MirrorState destination a backend reports
// when a mirror IS installed and the device it copies to cannot be named. The
// dataplane holds an index, and no device in this namespace answers to it.
//
// It is not a device name and ValidateIfaceName refuses it, so it can never
// equal a destination the configuration asks for. That is the point: a
// reconcile then sees live and desired disagree, and it retires the mirror. An
// empty destination would have hidden that disagreement by reporting "no mirror
// is installed" for one that is.
const MirrorDestinationUnresolved = "?"

// InterfaceStats holds interface traffic counters from the kernel.
type InterfaceStats struct {
	RxBytes     uint64 `json:"rx-bytes"`
	RxPackets   uint64 `json:"rx-packets"`
	RxErrors    uint64 `json:"rx-errors"`
	RxDropped   uint64 `json:"rx-dropped"`
	RxMulticast uint64 `json:"rx-multicast,omitempty"`
	TxBytes     uint64 `json:"tx-bytes"`
	TxPackets   uint64 `json:"tx-packets"`
	TxErrors    uint64 `json:"tx-errors"`
	TxDropped   uint64 `json:"tx-dropped"`
}

// DHCPPayload is the JSON payload for DHCP lease events.
// Field names use kebab-case per rules/json-format.md.
type DHCPPayload struct {
	Name         string   `json:"name"`
	Unit         string   `json:"unit"`
	Address      string   `json:"address"`
	PrefixLength int      `json:"prefix-length"`
	Router       string   `json:"router,omitempty"`
	DNS          string   `json:"dns,omitempty"`
	DNSAll       []string `json:"dns-all,omitempty"`
	NTPServers   []string `json:"ntp-servers,omitempty"`
	LeaseTime    int      `json:"lease-time"`
}

// InterfaceInfo describes an OS network interface for display.
type InterfaceInfo struct {
	plugin.DataMarker
	Name string `json:"name"`
	// OsName is the OS/kernel device name. Today it equals Name; once a
	// resolver maps an operator-chosen logical name to a kernel device
	// (iface-resolve-2), Name carries the logical name and OsName keeps the
	// kernel device, so `show interface` shows both sides of the mapping.
	OsName string `json:"os-name,omitempty"`
	Index  int    `json:"index"`
	Type   string `json:"type"`
	State  string `json:"state"`
	MTU    int    `json:"mtu"`
	MAC    string `json:"mac-address,omitempty"`
	// PermanentMAC is the NIC's factory/permanent hardware address
	// (IFLA_PERM_ADDRESS), distinct from the operational MAC which an
	// operator may override. Empty for virtual/created kinds that have no
	// permanent address.
	PermanentMAC string `json:"permanent-mac-address,omitempty"`
	// Alias is the kernel IFLA_IFALIAS link alias. ze's owned-device registry
	// writes "ze:owned:<owner>" here to mark plugin-owned macvlan devices it
	// manages; the reconcile orphan scan reads it back to detect owner release
	// and crash leftovers. Empty for links with no alias.
	Alias       string          `json:"alias,omitempty"`
	Addresses   []AddrInfo      `json:"addresses,omitempty"`
	Stats       *InterfaceStats `json:"stats,omitempty"`
	ParentIndex int             `json:"parent-index,omitempty"`
	// MasterIndex is the index of the aggregating device this one is a member
	// of (IFLA_MASTER): the bridge it is a port of, or the bond it is enslaved
	// to. Zero when the device is a member of none. An aggregator takes its
	// hardware address from a member, so this is what tells a hardware selector
	// which of the two devices carrying one address owns it (devicesWithMAC).
	MasterIndex int `json:"master-index,omitempty"`
	// MacvlanMode is the delivery mode ("bridge"/"private") read back for a
	// macvlan device, so the owned-device reconcile can detect a mode drift
	// (e.g. a device created by an older binary in the wrong mode). Empty for
	// non-macvlan links and backends that do not report it.
	MacvlanMode string `json:"macvlan-mode,omitempty"`
	VlanID      int    `json:"vlan-id,omitempty"`
	Promisc     bool   `json:"promiscuous,omitempty"`
	// 802.1p QoS maps reported by the kernel for VLAN sub-interfaces
	// (IEEE 802.1Q PCP, 0-7). nil when unconfigured.
	IngressQoSMap map[uint32]uint32 `json:"ingress-qos-map,omitempty"` // received PCP -> internal priority
	EgressQoSMap  map[uint32]uint32 `json:"egress-qos-map,omitempty"`  // internal priority -> transmitted PCP
}

// AddrInfo describes an IP address assigned to an interface.
type AddrInfo struct {
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
	// LinkLocal is true for an IPv6 link-local (fe80::/10) address. It is set
	// by the resolver's Addresses() classifier so consumers (IS-IS) can split
	// v6 link-local from v6 global without re-parsing each address. Always
	// false for IPv4. Omitted from JSON when false.
	LinkLocal bool `json:"link-local,omitempty"`
	// Tentative is true while an IPv6 address is still completing Duplicate
	// Address Detection (kernel IFA_F_TENTATIVE). OSPFv3 prefers a DAD-complete
	// link-local over a tentative one as its source (falling back to a tentative
	// address only when the interface has no other link-local).
	// Always false for IPv4. Omitted from JSON when false.
	Tentative bool `json:"tentative,omitempty"`
	// Origin classifies how the kernel assigned the address, so status/CLI can
	// distinguish a stateless-autoconfigured (SLAAC/RA) address from a static
	// one (spec followup-subsystem AC-6). One of:
	//   "static"    - permanent, operator/kernel-configured (IFA_F_PERMANENT)
	//   "slaac"     - IPv6 SLAAC from a Router Advertisement (RFC 4862)
	//   "temporary" - IPv6 SLAAC privacy/temporary address (RFC 4941, IFA_F_TEMPORARY)
	//   "dynamic"   - non-permanent IPv4 (e.g. DHCP)
	// Empty when unknown (non-Linux, or not captured). Omitted from JSON when empty.
	Origin string `json:"origin,omitempty"`
	// ValidLifetime / PreferredLifetime are the kernel address lifetimes in
	// seconds an RA/lease carries (SLAAC/DHCP). Zero for a permanent address or
	// an infinite lifetime, and omitted from JSON. Non-zero values let an
	// operator see the remaining RA lease on a tracked SLAAC address.
	ValidLifetime     uint32 `json:"valid-lifetime,omitempty"`
	PreferredLifetime uint32 `json:"preferred-lifetime,omitempty"`
}

// Binding is the value snapshot the resolver returns from Resolve. It is a
// pure value type -- it carries NO netlink.Link or *net.Interface -- so a
// consumer that holds a logical interface name (IS-IS, PPPoE, ...) gets the
// ifindex/MAC/MTU it needs without coupling to the netlink backend
// (rules/plugin-design.md cross-boundary value types). It carries exactly
// what the old per-consumer ioctl wrappers produced (Ifindex, OperMAC, MTU)
// plus the os-name / permanent-MAC / state the resolver now owns.
type Binding struct {
	// Ifindex is the kernel interface index of the resolved OS device.
	Ifindex int
	// OsName is the kernel device name the logical name resolved to via the
	// os-name selector (defaulting to the logical name itself).
	OsName string
	// OperMAC is the current (operational) hardware address, which an operator
	// may have overridden.
	OperMAC string
	// PermMAC is the permanent/factory hardware address (IFLA_PERM_ADDRESS),
	// empty for virtual/created kinds that have none.
	PermMAC string
	// MTU is the device MTU.
	MTU int
	// State is the operational state ("up", "down", ...).
	State string
}

// LinkEventKind classifies a resolver LinkEvent.
type LinkEventKind string

const (
	// LinkAppeared signals the subscribed interface was created (first seen).
	LinkAppeared LinkEventKind = "appeared"
	// LinkUp signals the subscribed interface transitioned to up.
	LinkUp LinkEventKind = "up"
	// LinkDown signals the subscribed interface went down or was removed
	// (RTM_DELLINK arrives from the monitor as a down event).
	LinkDown LinkEventKind = "down"
)

// LinkEvent is delivered on a Subscribe channel when the link state of the
// subscribed logical name changes. It lets async consumers (IS-IS circuit
// lifecycle, LDP, DHCP) react to an interface appearing or going up/down
// instead of polling. Name is the logical name the consumer subscribed with,
// even when an os-name selector maps it to a different kernel device.
type LinkEvent struct {
	Name  string
	Kind  LinkEventKind
	Index int
}

// RouteInfo describes a routing table entry. Used by ListRoutes for
// stale route cleanup after suppressing kernel RA default routes.
type RouteInfo struct {
	Destination string `json:"destination"` // CIDR (e.g., "::/0")
	Gateway     string `json:"gateway"`     // next-hop IP
	Metric      int    `json:"metric"`
}

// NeighborInfo describes a kernel neighbor-table entry (IPv4 ARP or
// IPv6 ND). Backends produce this shape from their respective neighbor
// sources (netlink NeighList on Linux; GoVPP ip_neighbor_dump on VPP;
// unsupported elsewhere). See Backend.ListNeighbors.
type NeighborInfo struct {
	Address string `json:"address"`               // IP address
	MAC     string `json:"mac-address,omitempty"` // hardware address (may be empty for INCOMPLETE/FAILED)
	Device  string `json:"device"`                // interface name (resolved from link index)
	Family  string `json:"family"`                // "ipv4" or "ipv6"
	State   string `json:"state"`                 // reachable, stale, delay, probe, failed, permanent, noarp, incomplete
}

// Neighbor family selector for Backend.ListNeighbors. The backend
// translates to its native family constant; 0 means both families.
const (
	NeighborFamilyAny  = 0
	NeighborFamilyIPv4 = 4
	NeighborFamilyIPv6 = 6
)

// KernelRoute describes one entry in the kernel's routing table, dumped
// by Backend.ListKernelRoutes. Unlike RouteInfo (which is per-interface,
// used by IPv6 RA default-route cleanup), this shape covers every route
// the kernel can report: protocol, metric, device, and source-address
// fields.
//
// The `protocol` field renders the numeric rtm_protocol as a name, and
// an operator reads it in `show route` and on the web IP Routes page.
// A route Ze installed is named by its producer, from rtproto.Name:
// ze-fib, ze-static, ze-policy-route, or ze-iface for the interface
// layer (DHCP, RA, PPPoE and PPP routes, which carried the kernel's own
// "boot" until Ze started stamping them). Another producer's route is
// named from the well-known RTPROT_* values, which rtProtoNames
// (internal/plugins/iface/netlink/route_linux.go) holds keyed by the
// golang.org/x/sys/unix constants. Anything else surfaces as the decimal
// number, so the operator still gets a disambiguating hint.
//
// The names are not repeated here. A copy of that map's contents in this
// comment is what let 42 read as "ra" until 2026-08-11: the wrong number
// and the prose agreeing with each other proved nothing about either.
type KernelRoute struct {
	Destination string `json:"destination"`       // CIDR (e.g., "10.0.0.0/8", "::/0", "default")
	NextHop     string `json:"nexthop,omitempty"` // gateway IP; empty for connected routes
	Device      string `json:"device,omitempty"`  // egress interface name
	Protocol    string `json:"protocol"`          // producer name (ze-iface, bgp, kernel, ...) or decimal
	Metric      int    `json:"metric"`
	Family      string `json:"family"`           // "ipv4" or "ipv6"
	Source      string `json:"source,omitempty"` // IFA_LOCAL-style preferred source, if set
}

// RouterEventPayload is the JSON payload for router discovery/loss events.
// Emitted by the netlink monitor when a neighbor's NTF_ROUTER flag changes.
type RouterEventPayload struct {
	Name     string `json:"name"`      // interface name
	RouterIP string `json:"router-ip"` // link-local address of the router
}

// InterfaceRate holds computed per-second rates and the raw stats snapshot
// for a single interface. Produced by the rate tracker goroutine.
type InterfaceRate struct {
	plugin.DataMarker
	Name  string          `json:"name"`
	RxBps float64         `json:"rx-bps"`
	TxBps float64         `json:"tx-bps"`
	RxPps float64         `json:"rx-pps"`
	TxPps float64         `json:"tx-pps"`
	Stats *InterfaceStats `json:"stats,omitempty"`
}

// DiscoveredInterface describes an OS network interface found during discovery.
// Used by ze init to generate initial interface config and by the MAC address
// validator for autocomplete suggestions.
//
// Wireguard is set only for Type == "wireguard" entries; it carries the
// kernel-reported private key, listen port, firewall mark, and peer list so
// ze init can emit a complete wireguard config block from a manually-created
// netdev. Sensitive fields (PrivateKey, peer PresharedKey) are plaintext at
// this layer -- the emitter is responsible for passing them through
// secret.Encode before writing them to the config file.
type DiscoveredInterface struct {
	plugin.DataMarker
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	MAC       string         `json:"mac-address,omitempty"`
	Wireguard *WireguardSpec `json:"-"`
	XFRM      *XFRMInfo      `json:"-"`
}
