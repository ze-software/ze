package format

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// buildBenchUpdate builds a representative UPDATE fixture (INET prefix +
// origin + as-path + med + local-pref) for the UPDATE-path benchmarks.
func buildBenchUpdate(tb testing.TB) (plugin.PeerInfo, bgptypes.RawMessage, bgptypes.ContentConfig) {
	tb.Helper()
	ctxID := testEncodingContext()
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		Name:    "bench-peer",
		PeerAS:  65001,
	}
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0,   // igp
		100, // local-pref
		[]uint32{65001, 65002, 65003},
	)
	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		tb.Fatalf("Attrs() error = %v", err)
	}
	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  rpc.DirectionReceived,
		MessageID:  42,
	}
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}
	return peer, msg, content
}

// VALIDATES: AC-15 — AppendMessage on a warm scratch reports 0 allocs/op
// for a representative INET UPDATE. The same property as fmt-0's
// BenchmarkAppendAttrsForFilter_Reused extended to the UPDATE path.
// PREVENTS: regressions that reintroduce hidden allocations in the parsed
// JSON UPDATE formatter.
func BenchmarkAppendUpdate_Reused(b *testing.B) {
	peer, msg, content := buildBenchUpdate(b)
	scratch := make([]byte, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = AppendMessage(scratch[:0], &peer, msg, content, "")
	}
	_ = scratch
}

// VALIDATES: AC-16 — the boundary allocation is accounted for separately:
// after AppendMessage fills a warm scratch, the single `string(scratch)`
// conversion at the plugin-IPC cache boundary costs 1 alloc/op.
// PREVENTS: regressions that double the boundary cost (e.g., an interim
// strings.Builder reintroduced in a helper on the UPDATE path).
func BenchmarkAppendUpdate_Boundary_StringConvert(b *testing.B) {
	peer, msg, content := buildBenchUpdate(b)
	scratch := make([]byte, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	var out string
	for range b.N {
		scratch = AppendMessage(scratch[:0], &peer, msg, content, "")
		out = string(scratch)
	}
	_ = out
}

// VALIDATES: AppendMessage full-format (parsed + raw + route-meta) stays
// on a bounded allocation path. Tracked rather than asserted at 0 -- the
// full format legitimately allocates for the map iteration over
// rawComps.NLRI / Withdrawn.
func BenchmarkAppendUpdate_FullPath(b *testing.B) {
	peer, msg, content := buildBenchUpdate(b)
	content.Format = plugin.FormatFull
	scratch := make([]byte, 0, 8192)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		scratch = AppendMessage(scratch[:0], &peer, msg, content, "")
	}
	_ = scratch
}
