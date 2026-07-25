// VALIDATES: spec-ospf-10 redistribution source -- single "ospf" config source
// registered; SPF deltas (intra + inter) become one redistevents batch under the
// ospf ProtocolID; removals become ActionRemove entries.
// PREVENTS: regressions where a per-area source splits the stream, or an SPF
// withdrawal fails to propagate as an ActionRemove to BGP.
package ospfredistribute

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ospfredistevents "github.com/ze-software/ze/internal/plugins/ospf/redistribute/events"
	"github.com/ze-software/ze/internal/plugins/ospf/spf"
)

func TestOSPFRegisterSource(t *testing.T) {
	RegisterOSPFSources()
	RegisterOSPFSources() // idempotent (sync.Once)

	assert.True(t, slices.Contains(configredist.SourceNames(), "ospf"), "source 'ospf' registered")
	src, ok := configredist.LookupSource("ospf")
	require.True(t, ok)
	assert.Equal(t, "ospf", src.Protocol, "single source, protocol ospf (no per-area names)")
}

// captureSink copies the fields under test out of the pooled batch (it is recycled
// after the sink returns).
type capturedBatch struct {
	protocol redistevents.ProtocolID
	afi      uint16
	entries  []redistevents.RouteChangeEntry
}

func capture(into *[]capturedBatch) func(*redistevents.RouteChangeBatch) {
	return func(b *redistevents.RouteChangeBatch) {
		*into = append(*into, capturedBatch{
			protocol: b.Protocol,
			afi:      b.AFI,
			entries:  slices.Clone(b.Entries),
		})
	}
}

func extRoute(prefix string, metric uint64, nh string, rt spf.RouteType) spf.RouteEntry {
	return spf.RouteEntry{
		Prefix:   netip.MustParsePrefix(prefix),
		Metric:   metric,
		Type:     rt,
		NextHops: []spf.NextHop{{Addr: netip.MustParseAddr(nh)}},
	}
}

func TestOSPFRedistSourceToBGP(t *testing.T) {
	var got []capturedBatch
	delta := spf.RouteDelta{Added: []spf.RouteEntry{
		extRoute("10.10.0.0/24", 10, "10.0.0.2", spf.RouteIntraArea),
		extRoute("10.20.0.0/24", 25, "10.0.0.3", spf.RouteInterArea),
	}}
	emitDelta(delta, ospfredistevents.ProtocolID, capture(&got))

	require.Len(t, got, 1, "intra + inter routes go in ONE batch (single source, AC-2)")
	assert.Equal(t, ospfredistevents.ProtocolID, got[0].protocol)
	assert.Equal(t, uint16(family.AFIIPv4), got[0].afi)
	require.Len(t, got[0].entries, 2)
	for _, e := range got[0].entries {
		assert.Equal(t, redistevents.ActionAdd, e.Action)
	}
	assert.Equal(t, netip.MustParsePrefix("10.10.0.0/24"), got[0].entries[0].Prefix)
	assert.Equal(t, uint32(10), got[0].entries[0].Metric)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got[0].entries[0].NextHop)
}

func TestOSPFRedistSourceWithdrawToBGP(t *testing.T) {
	var got []capturedBatch
	delta := spf.RouteDelta{Removed: []netip.Prefix{netip.MustParsePrefix("10.30.0.0/24")}}
	emitDelta(delta, ospfredistevents.ProtocolID, capture(&got))

	require.Len(t, got, 1)
	require.Len(t, got[0].entries, 1)
	assert.Equal(t, redistevents.ActionRemove, got[0].entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.30.0.0/24"), got[0].entries[0].Prefix)
}

func TestOSPFRedistSourceEmptyDelta(t *testing.T) {
	var got []capturedBatch
	emitDelta(spf.RouteDelta{}, ospfredistevents.ProtocolID, capture(&got))
	assert.Empty(t, got, "an empty delta emits nothing")
}

func TestOSPFRedistRegistrationOrder(t *testing.T) {
	// The producer ProtocolID is stable regardless of when sources are (re)registered,
	// so an orchestrator that subscribes before or after RegisterOSPFSources resolves
	// the same identity.
	RegisterOSPFSources()
	id, ok := redistevents.ProtocolIDOf("ospf")
	require.True(t, ok)
	assert.Equal(t, ospfredistevents.ProtocolID, id)
}
