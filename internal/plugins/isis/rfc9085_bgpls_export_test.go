// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS BGP-LS origination.
// RFC: rfc/short/rfc9085.md
// Related: bgpls_export.go -- srBlock translates the SRGB into TLV 1034
//
// VALIDATES: the SID/Label sub-TLV 1161 Ze originates inside the SR
// Capabilities TLV 1034 carries a 3-octet label whose 4 leftmost bits are 0,
// as RFC 9085 Section 2.1.1 requires, whatever the IS-IS source carried in
// those bits. The SR Capabilities TLV 1034 and SR Local Block TLV 1036 it
// originates carry a Reserved octet of 0 (RFC 9085 Sections 2.1.2, 2.1.4).
// PREVENTS: a source's high-order label bits reaching a collector, which then
// reads a label outside the 20-bit label space, and native octets reaching the
// Reserved octet.
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

// originatedCapabilityRanges stores one LSP whose Router Capability (TLV 242)
// carries an SR-Capabilities sub-TLV 2 and an SRLB sub-TLV 22, each with the
// given native flags octet and one range of the given size starting at label
// 16000, and returns the TLV 1034 and TLV 1036 values the BGP-LS builder
// originates.
func originatedCapabilityRanges(t *testing.T, flags byte, size [3]byte) (srgb, srlb []byte) {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	block := []byte{flags, size[0], size[1], size[2], 1, 3, 0x00, 0x3e, 0x80}
	capabilities := []byte{192, 0, 2, 2, 0}
	capabilities = append(capabilities, 2, byte(len(block)))
	capabilities = append(capabilities, block...)
	capabilities = append(capabilities, 22, byte(len(block)))
	capabilities = append(capabilities, block...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{{Type: 242, Value: capabilities}})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Nodes) != 1 {
		t.Fatalf("nodes = %+v", builder.snapshot.Nodes)
	}
	global := rfc9514Attributes(builder.snapshot.Nodes[0].Attributes, 1034)
	local := rfc9514Attributes(builder.snapshot.Nodes[0].Attributes, 1036)
	if len(global) != 1 || len(local) != 1 {
		t.Fatalf("TLV 1034 = %x, TLV 1036 = %x, want one of each", global, local)
	}
	return global[0], local[0]
}

// TestRFC9085ISISOriginatedCapabilitiesReservedZero originates an ordinary
// IS-IS SRGB (I flag set) and SRLB and compares the exact TLV 1034 and 1036
// values, whose second octet is the Reserved octet.
//
// RFC requirement: RFC9085-2.1.2-2 positive -- the SR Capabilities TLV 1034 Ze originates from an IS-IS SR-Capabilities sub-TLV has Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-2 positive -- the SR Local Block TLV 1036 Ze originates from an IS-IS SRLB sub-TLV has Reserved octet 0 (§2.1.4).
func TestRFC9085ISISOriginatedCapabilitiesReservedZero(t *testing.T) {
	srgb, srlb := originatedCapabilityRanges(t, 0x80, [3]byte{0, 0, 100})
	wantSRGB := []byte{0x80, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	wantSRLB := []byte{0, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	if !bytes.Equal(srgb, wantSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, wantSRGB)
	}
	if !bytes.Equal(srlb, wantSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, wantSRLB)
	}
}

// TestRFC9085ISISAllOnesSourceNeverReachesReserved hands the producer native
// flags ff and a range size ff ff ff, the octets that land in the Reserved
// octet if the producer shifts or copies the native header.
//
// RFC requirement: RFC9085-2.1.2-2 negative -- an IS-IS SR-Capabilities sub-TLV with flags ff and range ffffff is originated as TLV 1034 with Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-2 negative -- an IS-IS SRLB sub-TLV with flags ff and range ffffff is originated as TLV 1036 with Reserved octet 0 (§2.1.4).
func TestRFC9085ISISAllOnesSourceNeverReachesReserved(t *testing.T) {
	srgb, srlb := originatedCapabilityRanges(t, 0xff, [3]byte{0xff, 0xff, 0xff})
	wantSRGB := []byte{0xc0, 0, 0xff, 0xff, 0xff, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	wantSRLB := []byte{0, 0, 0xff, 0xff, 0xff, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	if !bytes.Equal(srgb, wantSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, wantSRGB)
	}
	if !bytes.Equal(srlb, wantSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, wantSRLB)
	}
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
