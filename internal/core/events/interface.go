// Design: docs/architecture/api/process-protocol.md -- interface monitor event types

package events

// Interface event types.
const (
	EventInterfaceCreated          = "created"
	EventInterfaceUp               = "up"
	EventInterfaceDown             = "down"
	EventInterfaceAddrAdded        = "addr-added"
	EventInterfaceAddrRemoved      = "addr-removed"
	EventInterfaceDHCPAcquired     = "dhcp-acquired"
	EventInterfaceDHCPRenewed      = "dhcp-renewed"
	EventInterfaceDHCPExpired      = "dhcp-expired"
	EventInterfaceRollback         = "rollback"
	EventInterfaceRouterDiscovered = "router-discovered"
	EventInterfaceRouterLost       = "router-lost"
)

// ValidInterfaceEvents is the set of valid interface monitor event types.
var ValidInterfaceEvents = map[string]bool{
	EventInterfaceCreated:          true,
	EventInterfaceUp:               true,
	EventInterfaceDown:             true,
	EventInterfaceAddrAdded:        true,
	EventInterfaceAddrRemoved:      true,
	EventInterfaceDHCPAcquired:     true,
	EventInterfaceDHCPRenewed:      true,
	EventInterfaceDHCPExpired:      true,
	EventInterfaceRollback:         true,
	EventInterfaceRouterDiscovered: true,
	EventInterfaceRouterLost:       true,
}
