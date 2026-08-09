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
	Rate           uint64
	RateUnit       string
	Dimension      firewall.RateDimension
	Burst          uint32
	ProtectedPorts []uint16
	TrustedSources []netip.Prefix
	OverPolicy     string
}
