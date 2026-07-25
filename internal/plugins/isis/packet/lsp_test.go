// Design: plan/spec-isis-2-wire.md -- LSP round-trip tests
package packet

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ext135TLV builds an opaque TLV 135 carrying one prefix for an LSP body test.
func ext135TLV(prefix netip.Prefix, metric uint32) TLV {
	in := ExtendedIPReachTLV{Entries: []ExtIPReachEntry{{
		Metric: types.NewPrefixMetric(metric),
		Prefix: prefix,
	}}}
	buf := make([]byte, 64)
	n := in.WriteTo(buf, 0)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	cp := make([]byte, len(value))
	copy(cp, value)
	return TLV{Type: TLVExtendedIPReach, Value: cp}
}

// VALIDATES: AC-2, AC-3 -- L1 and L2 LSP round-trip every field (lifetime,
// LSPID, sequence, type block, TLVs), the Fletcher checksum is backfilled on
// encode, and VerifyChecksum over the decoded raw bytes returns valid.
// Story 2: the runtime originates an LSP -> WriteTo -> checksum -> bytes.
func TestISISLSPRoundTrip(t *testing.T) {
	sys := types.SystemID{0, 1, 0, 2, 0, 3}
	for _, pt := range []PDUType{PDUTypeL1LSP, PDUTypeL2LSP} {
		t.Run(pt.String(), func(t *testing.T) {
			in := &LSP{
				PDUType:           pt,
				RemainingLifetime: 1199,
				LSPID:             types.NewLSPID(types.NewSourceID(sys, 0), 0),
				SequenceNumber:    0x00000010,
				TypeBlock:         LSPFlagISTypeL2 | LSPFlagOverload,
				TLVs: []TLV{
					ext135TLV(netip.MustParsePrefix("10.0.0.0/24"), 10),
					ext135TLV(netip.MustParsePrefix("192.0.2.0/30"), 20),
				},
			}
			buf := make([]byte, in.EncodedLen())
			n := in.WriteTo(buf, 0)
			if n != in.EncodedLen() {
				t.Fatalf("WriteTo returned %d, want %d", n, in.EncodedLen())
			}
			// The checksum field was backfilled and stored back into in.Checksum.
			if in.Checksum == 0 {
				t.Fatal("encoded LSP checksum is 0 (not backfilled)")
			}

			pdu, err := DecodePDU(buf)
			if err != nil {
				t.Fatalf("DecodePDU: %v", err)
			}
			out := pdu.LSP
			if out == nil {
				t.Fatalf("expected LSP, header type %v", pdu.Header.PDUType)
			}
			if out.PDUType != pt || out.RemainingLifetime != 1199 ||
				out.LSPID != in.LSPID || out.SequenceNumber != 0x10 || out.TypeBlock != in.TypeBlock {
				t.Errorf("field mismatch: %+v", out)
			}
			if out.Checksum != in.Checksum {
				t.Errorf("decoded checksum %#04x != encoded %#04x", out.Checksum, in.Checksum)
			}
			if !out.IsOverloaded() {
				t.Error("OL bit lost (IsOverloaded false)")
			}
			if len(out.TLVs) != 2 {
				t.Fatalf("got %d TLVs, want 2", len(out.TLVs))
			}
			// AC-3: VerifyChecksum over the decoded raw bytes must be valid.
			if !out.VerifyChecksum() {
				t.Error("VerifyChecksum() false on a freshly-encoded LSP")
			}
		})
	}
}

// VALIDATES: AC-3 -- flipping any byte of an encoded LSP (after the Remaining
// Lifetime field, inside the checksummed region) makes VerifyChecksum fail.
func TestISISLSPChecksumDetectsCorruption(t *testing.T) {
	sys := types.SystemID{1, 1, 1, 1, 1, 1}
	in := &LSP{
		PDUType:           PDUTypeL2LSP,
		RemainingLifetime: 600,
		LSPID:             types.NewLSPID(types.NewSourceID(sys, 0), 0),
		SequenceNumber:    1,
		TLVs:              []TLV{ext135TLV(netip.MustParsePrefix("203.0.113.0/24"), 5)},
	}
	buf := make([]byte, in.EncodedLen())
	in.WriteTo(buf, 0)

	// The checksummed region begins after Remaining Lifetime (common header +
	// PDU length + lifetime).
	regionStart := CommonHeaderLen + lspRemLifetimeOff + types.LifetimeLen
	// Corrupt a byte inside the region (the LSPID) and re-decode.
	buf[regionStart] ^= 0xff
	pdu, err := DecodePDU(buf)
	if err != nil {
		// A corrupted LSPID still parses structurally; we want the checksum to
		// be the failing signal, so the decode itself should succeed.
		t.Fatalf("DecodePDU after corruption: %v", err)
	}
	if pdu.LSP.VerifyChecksum() {
		t.Error("VerifyChecksum() true after corrupting a region byte")
	}
}

// VALIDATES: an LSP with no TLVs (minimal body) round-trips and checksums.
func TestISISLSPEmptyTLVs(t *testing.T) {
	in := &LSP{
		PDUType:           PDUTypeL1LSP,
		RemainingLifetime: 1200,
		LSPID:             types.NewLSPID(types.NewSourceID(types.SystemID{2, 2, 2, 2, 2, 2}, 0), 0),
		SequenceNumber:    1,
	}
	buf := make([]byte, in.EncodedLen())
	n := in.WriteTo(buf, 0)
	if n != CommonHeaderLen+lspFixedLen {
		t.Fatalf("minimal LSP size = %d, want %d", n, CommonHeaderLen+lspFixedLen)
	}
	pdu, err := DecodePDU(buf)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if !pdu.LSP.VerifyChecksum() {
		t.Error("VerifyChecksum() false on a no-TLV LSP")
	}
	if len(pdu.LSP.TLVs) != 0 {
		t.Errorf("expected 0 TLVs, got %d", len(pdu.LSP.TLVs))
	}
}

// VALIDATES: an unknown TLV inside an LSP survives an encode->decode->encode
// cycle byte-for-byte (AC-5 at the PDU level): the LSDB can re-flood verbatim.
func TestISISLSPUnknownTLVReencode(t *testing.T) {
	in := &LSP{
		PDUType:           PDUTypeL2LSP,
		RemainingLifetime: 1000,
		LSPID:             types.NewLSPID(types.NewSourceID(types.SystemID{3, 3, 3, 3, 3, 3}, 0), 0),
		SequenceNumber:    2,
		TLVs: []TLV{
			{Type: 222, Value: []byte{0xca, 0xfe, 0xba, 0xbe}}, // unknown TLV
			ext135TLV(netip.MustParsePrefix("10.9.0.0/16"), 7),
		},
	}
	buf := make([]byte, in.EncodedLen())
	n := in.WriteTo(buf, 0)

	pdu, err := DecodePDU(buf)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	// Re-encode the decoded LSP (copying TLV values out so they don't alias buf).
	reencoded := &LSP{
		PDUType:           pdu.LSP.PDUType,
		RemainingLifetime: pdu.LSP.RemainingLifetime,
		LSPID:             pdu.LSP.LSPID,
		SequenceNumber:    pdu.LSP.SequenceNumber,
		TypeBlock:         pdu.LSP.TypeBlock,
	}
	for _, tlv := range pdu.LSP.TLVs {
		reencoded.TLVs = append(reencoded.TLVs, tlv.CopyValue())
	}
	buf2 := make([]byte, reencoded.EncodedLen())
	n2 := reencoded.WriteTo(buf2, 0)
	if n != n2 {
		t.Fatalf("re-encode length %d != original %d", n2, n)
	}
	for i := range n {
		if buf[i] != buf2[i] {
			t.Fatalf("re-encode drift at byte %d: %#02x != %#02x", i, buf2[i], buf[i])
		}
	}
}
