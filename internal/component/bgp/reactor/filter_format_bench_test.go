package reactor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
)

// buildAttrsWireFixture constructs a representative parsed-attributes fixture
// for benchmarking: origin + as-path + med + community + large-community.
// The underlying attributes are built via the attribute.Builder so the
// fixture exercises the real lazy-parse path used in production.
func buildAttrsWireFixture(tb testing.TB) *attribute.AttributesWire {
	tb.Helper()
	b := attribute.NewBuilder()
	if err := b.ParseOrigin("igp"); err != nil {
		tb.Fatalf("origin: %v", err)
	}
	if err := b.ParseASPath("65001 65002 65003"); err != nil {
		tb.Fatalf("as-path: %v", err)
	}
	if err := b.ParseMED("100"); err != nil {
		tb.Fatalf("med: %v", err)
	}
	if err := b.ParseLocalPref("100"); err != nil {
		tb.Fatalf("local-preference: %v", err)
	}
	if err := b.ParseCommunity("65001:100 65001:200"); err != nil {
		tb.Fatalf("community: %v", err)
	}
	if err := b.ParseLargeCommunity("65001:1:2 65001:1:3"); err != nil {
		tb.Fatalf("large-community: %v", err)
	}
	wire := b.Build()
	// Register a minimal ASN4 encoding context so lazy-parse succeeds without
	// allocating an error. Otherwise AttributesWire.Get would call fmt.Errorf
	// on every invocation (sourceCtxID 0 is not registered).
	ctxID := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	return attribute.NewAttributesWire(wire, ctxID)
}

// VALIDATES: AppendAttrsForFilter on a warm scratch reports 0 allocs/op for
// a representative multi-attribute UPDATE. Regression guard for spec AC-6.
func BenchmarkAppendAttrsForFilter_Reused(b *testing.B) {
	attrs := buildAttrsWireFixture(b)
	scratch := make([]byte, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = AppendAttrsForFilter(scratch[:0], attrs, nil)
	}
	_ = scratch
}

// VALIDATES: AppendUpdateForFilter on a warm scratch reports 0 allocs/op
// when called without a WireUpdate (attrs-only path). Regression guard for
// spec AC-6.
func BenchmarkAppendUpdateForFilter_Reused(b *testing.B) {
	attrs := buildAttrsWireFixture(b)
	scratch := make([]byte, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
	}
	_ = scratch
}

// VALIDATES: the boundary allocation is measured separately: after the
// Append* path produces bytes, the single `string(scratch)` conversion costs
// 1 allocation. Regression guard for the AC-6 accounting split.
func BenchmarkFormat_Boundary_StringConvert(b *testing.B) {
	attrs := buildAttrsWireFixture(b)
	scratch := make([]byte, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	var out string
	for range b.N {
		scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
		out = string(scratch)
	}
	_ = out
}
