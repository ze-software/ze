// Design: docs/architecture/wire/isis.md -- PSNP round-trip tests
package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// VALIDATES: AC-2 -- L1 and L2 PSNP round-trip the source ID and the TLV 9
// LSP-entry list (used to request/acknowledge individual LSPs).
func TestISISPSNPRoundTrip(t *testing.T) {
	sys := types.SystemID{7, 7, 7, 7, 7, 7}
	src := types.NewSourceID(sys, 0)
	entries := []LSPEntry{
		{RemainingLifetime: 0, LSPID: types.NewLSPID(src, 0), SequenceNumber: 0, Checksum: 0}, // a request entry (zeros)
		{RemainingLifetime: 1199, LSPID: types.NewLSPID(src, 1), SequenceNumber: 9, Checksum: 0xbeef},
	}
	for _, pt := range []PDUType{PDUTypeL1PSNP, PDUTypeL2PSNP} {
		t.Run(pt.String(), func(t *testing.T) {
			in := &PSNP{
				PDUType:  pt,
				SourceID: src,
				TLVs:     []TLV{lspEntriesTLV(entries)},
			}
			buf := make([]byte, in.EncodedLen())
			n := in.WriteTo(buf, 0)
			if n != in.EncodedLen() {
				t.Fatalf("WriteTo returned %d, want %d", n, in.EncodedLen())
			}
			pdu, err := DecodePDU(buf)
			if err != nil {
				t.Fatalf("DecodePDU: %v", err)
			}
			out := pdu.PSNP
			if out == nil {
				t.Fatalf("expected PSNP, header type %v", pdu.Header.PDUType)
			}
			if out.PDUType != pt || out.SourceID != src {
				t.Errorf("field mismatch: %+v", out)
			}
			if len(out.TLVs) != 1 || out.TLVs[0].Type != TLVLSPEntries {
				t.Fatalf("TLVs = %+v", out.TLVs)
			}
			got, err := DecodeLSPEntriesTLV(out.TLVs[0].Value)
			if err != nil {
				t.Fatalf("DecodeLSPEntriesTLV: %v", err)
			}
			for i := range entries {
				if got.Entries[i] != entries[i] {
					t.Errorf("entry[%d] = %+v, want %+v", i, got.Entries[i], entries[i])
				}
			}
		})
	}
}
