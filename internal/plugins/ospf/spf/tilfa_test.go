// VALIDATES: TI-LFA repair list (spec A-5/AC-8/AC-9): post-convergence graph
// clone drops the protected resource; P-space/Q-space select the P-node and
// Q-node; the repair list is a Prefix-SID (SRGB-resolved, RFC 8665 Section 3.2/5)
// plus an Adj-SID Q-segment (Section 6.1), each a 20-bit MPLS label.
// PREVENTS: pushing a SID index instead of a resolved label, or a repair that
// crosses the protected resource.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// fakeSR resolves Prefix-SIDs through a real SRGB (proving the RFC 8665
// Section 3.2 index->label mapping) and Adj-SIDs from a direct (from,to) map. It
// faithfully models the production srTILFAResolver, which resolves BOTH a local
// Adj-SID (from == self, srAdj.adjLabelForRouter) and a REMOTE-node Adj-SID
// (from != self, srRemoteAdjSID reads the P-node's advertised Extended-Link
// Adj-SID). The engine-level proof that the remote decode is real, not just this
// fake, is TestSRTILFAResolverRemoteAdjSID* in internal/plugins/ospf.
type fakeSR struct {
	srgb    map[types.RouterID]sr.SRGB
	prefIdx map[types.RouterID]uint32
	adj     map[[2]types.RouterID]uint32
}

func (f fakeSR) PrefixSIDLabel(r types.RouterID) (uint32, bool) {
	g, ok := f.srgb[r]
	if !ok {
		return 0, false
	}
	idx, ok := f.prefIdx[r]
	if !ok {
		return 0, false
	}
	return g.Label(idx)
}

func (f fakeSR) AdjSIDLabel(from, to types.RouterID) (uint32, bool) {
	l, ok := f.adj[[2]types.RouterID{from, to}]
	return l, ok
}

func TestPostConvergenceSPFExcludesResource(t *testing.T) {
	// A-5: cloning the area graph and removing the protected edge/vertex, then
	// re-running Compute, produces the post-convergence tree without touching the
	// live graph.
	area := testArea()
	g := BuildGraph(triangleAltSource(t), area) // S-E-N triangle, all links cost 10
	s := testRID(t, "1.1.1.1")
	e := testRID(t, "2.2.2.2")
	if got := vertexDist(Compute(g, s, 8), routerVertex(e)); got != 10 {
		t.Fatalf("baseline D_opt(S,E) = %d, want 10", got)
	}

	noNode := g.Clone()
	noNode.excludeRouter(e)
	if Compute(noNode, s, 8).Nodes[routerVertex(e)] != nil {
		t.Fatalf("excludeRouter did not remove E from the post-convergence tree")
	}

	noLink := g.Clone()
	noLink.excludeLink(s, e)
	if got := vertexDist(Compute(noLink, s, 8), routerVertex(e)); got != 20 {
		t.Fatalf("post-convergence D_opt(S,E) without the S-E link = %d, want 20 (via N)", got)
	}

	// The live graph is untouched by the clones.
	if got := vertexDist(Compute(g, s, 8), routerVertex(e)); got != 10 {
		t.Fatalf("live graph mutated by Clone: D_opt(S,E) = %d, want 10", got)
	}
}

// tilfaSource: S->A is the primary path to D (cost 2). B is S's only other
// neighbor and its shortest path to D loops back through S (D_opt(B,D)=3 =
// D_opt(B,S)+D_opt(S,D)=1+2), so there is NO base LFA. TI-LFA must build a repair
// via P-node B (P-space) and an Adj-SID across B->D into Q-node D.
func tilfaSource(t *testing.T) Source {
	t.Helper()
	return testSource(t, testArea(),
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.12.1", 1), p2pLink(t, "3.3.3.3", "10.0.13.1", 1)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.12.2", 1), p2pLink(t, "4.4.4.4", "10.0.24.2", 1), p2pLink(t, "3.3.3.3", "10.0.23.2", 5)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.13.3", 1), p2pLink(t, "2.2.2.2", "10.0.23.3", 5), p2pLink(t, "4.4.4.4", "10.0.34.3", 5)),
		routerLSA(t, "4.4.4.4", p2pLink(t, "2.2.2.2", "10.0.24.4", 1), p2pLink(t, "3.3.3.3", "10.0.34.4", 5), stubLink(t, "203.0.113.0", 1)),
	)
}

func tilfaResolver() fakeSR {
	b := mustRID("3.3.3.3")
	d := mustRID("4.4.4.4")
	return fakeSR{
		srgb:    map[types.RouterID]sr.SRGB{b: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})},
		prefIdx: map[types.RouterID]uint32{b: 10}, // index 10 -> label 16010 (SRGB base+index)
		adj:     map[[2]types.RouterID]uint32{{b, d}: 24003},
	}
}

func mustRID(s string) types.RouterID {
	id, _ := types.ParseRouterID(s)
	return id
}

func TestTILFAPQSpace(t *testing.T) {
	// AC-8: no base LFA, but SR coverage exists -> a TI-LFA repair steered via the
	// P-node B (the backup next-hop is S's next-hop toward B).
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, tilfaSource(t), cfg, tilfaResolver())
	r, ok := backupFor(routes, "203.0.113.0/24")
	if !ok {
		t.Fatalf("destination prefix not routed: %+v", routes)
	}
	if len(r.Backups) != 1 || !r.Backups[0].Valid() {
		t.Fatalf("no TI-LFA backup attached: %+v", r.Backups)
	}
	b := r.Backups[0]
	if b.Kind != BackupTILFA {
		t.Fatalf("backup kind = %v, want TI-LFA", b.Kind)
	}
	if b.NextHop != netip.MustParseAddr("10.0.13.3") {
		t.Fatalf("TI-LFA backup next-hop = %v, want 10.0.13.3 (toward P-node B)", b.NextHop)
	}
	if !b.NodeProtect || !b.LinkProtect {
		t.Fatalf("TI-LFA backup should be node+link protecting; got node:%v link:%v", b.NodeProtect, b.LinkProtect)
	}
}

func TestTILFARepairListFromSRMaps(t *testing.T) {
	// AC-8/AC-9, R-3: the repair list is [Prefix-SID(B) resolved via B's SRGB,
	// Adj-SID(B->D)]. The Prefix-SID is the resolved 20-bit label 16010 (base 16000
	// + index 10), NOT the index 10; the Q-segment is the Adj-SID 24003.
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, tilfaSource(t), cfg, tilfaResolver())
	r, _ := backupFor(routes, "203.0.113.0/24")
	if len(r.Backups) != 1 {
		t.Fatalf("no backup: %+v", r.Backups)
	}
	got := r.Backups[0].RepairLabels
	want := []uint32{16010, 24003}
	if len(got) != len(want) {
		t.Fatalf("repair labels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("repair labels = %v, want %v (label form, not index)", got, want)
		}
		if got[i] > maxMPLSLabel {
			t.Fatalf("repair label %d exceeds the 20-bit MPLS maximum", got[i])
		}
	}
}

func TestTILFANoSRNoRepair(t *testing.T) {
	// With no SR resolver, a no-base-LFA prefix is left unprotected (no repair
	// tunnel can be built) rather than getting a wrong backup.
	cfg := FastRerouteConfig{Enabled: true, Mode: FastRerouteTILFA, NodeProtection: true}
	routes := attachTopo(t, tilfaSource(t), cfg, nil)
	r, _ := backupFor(routes, "203.0.113.0/24")
	if len(r.Backups) != 0 {
		t.Fatalf("expected no backup without an SR resolver, got %+v", r.Backups)
	}
}
