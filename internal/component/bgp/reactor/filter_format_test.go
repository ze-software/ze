package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/family"
)

// TestAppendMPBlockNonCIDRMarker covers dest-2's hybrid emit strategy:
// CIDR families get prefixes inline, non-CIDR families get a marker-only
// block, and empty CIDR updates emit nothing at all.
//
// VALIDATES: dest-2 -- non-CIDR families produce "mp <family> add|del"
//
//	markers without prefixes, so text-mode filters on non-CIDR
//	sessions can still learn a family is present.
//
// PREVENTS:  regression to the pre-fix state where MP_REACH for EVPN /
//
//	Flowspec was silently dropped from the filter text protocol.
func TestAppendMPBlockNonCIDRMarker(t *testing.T) {
	evpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	flowspec := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	ipv6u := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}

	// Ensure L2VPN/EVPN and IPv4/FlowSpec are registered so Family.String()
	// returns canonical names; otherwise fallback is "afi-25/safi-70" which
	// obscures the assertion intent. Use the test-helper to register the
	// canonical names (idempotent).
	family.RegisterTestFamilies()

	// Non-CIDR: marker only, no prefixes. bufEmpty=true skips leading space.
	got := string(appendMPBlock(nil, evpn, "add", nil, false, true))
	assert.Equal(t, "mp l2vpn/evpn add", got)

	// Non-CIDR with a non-empty section: still marker only. The section is
	// intentionally not read.
	got = string(appendMPBlock(nil, flowspec, "del", []byte{3, 5, 0x81, 80}, false, true))
	assert.Equal(t, "mp ipv4/flow del", got)

	// CIDR with an empty section: emit nothing (caller appends blocks).
	got = string(appendMPBlock(nil, ipv6u, "add", nil, false, true))
	assert.Equal(t, "", got)
}
