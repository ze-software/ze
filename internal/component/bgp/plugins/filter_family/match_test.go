package filter_family

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
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

// TestStripMPAttrs validates the MP_REACH/MP_UNREACH surgery.
//
// VALIDATES: AC-2/AC-3 -- pure MP UPDATE empties (suppress); mixed keeps legacy NLRI.
// PREVENTS: removing the wrong attribute or corrupting the legacy NLRI tail.
func TestStripMPAttrs(t *testing.T) {
	// Pure MP-only flowspec: stripping the MP attr empties the UPDATE.
	_, emptied, ok := stripMPAttrs(buildUpdate(mpFlowReachAttr(), nil))
	require.True(t, ok)
	assert.True(t, emptied)

	// Mixed: ORIGIN + MP flowspec + legacy NLRI. Strip MP, keep ORIGIN and NLRI.
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	attrs := append(append([]byte{}, originAttr...), mpFlowReachAttr()...)
	out, emptied, ok := stripMPAttrs(buildUpdate(attrs, nlri))
	require.True(t, ok)
	assert.False(t, emptied)

	// The MP family must be gone, the legacy NLRI preserved.
	stillHasMP := familyFromMPOnly(out)
	assert.False(t, stillHasMP, "MP_REACH must be removed")
	assert.Equal(t, nlri, legacyNLRI(t, out), "legacy NLRI must be intact")
}

// familyFromMPOnly reports whether the payload still carries an MP family.
func familyFromMPOnly(payload []byte) bool {
	_, attrStart, attrEnd, ok := payloadSections(payload)
	if !ok {
		return false
	}
	_, found := message.ExtractMPFamily(payload[attrStart:attrEnd])
	return found
}

// legacyNLRI returns the legacy IPv4 NLRI tail of an UPDATE body.
func legacyNLRI(t *testing.T, payload []byte) []byte {
	t.Helper()
	_, _, attrEnd, ok := payloadSections(payload)
	require.True(t, ok)
	return payload[attrEnd:]
}
