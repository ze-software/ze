package reactor

import (
	"encoding/hex"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// medSource is one relayed UPDATE as a neighboring AS sends it: ORIGIN, MED and
// a community, so the rebuilt section has neighbors on both sides of code 4 and
// a strip that took the wrong span would move them.
type medSource struct {
	payload   []byte
	origin    []byte
	med       []byte
	community []byte
	nlri      []byte
}

func newMEDSource(metric []byte) medSource {
	s := medSource{
		origin:    makeAttr(0x40, 1, []byte{0x00}),                   // IGP
		med:       makeAttr(0x80, 4, metric),                         // MULTI_EXIT_DISC
		community: makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01}), // 100:1
		nlri:      []byte{24, 10, 0, 0},                              // 10.0.0.0/24
	}
	s.payload = buildModTestPayload(slices.Concat(s.origin, s.med, s.community), s.nlri)
	return s
}

// VALIDATES: spec-rfc4271-med-across-as AC-1, AC-4 -- a MULTI_EXIT_DISC received
// from one neighboring AS does not reach another, and the same route reaching an
// internal peer keeps it.
//
// RFC requirement: RFC4271-5.1.4-1 positive -- "The MULTI_EXIT_DISC attribute
// received from a neighboring AS MUST NOT be propagated to other neighboring
// ASes" (Section 5.1.4). applyFactsMED (forward_med.go) records the Suppress and
// genericAttrSetHandler drops the attribute, so the rebuilt section holds ORIGIN
// and COMMUNITY only.
// RFC requirement: RFC4271-5.1.4-1 negative -- the strip is confined to external
// destinations. RFC 4271 Section 5.1.4 permits the metric within the AS ("MAY be
// propagated over IBGP to other BGP speakers within the same AS"), and the same
// source relayed to an internal peer keeps it byte-identical. That half is what
// makes the positive non-vacuous: a change that dropped code 4 everywhere fails
// it.
//
// PREVENTS: the live MUST NOT violation this spec closes. Both forward rails
// relayed the source attribute block verbatim for code 4, so a route learned
// from one transit provider and readvertised to another carried the first
// provider's internal metric into a network that was never meant to see it.
func TestForwardSuppressesReceivedMEDToAnotherAS(t *testing.T) {
	// Boundary: 0 is a value, not an absence, and 2^32-1 is the other end.
	for _, tc := range []struct {
		name   string
		metric []byte
	}{
		{"metric-100", []byte{0x00, 0x00, 0x00, 0x64}},
		{"metric-zero", []byte{0x00, 0x00, 0x00, 0x00}},
		{"metric-max", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMEDSource(tc.metric)
			src := payloadMED(s.payload)

			t.Run("external-destination-drops-it", func(t *testing.T) {
				facts := &peerForwardFacts{isEBGP: true}

				var mods filterapi.ModAccumulator
				applyFactsMED(facts, src, src, &mods)
				require.Equal(t, 1, mods.Len(), "an external destination must record the Section 5.1.4 strip")

				result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
				require.Equal(t, modifyFailureNone, fail)
				require.NotNil(t, result, "the strip requires a rebuild")

				want := buildModTestPayload(slices.Concat(s.origin, s.community), s.nlri)
				assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
					"RFC 4271 Section 5.1.4: a received MED must not reach another neighboring AS")

				got := rebuiltAttrs(t, result)
				assert.NotContains(t, got, byte(attribute.AttrMED))
				assert.Contains(t, got, byte(8), "only MED was stripped")
			})

			t.Run("internal-destination-keeps-it", func(t *testing.T) {
				facts := &peerForwardFacts{isEBGP: false}

				var mods filterapi.ModAccumulator
				applyFactsMED(facts, src, src, &mods)
				assert.Zero(t, mods.Len(), "an internal destination is owed the metric, not a strip")

				result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
				assert.Equal(t, modifyFailureNone, fail)
				assert.Nil(t, result, "no operation means the route stays on the zero-copy path")
			})
		})
	}
}

// VALIDATES: spec-rfc4271-med-across-as AC-2 -- a metric Ze originates toward a
// peer still reaches it. Section 5.1.4 governs a RECEIVED value only.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the prohibition does not reach a
// locally originated metric. A route whose source carried no MULTI_EXIT_DISC has
// nothing received to leak, so a metric on this destination's base was put there
// here and applyFactsMED records nothing.
//
// PREVENTS: the blanket suppression of attribute 4 on external egress. That
// design satisfies the MUST NOT above and breaks MULTI_EXIT_DISC as a feature:
// an operator who steers inbound traffic by advertising different metrics to two
// providers would advertise none. The announce rails carry the other half of the
// same guarantee and never reach this function (writeAnnounceUpdate,
// reactor_wire.go; buildRIBRouteUpdate, peer_rib_routes.go).
func TestForwardWritesLocallySetMED(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	nlri := []byte{24, 10, 0, 0}
	received := buildModTestPayload(origin, nlri) // no MED from the source
	metric := makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x32})
	originated := buildModTestPayload(slices.Concat(origin, metric), nlri)

	facts := &peerForwardFacts{isEBGP: true}

	var mods filterapi.ModAccumulator
	applyFactsMED(facts, payloadMED(received), payloadMED(originated), &mods)
	assert.Zero(t, mods.Len(), "a metric nobody received is Ze's own, and Section 5.1.4 does not touch it")

	result, _, fail := buildModifiedPayload(originated, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.Equal(t, modifyFailureNone, fail)
	require.Nil(t, result, "no operation: the locally set metric reaches the wire unchanged")
	assert.True(t, payloadMED(originated).present, "the metric is still on the base this destination is sent")
}

// VALIDATES: spec-rfc4271-med-across-as AC-3 -- an egress filter that SETS
// MULTI_EXIT_DISC wins, through either route a filter has to the wire.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the prohibition covers relaying
// somebody else's metric, so an operator originating one is left alone. This is
// the exact reverse of the Section 5.1.5 sibling, where the strip beats a
// filter's Set (TestForwardLocalPrefStripBeatsAFilterSet), and both are correct:
// 5.1.5 prohibits the attribute outright, 5.1.4 prohibits only the relay.
//
// PREVENTS: copying the sibling's precedence. A policy chain that sets a metric
// per destination would never reach the wire, and the failure is silent at both
// ends.
func TestForwardKeepsFilterSetMED(t *testing.T) {
	s := newMEDSource([]byte{0x00, 0x00, 0x00, 0x64})
	src := payloadMED(s.payload)
	facts := &peerForwardFacts{isEBGP: true}

	t.Run("an-accumulator-set-is-honored", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		mods.Op(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0x00, 0x00, 0x00, 0x07})
		applyFactsMED(facts, src, src, &mods)
		require.Equal(t, 1, mods.Len(), "the filter's Set stays alone; no strip is added after it")

		result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		want := buildModTestPayload(slices.Concat(s.origin, makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x07}), s.community), s.nlri)
		assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
			"the operator's metric reaches the peer, not the received one")
	})

	t.Run("a-policy-chain-override-is-honored", func(t *testing.T) {
		// The export policy chain returns a whole payload rather than an
		// operation (runEgressPolicyChainASN4, filter_ordered.go), so the only
		// evidence that it originated a metric is that the base differs from
		// what arrived.
		override := buildModTestPayload(
			slices.Concat(s.origin, makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x07}), s.community), s.nlri)

		var mods filterapi.ModAccumulator
		applyFactsMED(facts, src, payloadMED(override), &mods)
		assert.Zero(t, mods.Len(), "a base carrying a different metric is origination, not propagation")

		result, _, fail := buildModifiedPayload(override, &mods, attrModHandlersWithDefaults(), nil, nil)
		assert.Equal(t, modifyFailureNone, fail)
		assert.Nil(t, result, "the override reaches the wire as the filter wrote it")
	})
}

// VALIDATES: spec-rfc4271-med-across-as R-1 and the RFC 7947 exemption -- a route
// server client keeps the metric its fabric exists to carry.
//
// RFC requirement: RFC7947-x-3 positive -- "if applied to an NLRI UPDATE sent to
// a route server, this attribute SHOULD be propagated to other route server
// clients, and the route server SHOULD NOT modify its value" (RFC 7947 Section
// 2.2.3). An RS client is external, so the Section 5.1.4 rule would otherwise
// strip the metric on every IXP route Ze forwards.
//
// PREVENTS: closing one gated MUST by breaking another. The exemption is gated
// on the exact condition RFC 7947 names (the destination is a route server
// client), never applied more widely (ai/rules/rfc-compliance.md).
func TestForwardKeepsMEDForRouteServerClient(t *testing.T) {
	s := newMEDSource([]byte{0x00, 0x00, 0x00, 0x64})
	src := payloadMED(s.payload)
	facts := &peerForwardFacts{isEBGP: true, rsClient: true}

	var mods filterapi.ModAccumulator
	applyFactsMED(facts, src, src, &mods)
	assert.Zero(t, mods.Len(), "RFC 7947 Section 2.2.3: a route server does not modify the metric")

	result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	assert.Equal(t, modifyFailureNone, fail)
	assert.Nil(t, result, "the client's route stays byte-identical on the fast path")
}

// VALIDATES: spec-rfc4271-med-across-as AC-9 -- a route that needs no metric
// change stays off the payload-rebuild path.
//
// PREVENTS: recording the operation unconditionally. Every route to every
// external peer would then rebuild its payload, which is the cost the
// route-server fast path exists to avoid (R-3). The once-per-UPDATE src the
// callers compute is what makes the question cheap.
func TestForwardMEDStaysOffRebuildPathWhenUnchanged(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	community := makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01})
	payload := buildModTestPayload(slices.Concat(origin, community), []byte{24, 10, 0, 0})
	med := payloadMED(payload)
	require.False(t, med.present, "the fixture carries no metric")

	facts := &peerForwardFacts{isEBGP: true}

	var mods filterapi.ModAccumulator
	applyFactsMED(facts, med, med, &mods)
	assert.Zero(t, mods.Len(), "nothing to strip must cost nothing")

	result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	assert.Equal(t, modifyFailureNone, fail)
	assert.Nil(t, result)
}

// VALIDATES: one predicate answers RFC 4271 Section 5.1.4 for every egress rail,
// and it carries the RFC 7947 exemption.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the prohibition is not applied to
// an internal session, nor to the route server fabric RFC 7947 Section 2.2.3
// exempts. A second copy of "isEBGP" elsewhere is how the announce rail and the
// forward rail came to disagree about LOCAL_PREF.
func TestMEDPropagationAllowedTo(t *testing.T) {
	assert.True(t, medPropagationAllowedTo(false, false), "an internal peer is inside the same AS (Section 5.1.4, the MAY half)")
	assert.False(t, medPropagationAllowedTo(true, false), "another neighboring AS must not see it (Section 5.1.4, the MUST NOT half)")
	assert.True(t, medPropagationAllowedTo(true, true), "a route server client keeps it (RFC 7947 Section 2.2.3)")
	assert.True(t, medPropagationAllowedTo(false, true), "an internal route server client keeps it either way")
}

// VALIDATES: payloadMED reads the attribute SECTION, not the payload bytes, so a
// prefix in the NLRI or a withdrawn route that happens to hold the byte 0x04
// cannot be read as the attribute.
// PREVENTS: a presence check written as bytes.IndexByte, which would answer yes
// for 10.4.0.0/24 and force a pointless rebuild, and no for an UPDATE whose MED
// sits behind a withdrawn-routes section it never skipped.
func TestPayloadMED(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	med := makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x64})

	found := payloadMED(buildModTestPayload(slices.Concat(origin, med), []byte{24, 10, 0, 0}))
	require.True(t, found.present)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x64}, found.raw)

	// 10.4.0.0/24 puts a 0x04 in the NLRI and none in the attribute section.
	assert.False(t, payloadMED(buildModTestPayload(origin, []byte{24, 10, 4, 0})).present)
	assert.False(t, payloadMED(nil).present, "a payload too short to parse has no attribute to strip")
	assert.False(t, payloadMED([]byte{0x00}).present, "a truncated payload has no attribute to strip")

	// A metric of zero is a value. sameAs must not read it as an absence.
	zero := payloadMED(buildModTestPayload(
		slices.Concat(origin, makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x00})), []byte{24, 10, 0, 0}))
	require.True(t, zero.present)
	assert.False(t, zero.sameAs(medValue{}), "zero is not absent")
	assert.False(t, zero.sameAs(found), "zero is not one hundred")
}
