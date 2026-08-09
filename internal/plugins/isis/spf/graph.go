// Design: docs/architecture/isis/isis-9-spf-rib.md -- SPF graph build from the synced LSDB.
// ISO/IEC 10589 clause 7.2 (the decision process / SPF) builds a directed graph
// over the link-state database: vertices are nodes (System IDs and LAN
// pseudo-nodes, isis-8), edges are the adjacencies each node advertises.
//
// RFC: rfc/short/rfc5305.md -- Extended IS Reachability (TLV 22) carries the
//   24-bit wide edge metric; Extended IP Reachability (TLV 135) carries the
//   32-bit prefix metric and the up/down bit in the control octet (NOT the
//   metric). Read in full; the prefix metric is a separate, wider field.
// RFC: rfc/short/rfc3787.md -- a node advertising the LSP-database-overload (OL)
//   bit is reachable as a destination but MUST NOT be used as a transit node.
//
// The graph is a parsed snapshot of one level. SPF (spf.go) consumes it; the
// builder (BuildGraph) reads the LSDB once per run via the small Source
// interface, so SPF never re-parses raw LSP bytes on the hot path more than once
// (spec A-9). Pseudo-node edges carry metric 0 (the LAN itself is free; the cost
// is the metric on the router->pseudonode edge), per ISO/IEC 10589 clause 7.2.

package spf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Level is the routing level a graph/SPF run covers (1 or 2). It mirrors the
// LSDB level; an L1L2 node runs SPF once per level.
type Level uint8

// Graph levels.
const (
	Level1 Level = 1
	Level2 Level = 2
)

// String renders the level as the canonical metric/CLI token "l1"/"l2".
func (l Level) String() string {
	if l == Level2 {
		return "l2"
	}
	return "l1"
}

// Edge is one directed adjacency in the graph: from the node that advertised it
// (the LSP originator) to a neighbor, weighted by the TLV 22 wide metric. A
// pseudo-node's edges to its members carry metric 0 (the LAN is free).
type Edge struct {
	// To is the neighbor Source ID (a router with pseudonode 0, or a LAN
	// pseudo-node with pseudonode != 0).
	To types.SourceID
	// Metric is the 24-bit IS-reachability edge metric (RFC 5305 sec 3). A
	// pseudo-node edge is 0.
	Metric uint32
}

// Prefix is one IP reachability advertised by a node (TLV 135). SPF attaches it
// to the destination at the node's computed distance plus this prefix metric.
type Prefix struct {
	// Prefix is the advertised IPv4 prefix.
	Prefix netip.Prefix
	// Metric is the 32-bit prefix metric (RFC 5305 sec 4); read in full, never
	// capped at 24-bit.
	Metric uint32
	// UpDown is the up/down bit from the TLV 135 control octet (RFC 5305 sec 4.1
	// / RFC 2966): set means the prefix was leaked DOWN a level (L2 -> L1). A
	// prefix with this bit set MUST NOT be re-leaked UP into L2 (loop prevention).
	UpDown bool
}

// Node is one vertex: a router or a LAN pseudo-node. It holds the edges it
// advertises (TLV 22), the prefixes it originates (TLV 135), its own interface
// addresses (TLV 132, the next-hop source for a directly adjacent neighbor),
// and the overload flag (RFC 3787, routers only).
type Node struct {
	// ID is the node Source ID (System ID + pseudonode number).
	ID types.SourceID
	// Edges are the outgoing adjacencies (TLV 22 entries, pseudo-node edges 0).
	Edges []Edge
	// Prefixes are the IP reachabilities the node originates (TLV 135).
	Prefixes []Prefix
	// PrefixesV6 are the IPv6 reachabilities the node originates (TLV 236,
	// RFC 5308 sec 2). Populated only when IPv6 is in use; the IPv6 route builder
	// (ipv6.go) attaches them over the same shared SPF tree.
	PrefixesV6 []Prefix
	// Addrs are the node's own IPv4 interface addresses (TLV 132). Used only as a
	// fallback next-hop source; the primary next-hop comes from the local
	// adjacency table (Shared Contracts "Next-hop derivation for SPF").
	Addrs []netip.Addr
	// AddrsV6 are the node's own IPv6 interface addresses (TLV 232, RFC 5308
	// sec 3). The LSP carries only non-link-local addresses; the link-local
	// next-hop comes from the local adjacency table (neighbor IIH TLV 232).
	AddrsV6 []netip.Addr
	// Overload is the LSP-database-overload (OL) bit (RFC 3787): the node is a
	// valid destination but excluded as a transit node in SPF.
	Overload bool
}

// IsPseudonode reports whether this vertex is a LAN pseudo-node (pseudonode != 0)
// rather than a real router.
func (n Node) IsPseudonode() bool { return n.ID.IsPseudonode() }

// Graph is a parsed, per-level snapshot of the link-state database: the set of
// nodes keyed by Source ID. SPF runs over it without touching raw LSP bytes.
type Graph struct {
	// Nodes is every vertex keyed by its Source ID. A router and its
	// pseudo-nodes are distinct entries.
	Nodes map[types.SourceID]*Node
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{Nodes: make(map[types.SourceID]*Node)}
}

// node returns the Node for id, creating an empty one on first use. The builder
// merges LSP fragments (0..255) of the same originator into one Node, so a
// node's edges/prefixes accumulate across fragments.
func (g *Graph) node(id types.SourceID) *Node {
	n, ok := g.Nodes[id]
	if !ok {
		n = &Node{ID: id}
		g.Nodes[id] = n
	}
	return n
}

// LSPRecord is one LSP the graph builder consumes: its originator Source ID, the
// overload bit, and the decoded TLVs. The Source interface yields these so SPF
// reads the LSDB through a narrow, testable boundary (a hand-built slice in
// unit tests; the engine's LSDB in production).
type LSPRecord struct {
	// Source is the LSP originator's Source ID (System ID + pseudonode). A
	// pseudo-node LSP has a non-zero pseudonode number.
	Source types.SourceID
	// Overload is the OL bit from the LSP type block (fragment 0; RFC 3787). The
	// builder ORs it across fragments so any fragment carrying it marks the node.
	Overload bool
	// LSP is the decoded LSP whose TLV 22 / 132 / 135 the builder reads. The TLV
	// value slices may alias the LSDB's stable raw bytes (lazy decode).
	LSP packet.LSP
}

// Source yields the LSPs of one level for graph construction. The engine's LSDB
// implements it; tests supply a stub. It is read ONCE per SPF run (spec A-9): a
// burst of LSP arrivals is coalesced by the debounce before BuildGraph runs.
type Source interface {
	// Records returns every LSP held at the level. A purged (zero-lifetime) LSP
	// is omitted by the implementation; a malformed LSP excludes that one node,
	// not the whole run (security review: error isolation).
	Records(level Level) []LSPRecord
}

// BuildGraph constructs the directed graph for one level from src. Each LSP
// contributes its originator's edges (TLV 22), prefixes (TLV 135), and interface
// addresses (TLV 132); the overload bit (RFC 3787) marks the node transit-
// excluded. Pseudo-node edges keep their advertised metric (a pseudo-node
// advertises metric 0 to its members per ISO/IEC 10589 clause 7.2.5; a router
// advertises its circuit metric to the pseudo-node). A TLV whose decode fails is
// skipped without aborting the build (one bad LSP excludes one contribution).
func BuildGraph(src Source, level Level) *Graph {
	g := NewGraph()
	if src == nil {
		return g
	}
	for _, rec := range src.Records(level) {
		n := g.node(rec.Source)
		if rec.Overload {
			// RFC 3787 sec 4: the OL bit makes the node transit-excluded. OR
			// across fragments so it survives even if only fragment 0 carries it.
			n.Overload = true
		}
		addTLVs(n, rec.LSP)
		packet.ReleaseTLVs(rec.LSP.TLVs)
	}
	return g
}

// addTLVs reads the SPF-relevant TLVs out of one decoded LSP and appends them to
// the node. TLV 22 -> edges, TLV 135 -> prefixes, TLV 132 -> interface
// addresses. A TLV that fails to decode is skipped (the wire codec already
// validated lengths on store; this is defense in depth).
func addTLVs(n *Node, lsp packet.LSP) {
	for _, tlv := range lsp.TLVs {
		switch tlv.Type {
		case packet.TLVExtendedISReach:
			t, err := packet.DecodeExtendedISReachTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, e := range t.Entries {
				n.Edges = append(n.Edges, Edge{To: e.Neighbor, Metric: e.Metric.Value()})
			}
		case packet.TLVExtendedIPReach:
			t, err := packet.DecodeExtendedIPReachTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, e := range t.Entries {
				if !e.Prefix.IsValid() {
					continue
				}
				n.Prefixes = append(n.Prefixes, Prefix{
					Prefix: e.Prefix.Masked(),
					Metric: e.Metric.Value(),
					UpDown: e.UpDown,
				})
			}
		case packet.TLVIPInterfaceAddress:
			t, err := packet.DecodeIPv4InterfaceAddrTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, a := range t.Addresses {
				if a.IsValid() {
					n.Addrs = append(n.Addrs, a)
				}
			}
		case packet.TLVIPv6Reachability:
			// RFC 5308 sec 2: IPv6 reachability. The shared SPF tree (spf.go) is
			// unchanged; only this leaf set differs from the IPv4 (TLV 135) one.
			t, err := packet.DecodeIPv6ReachabilityTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, e := range t.Entries {
				if !e.Prefix.IsValid() {
					continue
				}
				n.PrefixesV6 = append(n.PrefixesV6, Prefix{
					Prefix: e.Prefix.Masked(),
					Metric: e.Metric.Value(),
					UpDown: e.UpDown,
				})
			}
		case packet.TLVIPv6InterfaceAddress:
			// RFC 5308 sec 3: IPv6 interface addresses (LSP scope = non-link-local).
			t, err := packet.DecodeIPv6InterfaceAddrTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, a := range t.Addresses {
				if a.IsValid() {
					n.AddrsV6 = append(n.AddrsV6, a)
				}
			}
		}
	}
}
