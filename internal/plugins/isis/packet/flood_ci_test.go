// Design: docs/architecture/isis/isis-7-flooding.md -- fixture pin for test/isis/isis-flooding.ci
package packet

import (
	"encoding/hex"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// csnpCIHex and psnpCIHex are the EXACT CSNP/PSNP fixtures used by
// test/isis/isis-flooding.ci. They are pinned here so the flooding-spec (isis-7)
// SNP wire format and the functional fixture cannot drift: if the CSNP/PSNP body
// codec or the TLV 9 (LSP Entries) encoder changes the bytes, this test fails
// alongside the .ci. Both carry one TLV 9 entry for LSP 0000.0000.0001.00-00
// (seq 7, lifetime 1000, checksum 0x1234); the CSNP spans the whole LSP-ID range.
const (
	csnpCIHex = "83210106190100000033000000000002000000000000000000ffffffffffffffff091003e80000000000010000000000071234"
	psnpCIHex = "831101061b010000002300000000000200091003e80000000000010000000000071234"
)

// VALIDATES: spec-isis-7 wiring -- the CSNP/PSNP fixtures the functional test
// feeds `ze isis decode` decode through DecodePDU into the expected L2 CSNP/PSNP
// with the TLV 9 entry isis-7 builds. The .ci runs the same bytes through the
// CLI; this keeps the fixture and the codec in lock-step.
// PREVENTS: the functional flooding fixture silently diverging from codec output.
func TestISISFloodCIFixtures(t *testing.T) {
	wantEntryLSPID := types.NewLSPID(types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 1}, 0), 0)
	wantSrc := types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 2}, 0)

	t.Run("csnp", func(t *testing.T) {
		wire, err := hex.DecodeString(csnpCIHex)
		if err != nil {
			t.Fatalf("fixture hex: %v", err)
		}
		pdu, err := DecodePDU(wire)
		if err != nil {
			t.Fatalf("DecodePDU(csnp): %v", err)
		}
		if pdu.Header.PDUType != PDUTypeL2CSNP || pdu.CSNP == nil {
			t.Fatalf("PDU type = %v, want l2-csnp", pdu.Header.PDUType)
		}
		c := pdu.CSNP
		if c.SourceID != wantSrc {
			t.Errorf("source-id = %s, want %s", c.SourceID, wantSrc)
		}
		if c.StartLSPID != (types.LSPID{}) {
			t.Errorf("start-lsp-id = %s, want all-zero", c.StartLSPID)
		}
		var maxID types.LSPID
		for i := range maxID {
			maxID[i] = 0xff
		}
		if c.EndLSPID != maxID {
			t.Errorf("end-lsp-id = %s, want all-ones", c.EndLSPID)
		}
		assertOneTLV9Entry(t, c.TLVs, wantEntryLSPID)
	})

	t.Run("psnp", func(t *testing.T) {
		wire, err := hex.DecodeString(psnpCIHex)
		if err != nil {
			t.Fatalf("fixture hex: %v", err)
		}
		pdu, err := DecodePDU(wire)
		if err != nil {
			t.Fatalf("DecodePDU(psnp): %v", err)
		}
		if pdu.Header.PDUType != PDUTypeL2PSNP || pdu.PSNP == nil {
			t.Fatalf("PDU type = %v, want l2-psnp", pdu.Header.PDUType)
		}
		if pdu.PSNP.SourceID != wantSrc {
			t.Errorf("source-id = %s, want %s", pdu.PSNP.SourceID, wantSrc)
		}
		assertOneTLV9Entry(t, pdu.PSNP.TLVs, wantEntryLSPID)
	})
}

// assertOneTLV9Entry checks the TLV list carries exactly one TLV 9 with one LSP
// entry for wantID at seq 7 / lifetime 1000 / checksum 0x1234.
func assertOneTLV9Entry(t *testing.T, tlvs []TLV, wantID types.LSPID) {
	t.Helper()
	if len(tlvs) != 1 || tlvs[0].Type != TLVLSPEntries {
		t.Fatalf("TLVs = %+v, want one TLV 9", tlvs)
	}
	dec, err := DecodeLSPEntriesTLV(tlvs[0].Value)
	if err != nil {
		t.Fatalf("DecodeLSPEntriesTLV: %v", err)
	}
	if len(dec.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(dec.Entries))
	}
	e := dec.Entries[0]
	if e.LSPID != wantID {
		t.Errorf("entry LSPID = %s, want %s", e.LSPID, wantID)
	}
	if e.SequenceNumber != 7 {
		t.Errorf("entry seq = %d, want 7", e.SequenceNumber)
	}
	if e.RemainingLifetime.Seconds() != 1000 {
		t.Errorf("entry lifetime = %d, want 1000", e.RemainingLifetime.Seconds())
	}
	if e.Checksum != 0x1234 {
		t.Errorf("entry checksum = %#04x, want 0x1234", e.Checksum)
	}
}
