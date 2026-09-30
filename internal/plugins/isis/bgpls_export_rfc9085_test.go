// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS BGP-LS origination.
// RFC: rfc/short/rfc9085.md
// Related: bgpls_export.go -- srBlock translates the SRGB into TLV 1034
//
// VALIDATES: the SID/Label sub-TLV 1161 Ze originates inside the SR
// Capabilities TLV 1034 carries a 3-octet label whose 4 leftmost bits are 0,
// as RFC 9085 Section 2.1.1 requires, whatever the IS-IS source carried in
// those bits.
// PREVENTS: a source's high-order label bits reaching a collector, which then
// reads a label outside the 20-bit label space.
package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// originatedSRGBLabel stores one LSP whose Router Capability (TLV 242) carries
// an SR-Capabilities sub-TLV 2 with one SRGB of range 100 starting at the given
// 3-octet label, and returns the TLV 1034 values the BGP-LS builder originates.
func originatedSRGBLabel(t *testing.T, label [3]byte) [][]byte {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	srgb := []byte{0x80, 0, 0, 100, 1, 3, label[0], label[1], label[2]}
	capabilities := append([]byte{192, 0, 2, 2, 0, 2, byte(len(srgb))}, srgb...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{{Type: 242, Value: capabilities}})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Nodes) != 1 {
		t.Fatalf("nodes = %+v", builder.snapshot.Nodes)
	}
	return rfc9514Attributes(builder.snapshot.Nodes[0].Attributes, 1034)
}

// TestRFC9085OriginatedSRGBLabelTwentyBits originates an ordinary label.
//
// RFC requirement: RFC9085-2.1.1-1 positive -- an IS-IS SRGB starting at label 16000 is originated as TLV 1034 whose SID/Label sub-TLV 1161 has length 3 and value 00 3e 80: the label in the 20 rightmost bits, the 4 leftmost bits 0 (§2.1.1).
func TestRFC9085OriginatedSRGBLabelTwentyBits(t *testing.T) {
	got := originatedSRGBLabel(t, [3]byte{0x00, 0x3e, 0x80})
	want := []byte{0x80, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	if len(got) != 1 || !bytes.Equal(got[0], want) {
		t.Fatalf("TLV 1034 = %x, want one %x", got, want)
	}
}

// TestRFC9085OriginatedSRGBLabelHighBitsCleared hands the producer a source
// label whose 4 leftmost bits are set, the input that reaches the wire if the
// producer copies the three octets verbatim.
//
// RFC requirement: RFC9085-2.1.1-1 negative -- an IS-IS SRGB label f0 3e 80 is originated as SID/Label value 00 3e 80: the 4 leftmost bits are set to 0 and the 20 rightmost bits keep the label (§2.1.1).
func TestRFC9085OriginatedSRGBLabelHighBitsCleared(t *testing.T) {
	got := originatedSRGBLabel(t, [3]byte{0xf0, 0x3e, 0x80})
	want := []byte{0x80, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	if len(got) != 1 || !bytes.Equal(got[0], want) {
		t.Fatalf("TLV 1034 = %x, want one %x", got, want)
	}
}
