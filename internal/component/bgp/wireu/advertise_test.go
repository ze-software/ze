// Design: docs/architecture/wire/attributes.md — the AS-path family as generate slots
// RFC: rfc/short/rfc4271.md — Section 4.3 (a withdraw-only UPDATE carries no path attributes), Section 5.1.2 (prepend when ADVERTISING to an external peer)
// RFC: rfc/short/rfc4760.md — Section 3, an advertisement can ride in MP_REACH_NLRI

package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// probeWithdrawPayload packs an UPDATE body with a withdrawn-routes field, so a
// withdraw-only shape can be built. buildProbePayload always writes a withdrawn
// length of zero.
func probeWithdrawPayload(withdrawn, attrs, nlri []byte) []byte {
	body := make([]byte, 0, 4+len(withdrawn)+len(attrs)+len(nlri))
	body = append(body, byte(len(withdrawn)>>8), byte(len(withdrawn)))
	body = append(body, withdrawn...)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

// probeMPValue packs an AFI/SAFI-prefixed MP attribute value.
func probeMPValue(afi uint16, safi byte, rest ...byte) []byte {
	return append([]byte{byte(afi >> 8), byte(afi), safi}, rest...)
}

// VALIDATES: PayloadAdvertisesNLRI answers "is a route carried" on positive
// evidence only, over every UPDATE shape the forward rail can meet.
// PREVENTS: an egress rule acting on a message that advertises nothing. RFC 4271
// Section 4.3 -- a withdraw-only UPDATE "will not include path attributes or
// Network Layer Reachability Information".
func TestPayloadAdvertisesNLRIShapes(t *testing.T) {
	withdrawn := []byte{24, 10, 0, 0} // 10.0.0.0/24
	nlri := []byte{24, 10, 1, 0}      // 10.1.0.0/24
	origin := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	mpReach := probeAttr(0x80, attribute.AttrMPReachNLRI, probeMPValue(2, 1, 16, 0x20, 0x01, 0x0d, 0xb8))
	mpUnreach := probeAttr(0x80, attribute.AttrMPUnreachNLRI, probeMPValue(2, 1, 32, 0x20, 0x01, 0x0d, 0xb8))

	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"withdraw only, no attributes", probeWithdrawPayload(withdrawn, nil, nil), false},
		{"withdraw only, MP_UNREACH", probeWithdrawPayload(nil, mpUnreach, nil), false},
		{"End-of-RIB, completely empty", probeWithdrawPayload(nil, nil, nil), false},
		{"attributes but no NLRI at all", probeWithdrawPayload(nil, origin, nil), false},
		{"IPv4 NLRI present", probeWithdrawPayload(nil, origin, nlri), true},
		{"MP_REACH present", probeWithdrawPayload(nil, mpReach, nil), true},
		{"mixed withdraw and NLRI", probeWithdrawPayload(withdrawn, origin, nlri), true},
		{"truncated below the length fields", []byte{0, 0, 0}, false},
		{"withdrawn length overruns the body", []byte{0xff, 0xff, 0, 0}, false},
		{"attribute length overruns the body", []byte{0, 0, 0xff, 0xff}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PayloadAdvertisesNLRI(tc.payload))
		})
	}
}

// recordedASPath materializes the AS numbers a Record call left for AS_PATH, or
// reports that no AS_PATH operation was recorded at all.
func recordedASPath(t *testing.T, mods *filterapi.ModAccumulator, asn4 bool) ([]uint32, bool) {
	t.Helper()
	op, ok := recordedOp(t, mods, attribute.AttrASPath)
	if !ok {
		return nil, false
	}
	var value []byte
	if op.GenIdx != 0 {
		value = materialize(t, recordedGen(t, mods, op))
	} else {
		value = op.Buf
	}
	parsed, err := attribute.ParseASPath(value, asn4)
	require.NoError(t, err)
	if len(parsed.Segments) == 0 {
		return nil, true
	}
	return parsed.Segments[0].ASNs, true
}

// VALIDATES: RFC 4271 Section 5.1.2 -- the prepend applies "when a given BGP
// speaker advertises the route to an external peer", so an UPDATE that
// advertises no route gets none, while the very same intent over an advertising
// payload still does.
//
// RFC requirement: RFC4271-5.1.2-3 positive -- an EBGP intent over an UPDATE that
// ADVERTISES 10.1.0.0/24 leaves with 65000 at the head of the AS_SEQUENCE.
// RFC requirement: RFC4271-5.1.2-3 negative -- the identical intent over a
// withdraw-only UPDATE records no AS_PATH at all, because the requirement's own
// condition ("advertises the route") is not met. RFC 4271 Section 4.3 states the
// shape: such a message "will not include path attributes".
//
// PREVENTS: a lone synthesized AS_PATH on a relayed withdrawal. It is a
// well-known attribute set that is incomplete by construction -- AS_PATH present,
// ORIGIN and NEXT_HOP absent -- which RFC 4271 Section 6.3 makes a Missing
// Well-known Attribute error, and FRR 10.3.1 answers with "rcvd UPDATE with
// errors in attr(s)!! Withdrawing route" (test/interop/scenarios/52).
//
// NON-VACUITY (ai/rules/interop-and-goal-validation.md): the negative asserts an
// ABSENCE, which survives deleting the prepend mechanism outright. The positive
// runs the SAME intent through the SAME Record call one line above it, so a
// wholly disabled prepend reddens this test rather than passing it.
func TestASPathSlotPrependOnlyWhenAdvertising(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath4(64500))...)
	intent := ASPathIntent{Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true}

	t.Run("advertising", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, probeWithdrawPayload(nil, attrs, []byte{24, 10, 1, 0}), intent)
		require.NoError(t, err)
		require.True(t, changed, "an advertisement to an EBGP peer must record the prepend")
		asns, ok := recordedASPath(t, &mods, true)
		require.True(t, ok, "no AS_PATH operation was recorded for an advertisement")
		assert.Equal(t, []uint32{65000, 64500}, asns)
	})

	t.Run("withdraw only, no attributes", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, probeWithdrawPayload([]byte{24, 10, 1, 0}, nil, nil), intent)
		require.NoError(t, err)
		assert.False(t, changed, "a withdraw-only UPDATE needs no AS-path change")
		_, ok := recordedASPath(t, &mods, true)
		assert.False(t, ok,
			"an AS_PATH was synthesized onto an UPDATE that advertises no route; "+
				"RFC 4271 Section 4.3 says such a message carries no path attributes")
		assert.Empty(t, mods.Ops(), "a withdraw-only relay must record no attribute operation at all")
	})

	t.Run("MP_UNREACH only", func(t *testing.T) {
		mpUnreach := probeAttr(0x80, attribute.AttrMPUnreachNLRI, probeMPValue(2, 1, 32, 0x20, 0x01, 0x0d, 0xb8))
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, probeWithdrawPayload(nil, mpUnreach, nil), intent)
		require.NoError(t, err)
		assert.False(t, changed, "an MP_UNREACH-only UPDATE advertises nothing")
		_, ok := recordedASPath(t, &mods, true)
		assert.False(t, ok, "an AS_PATH was synthesized onto an MP_UNREACH-only UPDATE")
	})
}

// VALIDATES: RFC 6793 Section 4.1 -- "The new attributes, AS4_PATH and
// AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP
// speakers." Dropping the prepend must not drop that, and the widths matching
// must not either.
//
// PREVENTS: a withdraw-only UPDATE becoming the one shape on the EBGP rail that
// carries a received AS4_PATH onward. The prepend rail drops it through
// recordAS4Path, and recordTranscode returns before that when the widths match,
// because the RFC 7947 route-server rail it also serves must leave the family
// untouched. recordWithdrawOnly is the difference between the two.
//
// NON-VACUITY: the second subtest is the same fixture with a two-octet
// destination, where an AS4_PATH is what an OLD speaker is SUPPOSED to receive.
// A blanket suppression passes the first and fails the second.
func TestASPathSlotWithdrawOnlyDropsAS4Path(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64500))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAS4Path, probeASPath4(64500))...)
	payload := probeWithdrawPayload([]byte{24, 10, 1, 0}, attrs, nil)

	t.Run("both speakers NEW: the received AS4_PATH is dropped", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, payload, ASPathIntent{
			Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true,
		})
		require.NoError(t, err)
		require.True(t, changed, "dropping the AS4_PATH is a change")

		op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
		require.True(t, ok, "RFC 6793 Section 4.1 forbids carrying it between NEW speakers")
		assert.Equal(t, filterapi.AttrModSuppress, op.Action)

		_, hasASPath := recordedASPath(t, &mods, true)
		assert.False(t, hasASPath, "no AS_PATH is created or prepended on a withdrawal")
	})

	t.Run("OLD destination: the AS4_PATH is what it must receive", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		_, err := edit.Record(&mods, payload, ASPathIntent{
			Prepend: []uint32{65000}, SrcASN4: false, DstASN4: false,
		})
		require.NoError(t, err)

		op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
		if ok {
			assert.NotEqual(t, filterapi.AttrModSuppress, op.Action,
				"an OLD speaker reads AS4_PATH; suppressing it loses the real AS numbers")
		}
	})
}

// VALIDATES: RFC 6793 Section 4.2.2 still applies to an AS_PATH that rode along
// on a withdraw-only UPDATE. Dropping the prepend must not drop the width
// transcode: a two-octet peer reads a four-octet path as garbage.
// PREVENTS: fixing the synthesis by returning early, which would forward
// four-octet AS numbers to an OLD speaker.
func TestASPathSlotWithdrawOnlyStillTranscodes(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64500, 64501))
	payload := probeWithdrawPayload([]byte{24, 10, 1, 0}, attrs, nil)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{Prepend: []uint32{65000}, SrcASN4: true, DstASN4: false})
	require.NoError(t, err)
	require.True(t, changed, "a width change must still be recorded on a withdraw-only UPDATE")

	asns, ok := recordedASPath(t, &mods, false)
	require.True(t, ok, "no AS_PATH operation was recorded for the transcode")
	assert.Equal(t, []uint32{64500, 64501}, asns,
		"the path must be re-encoded at the destination width and NOT prepended")
}
