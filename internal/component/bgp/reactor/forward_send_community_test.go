package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"

	// The community AttrModHandlers this file exercises are registered by
	// filter_community's init(), and the whole point of the test is that
	// PRODUCTION dispatch reaches them. Importing the plugin here rather than
	// leaning on a neighboring test file's import keeps that guarantee owned by
	// the test that depends on it: the defect these tests pin survived because
	// the existing coverage built its own handler instead of using the
	// registered one.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
)

// rebuiltAttrs walks the attribute section of a rebuilt UPDATE payload and
// returns each attribute's FULL wire bytes (flags, code, length, value) keyed by
// type code.
//
// It reads the bytes the writer actually produced. Asserting on recorded
// operations instead is how `send community` shipped broken: the ops were
// correct at every producer and were thrown away by the handler.
func rebuiltAttrs(t *testing.T, payload []byte) map[byte][]byte {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4, "payload too short to hold an attribute section")
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	require.GreaterOrEqual(t, len(payload), attrOffset+2)
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	start := attrOffset + 2
	require.GreaterOrEqual(t, len(payload), start+attrLen)
	section := payload[start : start+attrLen]

	out := make(map[byte][]byte)
	for off := 0; off < len(section); {
		require.LessOrEqual(t, off+3, len(section), "truncated attribute header")
		flags := section[off]
		code := section[off+1]
		var valLen, hdrLen int
		if flags&0x10 != 0 {
			require.LessOrEqual(t, off+4, len(section), "truncated extended-length header")
			valLen = int(binary.BigEndian.Uint16(section[off+2 : off+4]))
			hdrLen = 4
		} else {
			valLen = int(section[off+2])
			hdrLen = 3
		}
		require.LessOrEqual(t, off+hdrLen+valLen, len(section), "attribute runs past section end")
		out[code] = section[off : off+hdrLen+valLen]
		off += hdrLen + valLen
	}
	return out
}

// communitySource is one UPDATE carrying a single value of each community
// attribute, plus an ORIGIN so the section is never empty after a full strip.
type communitySource struct {
	payload   []byte
	origin    []byte
	comm      []byte
	extComm   []byte
	largeComm []byte
	nlri      []byte
}

func newCommunitySource() communitySource {
	s := communitySource{
		origin:  makeAttr(0x40, 1, []byte{0x00}),                                            // IGP
		comm:    makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01}),                          // RFC 1997: 100:1
		extComm: makeAttr(0xC0, 16, []byte{0x00, 0x02, 0x00, 0x64, 0x00, 0x00, 0x00, 0x01}), // RFC 4360: RT 100:1
		largeComm: makeAttr(0xC0, 32, []byte{ // RFC 8092: 100:1:2
			0x00, 0x00, 0x00, 0x64,
			0x00, 0x00, 0x00, 0x01,
			0x00, 0x00, 0x00, 0x02,
		}),
		nlri: []byte{24, 10, 0, 0}, // 10.0.0.0/24
	}
	s.payload = buildModTestPayload(slices.Concat(s.origin, s.comm, s.extComm, s.largeComm), s.nlri)
	return s
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-1, AC-2, AC-3 --
// `session { community { send <list> } }` reaches the WIRE. A suppressed
// community attribute is absent from the rebuilt UPDATE; a permitted one is
// byte-identical to the source.
// PREVENTS: the live fail-open where every AttrModSuppress for codes 8, 16 and
// 32 was consumed and discarded by genericCommunityHandler, so a peer
// configured `send none` still received every community. The existing coverage
// missed it twice: TestGenericAttrSetHandler_Suppress builds a handler
// production never dispatches for these codes, and TestPrecomputeSendCommunity
// asserts recorded ops rather than rebuilt bytes.
// Carries no RFC obligation: the RFC 1997, 4360 and 8092 attributes are all
// optional transitive, so omitting one entirely is legal. This is operator
// policy, not conformance.
func TestSendCommunitySuppressEmittedBytes(t *testing.T) {
	s := newCommunitySource()

	tests := []struct {
		name  string
		send  []string
		want  []byte // the attribute section the peer must receive
		codes []byte // codes that must NOT appear
	}{
		{"none", []string{"none"}, s.origin, []byte{8, 16, 32}},
		{"standard-only", []string{"standard"}, slices.Concat(s.origin, s.comm), []byte{16, 32}},
		{"extended-only", []string{"extended"}, slices.Concat(s.origin, s.extComm), []byte{8, 32}},
		{"large-only", []string{"large"}, slices.Concat(s.origin, s.largeComm), []byte{8, 16}},
		{
			"standard-and-large",
			[]string{"standard", "large"},
			slices.Concat(s.origin, s.comm, s.largeComm),
			[]byte{16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Drive the REAL entry point: the config leaf folds into scMask,
			// which is what both forward rails apply per destination.
			var facts peerForwardFacts
			precomputeSendCommunity(&PeerSettings{SendCommunity: tt.send}, &facts)

			var mods filterapi.ModAccumulator
			applyFactsSendCommunity(&facts, &mods)

			result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, fail)
			require.NotNil(t, result, "suppression requested: the payload must be rebuilt")

			want := buildModTestPayload(tt.want, s.nlri)
			assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
				"emitted bytes must carry only the permitted community attributes")

			got := rebuiltAttrs(t, result)
			for _, code := range tt.codes {
				assert.NotContains(t, got, code,
					"community attribute %d was suppressed by config and must not reach the wire", code)
			}
		})
	}
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-3 -- `all` and unset
// record no operation at all, so the route stays on the zero-copy path.
// PREVENTS: a fix that suppresses correctly but rebuilds every route.
func TestSendCommunityAllKeepsZeroCopy(t *testing.T) {
	s := newCommunitySource()

	// Unset, the explicit "all" shorthand, and all three types named
	// individually are the same answer: nothing is suppressed, so no operation
	// is recorded and the source bytes are forwarded as they are.
	for _, send := range [][]string{nil, {"all"}, {"standard", "extended", "large"}} {
		var facts peerForwardFacts
		precomputeSendCommunity(&PeerSettings{SendCommunity: send}, &facts)

		var mods filterapi.ModAccumulator
		applyFactsSendCommunity(&facts, &mods)
		assert.Zero(t, mods.Len(), "send=%v must record no operation at all", send)

		result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		assert.Equal(t, modifyFailureNone, fail)
		assert.Nil(t, result, "send=%v must leave the route on the zero-copy path", send)
	}
}

// VALIDATES: spec-fixit-send-community-suppress-ignored AC-4, AC-5 -- Set and
// Suppress obey one rule for community codes, the same last-wins rule
// filterapi.LastSetOrSuppress applies to every generically handled code.
// PREVENTS: a Suppress branch that wins unconditionally and silently discards a
// later Set from a policy filter.
func TestCommunitySetSuppressLastWins(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	comm := makeAttr(0xC0, 8, []byte{0x00, 0x64, 0x00, 0x01})
	nlri := []byte{24, 10, 0, 0}
	payload := buildModTestPayload(slices.Concat(origin, comm), nlri)
	replacement := []byte{0xFF, 0xFF, 0xFF, 0x01} // NO_EXPORT

	t.Run("suppress-then-set-emits-the-set-value", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		mods.Op(8, filterapi.AttrModSuppress, nil)
		mods.Op(8, filterapi.AttrModSet, replacement)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		got := rebuiltAttrs(t, result)
		require.Contains(t, got, byte(8), "the later Set must win over the earlier Suppress")
		// emitCommunity always writes the Extended Length header class
		// (0xC0|0x10), whatever the value length.
		assert.Equal(t, "d0080004ffffff01", hex.EncodeToString(got[8]))
	})

	// The ACTION decides, not the buffer length. Every producer records Suppress
	// with a nil Buf, which also satisfies the handler's separate "empty Set
	// value" drop, so without this case the Suppress branch could be deleted and
	// the suite would stay green off that coincidence. A Suppress carrying bytes
	// is the one input only the action check answers.
	t.Run("suppress-carrying-a-buffer-still-drops", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		mods.Op(8, filterapi.AttrModSuppress, replacement)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		want := buildModTestPayload(origin, nlri)
		assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
			"a Suppress op is suppression whatever its buffer holds")
	})

	t.Run("set-then-suppress-drops-the-attribute", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		mods.Op(8, filterapi.AttrModSet, replacement)
		mods.Op(8, filterapi.AttrModSuppress, nil)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		want := buildModTestPayload(origin, nlri)
		assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
			"the later Suppress must win over the earlier Set")
	})
}

// VALIDATES: spec-fixit-send-community-suppress-ignored, sibling audit -- the
// two OTHER handlers that read AttrModSet alone. CLUSTER_LIST honors a Suppress
// (Optional Non-transitive, no preservation clause); ORIGINATOR_ID refuses one
// when the source carries a value, because RFC4456-8-4 is a gated MUST.
// PREVENTS: the same blind spot the community handler shipped. Neither code has
// a producer today, so these branches are latent by design: they exist so the
// producer that eventually suppresses one gets the documented answer instead of
// silent re-emission.
func TestClusterListAndOriginatorIDSuppress(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	clusterList := makeAttr(0x80, 10, []byte{0x0A, 0x00, 0x00, 0x01})
	originatorID := makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x02})
	nlri := []byte{24, 10, 0, 0}

	t.Run("cluster-list-suppress-drops", func(t *testing.T) {
		payload := buildModTestPayload(slices.Concat(origin, originatorID, clusterList), nlri)

		var mods filterapi.ModAccumulator
		mods.Op(10, filterapi.AttrModSuppress, nil)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		got := rebuiltAttrs(t, result)
		assert.NotContains(t, got, byte(10), "CLUSTER_LIST is Optional Non-transitive: a Suppress removes it")
		assert.Contains(t, got, byte(9), "only CLUSTER_LIST was suppressed")
	})

	t.Run("cluster-list-suppress-beats-prepend", func(t *testing.T) {
		payload := buildModTestPayload(slices.Concat(origin, clusterList), nlri)

		var mods filterapi.ModAccumulator
		mods.Op(10, filterapi.AttrModPrepend, []byte{0x0A, 0x00, 0x00, 0x09})
		mods.Op(10, filterapi.AttrModSuppress, nil)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		want := buildModTestPayload(origin, nlri)
		assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(result),
			"a prepend only shapes an attribute that exists; Suppress decides that it does not")
	})

	t.Run("originator-id-suppress-refused-when-present", func(t *testing.T) {
		payload := buildModTestPayload(slices.Concat(origin, originatorID), nlri)

		var mods filterapi.ModAccumulator
		mods.Op(9, filterapi.AttrModSuppress, nil)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		got := rebuiltAttrs(t, result)
		require.Contains(t, got, byte(9),
			"RFC 4456 Section 8 (RFC4456-8-4): the ORIGINATOR_ID value MUST be preserved unchanged")
		assert.Equal(t, originatorID, got[9], "preserved byte-identical to the source")
	})

	t.Run("originator-id-suppress-prevents-creation-when-absent", func(t *testing.T) {
		payload := buildModTestPayload(origin, nlri)

		var mods filterapi.ModAccumulator
		mods.Op(9, filterapi.AttrModSet, []byte{0x0A, 0x00, 0x00, 0x03})
		mods.Op(9, filterapi.AttrModSuppress, nil)

		result, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)

		got := rebuiltAttrs(t, result)
		assert.NotContains(t, got, byte(9),
			"nothing is yet set, so there is nothing to preserve and the Suppress wins")
	})
}
