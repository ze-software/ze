// VALIDATES: spec-ospf-af-unify Phase 4 -- the Computer drives graph decode and
// every prefix-attachment stage through the AFPrefixStrategy seam, the default is
// the OSPFv2 strategy, and routing through it is behavior-identical to the prior
// direct calls. PREVENTS: a regression where the seam is bypassed (so a v6
// strategy could never plug in) or the v4 strategy diverges from current SPF.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// recordingStrategy wraps the v4 strategy and records which seam methods the
// Computer drove, proving Run routes graph-decode + prefix attachment through the
// injected AFPrefixStrategy (so a v6 strategy can replace it).
type recordingStrategy struct {
	inner         v4Strategy
	graph         bool
	routes        bool
	interArea     bool
	external      bool
	externalInput ExternalInput
	summaries     bool
	nextHop       bool
}

func (s *recordingStrategy) BuildGraph(src Source, area types.AreaID) *Graph {
	s.graph = true
	return s.inner.BuildGraph(src, area)
}

func (s *recordingStrategy) BuildRoutes(res *Result, maxPaths int, resolver InterfaceResolver) []RouteEntry {
	s.routes = true
	return s.inner.BuildRoutes(res, maxPaths, resolver)
}

func (s *recordingStrategy) ComputeInterArea(in InterAreaInput) ([]RouteEntry, []BorderRouterEntry) {
	s.interArea = true
	return s.inner.ComputeInterArea(in)
}

func (s *recordingStrategy) ComputeExternal(in ExternalInput) []RouteEntry {
	s.external = true
	s.externalInput = in
	return s.inner.ComputeExternal(in)
}

func (s *recordingStrategy) OriginateSummaries(in SummaryInput) SummaryOriginResult {
	s.summaries = true
	return s.inner.OriginateSummaries(in)
}

func (s *recordingStrategy) NextHopSource() NextHopSource {
	s.nextHop = true
	return s.inner.NextHopSource()
}

func (s *recordingStrategy) SummaryReader(src Source) SummaryReader {
	return s.inner.SummaryReader(src)
}

func TestOSPFAFPrefixStrategyV4(t *testing.T) {
	area := testArea()
	pfx := netip.MustParsePrefix("192.0.2.0/24")

	// 1. The default strategy is the OSPFv2 one: a Computer with no Strategy set
	//    produces the same intra-area route as the direct package path.
	loc := locrib.NewRIB()
	c := NewComputer(Config{Source: baseP2PSource(t, area), Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})
	delta := c.Run()
	if len(delta.Added) != 1 || delta.Added[0].Prefix != pfx {
		t.Fatalf("default (v4) strategy delta = %+v, want added %s", delta, pfx)
	}

	// 2. The injected strategy is the one the Computer drives: graph decode and
	//    every prefix-attachment stage route through AFPrefixStrategy, and the
	//    route output is unchanged (the v4 delegation is behavior-identical).
	rec := &recordingStrategy{}
	loc2 := locrib.NewRIB()
	c2 := NewComputer(Config{Source: baseP2PSource(t, area), Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, AreaConfigs: []AreaConfig{{AreaID: area, AreaType: AreaTypeNSSA, NoSummary: true}}, Installer: NewInstaller(loc2), Strategy: rec})
	delta2 := c2.Run()
	if len(delta2.Added) != 1 || delta2.Added[0].Prefix != pfx {
		t.Fatalf("injected v4 strategy delta = %+v, want added %s", delta2, pfx)
	}
	if !rec.graph || !rec.routes || !rec.interArea || !rec.external || !rec.summaries || !rec.nextHop {
		t.Fatalf("Computer did not route all SPF stages through the strategy: %+v", *rec)
	}
	if !rec.externalInput.NSSAPolicies[area].NoSummary {
		t.Fatal("Computer did not pass the NSSA summary-import policy to external calculation")
	}
}
