// Design: plan/design-rib-unified.md -- Phase 3 (unified Loc-RIB)

package locrib

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
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
}

// Valid reports whether p can be selected as a best path. An invalid Path is
// never returned by (*Manager).Best.
func (p Path) Valid() bool {
	return p.Source != redistevents.ProtocolUnspecified
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
