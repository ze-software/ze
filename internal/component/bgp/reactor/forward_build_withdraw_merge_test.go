// Design: docs/architecture/bgp/structural-forwarding.md -- forwarded routes and next-hop self
// Related: forward_build.go -- buildWithdrawalPayload, the conversion under test

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// withdrawMergePayload builds an UPDATE payload from its three sections.
// withdrawalOf runs buildWithdrawalPayload over payload into a buffer of the
// size its contract requires, and returns the octets written, or nil for a
// payload it refuses.
func withdrawalOf(payload []byte) []byte {
	buf := make([]byte, len(payload)+4)
	n := buildWithdrawalPayload(payload, buf)
	if n == 0 {
		return nil
	}
	return buf[:n]
}

func withdrawMergePayload(withdrawn, attrs, nlri []byte) []byte {
	out := binary.BigEndian.AppendUint16(nil, uint16(len(withdrawn)))
	out = append(out, withdrawn...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(attrs)))
	out = append(out, attrs...)
	return append(out, nlri...)
}

// withdrawMergeReach is an IPv6 unicast MP_REACH_NLRI (next hop ::1) for nlri.
func withdrawMergeReach(nlri []byte) []byte {
	value := []byte{0, 2, 1, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}
	return makeAttr(0x80, 14, append(value, nlri...))
}

// withdrawMergeUnreach is an MP_UNREACH_NLRI of afi/safi for nlri.
func withdrawMergeUnreach(afi uint16, safi byte, nlri []byte) []byte {
	value := binary.BigEndian.AppendUint16(nil, afi)
	value = append(value, safi)
	return makeAttr(0x80, 15, append(value, nlri...))
}

var (
	mergeV4Old = []byte{24, 10, 0, 1}                     // 10.0.1.0/24, withdrawn by the source
	mergeV4New = []byte{24, 10, 0, 2}                     // 10.0.2.0/24, announced by the source
	mergeV6Old = []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0, 1} // 2001:db8:1::/48, withdrawn
	mergeV6New = []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0, 2} // 2001:db8:2::/48, announced
)

// TestBuildWithdrawalPayloadKeepsSourceWithdrawals converts a mixed UPDATE to a
// withdrawal and reads back every prefix the result removes.
//
// VALIDATES: the source UPDATE's own Withdrawn Routes and MP_UNREACH_NLRI travel
// beside the converted announcement, in one Withdrawn Routes field and ONE
// MP_UNREACH_NLRI, and an empty source MP_UNREACH_NLRI (the End-of-RIB form)
// adds nothing.
// PREVENTS: a withheld or LLGR-converted destination losing the withdrawals the
// source sent in the same message, so it keeps a prefix the source took back.
func TestBuildWithdrawalPayloadKeepsSourceWithdrawals(t *testing.T) {
	cases := []struct {
		name          string
		withdrawn     []byte
		attrs         []byte
		nlri          []byte
		wantWithdrawn []byte
		wantUnreach   []byte // MP_UNREACH_NLRI value, nil for none
	}{
		{
			name:          "ipv4 withdrawn plus ipv4 announce",
			withdrawn:     mergeV4Old,
			attrs:         makeAttr(0x40, 1, []byte{0}),
			nlri:          mergeV4New,
			wantWithdrawn: append(append([]byte(nil), mergeV4Old...), mergeV4New...),
		},
		{
			name:        "mp unreach plus mp reach of one family",
			attrs:       append(withdrawMergeUnreach(2, 1, mergeV6Old), withdrawMergeReach(mergeV6New)...),
			wantUnreach: append(append([]byte{0, 2, 1}, mergeV6Old...), mergeV6New...),
		},
		{
			name:          "ipv4 withdrawn plus mp reach",
			withdrawn:     mergeV4Old,
			attrs:         withdrawMergeReach(mergeV6New),
			wantWithdrawn: mergeV4Old,
			wantUnreach:   append([]byte{0, 2, 1}, mergeV6New...),
		},
		{
			name:          "ipv4 announce plus mp reach",
			attrs:         withdrawMergeReach(mergeV6New),
			nlri:          mergeV4New,
			wantWithdrawn: mergeV4New,
			wantUnreach:   append([]byte{0, 2, 1}, mergeV6New...),
		},
		{
			name:          "ipv4 announce plus mp unreach",
			attrs:         withdrawMergeUnreach(2, 1, mergeV6Old),
			nlri:          mergeV4New,
			wantWithdrawn: mergeV4New,
			wantUnreach:   append([]byte{0, 2, 1}, mergeV6Old...),
		},
		{
			name:        "empty mp unreach is not carried",
			attrs:       append(withdrawMergeUnreach(2, 1, nil), withdrawMergeReach(mergeV6New)...),
			wantUnreach: append([]byte{0, 2, 1}, mergeV6New...),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withdrawalOf(withdrawMergePayload(tc.withdrawn, tc.attrs, tc.nlri))
			require.NotNil(t, got)
			update, err := message.UnpackUpdate(got)
			require.NoError(t, err)
			assert.Equal(t, len(tc.wantWithdrawn), len(update.WithdrawnRoutes))
			if len(tc.wantWithdrawn) > 0 {
				assert.Equal(t, tc.wantWithdrawn, update.WithdrawnRoutes)
			}
			assert.Empty(t, update.NLRI, "a withdrawal announces nothing")
			_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			assert.False(t, hasReach, "no MP_REACH_NLRI survives: %x", reach)
			_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			assert.Equal(t, tc.wantUnreach != nil, hasUnreach)
			if tc.wantUnreach != nil {
				assert.Equal(t, tc.wantUnreach, unreach)
			}
		})
	}
}

// TestBuildWithdrawalPayloadRefusesTwoUnreachFamilies is the shape one message
// cannot carry.
//
// VALIDATES: an MP_UNREACH_NLRI of one family beside an MP_REACH_NLRI of another
// is refused (nil), because the merged withdrawal would need two MP_UNREACH_NLRI
// attributes, which RFC 7606 Section 3(g) answers with a NOTIFICATION.
// PREVENTS: a withdrawal that resets the destination's session, or one that
// silently drops one family's half.
func TestBuildWithdrawalPayloadRefusesTwoUnreachFamilies(t *testing.T) {
	attrs := append(withdrawMergeUnreach(1, 128, []byte{88, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 24, 10, 0, 1}), withdrawMergeReach(mergeV6New)...)
	got := withdrawalOf(withdrawMergePayload(nil, attrs, nil))
	assert.Nil(t, got)
}
