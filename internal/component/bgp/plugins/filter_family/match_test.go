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

// TestFamilyMatch validates target matching and the legacy no-MP default.
func TestFamilyMatch(t *testing.T) {
	matched, only, ok := familyMatch(buildUpdate(mpFlowReachAttr(), nil), ipv4Flow)
	require.True(t, ok)
	assert.True(t, matched)
	assert.True(t, only)
	matched, _, ok = familyMatch(buildUpdate(mpFlowReachAttr(), nil), family.IPv4Unicast)
	require.True(t, ok)
	assert.False(t, matched)

	matched, only, ok = familyMatch(buildUpdate(originAttr, []byte{24, 10, 0, 0}), family.IPv4Unicast)
	require.True(t, ok)
	assert.True(t, matched)
	assert.True(t, only)
	matched, _, ok = familyMatch(buildUpdate(originAttr, []byte{24, 10, 0, 0}), ipv4Flow)
	require.True(t, ok)
	assert.False(t, matched)

	_, _, ok = familyMatch([]byte{0x00}, family.IPv4Unicast)
	assert.False(t, ok)
}

// TestFamilyMatchOnly validates the emptied check behind whole-UPDATE suppression.
//
// VALIDATES: AC-2/AC-3 -- pure MP UPDATE of the family empties (suppress); a
// legacy NLRI tail, a withdrawn route or another MP family keeps the UPDATE.
// PREVENTS: suppressing an UPDATE that still carries other routes, or sending
// one the strip left with no NLRI.
func TestFamilyMatchOnly(t *testing.T) {
	flow, _ := family.LookupFamily("ipv4/flow")
	matched, only, ok := familyMatch(buildUpdate(mpFlowReachAttr(), nil), flow)
	require.True(t, ok)
	assert.True(t, matched)
	assert.True(t, only, "MP-only flowspec UPDATE")

	attrs := append(append([]byte{}, originAttr...), mpFlowReachAttr()...)
	matched, only, ok = familyMatch(buildUpdate(attrs, []byte{24, 10, 0, 0}), flow)
	require.True(t, ok)
	assert.True(t, matched)
	assert.False(t, only, "legacy NLRI tail remains")

	withdrawn := append([]byte{0x00, 0x04, 24, 10, 0, 0}, buildUpdate(mpFlowReachAttr(), nil)[2:]...)
	matched, only, ok = familyMatch(withdrawn, flow)
	require.True(t, ok)
	assert.True(t, matched)
	assert.False(t, only, "withdrawn routes remain")

	two := append(append([]byte{}, mpFlowReachAttr()...), mpIPv6UnreachAttr()...)
	matched, only, ok = familyMatch(buildUpdate(two, nil), flow)
	require.True(t, ok)
	assert.True(t, matched)
	assert.False(t, only, "another MP family remains")

	_, _, ok = familyMatch(buildUpdate([]byte{0x80, 14, 2, 0x00, 0x01}, nil), flow)
	assert.False(t, ok, "an MP attribute too short for AFI and SAFI is malformed")
}

// mpIPv6UnreachAttr builds an MP_UNREACH_NLRI (code 15) withdrawing
// 2001:db8::/32 in ipv6/unicast.
func mpIPv6UnreachAttr() []byte {
	val := []byte{0x00, 0x02, 0x01, 32, 0x20, 0x01, 0x0d, 0xb8}
	return append([]byte{0x80, 15, byte(len(val))}, val...)
}
