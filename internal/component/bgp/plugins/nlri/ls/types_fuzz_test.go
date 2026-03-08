package bgp_nlri_ls

import (
	"testing"
)

// FuzzParseBGPLS tests BGP-LS NLRI binary parsing robustness.
//
// VALIDATES: ParseBGPLS handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated TLVs, unknown NLRI types, descriptor overflow.
// SECURITY: BGP-LS bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseBGPLS(f *testing.F) {
	// Node NLRI (type 1): protocol-id=IS-IS-L2(2), identifier=0, local node descriptor
	f.Add([]byte{
		0x00, 0x01, // NLRI type: Node
		0x00, 0x1D, // length = 29
		0x02,                                           // protocol-id: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // identifier
		// Local Node Descriptors TLV (type 256)
		0x01, 0x00, 0x00, 0x10, // type=256, length=16
		// AS Number sub-TLV (type 512)
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xFD, 0xE8, // type=512, len=4, AS=65000
		// BGP LS Identifier sub-TLV (type 513)
		0x02, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00, 0x01, // type=513, len=4, id=1
	})
	// Link NLRI (type 2): minimal header
	f.Add([]byte{
		0x00, 0x02, // NLRI type: Link
		0x00, 0x09, // length = 9
		0x01,                                           // protocol-id: IS-IS Level 1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // identifier
	})
	// IPv4 Prefix NLRI (type 3)
	f.Add([]byte{
		0x00, 0x03, // NLRI type: IPv4 Topology Prefix
		0x00, 0x09,
		0x03, // protocol-id: OSPF
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	// IPv6 Prefix NLRI (type 4)
	f.Add([]byte{
		0x00, 0x04, // NLRI type: IPv6 Topology Prefix
		0x00, 0x09,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	// SRv6 SID NLRI (type 6)
	f.Add([]byte{
		0x00, 0x06, // NLRI type: SRv6 SID
		0x00, 0x09,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	f.Add([]byte{})                                               // Empty
	f.Add([]byte{0x00, 0x01})                                     // Type only, no length
	f.Add([]byte{0x00, 0x01, 0x00})                               // Type + partial length
	f.Add([]byte{0x00, 0x01, 0x00, 0x00})                         // Type + length 0
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})                         // Unknown type, max length
	f.Add([]byte{0x00, 0x01, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04}) // Node, short body

	f.Fuzz(func(t *testing.T, data []byte) {
		nlri, err := ParseBGPLS(data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = nlri.NLRIType()
		_ = nlri.ProtocolID()
		_ = nlri.Identifier()
		_ = nlri.Family()
		_ = nlri.String()
		_ = nlri.Bytes()
		_ = nlri.Len()
		buf := make([]byte, nlri.Len()+10)
		_ = nlri.WriteTo(buf, 0)
	})
}

// FuzzParseBGPLSWithRest tests BGP-LS NLRI parsing with remaining bytes.
//
// VALIDATES: ParseBGPLSWithRest handles arbitrary bytes without crashing.
// PREVENTS: Panic on boundary between NLRI and remaining data.
// SECURITY: BGP-LS wire bytes are untrusted.
func FuzzParseBGPLSWithRest(f *testing.F) {
	// Node NLRI followed by extra bytes
	f.Add([]byte{
		0x00, 0x01, 0x00, 0x09,
		0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xDE, 0xAD, // trailing bytes
	})
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x00, 0x00}) // Zero length

	f.Fuzz(func(t *testing.T, data []byte) {
		nlri, rest, err := ParseBGPLSWithRest(data)
		if err != nil {
			return
		}
		_ = nlri.NLRIType()
		_ = nlri.String()
		_ = len(rest)
	})
}
