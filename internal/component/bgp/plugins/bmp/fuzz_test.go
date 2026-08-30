package bmp

import (
	"testing"
)

// FuzzDecodeBMPTLV feeds arbitrary bytes into the BMP information-TLV decoder.
// DecodeTLVs is the framing loop the message decoders in msg.go use
// (DecodeTLVs(buf, off, end)); driving it over arbitrary bytes exercises the
// framing and the DecodeTLV primitive it calls. The bytes originate from a
// configured (still remote / MITM-able) monitoring station.
//
// The decoder must never panic on any input, and every TLV it accepts must
// satisfy len(Value) == Length with the value sub-slicing the input, so a
// forged Length can never make Value escape the buffer. Seed corpus covers
// truncated headers, oversized lengths, and zero-length values.
//
// VALIDATES: DecodeTLV/DecodeTLVs bounds under adversarial input (AC-1).
// PREVENTS: regression where a future edit drops a bound and a crafted TLV
// Length panics or over-reads the input.
func FuzzDecodeBMPTLV(f *testing.F) {
	for _, seed := range bmpTLVSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Framing loop: every TLV returned (even alongside a trailing error)
		// was accepted by DecodeTLV, so each must hold the invariant.
		tlvs, _ := DecodeTLVs(data, 0, len(data))
		for i := range tlvs {
			if len(tlvs[i].Value) != int(tlvs[i].Length) {
				t.Fatalf("DecodeTLVs: TLV %d Value len %d != Length %d",
					i, len(tlvs[i].Value), tlvs[i].Length)
			}
		}
		// Primitive boundary call at off=0.
		tlv, n, err := DecodeTLV(data, 0)
		if err != nil {
			return
		}
		if n != TLVHeaderSize+int(tlv.Length) {
			t.Fatalf("DecodeTLV: consumed %d, want %d", n, TLVHeaderSize+int(tlv.Length))
		}
		if n > len(data) {
			t.Fatalf("DecodeTLV: consumed %d past data %d", n, len(data))
		}
		if len(tlv.Value) != int(tlv.Length) {
			t.Fatalf("DecodeTLV: Value len %d != Length %d", len(tlv.Value), tlv.Length)
		}
	})
}

// bmpTLVSeeds returns valid and malformed BMP TLV byte strings for the fuzzer:
// zero-length, a valid single TLV, a valid pair, a zero-length-value TLV, a
// truncated header, a header claiming more value bytes than present, and an
// oversized length with no value.
func bmpTLVSeeds() [][]byte {
	encode := func(tlvs ...TLV) []byte {
		total := 0
		for _, t := range tlvs {
			total += TLVHeaderSize + int(t.Length)
		}
		buf := make([]byte, total)
		writeTLVs(buf, 0, tlvs)
		return buf
	}

	return [][]byte{
		{}, // zero-length
		encode(makeStringTLV(InitTLVSysName, "ze")),                                          // one valid TLV
		encode(makeStringTLV(InitTLVSysDescr, "router-a"), makeStringTLV(InitTLVString, "")), // pair, second empty
		{0x00, 0x00, 0x00, 0x00},                                                             // zero-length value TLV (valid)
		{0x00, 0x01},                                                                         // header truncated below TLVHeaderSize
		{0x00, 0x02, 0x00, 0x08},                                                             // Length=8 with no value bytes (truncated)
		{0x00, 0x00, 0xFF, 0xFF},                                                             // Length=65535 with no value (oversized)
		{0x00, 0x02, 0x00, 0x02, 0x41},                                                       // valid header, one short of its 2 value bytes
	}
}
