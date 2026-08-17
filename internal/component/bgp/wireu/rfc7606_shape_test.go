package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 5.1, second bullet: "An UPDATE message MUST NOT contain more than one of
// the following: non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
//
// SplitWireUpdate is the sender-side enforcement point for relayed wire bytes. Before this
// file, it returned any payload that FIT unchanged, so a mixed shape received from a peer was
// relayed on verbatim. The shape verdict is cached on the WireUpdate because the same pointer
// is walked once per destination peer in the forward loop.

// VALIDATES: a mixed UPDATE that fits within the message size is still split.
// PREVENTS: the size fast path relaying a received mixed shape unchanged, which is the whole
// point of this change -- reverting the fast-path condition leaves one message here.
//
// RFC requirement: RFC7606-5.1-2 positive -- a relayed UPDATE that mixes NLRI-bearing fields
// is split into compliant messages even when its size needs no split.
func TestSplitWireUpdateSplitsMixedShapeThatFits(t *testing.T) {
	body := mixedUpdateBody()
	require.LessOrEqual(t, len(body), 4096, "guard: the fixture must FIT, so only shape can split it")
	before, which := nlriBearingFields(t, body)
	require.Equal(t, 4, before, "guard: the fixture must start non-compliant, got %v", which)

	wu := NewWireUpdate(body, 0)
	require.True(t, wu.MixesNLRIFields())

	chunks, err := SplitWireUpdate(wu, 4096, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "a mixed UPDATE must split even when it fits")

	counts := map[string]int{}
	for i, c := range chunks {
		n, present := nlriBearingFields(t, c.Payload())
		assert.LessOrEqualf(t, n, 1,
			"chunk %d carries %d NLRI-bearing fields (%v)", i, n, present)
		for _, p := range present {
			counts[p]++
		}
	}
	for _, want := range []string{"withdrawn-routes", "mp-unreach", "mp-reach", "nlri"} {
		assert.Positivef(t, counts[want], "%s vanished when the fitting UPDATE was split", want)
	}
}

// VALIDATES: a compliant UPDATE that fits is returned as the very same WireUpdate.
// PREVENTS: charging the shape check's cost to the common case. Identity is what matters:
// reactor/forward_body.go relies on the returned payload being the received bytes so it can
// append the slice header without copying. An equal-but-copied payload would pass a value
// comparison while having silently lost the zero-copy forward.
//
// RFC requirement: RFC7606-5.1-2 negative -- an UPDATE carrying at most one NLRI-bearing
// field is already compliant and is not split.
func TestSplitWireUpdateCompliantShapeUntouched(t *testing.T) {
	// Announce-only: attributes plus IPv4 NLRI, one NLRI-bearing field.
	payload := buildTestUpdatePayload(nil, []byte{0x40, 0x01, 0x01, 0x00}, []byte{0x18, 0xC0, 0xA8, 0x01})
	wu := NewWireUpdate(payload, 0)
	require.False(t, wu.MixesNLRIFields())

	chunks, err := SplitWireUpdate(wu, 4096, noAddPathCtx)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Same(t, wu, chunks[0], "the fast path must return the original WireUpdate")
}

// VALIDATES: the shape verdict is computed once per received UPDATE, not once per
// destination peer.
// PREVENTS: the forward loop re-walking the same UPDATE's attributes for every peer. A
// route reflector runs this loop once per client, so an uncached walk turns a per-message
// cost into a per-message-times-peers cost.
//
// The assertion is a timing RATIO because neither side allocates: an allocation assertion
// passes whether or not the cache exists, which is how the first version of this test
// managed to prove nothing.
//
// The cold measurement includes construction and the first parse, so the ratio is not the
// cache alone. What discriminates is that removing the cache collapses it: measured here,
// ~95x with the cache (warm 3ns) against ~6x without it (warm 42ns), the cold side being
// ~270ns either way. The threshold sits at 20x, between the two by a wide margin.
func TestWireUpdateMixesNLRIFieldsCachedPerMessage(t *testing.T) {
	body := mixedUpdateBody()

	// Cold: a fresh WireUpdate per iteration, so every call does the work.
	cold := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			wu := NewWireUpdate(body, 0)
			if !wu.MixesNLRIFields() {
				b.Fatal("fixture must be mixed")
			}
		}
	})
	// Warm: one WireUpdate, checked repeatedly -- the shape of the per-destination loop.
	warm := testing.Benchmark(func(b *testing.B) {
		wu := NewWireUpdate(body, 0)
		wu.MixesNLRIFields()
		b.ResetTimer()
		for range b.N {
			if !wu.MixesNLRIFields() {
				b.Fatal("verdict changed between calls")
			}
		}
	})

	require.Positive(t, warm.NsPerOp(), "benchmark produced no measurement")
	assert.Greaterf(t, float64(cold.NsPerOp())/float64(warm.NsPerOp()), 20.0,
		"repeat checks (%dns) are not meaningfully cheaper than first checks (%dns): the "+
			"verdict is being recomputed per destination", warm.NsPerOp(), cold.NsPerOp())
}

// VALIDATES: the two-field boundary, in the package that owns the wire-side comparison.
// PREVENTS: an off-by-one making the check fire only at three fields or more. The mixed
// fixture above carries all four, so it cannot see that mistake.
func TestWireUpdateMixesNLRIFieldsBoundary(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN only
	withdrawn := []byte{0x18, 0x0a, 0x00, 0x00}
	announced := []byte{0x18, 0xc0, 0x00, 0x02}

	one := NewWireUpdate(buildTestUpdatePayload(nil, attrs, announced), 0)
	assert.False(t, one.MixesNLRIFields(), "one NLRI-bearing field is compliant")

	two := NewWireUpdate(buildTestUpdatePayload(withdrawn, attrs, announced), 0)
	assert.True(t, two.MixesNLRIFields(), "two NLRI-bearing fields already violate the MUST NOT")
}

// VALIDATES: End-of-RIB markers are not mixed and stay on the fast path.
// PREVENTS: an EoR being rewritten by the new shape branch, which would break RFC 4724
// graceful restart.
func TestWireUpdateEndOfRIBNotMixed(t *testing.T) {
	ipv4EOR := NewWireUpdate(buildTestUpdatePayload(nil, nil, nil), 0)
	assert.False(t, ipv4EOR.MixesNLRIFields())

	// Multiprotocol EoR: one MP_UNREACH holding only AFI/SAFI.
	mpEOR := NewWireUpdate(buildTestUpdatePayload(nil,
		[]byte{0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}, nil), 0)
	assert.False(t, mpEOR.MixesNLRIFields())

	chunks, err := SplitWireUpdate(mpEOR, 4096, noAddPathCtx)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Same(t, mpEOR, chunks[0])
}

// VALIDATES: a payload whose sections do not parse is not reported as mixed.
// PREVENTS: turning a parse failure into an invented RFC violation. Classifying shape is not
// validating: enforceRFC7606 (reactor/session_read.go:162) has already run on everything the
// forward path sees, and TestSplitWireUpdate_FastPathNoValidation pins that SplitWireUpdate
// itself does not validate.
func TestWireUpdateMalformedNotMixed(t *testing.T) {
	wu := NewWireUpdate([]byte{0x00, 0x05, 0x01}, 0) // claims 5 withdrawn bytes, carries 1
	assert.False(t, wu.MixesNLRIFields())
}
