// Design: docs/architecture/api/process-protocol.md -- interface event types

// Package events defines event constants for the interface component.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for the interface component.
const Namespace = "interface"

// Interface event types.
const (
	EventCreated          = "created"
	EventUp               = "up"
	EventDown             = "down"
	EventAddrAdded        = "addr-added"
	EventAddrRemoved      = "addr-removed"
	EventDHCPAcquired     = "dhcp-acquired"
	EventDHCPRenewed      = "dhcp-renewed"
	EventDHCPExpired      = "dhcp-expired"
	EventRollback         = "rollback"
	EventRouterDiscovered = "router-discovered"
	EventRouterLost       = "router-lost"
	// EventNeighborChanged reports an IPv6 neighbor entry that appeared, went
	// away, changed its link-layer address, or crossed between a resolved and
	// an unresolved state. BGP re-reads the neighbor table on it, for RFC 2545
	// Section 3's third-party Link-Local.
	EventNeighborChanged = "neighbor-changed"
)
