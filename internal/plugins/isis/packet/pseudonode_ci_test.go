// Design: docs/architecture/isis/isis-8-dis-broadcast.md -- fixture pin for test/isis/isis-dis.ci.
//
// VALIDATES: the DIS pseudo-node LSP fixture used by the functional test decodes
// through DecodePDU into an L1 LSP whose LSP ID carries a NON-ZERO pseudonode
// octet (the LAN ID <system-id>.<pseudonode-id>) and whose TLV 22 lists every
// segment member at metric 0 (ISO/IEC 10589 clause 8.4.5). The .ci runs the same
// bytes through `ze isis decode`; pinning them here keeps the functional fixture
// and the codec in lock-step (a codec change that alters the bytes fails here too).
// PREVENTS: the isis-dis.ci pseudo-node fixture silently diverging from the codec.

package packet

import (
	"encoding/hex"
	"testing"
)

// pseudonodeLSPCIHex is the EXACT pseudo-node LSP used by test/isis/isis-dis.ci:
// an L1 LSP, LAN ID 0000.0000.0001.07 (pseudonode octet 0x07, non-zero), fragment
// 0, sequence 1, lifetime 1200, carrying one TLV 22 (Extended IS Reachability)
// listing members 0000.0000.0001 / 0000.0000.0002 / 0000.0000.0003 each at
// metric 0. Generated via the isis-2 LSP encoder (which stamps the Fletcher
// checksum); pinned so the codec and the .ci cannot drift.
const pseudonodeLSPCIHex = "831b010612010000003e04b0000000000001070000000001e3d4011621000000000001000000000000000000000200000000000000000000030000000000"

func TestISISPseudonodeCIFixtureDecodes(t *testing.T) {
	wire, err := hex.DecodeString(pseudonodeLSPCIHex)
	if err != nil {
		t.Fatalf("fixture hex: %v", err)
	}
	pdu, err := DecodePDU(wire)
	if err != nil {
		t.Fatalf("DecodePDU(pseudo-node fixture): %v", err)
	}
	if pdu.Header.PDUType != PDUTypeL1LSP {
		t.Fatalf("PDU type = %v, want l1-lsp", pdu.Header.PDUType)
	}
	lsp := pdu.LSP
	if lsp == nil {
		t.Fatal("LSP nil")
	}
	// A non-zero pseudonode octet is what makes this a pseudo-node LSP (AC-3).
	if pn := lsp.LSPID.PseudonodeID(); pn != 0x07 {
		t.Fatalf("pseudonode octet = %02x, want 07", pn)
	}
	if got := lsp.LSPID.String(); got != "0000.0000.0001.07-00" {
		t.Fatalf("lsp-id = %q, want 0000.0000.0001.07-00", got)
	}
	if lsp.SequenceNumber != 1 {
		t.Fatalf("sequence = %d, want 1", lsp.SequenceNumber)
	}
	// The TLV 22 must list all three members, each at metric 0 (clause 8.4.5).
	var entries int
	for _, tlv := range lsp.TLVs {
		if tlv.Type != TLVExtendedISReach {
			continue
		}
		dec, err := DecodeExtendedISReachTLV(tlv.Value)
		if err != nil {
			t.Fatalf("decode TLV 22: %v", err)
		}
		for _, e := range dec.Entries {
			entries++
			if e.Metric.Value() != 0 {
				t.Fatalf("member %s metric = %d, want 0", e.Neighbor, e.Metric.Value())
			}
		}
	}
	if entries != 3 {
		t.Fatalf("pseudo-node LSP lists %d members, want 3", entries)
	}

	// Re-encoding the decoded LSP reproduces the exact pinned bytes (round-trip),
	// so the .ci fixture is byte-stable against the encoder.
	out := make([]byte, lsp.EncodedLen())
	n := lsp.WriteTo(out, 0)
	if got := hex.EncodeToString(out[:n]); got != pseudonodeLSPCIHex {
		t.Fatalf("re-encode drift:\n got=%s\nwant=%s", got, pseudonodeLSPCIHex)
	}
}
