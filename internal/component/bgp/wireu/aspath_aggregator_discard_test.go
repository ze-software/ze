// Design: docs/architecture/wire/attributes.md — ATTR_TOMBSTONE over a discarded AGGREGATOR
// RFC: rfc/short/rfc7606.md — a malformed AGGREGATOR is an attribute discard (Section 7.7)
// RFC: rfc/short/rfc6793.md — AGGREGATOR carries a two-octet or a four-octet AS number (Section 4.2.2)
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt — the rebuild marker (Section 5.1, Section 5.7)

package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// probeAggregatorPayload packs ORIGIN, a four-octet AS_PATH and one AGGREGATOR
// whose value is aggValue, over an advertised prefix.
func probeAggregatorPayload(aggValue []byte) []byte {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath4(64500))...)
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, aggValue)...)
	return buildProbePayload(attrs, probeAdvertisedNLRI)
}

// VALIDATES: RFC 7606 Section 7.7 -- "The AGGREGATOR attribute SHALL be
// considered malformed if any of the following applies: Its length is not 6
// (when the 4-octet AS number capability is not advertised to or not received
// from the peer). Its length is not 8 (when the 4-octet AS number capability is
// both advertised to and received from the peer)." Such an UPDATE "SHALL be
// handled using the approach of 'attribute discard'". The prepend rail records
// the discard and marks it with an ATTR_TOMBSTONE carrying the discarded code
// and the invalid-length reason (draft-mangin-idr-attr-tombstone-00 Section 5.1,
// rebuild procedure).
// PREVENTS: the prepend rail forwarding an AGGREGATOR nobody could read, which
// is what it did while the transcode rail marked the same attribute -- two
// egress rails disagreeing about one malformed attribute
// (plan/journal/unwired-feature.md, 2026-09-09).
func TestPrependDiscardsUnreadableAggregatorAndMarksIt(t *testing.T) {
	// Seven octets is neither the six a two-octet peer writes nor the eight a
	// four-octet one writes, so no reader can say which octets are the AS number.
	payload := probeAggregatorPayload([]byte{0, 0, 0xFC, 0x14, 192, 0, 2})

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510},
		SrcASN4: true,
		DstASN4: false,
	})
	require.NoError(t, err)
	require.True(t, changed)

	agg, ok := recordedOp(t, &mods, attribute.AttrAggregator)
	require.True(t, ok, "the malformed AGGREGATOR must be named by an operation, not left to travel on")
	assert.Equal(t, filterapi.AttrModSuppress, agg.Action,
		"RFC 7606 Section 7.7 discards the attribute, so the rebuild must drop it")

	tomb, ok := recordedOp(t, &mods, attribute.AttrTombstone)
	require.True(t, ok, "the discard must be marked by an ATTR_TOMBSTONE the rebuild emits")
	assert.Equal(t, filterapi.AttrModSet, tomb.Action)
	assert.Equal(t, []byte{byte(attribute.AttrAggregator), TombstoneInvalidLength}, tomb.Buf,
		"draft Section 4.3: one (code, reason) pair, the discarded code first")
}

// VALIDATES: RFC 7606 Section 7.7 again, in its other polarity -- an AGGREGATOR
// whose length matches the source's AS number width is well formed, so it is
// re-encoded for the destination (RFC 6793 Section 4.2.2) and never marked.
// PREVENTS: a discard rule wide enough to destroy the attribute it exists to
// protect, which is the shape this rail already shipped once
// (TestPrependKeepsValidAggregatorOnEveryPath).
func TestPrependReEncodesReadableAggregatorWithoutMarkingIt(t *testing.T) {
	// Eight octets is what a four-octet source writes, so it reads exactly.
	payload := probeAggregatorPayload([]byte{0, 0, 0xFC, 0x14, 192, 0, 2, 1})

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510},
		SrcASN4: true,
		DstASN4: false,
	})
	require.NoError(t, err)
	require.True(t, changed)

	agg, ok := recordedOp(t, &mods, attribute.AttrAggregator)
	require.True(t, ok, "a width change re-encodes the AGGREGATOR")
	assert.Equal(t, filterapi.AttrModSet, agg.Action)
	assert.NotZero(t, agg.GenIdx, "the re-encode is written by a generator")

	_, marked := recordedOp(t, &mods, attribute.AttrTombstone)
	assert.False(t, marked, "a readable AGGREGATOR is never discarded, so nothing marks it")
}
