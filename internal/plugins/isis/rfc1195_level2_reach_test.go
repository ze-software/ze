// Design: docs/architecture/isis/isis-9-spf-rib.md -- L1/L2 route leaking into the level 2 LSP.
//
// VALIDATES: RFC 1195 Section 3.2: a level 2 router announces one entry for a
// prefix in its level 2 LSP, at the minimum metric, and its directly reachable
// prefixes count the same as the ones it learned from level 1 LSPs.
// PREVENTS: an L1L2 router listing its own connected subnet twice in its L2 LSP,
// once at the interface metric and once at the metric of the path through the
// level 1 neighbor that shares the subnet.

package isis

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// tlv135Metrics returns the metric of every TLV 135 entry for prefix in lsp, in
// wire order, so a duplicate shows as a second element.
func tlv135Metrics(t *testing.T, lsp *packet.LSP, prefix netip.Prefix) []uint32 {
	t.Helper()
	var metrics []uint32
	for _, tl := range lsp.TLVs {
		if tl.Type != packet.TLVExtendedIPReach {
			continue
		}
		ext, err := packet.DecodeExtendedIPReachTLV(tl.Value)
		if err != nil {
			t.Fatalf("decode TLV 135: %v", err)
		}
		for _, ent := range ext.Entries {
			if ent.Prefix == prefix {
				metrics = append(metrics, ent.Metric.Value())
			}
		}
	}
	return metrics
}

// TestRFC1195Level2OneEntryPerPrefix stores two connected prefixes and leaks the
// same two prefixes up from level 1, one cheaper through the leak and one
// cheaper directly, then decodes the stored L2 LSP. Each prefix MUST appear
// once, at the lower of its two metrics.
func TestRFC1195Level2OneEntryPerPrefix(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	// The shared link subnet: directly reachable at 10, and learned from the
	// level 1 neighbor's LSP at 20. The direct entry wins.
	link := netip.MustParsePrefix("172.30.0.0/24")
	// A subnet on an expensive interface: directly reachable at 50, through
	// the level 1 neighbor at 20. The leaked entry wins.
	far := netip.MustParsePrefix("192.0.2.0/24")

	eng.setPrefixes(lsdb.Level2, append(
		isisredistribute.ConnectedPrefixInfos([]netip.Prefix{link}, 10),
		isisredistribute.ConnectedPrefixInfos([]netip.Prefix{far}, 50)...))
	eng.applyLeak(spf.LeakResult{IntoL2: []spf.LeakedPrefix{
		{Prefix: link, Metric: 20},
		{Prefix: far, Metric: 20},
	}})

	l2 := mustFrag0(t, eng, node, lsdb.Level2)
	for _, c := range []struct {
		prefix netip.Prefix
		metric uint32
	}{
		{link, 10},
		{far, 20},
	} {
		got := tlv135Metrics(t, l2, c.prefix)
		if len(got) != 1 || got[0] != c.metric {
			t.Errorf("L2 LSP TLV 135 metrics for %s = %v, want exactly [%d]", c.prefix, got, c.metric)
		}
	}
}

// TestRFC1195Level2OneEntryPerPrefixV6 runs the same selection over the IPv6
// entries (TLV 236, RFC 5308 Section 5 leaks IPv6 by the IPv4 rules) through the
// helper the level 2 LSP assembly calls.
func TestRFC1195Level2OneEntryPerPrefixV6(t *testing.T) {
	link := netip.MustParsePrefix("2001:db8:1::/64")
	far := netip.MustParsePrefix("2001:db8:2::/64")
	direct := []lsdb.PrefixInfoV6{
		{Prefix: link, Metric: types.NewPrefixMetric(10)},
		{Prefix: far, Metric: types.NewPrefixMetric(50)},
	}
	leaked := []lsdb.PrefixInfoV6{
		{Prefix: link, Metric: types.NewPrefixMetric(20)},
		{Prefix: far, Metric: types.NewPrefixMetric(20)},
	}
	keptDirect, keptLeaked := level2Reachability(direct, leaked, prefixInfoV6Key)
	if len(keptDirect) != 1 || keptDirect[0].Prefix != link || keptDirect[0].Metric.Value() != 10 {
		t.Errorf("direct entries kept = %+v, want only %s at 10", keptDirect, link)
	}
	if len(keptLeaked) != 1 || keptLeaked[0].Prefix != far || keptLeaked[0].Metric.Value() != 20 {
		t.Errorf("leaked entries kept = %+v, want only %s at 20", keptLeaked, far)
	}
}
