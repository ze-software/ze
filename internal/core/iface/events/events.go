// Design: docs/architecture/api/process-protocol.md -- interface event types

// Package events defines event constants for the interface component.
// This is a core leaf package, safe to import from any component or plugin.
package events

// Namespace is the event namespace for the interface component.
const Namespace = "interface"

// Interface event types.
const (
	EventCreated              = "created"
	EventUp                   = "up"
	EventDown                 = "down"
	EventAddrAdded            = "addr-added"
	EventAddrRemoved          = "addr-removed"
	EventAddressReplayRequest = "address-replay-request"
	EventDHCPAcquired         = "dhcp-acquired"
	EventDHCPRenewed          = "dhcp-renewed"
	EventDHCPExpired          = "dhcp-expired"
	EventRollback             = "rollback"
	EventRouterDiscovered     = "router-discovered"
	EventRouterLost           = "router-lost"
	// EventNDUserOption carries one Neighbor Discovery option the kernel
	// handed to userspace from a Router Advertisement it accepted
	// (RTM_NEWNDUSEROPT): RDNSS among them.
	EventNDUserOption = "nd-user-option"
)
