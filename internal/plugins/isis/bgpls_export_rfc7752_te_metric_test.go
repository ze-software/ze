// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS BGP-LS origination.
// RFC: rfc/short/rfc7752.md
// Related: bgpls_export.go -- linkAttribute widens sub-TLV 18 into TLV 1092
//
// VALIDATES: the TE Default Metric TLV 1092 the IS-IS source originates from
// the 3-octet IS-IS TE Default metric (RFC 5305 sub-TLV 18) is 4 octets with
// the high-order octet zero, as RFC 7752 Section 3.3.2.3 requires of a metric
// narrower than 32 bits.
// PREVENTS: the 24-bit metric copied into the wrong octets, which would scale
// it by 256 or put garbage in the high-order octet a collector reads.
package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// originatedTEDefaultMetric stores one LSP whose Extended IS Reachability
// (TLV 22) entry carries a TE Default metric sub-TLV 18 of the given 3 octets,
// and returns the TLV 1092 values the BGP-LS builder originates for the link.
func originatedTEDefaultMetric(t *testing.T, metric [3]byte) [][]byte {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	reach := []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 10, 5, 18, 3, metric[0], metric[1], metric[2]}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 242, Value: []byte{192, 0, 2, 2, 0}},
		{Type: 22, Value: reach},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 1 {
		t.Fatalf("links = %+v", builder.snapshot.Links)
	}
	return rfc9514Attributes(builder.snapshot.Links[0].Attributes, 1092)
}

// TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder originates an ordinary
// 24-bit metric.
//
// RFC requirement: RFC7752-3.3.2.3-1 positive -- the IS-IS 3-octet TE Default metric 0x0a0b0c is originated as the 4-octet TLV 1092 value 00 0a 0b 0c: the source octets in the low-order three, the high-order octet padded with zero (Section 3.3.2.3).
func TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder(t *testing.T) {
	got := originatedTEDefaultMetric(t, [3]byte{0x0a, 0x0b, 0x0c})
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 0x0a, 0x0b, 0x0c}) {
		t.Fatalf("TLV 1092 = %x, want one 000a0b0c", got)
	}
}

// TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder originates the largest
// 24-bit metric, the input that sets the high-order octet if the producer
// copies into the wrong offset.
//
// RFC requirement: RFC7752-3.3.2.3-1 negative -- the IS-IS 3-octet TE Default metric ff ff ff is originated as 00 ff ff ff: the high-order octet stays zero, never ff (Section 3.3.2.3).
func TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder(t *testing.T) {
	got := originatedTEDefaultMetric(t, [3]byte{0xff, 0xff, 0xff})
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 0xff, 0xff, 0xff}) {
		t.Fatalf("TLV 1092 = %x, want one 00ffffff", got)
	}
}
