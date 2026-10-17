// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS BGP-LS origination.
// RFC: rfc/short/rfc9552.md
// Related: bgpls_export.go -- narrowLinks maps TLV 2 into TLV 1095, linkAttribute
// widens sub-TLV 18 into TLV 1092
//
// VALIDATES: the two metric widenings the IS-IS source performs when it
// originates BGP-LS link attributes. A narrow (TLV 2) default metric octet is
// originated as a 1-octet IGP Metric TLV 1095 with its two most significant
// bits zero, whatever reserved or I/E bit the source octet carried. A 3-octet TE Default
// metric (RFC 5305 sub-TLV 18) is originated as a 4-octet TLV 1092 with the
// high-order octet zero.
// PREVENTS: the reserved bit 8 and the I/E bit of a narrow metric octet reaching a
// collector as metric value bits, and a 24-bit TE metric landing in the wrong
// octets of the 32-bit field.
package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// originatedNarrowIGPMetric stores one LSP whose IS Reachability (TLV 2)
// entry carries the given default metric octet, and returns the TLV 1095
// values the BGP-LS builder originates for the link. The delay, expense and
// error metric octets carry the S bit (0x80), so they are unsupported and
// only the default metric is mapped.
func originatedNarrowIGPMetric(t *testing.T, defaultMetric byte) [][]byte {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	reach := []byte{0, defaultMetric, 0x80, 0x80, 0x80, 0, 0, 0, 0, 0, 3, 0}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 242, Value: []byte{192, 0, 2, 2, 0}},
		{Type: packet.TLVISReachabilityNarrow, Value: reach},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 1 {
		t.Fatalf("links = %+v", builder.snapshot.Links)
	}
	return rfc9514Attributes(builder.snapshot.Links[0].Attributes, 1095)
}

// TestRFC9552ISISSmallMetricOriginatedWithTopBitsZero originates a narrow
// metric octet 0xca: the reserved bit 8 (0x80) and the I/E bit (0x40) set over the
// 6-bit metric 10.
//
// RFC 9552 Section 5.3.2.4: "IS-IS small metrics are 6 bits in size but are
// encoded in a 1-octet field; therefore, the two most significant bits of the
// field MUST be set to 0 by the originator and MUST be ignored by the
// receiver."
//
// MUTATION: packet/tlv_neighbours.go DecodeNarrowISReachTLV with
// `def & narrowMetricValueMask` reduced to `def` originates 0xca and reddens
// this test.
//
// RFC requirement: RFC9552-5.3.2.3-2 positive -- the IS-IS narrow default metric octet ca (reserved bit and I/E set over metric 10) is originated as the 1-octet TLV 1095 value 0a: one octet, the two most significant bits zero (§5.3.2.4).
func TestRFC9552ISISSmallMetricOriginatedWithTopBitsZero(t *testing.T) {
	got := originatedNarrowIGPMetric(t, 0xca)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0x0a}) {
		t.Fatalf("TLV 1095 = %x, want one 0a", got)
	}
}

// TestRFC9552ISISSmallMetricFlagBitsNeverReachIGPMetric originates a narrow
// metric octet with every bit set, the input that puts the reserved and I/E bits on the
// wire if the originator copies the octet unmasked.
//
// MUTATION: packet/tlv_neighbours.go DecodeNarrowISReachTLV with
// `def & narrowMetricValueMask` reduced to `def` originates 0xff and reddens
// this test.
//
// RFC requirement: RFC9552-5.3.2.3-2 negative -- the IS-IS narrow default metric octet ff is originated as the 1-octet TLV 1095 value 3f, never ff: neither of the two most significant bits survives into the IGP Metric TLV (§5.3.2.4).
func TestRFC9552ISISSmallMetricFlagBitsNeverReachIGPMetric(t *testing.T) {
	got := originatedNarrowIGPMetric(t, 0xff)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0x3f}) {
		t.Fatalf("TLV 1095 = %x, want one 3f", got)
	}
}

// TestRFC9552ISISTEDefaultMetricZeroPaddedHighOrder originates a 24-bit TE
// Default metric whose own top bit is set, so a value shifted by one octet
// would be told apart from the padded one.
//
// RFC 9552 Section 5.3.2.3: "If a source protocol uses a metric width of fewer
// than 32 bits, then the high-order bits of this field MUST be padded with
// zero."
//
// MUTATION: bgpls_export.go linkAttribute case 18 with `copy(v[1:], value)`
// reduced to `copy(v, value)` originates 80 00 01 00 and reddens this test.
//
// RFC requirement: RFC9552-5.3.2.3-1 positive -- the IS-IS 3-octet TE Default metric 80 00 01 is originated as the 4-octet TLV 1092 value 00 80 00 01: the source octets in the low-order three, the high-order octet padded with zero (§5.3.2.3).
func TestRFC9552ISISTEDefaultMetricZeroPaddedHighOrder(t *testing.T) {
	got := originatedTEDefaultMetric(t, [3]byte{0x80, 0x00, 0x01})
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 0x80, 0x00, 0x01}) {
		t.Fatalf("TLV 1092 = %x, want one 00800001", got)
	}
}

// TestRFC9552ISISTEDefaultMetricSourceNeverInHighOrder originates a 24-bit TE
// Default metric with no zero octet, the input whose leading octet lands in
// the high-order octet if the producer copies into the wrong offset.
//
// MUTATION: bgpls_export.go linkAttribute case 18 with `copy(v[1:], value)`
// reduced to `copy(v, value)` originates fe dc ba 00 and reddens this test.
//
// RFC requirement: RFC9552-5.3.2.3-1 negative -- the IS-IS 3-octet TE Default metric fe dc ba is originated as 00 fe dc ba: the high-order octet of TLV 1092 is zero, never a source octet (§5.3.2.3).
func TestRFC9552ISISTEDefaultMetricSourceNeverInHighOrder(t *testing.T) {
	got := originatedTEDefaultMetric(t, [3]byte{0xfe, 0xdc, 0xba})
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 0xfe, 0xdc, 0xba}) {
		t.Fatalf("TLV 1092 = %x, want one 00fedcba", got)
	}
}

// TestRFC9552ISISTEDefaultMetricRebuildNeverCarriesStaleArena rebuilds on ONE
// builder, the way the exporter reuses its arena on every LSDB change. The
// first build originates an unmapped TLV of 0xff octets, which the builder
// copies into its arena as an Opaque Node value. The second build originates
// the TE Default metric 0a 0b 0c, whose 4-octet TLV 1092 value is carved from
// the same arena over those stale 0xff octets, so its high-order octet is zero
// only because bgplsBuilder.alloc clears what it hands out.
//
// MUTATION: bgpls_export.go alloc with `clear(value)` removed originates
// ff 0a 0b 0c on the second build and reddens this test.
//
// RFC requirement: RFC9552-5.3.2.3-1 negative -- on a second build over an arena whose octet under the new TLV 1092 still holds 0xff from the first build, the IS-IS 3-octet TE Default metric 0a 0b 0c is originated as 00 0a 0b 0c: the stale octet never reaches the high-order octet (§5.3.2.3).
func TestRFC9552ISISTEDefaultMetricRebuildNeverCarriesStaleArena(t *testing.T) {
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	stale := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(stale.shutdown)
	bgplsStoreLSP(t, stale, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 251, Value: bytes.Repeat([]byte{0xff}, 200)},
	})
	fresh := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(fresh.shutdown)
	reach := []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 10, 5, 18, 3, 0x0a, 0x0b, 0x0c}
	bgplsStoreLSP(t, fresh, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 242, Value: []byte{192, 0, 2, 2, 0}},
		{Type: 22, Value: reach},
	})

	var builder bgplsBuilder
	builder.build(stale.lsdb.RawSnapshot(lsdb.Level1))
	prior := bytes.Clone(builder.arena[:cap(builder.arena)])
	builder.build(fresh.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 1 {
		t.Fatalf("links = %+v", builder.snapshot.Links)
	}
	got := rfc9514Attributes(builder.snapshot.Links[0].Attributes, 1092)
	if len(got) != 1 || len(got[0]) != 4 {
		t.Fatalf("TLV 1092 = %x, want one 4-octet value", got)
	}
	// The value is a sub-slice of the arena the first build filled, so its
	// offset is what its capacity is short of the arena's.
	offset := cap(builder.arena) - cap(got[0])
	if offset < 0 || offset >= len(prior) || prior[offset] == 0 {
		t.Fatalf("fixture: arena octet %d under TLV 1092 was not stale after the first build", offset)
	}
	if !bytes.Equal(got[0], []byte{0, 0x0a, 0x0b, 0x0c}) {
		t.Fatalf("TLV 1092 = %x, want 000a0b0c", got[0])
	}
}
