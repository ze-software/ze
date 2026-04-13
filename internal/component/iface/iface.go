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
// (internal/plugins/ifacenetlink) handles Linux. DHCP is a separate plugin
// (internal/plugins/ifacedhcp). All interface types use a JunOS-style
// two-layer model: physical interface + logical units.
package iface

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

// InterfaceStats holds interface traffic counters from the kernel.
type InterfaceStats struct {
	RxBytes   uint64 `json:"rx-bytes"`
	RxPackets uint64 `json:"rx-packets"`
	RxErrors  uint64 `json:"rx-errors"`
	RxDropped uint64 `json:"rx-dropped"`
	TxBytes   uint64 `json:"tx-bytes"`
	TxPackets uint64 `json:"tx-packets"`
	TxErrors  uint64 `json:"tx-errors"`
	TxDropped uint64 `json:"tx-dropped"`
}

// DHCPPayload is the JSON payload for DHCP lease events.
// Field names use kebab-case per rules/json-format.md.
type DHCPPayload struct {
	Name         string   `json:"name"`
	Unit         int      `json:"unit"`
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
	Name        string          `json:"name"`
	Index       int             `json:"index"`
	Type        string          `json:"type"`
	State       string          `json:"state"`
	MTU         int             `json:"mtu"`
	MAC         string          `json:"mac-address,omitempty"`
	Addresses   []AddrInfo      `json:"addresses,omitempty"`
	Stats       *InterfaceStats `json:"stats,omitempty"`
	ParentIndex int             `json:"parent-index,omitempty"`
	VlanID      int             `json:"vlan-id,omitempty"`
}

// AddrInfo describes an IP address assigned to an interface.
type AddrInfo struct {
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
}

// RouteInfo describes a routing table entry. Used by ListRoutes for
// stale route cleanup after suppressing kernel RA default routes.
type RouteInfo struct {
	Destination string `json:"destination"` // CIDR (e.g., "::/0")
	Gateway     string `json:"gateway"`     // next-hop IP
	Metric      int    `json:"metric"`
}

// RouterEventPayload is the JSON payload for router discovery/loss events.
// Emitted by the netlink monitor when a neighbor's NTF_ROUTER flag changes.
type RouterEventPayload struct {
	Name     string `json:"name"`      // interface name
	RouterIP string `json:"router-ip"` // link-local address of the router
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
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	MAC       string         `json:"mac-address,omitempty"`
	Wireguard *WireguardSpec `json:"-"`
}
