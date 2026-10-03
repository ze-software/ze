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

// localPrefSource is one relayed UPDATE as an INTERNAL peer sends it: ORIGIN,
// LOCAL_PREF and a community, so the rebuilt section has neighbors on both
// sides of code 5 and a strip that took the wrong span would move them.
type localPrefSource struct {
	payload   []byte
	origin    []byte
	localPref []byte
	community []byte
	nlri      []byte
}

func newLocalPrefSource() localPrefSource {
	s := localPrefSource{
		origin:    makeAttr(0x40, 1, []byte{0x00}),                   // IGP
		localPref: makeAttr(0x40, 5, []byte{0x00, 0x00, 0x00, 0xC8}), // 200
		community: makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01}), // 100:1
		nlri:      []byte{24, 10, 0, 0},                              // 10.0.0.0/24
	}
	s.payload = buildModTestPayload(slices.Concat(s.origin, s.localPref, s.community), s.nlri)
	return s
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-13, AC-14, AC-15 --
// the forward rails strip LOCAL_PREF toward an EXTERNAL destination and leave it
// alone toward an internal one.
//
// RFC requirement: RFC4271-5.1.5-2 positive -- an UPDATE relayed to an external
// peer carries no LOCAL_PREF. applyFactsLocalPref (forward_local_pref.go)
// records the Suppress and genericAttrSetHandler drops the attribute, so the
// rebuilt section holds ORIGIN and COMMUNITY only.
// RFC requirement: RFC4271-5.1.5-1 negative -- the strip is confined to external
// destinations. The same source relayed to an internal peer keeps LOCAL_PREF,
// byte-identical, which is what makes the positive case above non-vacuous: a
// change that dropped code 5 everywhere fails this half.
//
// PREVENTS: the live MUST NOT violation this test pins. Both forward rails
// relayed the source attribute block verbatim for code 5, so a route LEARNED
// from an internal peer and RELAYED to an external one carried the internal
// preference across the AS boundary -- while the SAME prefix originated locally
// did not, because buildAnnounceUpdate (reactor_api_batch.go) and
// writeAnnounceUpdateWithPlan (reactor_wire.go) both stripped it. The two answers
// now come from one predicate, localPrefAllowedTo.
func TestForwardLocalPrefStrippedToExternalPeer(t *testing.T) {
	s := newLocalPrefSource()

	t.Run("external-destination-drops-it", func(t *testing.T) {
		facts := &peerForwardFacts{isEBGP: true}

		var mods filterapi.ModAccumulator
		applyFactsLocalPref(facts, payloadHasLocalPref(s.payload), &mods)
		require.Equal(t, 1, mods.Len(), "an external destination must record the Section 5.1.5 strip")

		result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result, "the strip requires a rebuild")

		want := buildModTestPayload(slices.Concat(s.origin, s.community), s.nlri)
		assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
			"RFC 4271 Section 5.1.5: LOCAL_PREF must not reach an external peer")

		got := rebuiltAttrs(t, result)
		assert.NotContains(t, got, byte(attribute.AttrLocalPref))
		assert.Contains(t, got, byte(8), "only LOCAL_PREF was stripped")
	})

	t.Run("internal-destination-keeps-it", func(t *testing.T) {
		facts := &peerForwardFacts{isEBGP: false}

		var mods filterapi.ModAccumulator
		applyFactsLocalPref(facts, payloadHasLocalPref(s.payload), &mods)
		assert.Zero(t, mods.Len(), "an internal destination is owed the attribute, not a strip")

		result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		assert.Equal(t, modifyFailureNone, fail)
		assert.Nil(t, result, "no operation means the route stays on the zero-copy path")
	})

	t.Run("external-destination-without-the-attribute-records-nothing", func(t *testing.T) {
		// A route that never carried LOCAL_PREF is already conformant. Recording
		// the operation anyway would force every route to every external peer
		// onto the rebuild path, which is the cost the route-server fast path
		// exists to avoid.
		payload := buildModTestPayload(slices.Concat(s.origin, s.community), s.nlri)
		facts := &peerForwardFacts{isEBGP: true}

		var mods filterapi.ModAccumulator
		applyFactsLocalPref(facts, payloadHasLocalPref(payload), &mods)
		assert.Zero(t, mods.Len(), "nothing to strip must cost nothing")

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		assert.Equal(t, modifyFailureNone, fail)
		assert.Nil(t, result)
	})
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-16 -- an egress filter
// cannot put LOCAL_PREF on an external peer's wire.
//
// RFC requirement: RFC4271-5.1.5-2 positive -- the prohibition beats a filter's
// Set. LLGREgressFilter (plugins/gr/gr_egress.go) sets LOCAL_PREF=0 for RFC 9494
// Section 4.6, a policy chain can set any value, and applyFactsLocalPref runs
// after both, so filterapi.LastSetOrSuppress makes the Suppress the last word.
//
// PREVENTS: a strip gated only on the SOURCE payload. A filter that creates the
// attribute on a route whose source carried none is invisible to that check, and
// the value would reach the external peer through the operation list instead of
// through the wire bytes.
func TestForwardLocalPrefStripBeatsAFilterSet(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(origin, nlri)
	facts := &peerForwardFacts{isEBGP: true}

	var mods filterapi.ModAccumulator
	mods.Op(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0x00, 0x00, 0x00, 0x00})
	applyFactsLocalPref(facts, payloadHasLocalPref(payload), &mods)
	require.Equal(t, 2, mods.Len(), "the filter's Set stays recorded; the strip is added after it")

	result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result)

	got := rebuiltAttrs(t, result)
	assert.NotContains(t, got, byte(attribute.AttrLocalPref),
		"RFC 4271 Section 5.1.5 is not a policy an egress filter may override")
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-17 -- one predicate
// answers RFC 4271 Section 5.1.5 for every egress rail, and it carries the
// confederation exception.
//
// RFC requirement: RFC4271-5.1.5-2 negative -- the prohibition is not applied to
// an internal session. localPrefAllowedTo is the ONE site the RFC 3065
// confederation exception grows from; a second copy of "isIBGP" somewhere else
// is how the announce rail and the forward rail came to disagree.
func TestLocalPrefAllowedToIsTheOnlyAnswer(t *testing.T) {
	assert.True(t, localPrefAllowedTo(true), "an internal peer is owed LOCAL_PREF (Section 5.1.5, the SHALL half)")
	assert.False(t, localPrefAllowedTo(false), "an external peer must never receive it (Section 5.1.5, the MUST NOT half)")
}

// VALIDATES: payloadHasLocalPref reads the attribute SECTION, not the payload
// bytes, so a prefix in the NLRI or a withdrawn route that happens to hold the
// byte 0x05 cannot be read as the attribute.
// PREVENTS: a presence check written as bytes.IndexByte, which would answer yes
// for 10.5.0.0/24 and force a pointless rebuild, and no for an UPDATE whose
// LOCAL_PREF sits behind a withdrawn-routes section it never skipped.
func TestPayloadHasLocalPref(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	localPref := makeAttr(0x40, 5, []byte{0x00, 0x00, 0x00, 0x64})

	assert.True(t, payloadHasLocalPref(buildModTestPayload(slices.Concat(origin, localPref), []byte{24, 10, 0, 0})))
	// 10.5.0.0/24 puts a 0x05 in the NLRI and none in the attribute section.
	assert.False(t, payloadHasLocalPref(buildModTestPayload(origin, []byte{24, 10, 5, 0})))
	assert.False(t, payloadHasLocalPref(nil), "a payload too short to parse has no attribute to strip")
	assert.False(t, payloadHasLocalPref([]byte{0x00}), "a truncated payload has no attribute to strip")
}
