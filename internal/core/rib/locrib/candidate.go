// Design: plan/learned/639-rib-unified.md -- Phase 3 (unified Loc-RIB)
// Related: entry.go -- Entry holds the per-prefix PathGroup these Paths live in

package locrib

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/redistevents"
)

// Path is one route option for a single (family, prefix), contributed by one
// source (protocol + instance). Value-typed and self-contained so copies
// cross component boundaries without pointer aliasing per rules/memory.md.
//
// Cross-protocol best-path is resolved by AdminDistance first (lower wins),
// then Metric (lower wins). Within-protocol tiebreakers (e.g. BGP RFC 4271
// §9.1.2.2) are applied by the producing protocol before it publishes its
// best Path here -- Loc-RIB never sees the internal candidate list of any
// single protocol.
//
// The type is called Path (rather than Candidate) because each entry here
// represents one already-selected best path per (source, instance) pair;
// Loc-RIB arbitrates across sources, not across raw candidates within one.
type Path struct {
	// Source is the numeric protocol identity (registered via
	// redistevents.RegisterProtocol). Zero is ProtocolUnspecified and
	// marks an invalid path.
	Source redistevents.ProtocolID

	// Instance is a within-protocol identifier used to distinguish multiple
	// route advertisements from the same protocol for the same prefix.
	// Examples: a peer-index for BGP, a process-ID for OSPF, 0 for kernel
	// and connected. Upsert replaces on (Source, Instance) match.
	Instance uint32

	// NextHop is the IP address the FIB should forward to. The zero Addr
	// means "directly connected" or "reject" depending on protocol.
	NextHop netip.Addr

	// AdminDistance is the protocol's trustworthiness rank. Classical
	// Cisco/Juniper defaults: Connected=0, Static=1, eBGP=20, OSPF=110,
	// RIP=120, iBGP=200. Lower wins across protocols.
	AdminDistance uint8

	// Metric is the per-protocol tiebreaker when AdminDistance ties. Lower
	// wins. Semantics are protocol-defined (BGP MED, OSPF cost, hop count).
	Metric uint32

	// Labels is the MPLS label stack to impose toward NextHop (outermost
	// first), for label-carrying sources such as BGP labeled-unicast (SAFI 4).
	// Empty for a plain IP route. The producer builds a fresh slice per
	// best-path change and does not mutate it afterward, so it is shared (not
	// copied) into the Loc-RIB and on to the FIB, the same as the event-bus
	// BestChange path.
	Labels []uint32

	// IsEBGP marks a BGP-sourced path learned from an external peer. Like
	// Labels, it is carry-through metadata: Loc-RIB never uses it for
	// arbitration (selection is AdminDistance then Metric). The producing BGP
	// RIB sets it from the peer ASN relationship, and sysrib reads it to key
	// its own admin-distance override by protocol type ("ebgp"/"ibgp") without
	// re-deriving the class from the (operator-overridable) AdminDistance.
	// False for non-BGP sources. Excluded from Equal/key: a single
	// (Source, Instance) cannot change its eBGP/iBGP class because a peer's
	// ASN relationship is fixed.
	IsEBGP bool

	// BackupNextHop is a pre-computed fast-reroute backup next-hop the FIB
	// programs alongside NextHop as a link-down/backup path (an IP fast-reroute
	// alternate). It is generic carry-through metadata: an invalid Addr means "no
	// backup". Like Labels it is EXCLUDED from key() and from best-path
	// arbitration (arbitration stays AdminDistance then Metric), so a source's
	// backup can never change which path wins. It IS compared by Equal so a
	// backup-only change re-programs the FIB (the same contract as Labels: a
	// change the FIB must observe is in Equal, not in the arbitration key).
	BackupNextHop netip.Addr

	// BackupRepairLabels is the MPLS repair label stack to impose toward
	// BackupNextHop (outermost first), for a Segment-Routing repair tunnel
	// (TI-LFA). Empty for a plain IP backup. Built once per best-path change and
	// shared, not mutated, exactly like Labels.
	BackupRepairLabels []uint32

	// ECMP carries this best Path's own equal-cost multipath next-hops, for a
	// source that arbitrates ONE best across many candidates (BGP multipath:
	// best-path selection picks one winner across peers, so the equal-cost
	// siblings never enter the PathGroup as separate Paths the way IS-IS/OSPF
	// insert one Path per next-hop). siblingNextHops returns this set directly
	// when present; intra-source producers leave it nil and let the PathGroup
	// scan compute their siblings. EXCLUDED from key() and Equal (an ECMP-set
	// change is detected via the Change.ECMP set comparison, not best identity),
	// so it never affects arbitration. Built once per best-path change, shared
	// not mutated, exactly like Labels.
	ECMP []netip.Addr
}

// Valid reports whether p can be selected as a best path. An invalid Path is
// never returned by (*Manager).Best.
func (p Path) Valid() bool {
	return p.Source != redistevents.ProtocolUnspecified
}

// Equal reports whether two Paths are identical for change detection: every
// selection field plus the MPLS label stack. Required because Path now has a
// slice field (Labels) and so cannot be compared with ==/!=. Without the label
// comparison a relabel (same next hop, new label) would be missed.
func (p Path) Equal(q Path) bool {
	return p.Source == q.Source &&
		p.Instance == q.Instance &&
		p.NextHop == q.NextHop &&
		p.AdminDistance == q.AdminDistance &&
		p.Metric == q.Metric &&
		p.BackupNextHop == q.BackupNextHop &&
		slices.Equal(p.Labels, q.Labels) &&
		slices.Equal(p.BackupRepairLabels, q.BackupRepairLabels)
}

// key returns the (Source, Instance) identity used to dedup a Path within an
// Entry. Two Paths with the same key are the same source re-advertising;
// Insert replaces in place.
func (p Path) key() pathKey {
	return pathKey{source: p.Source, instance: p.Instance}
}

type pathKey struct {
	source   redistevents.ProtocolID
	instance uint32
}
