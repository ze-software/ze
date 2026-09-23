// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing Adj-SID lifecycle.
// srAdjManager drives the Adjacency-SID lifecycle off the neighbor Full<->non-Full
// transition (the AF-neutral onFull/onLost seam, a subset of "2-Way or higher"): a
// label is allocated from the SRLB when a neighbor reaches Full, stored so the
// Extended Link LSA advertises the Adj-SID/LAN-Adj-SID sub-TLV, and a pop/forward
// mpls-fib entry is installed toward that neighbor; the label is freed and the entry
// withdrawn when the neighbor leaves Full (RFC 8665 §7.4.1 / RFC 8666 §8.4.1).
// RFC: rfc/short/rfc8665.md (§6.1 Adj-SID, §7.4.1 withdraw); rfc/short/rfc8666.md (§7, §8.4.1)

package ospf

import (
	"net/netip"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// srAdjKey identifies one adjacency for Adj-SID tracking.
type srAdjKey struct {
	iface  string
	router types.RouterID
}

// srAdjRecord is the SRLB label + link data allocated for one adjacency (kept so the
// exact allocation can be freed and its store entry cleared on withdraw). adj is the
// full decoded Adj-SID (flags/weight/neighbor) the OSPFv3 E-Router-LSA advertises.
type srAdjRecord struct {
	label    uint32
	linkData [4]byte
	adj      sr.AdjSID
}

// srAdjManager owns the SRLB allocator, the Adj-SID store, and the mpls-fib pop
// entries for one address family. It is not safe for concurrent use; the engine
// serializes neighbor events on its run goroutine.
type srAdjManager struct {
	alloc  *sr.LabelAllocator
	fib    *srFIB
	store  *srWireStore
	self   types.RouterID
	labels map[srAdjKey]srAdjRecord
}

// neighborFull allocates an Adj-SID for a neighbor that reached Full and installs its
// pop/forward entry. lan marks a broadcast/NBMA adjacency (a LAN-Adj-SID carrying the
// neighbor ID). It returns false when SR is disabled (nil allocator), the SRLB is
// exhausted, or the adjacency already has an Adj-SID (idempotent).
func (m *srAdjManager) neighborFull(iface string, router types.RouterID, linkData [4]byte, nh netip.Addr, lan bool, neighborID [4]byte) bool {
	if m.alloc == nil {
		return false
	}
	key := srAdjKey{iface: iface, router: router}
	if _, exists := m.labels[key]; exists {
		return false
	}
	label, ok := m.alloc.Allocate()
	if !ok {
		return false
	}
	adj := sr.AdjSID{
		Flags:      sr.AdjSIDFlags{V: true, L: true}, // local label form (V=1/L=1)
		Weight:     0,
		Label:      label,
		IsLabel:    true,
		IsLAN:      lan,
		NeighborID: neighborID,
	}
	m.store.setAdj(m.self, linkData, adj)
	m.labels[key] = srAdjRecord{label: label, linkData: linkData, adj: adj}
	m.fib.installAdjSID(label, nh)
	return true
}

// adjFor returns the Adj-SID allocated for one adjacency (interface + neighbor Router
// ID), for the OSPFv3 E-Router-LSA origination that advertises it. It is read on the same
// engine run goroutine that drives neighborFull/neighborLost, so it needs no extra lock.
func (m *srAdjManager) adjFor(iface string, router types.RouterID) (sr.AdjSID, bool) {
	if m == nil {
		return sr.AdjSID{}, false
	}
	rec, ok := m.labels[srAdjKey{iface: iface, router: router}]
	if !ok {
		return sr.AdjSID{}, false
	}
	return rec.adj, true
}

// neighborLost withdraws and frees the Adj-SID for a neighbor that left Full. It is a
// no-op when the adjacency has no Adj-SID.
func (m *srAdjManager) neighborLost(iface string, router types.RouterID) {
	key := srAdjKey{iface: iface, router: router}
	rec, ok := m.labels[key]
	if !ok {
		return
	}
	delete(m.labels, key)
	m.store.clearAdj(m.self, rec.linkData)
	m.fib.withdrawAdjSID(rec.label)
	if m.alloc != nil {
		m.alloc.Free(rec.label)
	}
}

// inUse reports how many Adj-SID labels are currently allocated (metrics).
func (m *srAdjManager) inUse() int {
	if m == nil {
		return 0
	}
	return len(m.labels)
}

// ensureAlloc seeds the SRLB allocator on first use (SR enabled with an SRLB range).
func (m *srAdjManager) ensureAlloc(srlb []sr.LabelRange) {
	if m.alloc == nil && len(srlb) > 0 {
		m.alloc = sr.NewLabelAllocator(srlb)
	}
}

// srAdjNeighborFull bridges a neighbor reaching Full to the Adj-SID lifecycle for both
// address families (RFC 8665 §7.4.1 IPv4, RFC 8666 §8.4.1 IPv6). It allocates an SRLB
// label, installs the pop/forward entry toward the neighbor, and re-originates the AF
// link carrier LSA (the OSPFv2 Extended Link LSA or the OSPFv3 E-Router-LSA) so the
// Adj-SID sub-TLV is advertised. The only per-AF difference is the link-data key: IPv4
// uses the interface address (the RFC 2328 Link Data), IPv6 the neighbor Router ID (the
// OSPFv3 Adj-SID is keyed by adjacency, read back through srAdj.adjFor at origination).
func (e *engine) srAdjNeighborFull(snap ospfneighbor.Snapshot) {
	if e.srAdj == nil {
		return
	}
	cfg, ok := srWire.get(e.cfg.RouterID)
	if !ok || !cfg.Enabled || len(cfg.SRLB) == 0 {
		return
	}
	router, err := types.ParseRouterID(snap.RouterID)
	if err != nil {
		return
	}
	nh, ok := e.neighbors.NeighborAddress(snap.Interface, router)
	if !ok {
		return
	}
	linkData := interfaceIPv4Address(snap.Interface)
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		linkData = [4]byte(router)
	}
	e.srAdj.self = e.cfg.RouterID
	e.srAdj.ensureAlloc(cfg.SRLB)
	if e.srAdj.neighborFull(snap.Interface, router, linkData, nh, false, [4]byte(router)) {
		srMetrics.Load().updateFromConfig(e.srAF(), cfg, e.srAdj.inUse())
		e.originateSelfLSAs()
	}
}

// srAdjNeighborLost withdraws the Adj-SID for a neighbor that left Full.
func (e *engine) srAdjNeighborLost(snap ospfneighbor.Snapshot) {
	if e.srAdj == nil {
		return
	}
	router, err := types.ParseRouterID(snap.RouterID)
	if err != nil {
		return
	}
	before := e.srAdj.inUse()
	e.srAdj.neighborLost(snap.Interface, router)
	if e.srAdj.inUse() != before {
		if cfg, ok := srWire.get(e.cfg.RouterID); ok {
			srMetrics.Load().updateFromConfig(e.srAF(), cfg, e.srAdj.inUse())
		}
		e.originateSelfLSAs()
	}
}
