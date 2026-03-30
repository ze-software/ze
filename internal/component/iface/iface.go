// Design: plan/spec-iface-0-umbrella.md — Interface plugin shared types
// Detail: register.go — plugin registration
// Detail: manage_linux.go — interface management via netlink
// Detail: monitor_linux.go — netlink interface monitor
// Detail: sysctl_linux.go — per-interface sysctl management
// Detail: mirror_linux.go — traffic mirroring via tc mirred
// Detail: slaac_linux.go — IPv6 SLAAC control
// Detail: migrate_linux.go — make-before-break interface migration
// Detail: dhcp_linux.go — DHCP client types and lifecycle
// Detail: dhcp_v4_linux.go — DHCPv4 worker, renewal, lease handling
// Detail: dhcp_v6_linux.go — DHCPv6 worker, renewal, lease handling

// Package iface implements the interface monitoring and management plugin.
//
// It monitors OS network interfaces via netlink (Linux), publishes events
// to the Bus, and manages Ze-created interfaces. All interface types use
// a JunOS-style two-layer model: physical interface + logical units.
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

// DHCPPayload is the JSON payload for DHCP lease events.
// Field names use kebab-case per rules/json-format.md.
type DHCPPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Router       string `json:"router,omitempty"`
	DNS          string `json:"dns,omitempty"`
	LeaseTime    int    `json:"lease-time"`
}
