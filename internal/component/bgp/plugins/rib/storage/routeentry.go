// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals
// Related: pathset.go -- per-prefix path-id bookkeeping used under ADD-PATH

package storage

import (
	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

// RouteEntry stores per-attribute handles for a single route.
// Each attribute type has its own pool, enabling fine-grained deduplication.
// Routes with identical ORIGIN but different MED share the ORIGIN pool entry.
//
// Use InvalidHandle for attributes not present in the route.
// Use Release() to decrement refcounts when removing from RIB.
//
// Received Partial facts for the four dedicated optional-transitive pools are
// retained separately from their shared values. OtherAttrs keeps full headers.
// StaleLevelFresh is the default stale level: route is not stale.
// Plugins define their own non-zero levels and pass them via RIB commands.
// The RIB is level-agnostic: it stores, compares, and filters by level
// without knowing what each level means.
const StaleLevelFresh uint8 = 0

// DepreferenceThreshold is the stale level at which best-path deprioritizes routes.
// Routes at or above this level lose to routes below it.
// Plugins that need depreference should use levels >= this value.
const DepreferenceThreshold uint8 = 2

// AdvertisedStaleLevel combines a route's two stale facts into the one level
// best path ranks it at and every RIB rail hands the readvertise egress filters:
// retention, the mark mark-stale and attach-community write (RouteEntry
// StaleLevel, or an Adj-RIB-Out copy's StaleLevel), and receivedLLGRStale, the
// route's ReceivedLLGRStale as the GR ingress step decided it. It is the higher
// of the two, so a received LLGR_STALE route never ranks or travels below
// DepreferenceThreshold, and a mark above it is kept.
//
// RFC 9494 Section 4.3: "upon receiving a route from that neighbor with the
// LLGR_STALE community or upon attaching the LLGR_STALE community itself per
// Section 4.2: * Treat the route as the least preferred in route selection
// [...] * The route SHOULD NOT be advertised to any neighbor from which the
// Long-Lived Graceful Restart Capability has not been received."
// Both bullets therefore read this one level.
func AdvertisedStaleLevel(retention uint8, receivedLLGRStale bool) uint8 {
	if receivedLLGRStale && retention < DepreferenceThreshold {
		return DepreferenceThreshold
	}
	return retention
}

// Each bit names one value-only pool whose received Partial fact is per route.
const (
	partialAggregator uint8 = 1 << iota
	partialCommunity
	partialExtCommunity
	partialLargeCommunity
)

type RouteEntry struct {
	// StaleLevel tracks route freshness. 0 = fresh (not stale).
	// Per-route metadata, not pooled -- each route has independent stale state.
	// RFC 4724: GR-stale routes (level 1) compete normally in best-path.
	// RFC 9494: LLGR-stale routes (level 2+) are least preferred.
	StaleLevel uint8
	// ReceivedLLGRStale says the route arrived carrying the LLGR_STALE
	// community from a neighbor Ze sent the LLGR Capability to, which is the
	// case RFC 9494 Section 4.3 governs. The GR plugin's ingress step decides
	// it (meta["stale"]), and the RIB's received handler writes it on every
	// insert of the route, the no-op and equal-attribute branches included, so
	// it always follows the latest UPDATE. It is kept apart from StaleLevel on
	// purpose. StaleLevel is the RETENTION
	// mark that mark-stale overwrites and purge-stale deletes on, so a received
	// LLGR_STALE route stored there would be purged at the next TCP loss and at
	// every EOR. extractCandidate combines the two for best path.
	ReceivedLLGRStale bool
	// partial keeps received Partial facts independently of shared values.
	// Zero means no retained received Partial, not a rule for new attachment.
	// It occupies padding before MsgID on the current 64-bit layout.
	partial uint8
	// MsgID identifies the received UPDATE for validation eligibility.
	// Zero is an unversioned producer; it asks the gate about the current path.
	MsgID uint64
	// SourcePolicyEpoch is the original admission provenance. Zero is
	// unrecorded; retained bytes MUST NOT acquire the source's current epoch.
	SourcePolicyEpoch uint64
	// SourceReceipt identifies the original source Session. Zero is explicitly
	// unrecorded; a retained route MUST NOT borrow a replacement session's receipt.
	SourceReceipt uint64
	// AttrFingerprint is an FNV-1a hash of the raw attribute wire bytes and
	// ASN4 flag at insert time. AttrLen stores the raw byte length. Together
	// they form a probabilistic equality check: matching hash + length skips
	// ParseAttributes for no-op re-announcements. Collision probability is
	// ~N^2/2^65 for N distinct attribute blobs (at 1M uniques, P < 10^-7).
	// A collision causes a missed attribute update; RIB refresh recovers.
	// Zero fingerprint means "not set" (first insert or legacy entry).
	AttrFingerprint uint64
	AttrLen         uint32
	// Bundle references a shared Bundle in BundlePool containing the 12
	// non-AS_PATH attribute handles (Origin, NextHop, LocalPref, MED, etc.).
	// 97% of routes share identical non-AS_PATH attributes.
	Bundle attrpool.Handle
	// ASPath is kept separate because best-path needs AS_PATH length/first-AS
	// on every candidate extraction, and AS_PATH diversity is much higher than
	// the other attributes (only 87% sharing vs 97%).
	ASPath attrpool.Handle
	// source owns the original Session's lease independently of attribute
	// sharing. Nil leaves SourceReceipt as metadata, never authorization.
	source *SourceReference
}

// NewRouteEntry creates a RouteEntry with all handles set to InvalidHandle.
func NewRouteEntry() RouteEntry {
	return RouteEntry{
		Bundle: attrpool.InvalidHandle,
		ASPath: attrpool.InvalidHandle,
	}
}

// receivedMatches separates a duplicate announcement from one with a new
// admission identity, even when its pooled attributes are identical.
func (e RouteEntry) receivedMatches(messageID, policyEpoch, sourceReceipt uint64) bool {
	if e.StaleLevel != StaleLevelFresh {
		return false
	}
	if e.MsgID != messageID {
		return false
	}
	if e.SourcePolicyEpoch != policyEpoch {
		return false
	}
	return e.SourceReceipt == sourceReceipt
}

// HasASPath returns true if AS_PATH attribute is present.
func (e *RouteEntry) HasASPath() bool { return e.ASPath.IsValid() }

// hasBundle returns true if Bundle handle is present.
func (e *RouteEntry) hasBundle() bool { return e.Bundle.IsValid() }

// GetBundle returns the Bundle from BundlePool. Returns an empty Bundle
// if the handle is invalid.
func (e *RouteEntry) GetBundle() Bundle {
	if !e.Bundle.IsValid() {
		return NewBundle()
	}
	return Bundles.Get(e.Bundle)
}

// AggregatorFlags returns the dedicated AGGREGATOR's standard Optional/Transitive
// bits and its retained received Partial fact. The receiver MUST be non-nil.
// It does not report presence, retain a reference or choose Extended Length;
// writers MUST keep their existing presence, ownership and header checks.
// This stored-flag contract also applies to the three community accessors.
// RFC 4271 Section 5 governs the retained Partial fact.
func (e *RouteEntry) AggregatorFlags() byte {
	return e.pooledTransitiveFlags(partialAggregator)
}

// CommunityFlags returns the dedicated COMMUNITIES flags under AggregatorFlags'
// stored-flag contract. It does not report whether the attribute is present.
// RFC 4271 Section 5 governs the retained Partial fact.
func (e *RouteEntry) CommunityFlags() byte {
	return e.pooledTransitiveFlags(partialCommunity)
}

// ExtCommunityFlags returns the dedicated EXTENDED_COMMUNITIES flags under
// AggregatorFlags' stored-flag contract, without choosing a length header.
// RFC 4271 Section 5 governs the retained Partial fact.
func (e *RouteEntry) ExtCommunityFlags() byte {
	return e.pooledTransitiveFlags(partialExtCommunity)
}

// LargeCommunityFlags returns the dedicated LARGE_COMMUNITIES flags under
// AggregatorFlags' stored-flag contract, independently of shared attribute values.
// RFC 4271 Section 5 governs the retained Partial fact.
func (e *RouteEntry) LargeCommunityFlags() byte {
	return e.pooledTransitiveFlags(partialLargeCommunity)
}

// pooledTransitiveFlags restores one dedicated pool's received Partial fact.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." A zero fact retains the encoder's existing flags;
// this does not decide how a new attribute must be attached to a path.
func (e *RouteEntry) pooledTransitiveFlags(mask uint8) byte {
	flags := byte(0xc0)
	if e.partial&mask != 0 {
		flags |= 0x20
	}
	return flags
}

// Release decrements refcount for Bundle (cascade-releases inner handles)
// and ASPath. Safe to call multiple times.
// An owner returned by parsing, Clone or retained lookup MUST call Release
// after its final use; AddRef MUST acquire a reference for each new owner.
func (e *RouteEntry) Release() {
	e.source.Release()
	e.source = nil
	if e.Bundle.IsValid() {
		Bundles.Release(e.Bundle)
		e.Bundle = attrpool.InvalidHandle
	}
	if e.ASPath.IsValid() {
		_ = pool.ASPath.Release(e.ASPath)
		e.ASPath = attrpool.InvalidHandle
	}
	e.partial = 0
}

// AddRef increments refcount for Bundle and ASPath handles.
// Use when sharing a RouteEntry between multiple owners.
// The new owner MUST call Release after its final use.
func (e *RouteEntry) AddRef() error {
	if e.Bundle.IsValid() {
		Bundles.AddRef(e.Bundle)
	}
	if e.ASPath.IsValid() {
		if err := pool.ASPath.AddRef(e.ASPath); err != nil {
			if e.Bundle.IsValid() {
				Bundles.Release(e.Bundle)
			}
			return err
		}
	}
	e.source.AddRef()
	return nil
}

// Clone creates a copy of the RouteEntry with AddRef called on both handles.
// Caller MUST call Release() on the clone when done.
// Returns nil if AddRef fails (e.g., pool shutdown).
// The received Partial facts MUST follow this copy independently of pool sharing
// to preserve RFC 4271 Section 5's rule: "If a path with a recognized, transitive
// optional attribute is accepted and passed along to other BGP peers and the
// Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it
// MUST NOT be set back to 0 by the current AS."
func (e *RouteEntry) Clone() *RouteEntry {
	clone := &RouteEntry{
		StaleLevel:        e.StaleLevel,
		ReceivedLLGRStale: e.ReceivedLLGRStale,
		partial:           e.partial,
		MsgID:             e.MsgID,
		SourcePolicyEpoch: e.SourcePolicyEpoch,
		SourceReceipt:     e.SourceReceipt,
		AttrFingerprint:   e.AttrFingerprint,
		AttrLen:           e.AttrLen,
		Bundle:            e.Bundle,
		ASPath:            e.ASPath,
		source:            e.source,
	}
	if err := clone.AddRef(); err != nil {
		return nil
	}
	return clone
}
