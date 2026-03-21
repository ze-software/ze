package ls

import (
	"testing"
)

// FuzzAttrTLVParse fuzzes the attribute TLV iterator and decoder.
//
// VALIDATES: No panics on arbitrary input to IterateAttrTLVs + DecodeAttrTLV.
// PREVENTS: Out-of-bounds reads, nil dereferences on malformed wire data.
func FuzzAttrTLVParse(f *testing.F) {
	// Seed: valid Node Name TLV
	f.Add([]byte{0x04, 0x02, 0x00, 0x04, 't', 'e', 's', 't'})
	// Seed: valid IGP Metric 1-byte
	f.Add([]byte{0x04, 0x47, 0x00, 0x01, 0x0A})
	// Seed: valid Admin Group
	f.Add([]byte{0x04, 0x40, 0x00, 0x04, 0xDE, 0xAD, 0xBE, 0xEF})
	// Seed: truncated header
	f.Add([]byte{0x04, 0x02})
	// Seed: empty
	f.Add([]byte{})
	// Seed: unknown TLV
	f.Add([]byte{0xFF, 0xFF, 0x00, 0x02, 0xAB, 0xCD})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic
		_ = IterateAttrTLVs(data, func(entry AttrTLVEntry) bool {
			// Attempt decode -- must not panic
			tlv, _ := DecodeAttrTLV(entry)
			if tlv != nil {
				_ = tlv.Code()
				_ = tlv.Len()
				_ = tlv.ToJSON()
			}
			return true
		})

		// Also test DecodeAllAttrTLVs
		tlvs, _ := DecodeAllAttrTLVs(data)
		for _, tlv := range tlvs {
			_ = tlv.ToJSON()
		}

		// And AttrTLVsToJSON
		_ = AttrTLVsToJSON(data)
	})
}
