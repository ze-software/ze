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
// two well-formed TLV values (RFC 9086 Section 5), each twice: once with the
// four reserved bits 0x0f set and once with them clear.
//   - A 7-octet label TLV: V and L set (Flags 0xcf against 0xc0), Weight 5,
//     Reserved 00 00, then the 3-octet label 00 5d c0 (label 24000).
//   - An 8-octet index TLV: V and L clear (Flags 0x0f against 0x00), Weight 5,
//     Reserved 00 00, then the 4-octet index 00 00 5d c0 (index 24000).
//
// Neither form is refused, and each pair yields the same V and L bits, Weight
// 5 and SID 24000, so only the reserved bits differ between the two inputs and
// they change nothing the decoder reports.
//
// RFC requirement: RFC9086-5-3 positive -- a received PeerNode, PeerAdj or
// PeerSet SID TLV with the reserved flag bits 0x0f set is accepted and decodes,
// as the label form (Flags&0xf0 0xc0, label 24000) and as the index form
// (Flags&0xf0 0x00, index 24000), to the same Weight 5 and SID as the same TLV
// with them clear.
func TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived(t *testing.T) {
	forms := []struct {
		name    string
		flagsVL byte
		sid     []byte
	}{
		{name: "label", flagsVL: 0xc0, sid: []byte{0x00, 0x5d, 0xc0}},
		{name: "index", flagsVL: 0x00, sid: []byte{0x00, 0x00, 0x5d, 0xc0}},
	}
	for _, code := range []uint16{TLVPeerNodeSID, TLVPeerAdjSID, TLVPeerSetSID} {
		decode := lookupLsAttrTLVDecoder(code)
		require.NotNil(t, decode, "TLV %d has a registered receive decoder", code)
		for _, form := range forms {
			for _, flags := range []byte{form.flagsVL | 0x0f, form.flagsVL} {
				value := append([]byte{flags, 0x05, 0x00, 0x00}, form.sid...)
				tlv, err := decode(value)
				require.NoError(t, err, "TLV %d %s flags %#x: reserved bits must not refuse the TLV", code, form.name, flags)
				sid, ok := tlv.(*lsPeerSID)
				require.True(t, ok, "TLV %d decodes to a Peer SID", code)
				require.Equal(t, form.flagsVL, sid.Flags&0xf0, "TLV %d %s flags %#x: V and L", code, form.name, flags)
				require.Equal(t, uint8(5), sid.Weight, "TLV %d %s flags %#x: Weight", code, form.name, flags)
				require.Equal(t, uint32(24000), sid.SID, "TLV %d %s flags %#x: SID", code, form.name, flags)
			}
		}
	}
}
