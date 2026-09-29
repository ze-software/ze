// Design: docs/architecture/isis/isis-7-flooding.md -- flooding a received LSP verbatim
// Related: flooding.go -- Flooder.ReceiveLSP and FloodTick, the producers under test
// Related: flooding_test.go -- recordingTx and the circuit helpers

package lsdb

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// VALIDATES: a received LSP that carries codes Ze does not recognize (TLV 131,
// the obsolete Inter-Domain Routing Protocol Information, and TLVs 199 and 250)
// is stored, and the flood sends it on the other circuit byte for byte, with
// the unrecognized TLVs still in it. It is not flooded back on its arrival circuit.
// PREVENTS: a receive path that refuses such an LSP, or a flood that rebuilds
// the LSP and strips or rewrites what it did not recognize.
// Method: Flooder.ReceiveLSP on a point-to-point circuit, one FloodTick, and the
// recordingTx capture of what the LAN circuit sent.
//
// RFC requirement: RFC1195-3.1-1 positive -- an LSP carrying TLVs 199 and 250 is stored by Flooder.ReceiveLSP and FloodTick sends the identical bytes, those TLVs included, on the other circuit.
// RFC requirement: RFC3787-x-1 positive -- a received LSP carrying TLV 131 is not refused: Flooder.ReceiveLSP stores it and FloodTick passes it on unchanged, TLV 131 included.
func TestRFC1195UnknownTLVsFloodedUnchanged(t *testing.T) {
	d := New(nil)
	rec := &recordingTx{}
	const cIn, cOut CircuitID = 1, 2
	f := NewFlooder(d, rec.tx, staticCircuits(p2pCircuit("in", cIn), l1l2Circuit("out", cOut)))

	unknown := []packet.TLV{
		{Type: 131, Value: []byte{0x00, 0x01}},
		{Type: 199, Value: []byte{0xde, 0xad}},
		{Type: 250, Value: []byte{0x01, 0x02, 0x03}},
	}
	id := lspID(30, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 5, 1000, unknown)

	if res := f.ReceiveLSP(cIn, true, lsp, raw); !res.Stored {
		t.Fatalf("LSP carrying unrecognized TLVs was not stored: %+v", res)
	}
	f.FloodTick()

	sent := rec.onCircuit("out")
	if len(sent) != 1 {
		t.Fatalf("out circuit sent %d LSPs, want 1", len(sent))
	}
	if !bytes.Equal(sent[0].pdu, raw) {
		t.Fatalf("flooded LSP differs from the received one:\n got % x\nwant % x", sent[0].pdu, raw)
	}
	for _, tlv := range unknown {
		want := append([]byte{tlv.Type, byte(len(tlv.Value))}, tlv.Value...)
		if !bytes.Contains(sent[0].pdu, want) {
			t.Errorf("flooded LSP lost TLV %d (% x)", tlv.Type, want)
		}
	}
	if got := len(rec.onCircuit("in")); got != 0 {
		t.Errorf("LSP flooded back on its arrival circuit %d times", got)
	}
}
