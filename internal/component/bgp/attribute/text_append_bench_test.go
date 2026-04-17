package attribute

import (
	"net/netip"
	"testing"
)

// VALIDATES: 0 allocs/op when AppendText is called on a warm-scratch buffer
// with a representative multi-community list. Regression guard for the spec's
// zero-alloc hot-path invariant (AC-2, AC-6).
func BenchmarkAppendTextCommunity(b *testing.B) {
	comms := Communities{
		Community(uint32(CommunityNoExport)),
		Community(0x12340005),
		Community(0x56780006),
		Community(0xFFFF029A),
	}
	scratch := make([]byte, 0, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = comms.AppendText(scratch[:0])
	}
	_ = scratch
}

// VALIDATES: *Aggregator.AppendText emits its value form with 0 allocs/op on
// a warm scratch. Regression guard for spec AC-1b.
func BenchmarkAppendTextAggregator(b *testing.B) {
	agg := &Aggregator{ASN: 65001, Address: netip.MustParseAddr("192.0.2.1")}
	scratch := make([]byte, 0, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = agg.AppendText(scratch[:0])
	}
	_ = scratch
}

// VALIDATES: ASPath.AppendText on a multi-ASN path hits 0 allocs/op on
// warm scratch. Regression guard for the common filter-dispatch case.
func BenchmarkAppendTextASPath(b *testing.B) {
	p := &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003, 65004, 65005}}}}
	scratch := make([]byte, 0, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = p.AppendText(scratch[:0])
	}
	_ = scratch
}
