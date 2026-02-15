// Package scenario generates deterministic BGP peer profiles and routes
// from a seed value for chaos testing.
package scenario

import "net/netip"

// ConnectionMode indicates whether a peer connects to Ze or Ze connects to it.
type ConnectionMode int

const (
	// ModeActive means the chaos tool connects to Ze.
	ModeActive ConnectionMode = iota
	// ModePassive means Ze connects to the chaos tool.
	ModePassive
)

// PeerProfile describes a simulated BGP peer's identity and behavior.
type PeerProfile struct {
	// Index is the peer's position in the scenario (0-based).
	Index int

	// ASN is the peer's autonomous system number.
	ASN uint32

	// RouterID is the peer's BGP router identifier.
	RouterID netip.Addr

	// IsIBGP is true when the peer shares the local AS (iBGP).
	IsIBGP bool

	// RouteCount is the number of routes this peer will announce.
	RouteCount int

	// Mode is whether this peer connects to Ze or Ze connects to it.
	Mode ConnectionMode

	// HoldTime is the peer's BGP hold time in seconds.
	HoldTime uint16

	// Port is the TCP port for this peer's BGP session.
	Port int

	// Families is the list of address families this peer supports.
	// Always includes "ipv4/unicast".
	Families []string
}
