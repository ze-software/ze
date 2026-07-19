package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// TestASPAVerifyValid verifies all hops authorized yields Valid.
//
// VALIDATES: AC-2 — route with all authorized providers -> Valid.
// PREVENTS: Valid paths incorrectly classified.
func TestASPAVerifyValid(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2 negative -- a path with no AS_SET
	// runs the normal upstream verification algorithm (here yielding Valid), rather than being
	// short-circuited to Unknown.
	c := NewASPACache()
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
	c := NewASPACache()
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
	c := NewASPACache()
	// Path: 100 -> 200 -> 300
	// No ASPA records at all.

	state := verifyASPA(c, []uint32{100, 200, 300})
	assert.Equal(t, ASPAUnknown, state)
}

// TestASPAVerifyASSet verifies AS_SET in path yields Unknown.
//
// VALIDATES: AC-9 — AS_SET -> Unknown.
// PREVENTS: Attempting to verify unordered sets.
func TestASPAVerifyASSet(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2 positive -- an AS_SET in the path is
	// flagged as unverifiable by normalization (the signal handleStructuredUpdate maps to Unknown).
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASSet, ASNs: []uint32{300, 400}},
	}

	_, hasASSet := normalizeASPath(segments)
	assert.True(t, hasASSet)
}

// TestASPAVerifySingleHop verifies single-hop path is trivially Valid.
//
// VALIDATES: Single-hop = nothing to verify -> Valid.
// PREVENTS: False Invalid on direct peering.
func TestASPAVerifySingleHop(t *testing.T) {
	c := NewASPACache()

	state := verifyASPA(c, []uint32{100})
	assert.Equal(t, ASPAValid, state)
}

// TestASPAVerifyEmptyPath verifies empty path is Valid.
//
// VALIDATES: Empty AS_PATH (IBGP/local) -> Valid.
// PREVENTS: Panic on nil/empty path.
func TestASPAVerifyEmptyPath(t *testing.T) {
	c := NewASPACache()

	assert.Equal(t, ASPAValid, verifyASPA(c, nil))
	assert.Equal(t, ASPAValid, verifyASPA(c, []uint32{}))
}

// TestASPANormalizePrepends verifies consecutive duplicate removal.
//
// VALIDATES: [A,A,B,B,B] -> [A,B]; [A,B,A] unchanged.
// PREVENTS: Incorrect prepend handling (must be consecutive-only).
func TestASPANormalizePrepends(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3 positive -- consecutive duplicate
	// ASNs (prepending artifacts) are collapsed: [100,100,200,200,200] -> [100,200].
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3 negative -- non-consecutive
	// duplicates are preserved: [100,200,100] is left unchanged (only consecutive dups collapse).
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 100, 200, 200, 200}},
	}
	path, hasSet := normalizeASPath(segments)
	assert.False(t, hasSet)
	assert.Equal(t, []uint32{100, 200}, path)

	// Non-consecutive duplicates must NOT be removed.
	segments = []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200, 100}},
	}
	path, hasSet = normalizeASPath(segments)
	assert.False(t, hasSet)
	assert.Equal(t, []uint32{100, 200, 100}, path)
}

// TestASPANormalizeConfed verifies confederation segments are stripped.
//
// VALIDATES: AS_CONFED_SEQUENCE stripped, AS_CONFED_SET yields Unknown.
// PREVENTS: Confederation-internal hops affecting verification.
func TestASPANormalizeConfed(t *testing.T) {
	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASConfedSequence, ASNs: []uint32{65001, 65002}},
		{Type: attribute.ASSequence, ASNs: []uint32{300}},
	}
	path, hasSet := normalizeASPath(segments)
	assert.False(t, hasSet)
	assert.Equal(t, []uint32{100, 200, 300}, path)

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
	c := NewASPACache()
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
	c := NewASPACache()
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

// TestASPAStateForPath verifies the received-UPDATE ASPA entry point: it normalizes a route's
// AS_PATH segments and either verifies them or maps an AS_SET to Unknown. This is the exact logic
// handleStructuredUpdate applies to every received UPDATE that carries an AS_PATH.
//
// VALIDATES: aspaStateForPath runs verification on received customer/lateral-peer paths and maps
// AS_SET to Unknown.
// PREVENTS: Received routes bypassing ASPA verification, or AS_SET paths being verified as if
// ordered.
func TestASPAStateForPath(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-1 positive -- ASPA verification is run
	// for a received route's AS_PATH: a customer/lateral-peer path with authorized providers
	// resolves to Valid via the same entry point handleStructuredUpdate uses.
	c := NewASPACache()
	// Path 100 -> 200 -> 300: 200 authorizes 100, 300 authorizes 200.
	c.Set(200, []uint32{100})
	c.Set(300, []uint32{200})

	segments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200, 300}},
	}
	state, normalized := aspaStateForPath(c, segments)
	assert.Equal(t, ASPAValid, state)
	assert.Equal(t, []uint32{100, 200, 300}, normalized)

	// An unauthorized hop on a received path resolves to Invalid (verification actually runs).
	badCache := NewASPACache()
	badCache.Set(200, []uint32{100})
	badCache.Set(300, []uint32{999}) // 200 not authorized
	state, _ = aspaStateForPath(badCache, segments)
	assert.Equal(t, ASPAInvalid, state)

	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2 positive -- an AS_SET in a received
	// path is mapped to Unknown (unverifiable) instead of being run through verification.
	setSegments := []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
		{Type: attribute.ASSet, ASNs: []uint32{300, 400}},
	}
	state, _ = aspaStateForPath(c, setSegments)
	assert.Equal(t, ASPAUnknown, state)
}
