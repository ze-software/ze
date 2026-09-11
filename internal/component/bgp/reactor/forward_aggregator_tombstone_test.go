// Design: docs/architecture/wire/attributes.md — ATTR_TOMBSTONE over a discarded AGGREGATOR
// RFC: rfc/short/rfc7606.md — a malformed AGGREGATOR is an attribute discard (Section 7.7)
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt — the rebuild marker (Section 5.1, Section 5.7)

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// payloadAttrValue returns one attribute's flags and value bytes from a rebuilt
// UPDATE body, and whether the attribute is there at all.
func payloadAttrValue(t testing.TB, payload []byte, code attribute.AttributeCode) (flags byte, value []byte, found bool) {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4)
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2)
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	off := attrLenOff + 2
	end := off + attrLen
	require.GreaterOrEqual(t, len(payload), end)
	for off < end {
		require.GreaterOrEqual(t, end, off+3)
		hdrLen := 3
		valueLen := int(payload[off+2])
		if payload[off]&0x10 != 0 {
			require.GreaterOrEqual(t, end, off+4)
			hdrLen = 4
			valueLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		}
		require.GreaterOrEqual(t, end, off+hdrLen+valueLen)
		if attribute.AttributeCode(payload[off+1]) == code {
			return payload[off], payload[off+hdrLen : off+hdrLen+valueLen], true
		}
		off += hdrLen + valueLen
	}
	return 0, nil, false
}

// VALIDATES: RFC 7606 Section 7.7 -- an AGGREGATOR whose length is neither 6 nor
// 8 "SHALL be handled using the approach of 'attribute discard'" -- reaches the
// WIRE through the rail an operator's EBGP peer is served by, rather than
// stopping at the operations the AS-path edit records.
// PREVENTS: the marker being recorded and never emitted, which is the failure
// class plan/journal/unwired-feature.md exists for: with no handler registered
// for code 252 the rebuild suppresses the route instead of marking it.
func TestPrependRebuildEmitsTombstoneForDiscardedAggregator(t *testing.T) {
	origin := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	asPath := makeAttr(0x40, byte(attribute.AttrASPath), []byte{
		byte(attribute.ASSequence), 1,
		0, 0, 0xFB, 0xF0, // 64496
	})
	// Seven octets: neither the six a two-octet peer writes nor the eight a
	// four-octet one writes.
	agg := makeAttr(0xC0, byte(attribute.AttrAggregator), []byte{0, 0, 0xFB, 0xF0, 192, 0, 2})
	attrs := append(append(append([]byte{}, origin...), asPath...), agg...)
	payload := buildModTestPayload(attrs, []byte{24, 10, 0, 0})

	var mods filterapi.ModAccumulator
	var edit wireu.ASPathEdit
	changed, err := edit.Record(&mods, payload, wireu.ASPathIntent{
		Prepend: []uint32{65000}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)
	require.True(t, changed)

	out, _, fail := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.False(t, fail.failed(), "the rebuild must not suppress the route over a marked attribute")
	require.NotNil(t, out)

	_, _, stillThere := payloadAttrValue(t, out, attribute.AttrAggregator)
	assert.False(t, stillThere, "the discarded AGGREGATOR must not reach the peer")

	flags, value, found := payloadAttrValue(t, out, attribute.AttrTombstone)
	require.True(t, found, "the rebuild must emit the ATTR_TOMBSTONE the edit recorded")
	assert.Equal(t, []byte{byte(attribute.AttrAggregator), wireu.TombstoneInvalidLength}, value,
		"draft Section 4.3: one (code, reason) pair naming the discarded attribute")
	assert.Equal(t, byte(0xC0), flags,
		"draft Section 4.2: Optional set, and the Transitive bit of the discarded AGGREGATOR (RFC 4271 Section 5.1.7)")
}
