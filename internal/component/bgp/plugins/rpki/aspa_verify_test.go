package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestASPAVerifyValid verifies all hops authorized yields Valid.
//
// VALIDATES: AC-2 — route with all authorized providers -> Valid.
// PREVENTS: Valid paths incorrectly classified.
func TestASPAVerifyValid(t *testing.T) {
	c := newASPACache()
	// Path: 100 -> 200 -> 300 (neighbor=100, origin=300)
	// 200 authorizes 100 as provider, 300 authorizes 200 as provider.
	c.Set(200, []uint32{100})
	c.Set(300, []uint32{200})

	state := verifyASPA(c, []uint32{100, 200, 300})
	assert.Equal(t, ASPAValid, state)
}

// TestASPAVerifyInvalid verifies unauthorized hop yields Invalid.
//
// VALIDATES: AC-3 — route with unauthorized provider -> Invalid.
// PREVENTS: Unauthorized hops accepted as valid.
func TestASPAVerifyInvalid(t *testing.T) {
	c := newASPACache()
	// Path: 100 -> 200 -> 300
	// 200 authorizes 100, but 300 has ASPA and does NOT authorize 200.
	c.Set(200, []uint32{100})
	c.Set(300, []uint32{999}) // 200 not in provider set

	state := verifyASPA(c, []uint32{100, 200, 300})
	assert.Equal(t, ASPAInvalid, state)
}

// TestASPAVerifyUnknown verifies missing ASPA records yields Unknown.
//
// VALIDATES: AC-4 — route with no ASPA coverage -> Unknown.
// PREVENTS: Routes without ASPA data incorrectly marked Valid or Invalid.
func TestASPAVerifyUnknown(t *testing.T) {
	c := newASPACache()
	// Path: 100 -> 200 -> 300
	// No ASPA records at all.

	state := verifyASPA(c, []uint32{100, 200, 300})
	assert.Equal(t, ASPAUnknown, state)
}

// TestASPAVerifyASSet checks the Invalid outcome in Section 5.5 step 3.
func TestASPAVerifyASSet(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 positive -- a path carrying an AS_SET after an AS_SEQUENCE resolves to Invalid, not Unknown, under the upstream procedure.
	// An AS_SET path resolves to Invalid rather than Unknown.
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASSet, ASNs: []uint32{300, 400}},
	}

	state, _ := aspaStateForPath(newASPACache(), segments, aspaUpstream)
	assert.Equal(t, ASPAInvalid, state)
}

// TestASPAVerifySingleHop verifies single-hop path is trivially Valid.
//
// VALIDATES: Single-hop = nothing to verify -> Valid.
// PREVENTS: False Invalid on direct peering.
func TestASPAVerifySingleHop(t *testing.T) {
	c := newASPACache()

	state := verifyASPA(c, []uint32{100})
	assert.Equal(t, ASPAValid, state)
}

// TestASPAVerifyEmptyPath checks the Invalid outcome of Section 5.5 step 1.
func TestASPAVerifyEmptyPath(t *testing.T) {
	c := newASPACache()

	assert.Equal(t, ASPAInvalid, verifyASPA(c, nil))
	assert.Equal(t, ASPAInvalid, verifyASPA(c, []uint32{}))
}

// TestASPANormalizeConfed rejects confederation segments on the external path.
func TestASPANormalizeConfed(t *testing.T) {
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASConfedSequence, ASNs: []uint32{65001, 65002}},
		{Type: attribute.ASSequence, ASNs: []uint32{300}},
	}
	path, hasSet := normalizeASPath(segments)
	assert.True(t, hasSet)
	assert.Nil(t, path)

	// AS_CONFED_SET -> has AS_SET flag.
	segments = []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100}},
		{Type: attribute.ASConfedSet, ASNs: []uint32{65001}},
	}
	_, hasSet = normalizeASPath(segments)
	assert.True(t, hasSet)
}

// TestASPAVerifyMixedHops verifies partial ASPA coverage.
//
// VALIDATES: One "No Attestation" + all others "Provider+" -> Unknown.
// PREVENTS: Unknown hops overridden by valid ones.
func TestASPAVerifyMixedHops(t *testing.T) {
	c := newASPACache()
	// Path: 100 -> 200 -> 300
	// 200 has ASPA: 100 is provider (Provider+).
	// 300 has no ASPA (No Attestation).
	c.Set(200, []uint32{100})

	state := verifyASPA(c, []uint32{100, 200, 300})
	assert.Equal(t, ASPAUnknown, state)
}

// TestASPAVerifyInvalidStopsEarly verifies Invalid returned on first unauthorized hop.
//
// VALIDATES: Algorithm short-circuits on first "Not Provider+".
// PREVENTS: Continuing verification past a known-bad hop.
func TestASPAVerifyInvalidStopsEarly(t *testing.T) {
	c := newASPACache()
	// Path: 100 -> 200 -> 300 -> 400
	// 200 has ASPA but does NOT authorize 100. (Not Provider+)
	// 300 and 400 also have ASPA records.
	c.Set(200, []uint32{999})
	c.Set(300, []uint32{200})
	c.Set(400, []uint32{300})

	state := verifyASPA(c, []uint32{100, 200, 300, 400})
	assert.Equal(t, ASPAInvalid, state)
}

// TestASPAStateString verifies state-to-JSON-string conversion.
//
// VALIDATES: AC-6 — aspa-state field values.
// PREVENTS: Wrong string values in event JSON.
func TestASPAStateString(t *testing.T) {
	assert.Equal(t, "valid", aspaStateString(ASPAValid))
	assert.Equal(t, "invalid", aspaStateString(ASPAInvalid))
	assert.Equal(t, "unknown", aspaStateString(ASPAUnknown))
	assert.Equal(t, "unknown", aspaStateString(255))
}

// TestASPANormalizeMultipleSequences verifies normalization across multiple AS_SEQUENCE segments.
//
// VALIDATES: Consecutive duplicates removed across segment boundaries.
// PREVENTS: Prepend artifacts surviving normalization at segment joins.
func TestASPANormalizeMultipleSequences(t *testing.T) {
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASSequence, ASNs: []uint32{200, 300}},
	}
	path, hasSet := normalizeASPath(segments)
	assert.False(t, hasSet)
	assert.Equal(t, []uint32{100, 200, 300}, path)
}

// TestASPANormalizeEmptySegments verifies empty segments produce empty path.
//
// VALIDATES: No panic on empty segments.
// PREVENTS: Nil pointer dereference on empty AS_PATH.
func TestASPANormalizeEmptySegments(t *testing.T) {
	path, hasSet := normalizeASPath(nil)
	assert.False(t, hasSet)
	assert.Nil(t, path)

	path, hasSet = normalizeASPath([]attribute.ASPathSegment{})
	assert.False(t, hasSet)
	assert.Nil(t, path)
}

// TestASPAStateForPath checks authorized and unauthorized ordered paths against
// the same structural entry point used for received UPDATEs.
func TestASPAStateForPath(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 negative -- the same authorized hops in one AS_SEQUENCE resolve to Valid, so only the AS_SET halts with Invalid.
	// An authorized ordered path does not take the AS_SET Invalid outcome.
	c := newASPACache()
	// Path 100 -> 200 -> 300: 200 authorizes 100, 300 authorizes 200.
	c.Set(200, []uint32{100})
	c.Set(300, []uint32{200})

	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200, 300}},
	}
	state, normalized := aspaStateForPath(c, segments, aspaUpstream)
	assert.Equal(t, ASPAValid, state)
	assert.Equal(t, []uint32{100, 200, 300}, normalized)

	// An unauthorized hop on a received path resolves to Invalid (verification actually runs).
	badCache := newASPACache()
	badCache.Set(200, []uint32{100})
	badCache.Set(300, []uint32{999}) // 200 not authorized
	state, _ = aspaStateForPath(badCache, segments, aspaUpstream)
	assert.Equal(t, ASPAInvalid, state)

	setSegments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASSet, ASNs: []uint32{300, 400}},
	}
	state, _ = aspaStateForPath(c, setSegments, aspaUpstream)
	assert.Equal(t, ASPAInvalid, state)
}

// TestASPAZeroDoesNotAuthorizeOrInvalidate checks the AS0 sentinel at the
// verification boundary, where SPAS is the union of valid ASPA records.
func TestASPAZeroDoesNotAuthorizeOrInvalidate(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1 positive -- a SPAS listing AS 0 beside the real provider verifies the path Valid, as the provider alone does.
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1 negative -- AS 0 is no wildcard: a SPAS holding only AS 0 leaves the hop unauthorized and the path Invalid.
	// A mixed AS0 does not invalidate a real authorization; AS0 alone does not
	// authorize an unlisted provider (Section 4).
	c := newASPACache()
	c.Set(200, []uint32{100})
	c.Set(300, []uint32{0, 200})
	assert.Equal(t, ASPAValid, verifyASPA(c, []uint32{100, 200, 300}))
	c.Set(300, []uint32{0})
	assert.Equal(t, ASPAInvalid, verifyASPA(c, []uint32{100, 200, 300}))
}

// TestASPADownstreamRampBounds checks the distinction between a peer edge, a
// proven valley, and an unproven gap in provider data.
func TestASPADownstreamRampBounds(t *testing.T) {
	c := newASPACache()
	c.Set(100, []uint32{0})
	c.Set(200, []uint32{0})
	c.Set(300, []uint32{0})
	assert.Equal(t, ASPAValid, verifyASPADownstream(c, []uint32{100, 200}),
		"a two-AS path may be the peer edge between the two ramps")
	assert.Equal(t, ASPAInvalid, verifyASPADownstream(c, []uint32{100, 200, 300}),
		"both ramps stop before covering a three-AS path")
	assert.Equal(t, ASPAUnknown, verifyASPADownstream(newASPACache(), []uint32{100, 200, 300}),
		"missing attestations are not proof of a valley")
	assert.Equal(t, ASPAInvalid, verifyASPADownstream(c, nil))
}
