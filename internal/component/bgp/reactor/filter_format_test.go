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

// TestDryRunNLRIBlocksRenderMalformedSectionInvalid proves the dry-run
// display tells a malformed NLRI section apart from an absent one.
//
// VALIDATES: an MP_REACH_NLRI or MP_UNREACH_NLRI whose accessor errors renders
// "mp afi-0/safi-0 add|del invalid": the zero family (AFI 0 and SAFI 0 are
// reserved, so no real family reads as it) because the attribute did not parse
// far enough to name one, and the "invalid" token. A legacy section that does
// not frame renders "nlri add|del invalid".
// PREVENTS: the display rendering no block, so an operator read a malformed
// section as an UPDATE that carries none.
func TestDryRunNLRIBlocksRenderMalformedSectionInvalid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"mp_reach shorter than 5", malformedMPReachBody, "mp afi-0/safi-0 add invalid"},
		{"mp_unreach shorter than 3", malformedMPUnreachBody, "mp afi-0/safi-0 del invalid"},
		{"mp_reach next hop past the attribute", nextHopOverrunMPReachBody, "mp afi-0/safi-0 add invalid"},
		// A Withdrawn Routes Length past the body: no section frames, so every
		// section renders as malformed.
		{"body that does not frame", "ffff", "nlri add invalid nlri del invalid mp afi-0/safi-0 add invalid mp afi-0/safi-0 del invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wireUpdate := testWireUpdate(nlriHex(t, tt.body))
			// appendNLRIBlocks is displayUpdate's NLRI renderer, called
			// directly: a body that does not frame has no attributes to read,
			// so displayUpdate refuses it before the blocks are rendered.
			got := string(appendNLRIBlocks(nil, wireUpdate))
			assert.Equal(t, tt.want, got, "a malformed MP attribute must not read as an absent section")
		})
	}
}
