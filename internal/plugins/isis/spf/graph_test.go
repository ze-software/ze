// Design: plan/spec-isis-9-spf-rib.md TDD plan -- TestISISGraphBuild.
//
// VALIDATES: the SPF graph builder turns LSDB records into vertices (System IDs +
// pseudo-nodes), TLV 22 edges weighted by the 24-bit wide metric (pseudo-node
// edges metric 0), TLV 135 prefixes read with the full 32-bit metric, TLV 132
// interface addresses, and the RFC 3787 overload flag.
// PREVENTS: a regression where edges/prefixes are dropped or the overload bit is
// lost, which would mis-compute SPF or use an overloaded node as transit.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// sysID builds a SystemID from a single low byte for compact test fixtures
// (0000.0000.000N).
func sysID(n byte) types.SystemID {
	return types.SystemID{0, 0, 0, 0, 0, n}
}

// srcID builds a SourceID (router pseudonode 0) from a low byte.
func srcID(n byte) types.SourceID {
	return types.NewSourceID(sysID(n), 0)
}

// pnID builds the pseudo-node SourceID owned by system 1 (pseudonode number 1),
// the DIS's pseudo-node for a LAN in these fixtures.
func pnID() types.SourceID {
	return types.NewSourceID(sysID(1), 1)
}

// isEdge is one TLV 22 entry for test fixtures: a neighbor Source ID and the
// 24-bit wide metric to it.
type isEdge struct {
	to     types.SourceID
	metric uint32
}

// tlv22 builds a TLV 22 (Extended IS Reachability) value carrying one entry per
// edge, with no sub-TLVs.
func tlv22(edges ...isEdge) packet.TLV {
	var buf []byte
	for _, e := range edges {
		var b [7]byte
		e.to.WriteTo(b[:], 0)
		buf = append(buf, b[:]...)
		buf = append(buf,
			byte(e.metric>>16), byte(e.metric>>8), byte(e.metric), // 24-bit metric
			0, // sub-TLV length 0
		)
	}
	return packet.TLV{Type: packet.TLVExtendedISReach, Value: buf}
}

// tlv135 builds a TLV 135 (Extended IP Reachability) value carrying one prefix
// entry with the given metric and up/down bit, no sub-TLVs.
func tlv135(prefix netip.Prefix, metric uint32, upDown bool) packet.TLV {
	var buf []byte
	buf = append(buf, byte(metric>>24), byte(metric>>16), byte(metric>>8), byte(metric)) // 32-bit metric
	ctrl := byte(prefix.Bits()) & 0x3f
	if upDown {
		ctrl |= 0x80
	}
	buf = append(buf, ctrl)
	poct := (prefix.Bits() + 7) / 8
	a4 := prefix.Addr().As4()
	buf = append(buf, a4[:poct]...)
	return packet.TLV{Type: packet.TLVExtendedIPReach, Value: buf}
}

// tlv132 builds a TLV 132 (IP Interface Address) value carrying the given IPv4
// addresses.
func tlv132(addrs ...netip.Addr) packet.TLV {
	var buf []byte
	for _, a := range addrs {
		a4 := a.As4()
		buf = append(buf, a4[:]...)
	}
	return packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: buf}
}

// stubSource is a hand-built spf.Source for graph/SPF tests, keyed by level.
type stubSource struct {
	byLevel map[Level][]LSPRecord
}

func (s stubSource) Records(level Level) []LSPRecord { return s.byLevel[level] }

func newStubSource() *stubSource {
	return &stubSource{byLevel: map[Level][]LSPRecord{}}
}

func (s *stubSource) add(level Level, rec LSPRecord) {
	s.byLevel[level] = append(s.byLevel[level], rec)
}

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", s, err)
	}
	return p
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse addr %q: %v", s, err)
	}
	return a
}

// TestISISGraphBuild verifies the graph builder produces the expected vertices,
// TLV 22 edges (with pseudo-node edges at metric 0), TLV 135 prefixes, TLV 132
// addresses, and the overload flag.
func TestISISGraphBuild(t *testing.T) {
	src := newStubSource()
	// Node 1: router, edge to node 2 metric 10, edge to pseudo-node (1,1) metric 0,
	// originates 10.1.0.0/24 metric 5, interface address 10.0.12.1, overloaded.
	src.add(Level1, LSPRecord{
		Source:   srcID(1),
		Overload: true,
		LSP: packet.LSP{TLVs: []packet.TLV{
			tlv22(
				isEdge{srcID(2), 10},
				isEdge{pnID(), 0},
			),
			tlv135(mustPrefix(t, "10.1.0.0/24"), 5, false),
			// A leaked prefix with the up/down bit set in the TLV 135 control
			// octet (RFC 5305 sec 4.1 / RFC 2966): the builder must read the bit
			// from the control octet, not the metric.
			tlv135(mustPrefix(t, "10.2.0.0/24"), 7, true),
			tlv132(mustAddr(t, "10.0.12.1")),
		}},
	})
	// Pseudo-node (1,1): edge back to node 1 and node 2, both metric 0.
	src.add(Level1, LSPRecord{
		Source: pnID(),
		LSP: packet.LSP{TLVs: []packet.TLV{
			tlv22(
				isEdge{srcID(1), 0},
				isEdge{srcID(2), 0},
			),
		}},
	})

	g := BuildGraph(src, Level1)

	n1, ok := g.Nodes[srcID(1)]
	if !ok {
		t.Fatal("node 1 missing from graph")
	}
	if !n1.Overload {
		t.Error("node 1 should be marked overloaded (RFC 3787)")
	}
	if len(n1.Edges) != 2 {
		t.Fatalf("node 1 edges = %d, want 2", len(n1.Edges))
	}
	// Find the edge to node 2 (metric 10) and the pseudo-node edge (metric 0).
	var sawN2, sawPN bool
	for _, e := range n1.Edges {
		switch e.To {
		case srcID(2):
			sawN2 = true
			if e.Metric != 10 {
				t.Errorf("edge 1->2 metric = %d, want 10", e.Metric)
			}
		case pnID():
			sawPN = true
			if e.Metric != 0 {
				t.Errorf("edge 1->pseudonode metric = %d, want 0", e.Metric)
			}
		}
	}
	if !sawN2 || !sawPN {
		t.Errorf("missing edges: sawN2=%v sawPN=%v", sawN2, sawPN)
	}
	if len(n1.Prefixes) != 2 {
		t.Fatalf("node 1 prefixes = %+v, want 2 (one normal, one leaked)", n1.Prefixes)
	}
	byPfx := map[string]Prefix{}
	for _, p := range n1.Prefixes {
		byPfx[p.Prefix.String()] = p
	}
	if p := byPfx["10.1.0.0/24"]; p.Metric != 5 || p.UpDown {
		t.Errorf("10.1.0.0/24 = metric %d up=%v, want metric 5 up=false", p.Metric, p.UpDown)
	}
	if p := byPfx["10.2.0.0/24"]; p.Metric != 7 || !p.UpDown {
		t.Errorf("10.2.0.0/24 = metric %d up=%v, want metric 7 up=true (up/down from control octet)", p.Metric, p.UpDown)
	}
	if len(n1.Addrs) != 1 || n1.Addrs[0] != mustAddr(t, "10.0.12.1") {
		t.Errorf("node 1 addrs = %v, want [10.0.12.1]", n1.Addrs)
	}

	pn, ok := g.Nodes[pnID()]
	if !ok {
		t.Fatal("pseudo-node (1,1) missing from graph")
	}
	if !pn.IsPseudonode() {
		t.Error("(1,1) should report IsPseudonode")
	}
	if len(pn.Edges) != 2 {
		t.Errorf("pseudo-node edges = %d, want 2", len(pn.Edges))
	}
}

// TestISISGraphBuildEmpty verifies a nil source and an empty level yield an empty
// graph without panicking.
func TestISISGraphBuildEmpty(t *testing.T) {
	if g := BuildGraph(nil, Level1); g == nil || len(g.Nodes) != 0 {
		t.Fatalf("nil source: want empty graph, got %+v", g)
	}
	if g := BuildGraph(newStubSource(), Level2); len(g.Nodes) != 0 {
		t.Fatalf("empty source: want 0 nodes, got %d", len(g.Nodes))
	}
}
