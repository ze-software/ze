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
)
