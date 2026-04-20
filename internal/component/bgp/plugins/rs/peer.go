// Design: docs/architecture/core-design.md — route server plugin
// Overview: server.go — RouteServer uses PeerState for peer tracking and forwarding decisions

package rs

import "codeberg.org/thomas-mangin/ze/internal/core/family"

// PeerState tracks the state of a BGP peer.
type PeerState struct {
	Address      string                 // Peer IP address
	ASN          uint32                 // Peer AS number
	Up           bool                   // Session is established
	Replaying    bool                   // True during RIB replay (excluded from selectForwardTargets)
	ReplayGen    uint64                 // Incremented on each handleStateUp, guards stale goroutines
	Capabilities map[string]bool        // Negotiated capabilities (e.g., "route-refresh": true)
	Families     map[family.Family]bool // Supported AFI/SAFI
}

// HasCapability returns true if peer supports the given capability.
func (p *PeerState) HasCapability(cap string) bool {
	if p.Capabilities == nil {
		return false
	}
	return p.Capabilities[cap]
}

// SupportsFamily returns true if peer supports the given AFI/SAFI.
// A nil Families map (no OPEN received yet) is treated as "accept all" to avoid
// dropping routes during the window between state-up and OPEN processing.
func (p *PeerState) SupportsFamily(fam family.Family) bool {
	if p.Families == nil {
		return true
	}
	return p.Families[fam]
}
