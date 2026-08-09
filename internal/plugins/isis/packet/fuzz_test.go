// Design: docs/architecture/wire/isis.md -- decode/iterator/round-trip fuzz targets
package packet

import (
	"bytes"
	"testing"
)

// ignoreDecode discards a (value, error) decode result. Fuzz targets call the
// per-TLV decoders solely to assert they never panic on arbitrary input
// (AC-11, R-3); the parsed value and the typed error are both irrelevant to
// that invariant. Centralizing the discard here keeps the call sites free of
// bare two-value blank assignments.
func ignoreDecode[T any](_ T, _ error) {}

// seedPDUs returns a few well-formed PDUs plus edge cases to seed the fuzz
// corpus. The fuzzer mutates these and feeds arbitrary bytes; the invariant
// under test is "no panic" (AC-11, R-3).
func seedPDUs() [][]byte {
	var seeds [][]byte

	// A minimal valid common header for an L1 LSP (no body): the decoder should
	// reject it as truncated, not panic.
	hdr := make([]byte, CommonHeaderLen)
	writeCommonHeader(hdr, 0, PDUTypeL1LSP, CommonHeaderLen, 0)
	seeds = append(seeds, hdr)

	// A full minimal LSP.
	lsp := &LSP{PDUType: PDUTypeL2LSP, RemainingLifetime: 1, SequenceNumber: 1}
	lbuf := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(lbuf, 0)
	seeds = append(seeds, lbuf)

	// A P2P Hello with one TLV.
	p2p := &P2PHello{CircuitType: CircuitL2, HoldingTime: 30, LocalCircuitID: 1,
		TLVs: []TLV{{Type: TLVAreaAddresses, Value: []byte{3, 0x49, 0, 1}}}}
	pbuf := make([]byte, p2p.EncodedLen())
	p2p.WriteTo(pbuf, 0)

	// pbuf plus degenerate inputs (empty, lone discriminator, all-ones).
	seeds = append(seeds, pbuf, nil, []byte{0x83}, []byte{0x83, 0x00}, bytes.Repeat([]byte{0xff}, 64))
	return seeds
}

// FuzzISISDecodePDU asserts DecodePDU never panics on arbitrary bytes and
// returns either a parsed PDU or a typed error (AC-11, R-3). When it succeeds,
// re-encoding the typed PDU must also not panic.
func FuzzISISDecodePDU(f *testing.F) {
	for _, s := range seedPDUs() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		pdu, err := DecodePDU(data)
		if err != nil {
			return // a typed error is the acceptable outcome
		}
		// On success exactly one body must be set; re-encoding must not panic.
		buf := make([]byte, len(data)+16)
		switch {
		case pdu.LANHello != nil:
			if pdu.LANHello.EncodedLen() <= len(buf) {
				pdu.LANHello.WriteTo(buf, 0)
			}
		case pdu.P2PHello != nil:
			if pdu.P2PHello.EncodedLen() <= len(buf) {
				pdu.P2PHello.WriteTo(buf, 0)
			}
		case pdu.LSP != nil:
			if pdu.LSP.EncodedLen() <= len(buf) {
				pdu.LSP.WriteTo(buf, 0)
			}
			// VerifyChecksum must not panic regardless of validity.
			_ = pdu.LSP.VerifyChecksum()
		case pdu.CSNP != nil:
			if pdu.CSNP.EncodedLen() <= len(buf) {
				pdu.CSNP.WriteTo(buf, 0)
			}
		case pdu.PSNP != nil:
			if pdu.PSNP.EncodedLen() <= len(buf) {
				pdu.PSNP.WriteTo(buf, 0)
			}
		default:
			t.Fatal("DecodePDU returned no error and no body")
		}
	})
}

// FuzzISISTLVIterator asserts the generic TLV walk terminates without panic on
// arbitrary bytes, and that the typed per-TLV decoders never panic on the
// values the iterator yields.
func FuzzISISTLVIterator(f *testing.F) {
	f.Add([]byte{1, 3, 0x49, 0, 1, 137, 2, 'h', 'i'})
	f.Add([]byte{22, 11, 0, 1, 0, 2, 0, 3, 0, 0, 0, 10, 0})
	f.Add(bytes.Repeat([]byte{0x16, 0x05}, 20)) // type 22, claimed len 5, repeated
	f.Add([]byte{})
	f.Fuzz(func(_ *testing.T, data []byte) {
		it := NewTLVIterator(data)
		guard := 0
		for {
			typ, value, ok := it.Next()
			if !ok {
				break
			}
			guard++
			if guard > len(data)+1 {
				panic("TLV iterator did not terminate")
			}
			// Feed the value to the matching typed decoder; none may panic. The
			// decode result/error is intentionally discarded (no-panic invariant).
			switch typ {
			case TLVAreaAddresses:
				ignoreDecode(DecodeAreaAddressesTLV(value))
			case TLVISNeighbors:
				ignoreDecode(DecodeISNeighborsTLV(value))
			case TLVISReachabilityNarrow:
				ignoreDecode(DecodeNarrowISReachTLV(value))
			case TLVLSPEntries:
				ignoreDecode(DecodeLSPEntriesTLV(value))
			case TLVAuthentication:
				ignoreDecode(DecodeAuthTLV(value))
			case TLVExtendedISReach:
				ignoreDecode(DecodeExtendedISReachTLV(value))
			case TLVProtocolsSupported:
				_ = DecodeProtocolsSupportedTLV(value)
			case TLVIPInterfaceAddress:
				ignoreDecode(DecodeIPv4InterfaceAddrTLV(value))
			case TLVExtendedIPReach:
				ignoreDecode(DecodeExtendedIPReachTLV(value))
			case TLVIPv6InterfaceAddress:
				ignoreDecode(DecodeIPv6InterfaceAddrTLV(value))
			case TLVIPv6Reachability:
				ignoreDecode(DecodeIPv6ReachabilityTLV(value))
			case TLVP2PThreeWay:
				ignoreDecode(DecodeP2PThreeWayTLV(value))
			}
		}
		_ = it.Err()
	})
}

// FuzzISISRoundTrip asserts that a valid LSP, once decoded, re-encodes to bytes
// that decode again to the same TLV-type sequence (encode/decode stability, no
// byte drift on the structural fields). It seeds from well-formed PDUs; mutated
// inputs that fail to decode are skipped.
func FuzzISISRoundTrip(f *testing.F) {
	for _, s := range seedPDUs() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		pdu, err := DecodePDU(data)
		if err != nil {
			return
		}
		// Only LSP re-encode is fully round-trip-stable here (it recomputes the
		// checksum); other PDU types are covered by the unit round-trip tests.
		if pdu.LSP == nil {
			return
		}
		first := pdu.LSP
		reenc := &LSP{
			PDUType:           first.PDUType,
			RemainingLifetime: first.RemainingLifetime,
			LSPID:             first.LSPID,
			SequenceNumber:    first.SequenceNumber,
			TypeBlock:         first.TypeBlock,
		}
		for _, tlv := range first.TLVs {
			reenc.TLVs = append(reenc.TLVs, tlv.CopyValue())
		}
		buf := make([]byte, reenc.EncodedLen())
		n := reenc.WriteTo(buf, 0)
		pdu2, err := DecodePDU(buf[:n])
		if err != nil {
			t.Fatalf("re-encoded LSP failed to decode: %v", err)
		}
		if len(pdu2.LSP.TLVs) != len(first.TLVs) {
			t.Fatalf("TLV count drift: %d -> %d", len(first.TLVs), len(pdu2.LSP.TLVs))
		}
		for i := range first.TLVs {
			if pdu2.LSP.TLVs[i].Type != first.TLVs[i].Type {
				t.Fatalf("TLV[%d] type drift: %d -> %d", i, first.TLVs[i].Type, pdu2.LSP.TLVs[i].Type)
			}
		}
		// The re-encoded LSP must have a valid checksum.
		if !pdu2.LSP.VerifyChecksum() {
			t.Fatal("re-encoded LSP has invalid checksum")
		}
	})
}
