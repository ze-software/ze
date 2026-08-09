// Design: docs/architecture/wire/isis.md -- CSNP round-trip tests
package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// lspEntriesTLV builds an opaque TLV 9 carrying the given entries for a
// CSNP/PSNP body test.
func lspEntriesTLV(entries []LSPEntry) TLV {
	in := LSPEntriesTLV{Entries: entries}
	buf := make([]byte, 256)
	n := writeLSPEntriesTLV(buf, 0, in)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	cp := make([]byte, len(value))
	copy(cp, value)
	return TLV{Type: TLVLSPEntries, Value: cp}
}

// VALIDATES: AC-2 -- L1 and L2 CSNP round-trip the source ID, the start/end
// LSP-ID range, and the TLV 9 LSP-entry list.
// Story 4: a CSNP advertising LSP summaries -> WriteTo -> decode -> entries
// match.
func TestISISCSNPRoundTrip(t *testing.T) {
	sys := types.SystemID{0, 1, 0, 2, 0, 3}
	src := types.NewSourceID(sys, 0)
	startID := types.NewLSPID(types.NewSourceID(types.SystemID{}, 0), 0)                                       // all-zero (range start)
	endID := types.NewLSPID(types.NewSourceID(types.SystemID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0xff), 0xff) // all-ones (range end)
	entries := []LSPEntry{
		{RemainingLifetime: 1199, LSPID: types.NewLSPID(src, 0), SequenceNumber: 1, Checksum: 0x1234},
		{RemainingLifetime: 900, LSPID: types.NewLSPID(src, 1), SequenceNumber: 2, Checksum: 0x5678},
	}
	for _, pt := range []PDUType{PDUTypeL1CSNP, PDUTypeL2CSNP} {
		t.Run(pt.String(), func(t *testing.T) {
			in := &CSNP{
				PDUType:    pt,
				SourceID:   src,
				StartLSPID: startID,
				EndLSPID:   endID,
				TLVs:       []TLV{lspEntriesTLV(entries)},
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
			out := pdu.CSNP
			if out == nil {
				t.Fatalf("expected CSNP, header type %v", pdu.Header.PDUType)
			}
			if out.PDUType != pt || out.SourceID != src || out.StartLSPID != startID || out.EndLSPID != endID {
				t.Errorf("field mismatch: %+v", out)
			}
			if len(out.TLVs) != 1 || out.TLVs[0].Type != TLVLSPEntries {
				t.Fatalf("TLVs = %+v", out.TLVs)
			}
			got, err := DecodeLSPEntriesTLV(out.TLVs[0].Value)
			if err != nil {
				t.Fatalf("DecodeLSPEntriesTLV: %v", err)
			}
			if len(got.Entries) != len(entries) {
				t.Fatalf("got %d entries, want %d", len(got.Entries), len(entries))
			}
			for i := range entries {
				if got.Entries[i] != entries[i] {
					t.Errorf("entry[%d] = %+v, want %+v", i, got.Entries[i], entries[i])
				}
			}
		})
	}
}
