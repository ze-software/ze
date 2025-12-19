// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"net/netip"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
)

// DefaultBGPPort is the standard BGP port per RFC 4271.
const DefaultBGPPort = 179

// DefaultHoldTime is the default hold time per RFC 4271.
const DefaultHoldTime = 90 * time.Second

// Neighbor represents a configured BGP neighbor.
type Neighbor struct {
	// Address is the peer's IP address.
	Address netip.Addr

	// Port is the peer's BGP port (default 179).
	Port uint16

	// LocalAS is our AS number.
	LocalAS uint32

	// PeerAS is the peer's AS number.
	PeerAS uint32

	// RouterID is our BGP router identifier (IPv4 format).
	RouterID uint32

	// HoldTime is the proposed hold time (default 90s).
	HoldTime time.Duration

	// Passive indicates listen-only mode (no outgoing connections).
	Passive bool

	// Capabilities to advertise in OPEN message.
	Capabilities []capability.Capability
}

// NewNeighbor creates a neighbor with default values.
func NewNeighbor(address netip.Addr, localAS, peerAS, routerID uint32) *Neighbor {
	return &Neighbor{
		Address:  address,
		Port:     DefaultBGPPort,
		LocalAS:  localAS,
		PeerAS:   peerAS,
		RouterID: routerID,
		HoldTime: DefaultHoldTime,
	}
}

// IsIBGP returns true if this is an internal BGP session (same AS).
func (n *Neighbor) IsIBGP() bool {
	return n.LocalAS == n.PeerAS
}

// IsEBGP returns true if this is an external BGP session (different AS).
func (n *Neighbor) IsEBGP() bool {
	return n.LocalAS != n.PeerAS
}
