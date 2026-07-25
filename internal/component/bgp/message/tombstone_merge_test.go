package message

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attrCodeAttrDiscard is a test-only alias for the single canonical ATTR_TOMBSTONE code
// point, attribute.AttrTombstone (252). The message tier's own second constant (253) was
// deleted when the code point was unified (spec-fixit-tombstone-code-point-split); the
// pre-existing tests in attr_discard_test.go reference this name, so it is retained here as
// an alias to the one canonical value, keeping those tests tracking the unified code. It is
// not itself a code-point declaration (it derives from attribute.AttrTombstone), and it is
// kept rather than renamed in attr_discard_test.go because that file is a hook-protected
// RFC-tagged test that must not be edited without user approval.
const attrCodeAttrDiscard = uint8(attribute.AttrTombstone)

// TestExtractUpstreamFindsZeOwnEgressMarker proves the upstream-merge search recognizes a
// marker stamped with the single canonical ATTR_TOMBSTONE code point
// (attribute.AttrTombstone, 252) -- the code ze's own egress path (wireu.WriteTombstone)
// writes on the wire.
//
// Before the code-point unification, the message tier searched code 253 only, so a marker
// a ze speaker wrote via WriteTombstone (252) was invisible to a second ze speaker's
// upstream merge, and draft-mangin-idr-attr-tombstone-00 Section 5.1's merge rule silently
// failed against ze's own egress marker. This is the LIVE bug (spec AC-2): the test is RED
// against the split (252 marker not found by a 253 search) and GREEN after unification.
//
// VALIDATES: AC-2 -- ExtractUpstreamAttrDiscard returns the (code, reason) pairs of a
// marker written with attribute.AttrTombstone.
func TestExtractUpstreamFindsZeOwnEgressMarker(t *testing.T) {
	// A marker as wireu.WriteTombstone stamps it for a discarded LOCAL_PREF (code 5,
	// reason 1 = EBGP invalid): flags 0xC0, code attribute.AttrTombstone (252),
	// value[0]=original code, value[1]=reason.
	marker := makeAttr(0xC0, byte(attribute.AttrTombstone), []byte{0x05, DiscardReasonEBGPInvalid})
	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),          // ORIGIN
		makeAttr(0x40, 2, []byte{}),              // AS_PATH
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}), // NEXT_HOP
		marker,
	)

	found := ExtractUpstreamAttrDiscard(pathAttrs)
	require.Len(t, found, 1, "ze's own egress marker (attribute.AttrTombstone/252) must be found by the merge search")
	assert.Equal(t, uint8(5), found[0].Code, "original code preserved")
	assert.Equal(t, DiscardReasonEBGPInvalid, found[0].Reason, "reason preserved")
}

// TestApplyAttrDiscardMergesUpstream proves an upstream ATTR_TOMBSTONE marker written with
// the unified code (252) plus a fresh local discard produce a SINGLE merged marker via the
// rebuild path (draft-mangin-idr-attr-tombstone-00 Section 5.1: remove the upstream marker
// and locally-discarded attributes, then insert one marker with upstream pairs followed by
// local pairs).
//
// Before unification the 252 upstream marker was invisible to the 253-only search, so no
// merge happened: ApplyAttrDiscard took the in-place path for the single local entry and
// LEFT the upstream 252 marker in place, producing two markers. The test is RED against the
// split (rebuilt == false, two markers) and GREEN after unification (rebuilt == true, one
// merged marker).
//
// VALIDATES: AC-3 -- rebuild-and-merge against ze's own unified-code upstream marker.
func TestApplyAttrDiscardMergesUpstream(t *testing.T) {
	// Upstream marker (non-transitive) recording a discarded AIGP (code 0x1A, reason 3).
	upstream := makeAttr(0x80, byte(attribute.AttrTombstone), []byte{0x1A, DiscardReasonMalformedValue})
	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),                   // ORIGIN
		makeAttr(0x40, 2, []byte{}),                       // AS_PATH
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),          // NEXT_HOP
		upstream,                                          // upstream ATTR_TOMBSTONE (code 252)
		makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // ORIGINATOR_ID (discarded locally)
	)
	entries := []DiscardEntry{{Code: 9, Reason: DiscardReasonEBGPInvalid}}

	result, rebuilt := ApplyAttrDiscard(pathAttrs, entries)
	require.True(t, rebuilt, "an upstream marker ze itself could have written must force the merge/rebuild path")

	found := ExtractUpstreamAttrDiscard(result)
	require.Len(t, found, 2, "upstream + local merged into one marker")
	assert.Equal(t, uint8(0x1A), found[0].Code, "upstream pair first")
	assert.Equal(t, DiscardReasonMalformedValue, found[0].Reason)
	assert.Equal(t, uint8(9), found[1].Code, "local pair second")
	assert.Equal(t, DiscardReasonEBGPInvalid, found[1].Reason)

	assert.Equal(t, 1, countAttrByCode(result, byte(attribute.AttrTombstone)),
		"exactly one merged ATTR_TOMBSTONE on the wire (RFC 4271 Section 5)")
}
