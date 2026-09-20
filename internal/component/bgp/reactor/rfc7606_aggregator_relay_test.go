// Design: docs/architecture/wire/attributes.md — ATTR_TOMBSTONE over a discarded AGGREGATOR
// RFC: rfc/short/rfc7606.md — a malformed AGGREGATOR is an attribute discard (Section 7.7)
// Related: internal/component/bgp/wireu/aspath_slot.go — ASPathEdit.recordAggregatorDiscard

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// VALIDATES: RFC 7606 Section 7.7 -- an AGGREGATOR of any length but 6 or 8
// "SHALL be handled using the approach of 'attribute discard'", which Section 2
// defines as "the malformed attribute MUST be discarded and the UPDATE message
// continues to be processed". Discarded is not forwarded, so the octets must be
// absent from the body ze rebuilds for the peer.
// PREVENTS: the two shapes that relayed such an attribute verbatim. A
// MATCHING-WIDTH destination reached neither rail's AGGREGATOR branch, because
// both returned on the width question first. A value under two octets reached
// the branch and was copied through, because the in-place marker could not fit
// and its refusal was read as leave to forward.
//
// RFC requirement: RFC7606-7.7-1 negative -- an AGGREGATOR whose length is
// neither 6 nor 8 is absent from the UPDATE body ze rebuilds for an EBGP peer of
// the same AS number width, and an ATTR_TOMBSTONE naming it stands in its place.
func TestForwardDiscardsMalformedAggregatorAtMatchingWidth(t *testing.T) {
	cases := []struct {
		name  string
		value []byte
	}{
		// Seven octets: neither the six a two-octet peer writes nor the eight a
		// four-octet one writes.
		{name: "seven octet value", value: []byte{0, 0, 0xFB, 0xF0, 192, 0, 2}},
		// One octet: too short to carry the marker in the space it occupies.
		{name: "one octet value", value: []byte{0xFF}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origin := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
			asPath := makeAttr(0x40, byte(attribute.AttrASPath), []byte{
				byte(attribute.ASSequence), 1,
				0, 0, 0xFB, 0xF0, // 64496
			})
			agg := makeAttr(0xC0, byte(attribute.AttrAggregator), tc.value)
			attrs := append(append(append([]byte{}, origin...), asPath...), agg...)
			payload := buildModTestPayload(attrs, []byte{24, 10, 0, 0})

			// Both ends four-octet, which is the width pair that asks the rails for
			// no AS_PATH re-encoding at all.
			var mods filterapi.ModAccumulator
			var edit wireu.ASPathEdit
			changed, err := edit.Record(&mods, payload, wireu.ASPathIntent{
				Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true,
			})
			require.NoError(t, err)
			require.True(t, changed)

			out, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.False(t, fail.failed(), "the rebuild must not suppress the route over a marked attribute")
			require.NotNil(t, out)

			_, _, stillThere := payloadAttrValue(t, out, attribute.AttrAggregator)
			assert.False(t, stillThere, "RFC 7606 Section 2: a discarded attribute does not reach the peer")
			assert.NotContains(t, string(out), string(tc.value),
				"the malformed value must not survive under any code")

			flags, value, found := payloadAttrValue(t, out, attribute.AttrTombstone)
			require.True(t, found, "the rebuild must emit the ATTR_TOMBSTONE the edit recorded")
			assert.Equal(t, []byte{byte(attribute.AttrAggregator), wireu.TombstoneInvalidLength}, value,
				"draft Section 4.3: one (code, reason) pair naming the discarded attribute")
			assert.Equal(t, byte(0xC0), flags,
				"draft Section 4.2: Optional set, and the Transitive bit of the discarded AGGREGATOR")
		})
	}
}

// TestForwardRelaysWellFormedAggregatorAtMatchingWidth is the conforming side of the
// same rebuild: an AGGREGATOR ze must NOT discard reaches the peer unchanged.
//
// VALIDATES: RFC 7606 Section 7.7 fixes the discard to an AGGREGATOR "of a length
// other than 6 (or 8 in the case of [RFC6793])", so an attribute of one of those two
// lengths is relayed with its octets intact and no ATTR_TOMBSTONE is written.
// PREVENTS: the shape the negative above cannot see. A recordAggregatorDiscard that
// fired on EVERY AGGREGATOR would keep that test green, strip a conformant
// AGGREGATOR from every UPDATE ze forwards, and stamp a marker for an attribute
// nothing was wrong with.
//
// RFC requirement: RFC7606-7.7-1 positive -- an AGGREGATOR whose length is 8 on a
// session where both ends negotiated four-octet AS numbers survives the rebuild
// byte for byte, and the body ze forwards carries no ATTR_TOMBSTONE.
func TestForwardRelaysWellFormedAggregatorAtMatchingWidth(t *testing.T) {
	// RFC 6793 Section 3: an AGGREGATOR sent by a NEW BGP speaker to another NEW BGP
	// speaker carries a four-octet AS number, so the value is eight octets.
	value := []byte{0x00, 0x00, 0xfb, 0xf0, 192, 0, 2, 1}

	origin := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	asPath := makeAttr(0x40, byte(attribute.AttrASPath), []byte{
		byte(attribute.ASSequence), 1,
		0, 0, 0xFB, 0xF0, // 64496
	})
	agg := makeAttr(0xC0, byte(attribute.AttrAggregator), value)
	attrs := append(append(append([]byte{}, origin...), asPath...), agg...)
	payload := buildModTestPayload(attrs, []byte{24, 10, 0, 0})

	// The same width pair as the negative above: four-octet on both ends, which asks
	// the rails for no AS_PATH re-encoding at all.
	var mods filterapi.ModAccumulator
	var edit wireu.ASPathEdit
	changed, err := edit.Record(&mods, payload, wireu.ASPathIntent{
		Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true,
	})
	require.NoError(t, err)
	require.True(t, changed)

	out, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.False(t, fail.failed(), "a well-formed AGGREGATOR suppresses no route")
	require.NotNil(t, out)

	flags, got, found := payloadAttrValue(t, out, attribute.AttrAggregator)
	require.True(t, found, "RFC 7606 Section 7.7 discards a MALFORMED AGGREGATOR, and this one is well-formed")
	assert.Equal(t, value, got, "the relayed value must be the octets the peer sent")
	assert.Equal(t, byte(0xC0), flags, "RFC 4271 Section 4.3g: AGGREGATOR is optional transitive")

	_, _, tombstoned := payloadAttrValue(t, out, attribute.AttrTombstone)
	assert.False(t, tombstoned, "no attribute was discarded, so no marker stands in for one")
}
