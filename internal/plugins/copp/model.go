// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- CoPP policy data types

package copp

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/firewall"
)

const (
	overPolicyAccept = "accept"
	overPolicyDrop   = "drop"
)

// coppPolicy holds the parsed control-plane policing configuration.
type coppPolicy struct {
	ProtectedPorts []uint16
	TrustedSources []netip.Prefix
	// Neighbors are the sources a configured BGP neighbor may connect from,
	// read from the bgp section (config.BGPNeighborSources): each static
	// peer's address as a host prefix and each dynamic group's ranges.
	Neighbors []netip.Prefix
	// RateLimit is the limiter on new connections from the neighbors. nil
	// means the operator wrote no `rate`: the ACL stands alone and the table
	// carries no limiter term.
	RateLimit *coppRateLimit
}

// coppRateLimit is the `rate`, `burst` and `over-limit-policy` leaves. A burst
// means nothing without the rate it qualifies, so parseCoppConfig refuses one
// written alone.
type coppRateLimit struct {
	Rate       uint64
	RateUnit   string
	Dimension  firewall.RateDimension
	Burst      uint32
	OverPolicy string
}
