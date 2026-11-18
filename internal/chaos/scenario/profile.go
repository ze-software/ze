// Design: docs/architecture/chaos-web-dashboard.md — scenario generation
//
// Package scenario generates deterministic BGP peer profiles and routes
// from a seed value for chaos testing.
package scenario

import (
	"net/netip"
	"time"
)

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

	// Port is the TCP port the simulator listens on (for Ze→peer active connections).
	Port int

	// ZePort is the TCP port Ze listens on for this specific peer.
	// Each peer gets a unique port (BasePort + Index) so all peers
	// can run on 127.0.0.1 without needing loopback aliases.
	ZePort int

	// Families is the list of address families this peer supports.
	// Always includes "ipv4/unicast".
	Families []string

	// Address is an optional per-peer neighbor address.
	// When valid, used instead of ConfigParams.LocalAddr in GenerateConfig.
	// This enables unique per-peer addresses for in-process mode where
	// the reactor needs distinct peer addresses for its peer map lookup.
	Address netip.Addr

	// SlowRead is the delay between reads in the simulator's read loop.
	// When non-zero, the peer reads BGP messages slowly, causing TCP
	// backpressure that prevents Ze from sending to this peer.
	// Zero means normal read speed (no artificial delay).
	SlowRead time.Duration

	// RouteSet is the exact IPv4 unicast routes this peer announces, each with
	// its own next hop and AS path. Nil means the peer announces RouteCount
	// routes generated from the scenario seed, as every profile of Generate
	// does. GenerateFromTopology sets it, with RouteCount as its length.
	RouteSet *RouteSet
}

// RouteSet is the whole set of IPv4 unicast routes one simulated peer
// announces, when the routes are given rather than generated from a seed. An
// empty set is a peer that announces nothing, which is why a profile holds a
// pointer: a nil pointer is the seed-generated mode, never an empty set.
type RouteSet struct {
	// Announcements holds one route per prefix, in the order the peer sends
	// them. Announcement.ASPath is the whole path on the wire, so a sender
	// MUST NOT prepend anything to it.
	Announcements []Announcement
}

// Prefixes returns the prefixes of the set, in the order of its announcements.
func (s *RouteSet) Prefixes() []netip.Prefix {
	prefixes := make([]netip.Prefix, len(s.Announcements))
	for index := range s.Announcements {
		prefixes[index] = s.Announcements[index].Prefix
	}
	return prefixes
}
