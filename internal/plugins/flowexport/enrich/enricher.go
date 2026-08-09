// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- BGP enrichment coordinator

package enrich

import (
	"net/netip"
	"sync/atomic"
)

// Enrichment holds BGP-derived metadata for a flow's source and
// destination addresses. Value type, safe to copy.
type Enrichment struct {
	SrcAS     uint32
	DstAS     uint32
	NextHop   netip.Addr
	LocalPref uint32
	SrcASPath []uint32
	DstASPath []uint32
}

// Enricher coordinates prefix-to-AS lookups for flow export. It holds
// an atomically-swappable RadixTree so readers (per-flow encoding)
// never block on writers (BGP RIB updates).
type Enricher struct {
	tree atomic.Pointer[RadixTree]
}

// NewEnricher returns an enricher with an empty tree.
func NewEnricher() *Enricher {
	e := &Enricher{}
	e.tree.Store(NewRadixTree())
	return e
}

// UpdateTree atomically replaces the current radix tree.
func (e *Enricher) UpdateTree(tree *RadixTree) {
	if tree == nil {
		tree = NewRadixTree()
	}
	e.tree.Store(tree)
}

// Lookup performs a longest-prefix-match for addr in the current tree.
func (e *Enricher) Lookup(addr netip.Addr) (ASEntry, bool) {
	t := e.tree.Load()
	if t == nil {
		return ASEntry{}, false
	}
	return t.Lookup(addr)
}

// Enrich looks up both source and destination addresses and returns
// combined BGP metadata. Uses the destination's next-hop and local-pref
// (the forwarding decision is based on destination).
func (e *Enricher) Enrich(srcAddr, dstAddr netip.Addr) Enrichment {
	var result Enrichment

	t := e.tree.Load()
	if t == nil {
		return result
	}

	if src, ok := t.Lookup(srcAddr); ok {
		result.SrcAS = src.AS
		result.SrcASPath = src.ASPath
	}

	if dst, ok := t.Lookup(dstAddr); ok {
		result.DstAS = dst.AS
		result.DstASPath = dst.ASPath
		result.NextHop = dst.NextHop
		result.LocalPref = dst.LocalPref
	}

	return result
}
