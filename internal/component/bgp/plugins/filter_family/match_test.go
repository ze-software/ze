package filter_family

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// mpFlowReachAttr builds a minimal ipv4/flow MP_REACH_NLRI (code 14) attribute
// whose value is AFI(1)+SAFI(133) — enough for family extraction (RFC 4760 §6).
func mpFlowReachAttr() []byte {
	val := []byte{0x00, 0x01, 133} // AFI=1 (IPv4), SAFI=133 (FlowSpec)
	return append([]byte{0x80, 14, byte(len(val))}, val...)
}

// originAttr is a well-formed ORIGIN (code 1) attribute used as a non-MP attr
// that must survive MP stripping.
var originAttr = []byte{0x40, 1, 1, 0}

// buildUpdate assembles an UPDATE body: withdrawn-len(0) + attrs + legacy NLRI.
func buildUpdate(attrs, nlri []byte) []byte {
	body := []byte{0x00, 0x00}
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

// TestFamilyFromPayload validates AFI/SAFI extraction and the no-MP default.
//
// VALIDATES: family match via ExtractMPFamily; no-MP UPDATE => ipv4/unicast.
// PREVENTS: misreading the family or panicking on a short body.
func TestFamilyFromPayload(t *testing.T) {
	// MP flowspec.
	fam, fromMP, ok := familyFromPayload(buildUpdate(mpFlowReachAttr(), nil))
	require.True(t, ok)
	assert.True(t, fromMP)
	assert.Equal(t, ipv4Flow, fam)

	// No MP attribute -> ipv4/unicast.
	fam, fromMP, ok = familyFromPayload(buildUpdate(originAttr, []byte{24, 10, 0, 0}))
	require.True(t, ok)
	assert.False(t, fromMP)
	assert.Equal(t, family.IPv4Unicast, fam)

	// Malformed (too short).
	_, _, ok = familyFromPayload([]byte{0x00})
	assert.False(t, ok)
}

// TestCarriesOnlyFamily validates the emptied check behind the whole-UPDATE
// suppress.
//
// VALIDATES: AC-2/AC-3 -- pure MP UPDATE of the family empties (suppress); a
// legacy NLRI tail, a withdrawn route or another MP family keeps the UPDATE.
// PREVENTS: suppressing an UPDATE that still carries other routes, or sending
// one the strip left with no NLRI.
func TestCarriesOnlyFamily(t *testing.T) {
	flow, _ := family.LookupFamily("ipv4/flow")
	only, ok := carriesOnlyFamily(buildUpdate(mpFlowReachAttr(), nil), flow)
	require.True(t, ok)
	assert.True(t, only, "MP-only flowspec UPDATE")

	attrs := append(append([]byte{}, originAttr...), mpFlowReachAttr()...)
	only, ok = carriesOnlyFamily(buildUpdate(attrs, []byte{24, 10, 0, 0}), flow)
	require.True(t, ok)
	assert.False(t, only, "legacy NLRI tail remains")

	withdrawn := append([]byte{0x00, 0x04, 24, 10, 0, 0}, buildUpdate(mpFlowReachAttr(), nil)[2:]...)
	only, ok = carriesOnlyFamily(withdrawn, flow)
	require.True(t, ok)
	assert.False(t, only, "withdrawn routes remain")

	two := append(append([]byte{}, mpFlowReachAttr()...), mpIPv6UnreachAttr()...)
	only, ok = carriesOnlyFamily(buildUpdate(two, nil), flow)
	require.True(t, ok)
	assert.False(t, only, "another MP family remains")

	_, ok = carriesOnlyFamily(buildUpdate([]byte{0x80, 14, 2, 0x00, 0x01}, nil), flow)
	assert.False(t, ok, "an MP attribute too short for AFI and SAFI is malformed")
}

// mpIPv6UnreachAttr builds an MP_UNREACH_NLRI (code 15) withdrawing
// 2001:db8::/32 in ipv6/unicast.
func mpIPv6UnreachAttr() []byte {
	val := []byte{0x00, 0x02, 0x01, 32, 0x20, 0x01, 0x0d, 0xb8}
	return append([]byte{0x80, 15, byte(len(val))}, val...)
}
