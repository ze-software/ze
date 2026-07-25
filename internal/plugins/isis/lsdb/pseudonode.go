// Design: plan/learned/923-isis-8-dis-broadcast.md -- pseudo-node LSP origination on a
// broadcast LAN where the local node is the elected DIS.
//
// RFC: rfc/short/rfc5305.md -- TLV 22 (Extended IS Reachability, 24-bit metric); the pseudo-node lists each member at metric 0
// RFC: rfc/short/rfc3786.md -- the 256-fragment model (fragment 0 valid, even empty)
// RFC: rfc/short/rfc3787.md -- the overload bit lives only in a real node's LSP, never a pseudo-node
//
// ISO/IEC 10589 clause 8.4.5: the Designated IS originates a pseudo-node LSP -- a
// virtual node representing the LAN -- whose LSP ID has a NON-ZERO pseudonode
// octet (LAN ID `<dis-system-id>.<pseudonode-id>`). The pseudo-node LSP lists
// every router on the segment (the DIS included) as a TLV 22 (Extended IS
// Reachability) neighbor at metric 0 (RFC 5305 sec 3). Every router on the
// segment, in turn, advertises the pseudo-node as a single TLV 22 neighbor
// (metric = circuit metric) instead of one entry per peer, collapsing the LAN
// mesh into a star (the own-LSP star encoding lives in the engine's levelState,
// lsdb_wiring.go; this file owns only the pseudo-node LSP itself).
//
// This is NOT a parallel store or a side channel: a pseudo-node LSP is an
// ordinary LSP. It reuses the spec-isis-6 origination path (the same Originator,
// LSDB.Insert, sequence-number/wraparound state, and fragmenter), differing only
// in the non-zero pseudonode Source ID and the metric-0 member list. Flooding
// (spec-isis-7) and aging treat it like any other LSP. On losing the DIS role the
// DIS purges (zero-age, sequence bump) the pseudo-node LSP before yielding so no
// phantom node lingers in another router's SPF (R-2).

package lsdb

import (
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// PseudonodeInfo is the input to a pseudo-node LSP origination: the identity of
// the pseudo-node (the DIS System ID plus the non-zero pseudonode octet) and the
// member set. The engine builds it from the DIS election result (circuit/dis.go)
// and the segment membership. It is a plain value so the lsdb package does not
// import the circuit/engine layer.
type PseudonodeInfo struct {
	// SystemID is the DIS's System ID; the pseudo-node's Source ID is
	// <SystemID>.<PseudonodeID> (ISO/IEC 10589 clause 8.4.5).
	SystemID types.SystemID
	// PseudonodeID is the non-zero pseudonode octet the DIS allocated for this
	// circuit+level. It MUST be 1..255 (0 means a real router, not a pseudo-node);
	// OriginatePseudonode rejects 0.
	PseudonodeID uint8
	// Members are the System IDs of every router on the segment (the DIS included),
	// each advertised as a TLV 22 neighbor at metric 0. The engine de-duplicates
	// and orders them (so the originated bytes are deterministic).
	Members []types.SystemID
	// MaxLifetime is the Remaining Lifetime stamped on the fresh pseudo-node LSP
	// (MaxAge). Zero defaults to DefaultMaxAge in seconds, like an own LSP.
	MaxLifetime uint16
	// MaxLSPSize is the maximum LSP PDU size for fragmentation (the circuit MTU).
	// Zero defaults to DefaultMaxLSPSize. A LAN with more members than fit one
	// fragment splits across fragments via the same packer the own LSP uses (A-4).
	MaxLSPSize int
}

// pseudonodeMetricZero is the wide IS-reachability metric the pseudo-node LSP
// advertises toward every member: 0. ISO/IEC 10589 clause 8.4.5 / RFC 5305: the
// pseudo-node is a virtual node with zero-cost edges to its members, so SPF (the
// star) charges only the real routers' edge metrics toward the pseudo-node.
const pseudonodeMetricZero = 0

// OriginatePseudonode builds and stores the pseudo-node LSP set for level from
// info (ISO/IEC 10589 clause 8.4.5), reusing the spec-isis-6 origination path: it
// assigns monotonically increasing sequence numbers from the Originator's own
// per-LSP-ID state, handles wraparound identically, and stores each fragment via
// LSDB.Insert. It returns the affected LSP IDs (the engine arms SRM so spec-isis-7
// floods them, and emits LSP-change events).
//
// The pseudo-node Source ID is <info.SystemID>.<info.PseudonodeID> with a NON-ZERO
// pseudonode octet; a zero PseudonodeID is rejected (it would collide with the
// node's own router LSP). Each member is a TLV 22 entry at metric 0, the DIS
// included. TLV 1 (area addresses) is intentionally NOT carried on the pseudo-node
// LSP: areas are advertised by each real router's own LSP, and a pseudo-node has
// no area of its own (FRR/IOS originate an area-less pseudo-node LSP); this keeps
// the pseudo-node a pure topology hub.
func (o *Originator) OriginatePseudonode(level Level, info PseudonodeInfo) OriginateResult {
	o.mu.Lock()
	defer o.mu.Unlock()

	if info.PseudonodeID == 0 {
		// A zero pseudonode octet is not a pseudo-node; refuse rather than corrupt
		// the node's own router LSP ID space.
		return OriginateResult{}
	}

	maxSize := info.MaxLSPSize
	if maxSize <= 0 {
		maxSize = DefaultMaxLSPSize
	}
	if maxSize < minLSPSize {
		maxSize = minLSPSize
	}
	lifetime := info.MaxLifetime
	if lifetime == 0 {
		lifetime = uint16(DefaultMaxAge.Seconds())
	}

	pt := lspPDUType(level)
	src := types.NewSourceID(info.SystemID, info.PseudonodeID)

	fragments := pseudonodeFragments(maxSize, info.Members)

	var res OriginateResult
	now := o.now()
	for num, tlvs := range fragments {
		id := types.NewLSPID(src, uint8(num))
		// A pseudo-node LSP type block carries the level IS-type bits; the overload
		// (OL) bit lives only in a real node's fragment 0, never a pseudo-node
		// (RFC 3787). Pass overload=false.
		typeBlock := lspTypeBlock(level, false, num == 0)
		wrote, wrapped := o.originateFragmentLocked(level, pt, id, typeBlock, lifetime, tlvs, now)
		if wrapped {
			res.Wrapped = true
			res.Purged = append(res.Purged, id)
			continue
		}
		if wrote {
			res.Originated = append(res.Originated, id)
		}
	}

	// Purge any pseudo-node fragment that exists in the LSDB but is no longer
	// produced this pass (the member set shrank to fewer fragments), so a stale
	// fragment does not linger (clause 7.3.16/17).
	purged := o.purgeStaleFragmentsLocked(level, src, len(fragments))
	res.Purged = append(res.Purged, purged...)

	if len(res.Originated) > 0 || len(res.Purged) > 0 {
		o.lsdb.incOriginations(level)
	}
	return res
}

// PurgePseudonode purges every fragment of the pseudo-node LSP
// <sys>.<pnid> at level (ISO/IEC 10589 clause 8.4.5 / 7.3.16.1): the local node
// has lost the DIS role (R-2) or the circuit went away, so the pseudo-node it
// originated MUST be withdrawn before another node's SPF can see a phantom node.
// Each existing fragment is re-flooded as a purge (Remaining Lifetime 0) at a
// bumped sequence so peers accept it as newer. The pseudonode ID is released (its
// sequence state cleared) once purged. It returns the purged LSP IDs (the engine
// arms SRM so the purge floods). A zero pnid is a no-op (not a pseudo-node).
func (o *Originator) PurgePseudonode(level Level, sys types.SystemID, pnid uint8) []types.LSPID {
	o.mu.Lock()
	defer o.mu.Unlock()
	if pnid == 0 {
		return nil
	}
	src := types.NewSourceID(sys, pnid)
	pt := lspPDUType(level)
	var purged []types.LSPID
	for num := range maxFragments {
		id := types.NewLSPID(src, uint8(num))
		if o.lsdb.Lookup(level, id) == nil {
			// Fragments are contiguous from 0; stop at the first absent one.
			break
		}
		prev := o.lastSeq[id]
		next, didWrap := prev.NextChecked()
		if didWrap {
			next = types.MaxSequenceNumber
		}
		typeBlock := lspTypeBlock(level, false, num == 0)
		o.purgeFragmentLocked(level, pt, id, typeBlock, next)
		// Release the pseudonode fragment's sequence state: a later re-election that
		// reuses this pseudonode ID restarts from 1 after the purge ages out (the
		// receiver retains the purge for ZeroAgeLifetime, clause 7.3.16.1).
		delete(o.lastSeq, id)
		purged = append(purged, id)
	}
	if len(purged) > 0 {
		o.lsdb.incOriginations(level)
	}
	return purged
}

// pseudonodeFragments packs the pseudo-node LSP's TLV 22 member entries (each at
// metric 0) into per-fragment TLV lists using the same fragmenter the own LSP
// uses, so no fragment exceeds maxSize and no single TLV 22 entry is split (A-4).
// Fragment 0 always exists (RFC 3786), even with no members (an empty pseudo-node
// LSP is valid -- a DIS on a segment where it is the only router still originates
// the pseudo-node listing just itself). The pseudo-node carries no fixed TLV 1 /
// 129 / 132 (see OriginatePseudonode): only the member TLV 22 entries.
func pseudonodeFragments(maxSize int, members []types.SystemID) [][]packet.TLV {
	budget := maxSize - packet.CommonHeaderLen - lspBodyFixedLen
	frags := newFragmentPacker(budget)
	for _, m := range members {
		frags.addEntry(packet.TLVExtendedISReach, pseudonodeMemberEntry(m))
	}
	return frags.fragments()
}

// pseudonodeMemberEntry builds one TLV 22 entry for a pseudo-node member: the
// member's Source ID (System ID, pseudonode 0 -- a real router) at metric 0
// (ISO/IEC 10589 clause 8.4.5 / RFC 5305 sec 3). It reuses extISReachEntryBytes
// (the same encoder the own LSP's TLV 22 entries use) so the entry layout cannot
// drift from the codec.
func pseudonodeMemberEntry(member types.SystemID) []byte {
	metric, _ := types.NewMetric(pseudonodeMetricZero)
	return extISReachEntryBytes(AdjacencyInfo{
		Neighbor: types.NewSourceID(member, 0),
		Metric:   metric,
	})
}
