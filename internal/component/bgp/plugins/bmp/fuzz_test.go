package bmp

import (
	"encoding/binary"
	"testing"
	"time"
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

// FuzzDecodeLocRIBPeerUp feeds arbitrary bytes into the Peer Up decoder and
// into the capability reader that runs on what it returns.
//
// This is the surface RFC 9069 Section 6.1.1 opened. decodePeerUp used to skip
// OPEN extraction for PeerTypeLocRIB; the branch is gone, so a Peer Up from a
// monitored router now reaches extractBGPOpen and openMultiprotocolFamilies for
// every peer type. Those bytes come from a remote BMP speaker, which makes the
// parse an attack surface rather than an internal decode.
//
// Two invariants, and neither is provable by the unit tests, which drive
// well-formed messages: nothing panics on any input, and every OPEN the decoder
// returns sub-slices its own input rather than escaping it, which is what a
// forged BGP length would have to break.
//
// VALIDATES: R-6 of spec-fixit-locrib-peer-fields-contradict-rfc9069 -- the
// peer-type-3 OPEN parse is bounds-safe.
// PREVENTS: regression where a future edit drops a bound in extractBGPOpen and
// a crafted Peer Up over-reads the session buffer.
func FuzzDecodeLocRIBPeerUp(f *testing.F) {
	for _, seed := range locRIBPeerUpSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		pu, err := decodePeerUp(data, 0, len(data))
		if err != nil {
			return
		}
		for _, open := range [][]byte{pu.SentOpenMsg, pu.ReceivedOpenMsg} {
			if len(open) > len(data) {
				t.Fatalf("decodePeerUp: OPEN of %d bytes escapes the %d-byte input", len(open), len(data))
			}
			// The families reader runs on the same remote bytes and must answer
			// nil rather than panic on anything it cannot parse.
			openMultiprotocolFamilies(open)
		}
	})
}

// locRIBPeerUpSeeds returns Peer Up bodies for the fuzzer: a well-formed Loc-RIB
// Peer Up carrying the fabricated OPEN twice, the same message truncated at each
// boundary the decoder checks, and a message whose sent OPEN declares a length
// past the end of the buffer.
func locRIBPeerUpSeeds() [][]byte {
	open := fabricateLocRIBOpen(localIdentity{asn: 65044, routerID: 0xAC1E0002})
	body := &PeerUp{
		Peer:            locRIBPeerHeader(localIdentity{asn: 65044, routerID: 0xAC1E0002}, time.Time{}),
		SentOpenMsg:     open,
		ReceivedOpenMsg: open,
		InfoTLVs:        []TLV{locRIBTableNameTLV()},
	}
	whole := make([]byte, CommonHeaderSize+PeerHeaderSize+peerUpFixedSize+2*len(open)+TLVHeaderSize+len(locRIBTableName))
	total := writePeerUp(whole, 0, body)
	whole = whole[:total]

	// The decoder is handed the message body, so every seed starts past the
	// common header, which DecodeMsg strips before it dispatches.
	valid := whole[CommonHeaderSize:]
	forged := append([]byte(nil), valid...)
	// The sent OPEN's BGP length field, two octets at offset 16 of the OPEN.
	binary.BigEndian.PutUint16(forged[PeerHeaderSize+peerUpFixedSize+16:], 4095)

	return [][]byte{
		{},
		valid,
		valid[:PeerHeaderSize],                 // peer header only, no fixed fields
		valid[:PeerHeaderSize+peerUpFixedSize], // fixed fields, no OPEN
		valid[:PeerHeaderSize+peerUpFixedSize+18], // OPEN one octet short of its header
		valid[:len(valid)-1],                      // TLV one octet short
		forged,                                    // sent OPEN claims 4095 bytes it does not have
	}
}
