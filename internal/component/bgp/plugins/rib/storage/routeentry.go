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
// Limitation: Attribute flags (especially Partial bit 0x20) are not preserved.
// For exact wire reproduction, use msg-id cache forwarding instead.
// StaleLevelFresh is the default stale level: route is not stale.
// Plugins define their own non-zero levels and pass them via RIB commands.
// The RIB is level-agnostic: it stores, compares, and filters by level
// without knowing what each level means.
const StaleLevelFresh uint8 = 0

// DepreferenceThreshold is the stale level at which best-path deprioritizes routes.
// Routes at or above this level lose to routes below it.
// Plugins that need depreference should use levels >= this value.
const DepreferenceThreshold uint8 = 2

type RouteEntry struct {
	// StaleLevel tracks route freshness. 0 = fresh (not stale).
	// Per-route metadata, not pooled -- each route has independent stale state.
	// RFC 4724: GR-stale routes (level 1) compete normally in best-path.
	// RFC 9494: LLGR-stale routes (level 2+) are least preferred.
	StaleLevel uint8
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
}

// NewRouteEntry creates a RouteEntry with all handles set to InvalidHandle.
func NewRouteEntry() RouteEntry {
	return RouteEntry{
		Bundle: attrpool.InvalidHandle,
		ASPath: attrpool.InvalidHandle,
	}
}

// HasASPath returns true if AS_PATH attribute is present.
func (e *RouteEntry) HasASPath() bool { return e.ASPath.IsValid() }

// HasBundle returns true if Bundle handle is present.
func (e *RouteEntry) HasBundle() bool { return e.Bundle.IsValid() }

// GetBundle returns the Bundle from BundlePool. Returns an empty Bundle
// if the handle is invalid.
func (e *RouteEntry) GetBundle() Bundle {
	if !e.Bundle.IsValid() {
		return NewBundle()
	}
	return Bundles.Get(e.Bundle)
}

// Release decrements refcount for Bundle (cascade-releases inner handles)
// and ASPath. Safe to call multiple times.
func (e *RouteEntry) Release() {
	if e.Bundle.IsValid() {
		Bundles.Release(e.Bundle)
		e.Bundle = attrpool.InvalidHandle
	}
	if e.ASPath.IsValid() {
		_ = pool.ASPath.Release(e.ASPath)
		e.ASPath = attrpool.InvalidHandle
	}
}

// AddRef increments refcount for Bundle and ASPath handles.
// Use when sharing a RouteEntry between multiple owners.
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
	return nil
}

// Clone creates a copy of the RouteEntry with AddRef called on both handles.
// Caller must call Release() on the clone when done.
// Returns nil if AddRef fails (e.g., pool shutdown).
func (e *RouteEntry) Clone() *RouteEntry {
	clone := &RouteEntry{
		StaleLevel:      e.StaleLevel,
		AttrFingerprint: e.AttrFingerprint,
		AttrLen:         e.AttrLen,
		Bundle:          e.Bundle,
		ASPath:          e.ASPath,
	}
	if err := clone.AddRef(); err != nil {
		return nil
	}
	return clone
}
