package ls

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived proves the receive clause of
// the Rsvd bits rule for all three Peer SID TLVs: a received TLV with every
// reserved flag bit set decodes to the same meaning as one with them clear.
//
// Method: the registered receive decoders of PeerNode (1101), PeerAdj (1102)
// and PeerSet (1103), the ones the BGP-LS attribute walk looks up, each decode
// a SID value twice: once with Flags 0xcf (V and L set plus the four reserved
// bits) and once with Flags 0xc0, both with Weight 5, Reserved 00 00 and the
// 4-octet SID 24000. Neither is refused, and each yields V and L set, Weight 5
// and SID 24000. The 4-octet SID requires L clear in a well-formed TLV, but
// the decoder reads the SID width from the length, so L does not change the
// reading and the comparison isolates the reserved bits.
//
// RFC requirement: RFC9086-5-3 positive -- a received PeerNode, PeerAdj or
// PeerSet SID TLV with the reserved flag bits 0x0f set is accepted and decodes
// to Flags&0xf0 0xc0, Weight 5 and SID 24000, as the same TLV with them clear.
func TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived(t *testing.T) {
	for _, code := range []uint16{TLVPeerNodeSID, TLVPeerAdjSID, TLVPeerSetSID} {
		decode := lookupLsAttrTLVDecoder(code)
		require.NotNil(t, decode, "TLV %d has a registered receive decoder", code)
		for _, flags := range []byte{0xcf, 0xc0} {
			tlv, err := decode([]byte{flags, 0x05, 0x00, 0x00, 0x00, 0x00, 0x5d, 0xc0})
			require.NoError(t, err, "TLV %d flags %#x: reserved bits must not refuse the TLV", code, flags)
			sid, ok := tlv.(*lsPeerSID)
			require.True(t, ok, "TLV %d decodes to a Peer SID", code)
			require.Equal(t, uint8(0xc0), sid.Flags&0xf0, "TLV %d flags %#x: V and L", code, flags)
			require.Equal(t, uint8(5), sid.Weight, "TLV %d flags %#x: Weight", code, flags)
			require.Equal(t, uint32(24000), sid.SID, "TLV %d flags %#x: SID", code, flags)
		}
	}
}
