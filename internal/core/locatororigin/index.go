// Design: docs/architecture/isis/isis-4-component-config.md -- in-daemon locator-originator index
// Related: snapshot.go -- EventBus value types and ReasonClass definitions

package locatororigin

import (
	"net/netip"
	"slices"
	"sync"
)

// sourceState holds one named source's latest applied snapshot.
type sourceState struct {
	generation uint64
	active     bool
	entries    []LocatorEntry
}

// Index holds the latest locator-originator snapshots per named source under a
// read-write mutex so BGP receive validation reads with zero RPC and zero write
// contention. The zero value is ready for use. Safe for concurrent use.
type Index struct {
	mu      sync.RWMutex
	sources map[string]sourceState
}

// NewIndex returns an empty Index ready for use.
func NewIndex() *Index {
	return &Index{}
}

var defaultIndex Index

// ApplySnapshot updates the package-level default index with snap.
// Safe for concurrent use.
func ApplySnapshot(snap *Snapshot) bool {
	return defaultIndex.ApplySnapshot(snap)
}

// ClearSource removes source from the package-level default index so an abrupt
// process exit clears stale entries and allows a replacement incarnation to
// start at generation 1. Safe for concurrent use.
func ClearSource(source string) {
	defaultIndex.ClearSource(source)
}

// Resolve queries the package-level default index for the originator of the
// longest advertised locator covering sid. Safe for concurrent use.
func Resolve(sid netip.Addr) Result {
	return defaultIndex.Resolve(sid)
}

// ResetForTest clears the package-level default index. Test use only.
func ResetForTest() {
	defaultIndex.Reset()
}

// Reset clears all sources from idx. Safe for concurrent use.
func (idx *Index) Reset() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.sources = nil
}

// ApplySnapshot replaces snap.Source's entries when snap.Generation is strictly
// greater than the last applied generation for that source (or when no snapshot
// has been applied for snap.Source yet). Returns true when applied and false
// when dropped as nil, unnamed, or stale. Safe for concurrent use.
func (idx *Index) ApplySnapshot(snap *Snapshot) bool {
	if snap == nil {
		return false
	}
	if snap.Source == "" {
		return false
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.sources == nil {
		idx.sources = make(map[string]sourceState)
	}
	if cur, exists := idx.sources[snap.Source]; exists {
		if snap.Generation <= cur.generation {
			return false
		}
	}

	next := sourceState{
		generation: snap.Generation,
		active:     snap.Active,
	}
	if snap.Active {
		next.entries = cloneEntries(snap.Entries)
	}
	idx.sources[snap.Source] = next
	return true
}

// ClearSource deletes source's state and generation floor from idx so a
// disconnected or stopped process leaves no stale locators and a replacement
// incarnation starting at generation 1 is accepted. Safe for concurrent use.
func (idx *Index) ClearSource(source string) {
	if source == "" {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.sources, source)
}

// Resolve finds the longest-matching advertised SRv6 locator covering sid across
// all active sources. When no source is active or no locator covers sid, it
// returns Known=false with a named ReasonClass and Reason, never an empty
// address set treated as a mismatch. Safe for concurrent use.
func (idx *Index) Resolve(sid netip.Addr) Result {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	activeSources := 0
	for _, src := range idx.sources {
		if src.active {
			activeSources++
		}
	}
	if activeSources == 0 {
		return Result{
			Known:       false,
			ReasonClass: ReasonNoActiveSource,
			Reason:      "no active IGP locator-originator source",
		}
	}
	if !sid.IsValid() {
		return Result{
			Known:       false,
			ReasonClass: ReasonNoCoveringLocator,
			Reason:      "SRv6 SID is not valid",
		}
	}
	if !sid.Is6() {
		return Result{
			Known:       false,
			ReasonClass: ReasonNoCoveringLocator,
			Reason:      "SRv6 SID " + sid.String() + " is not an IPv6 address",
		}
	}
	if sid.Is4In6() {
		return Result{
			Known:       false,
			ReasonClass: ReasonNoCoveringLocator,
			Reason:      "SRv6 SID " + sid.String() + " is an IPv4-mapped IPv6 address",
		}
	}

	bestBits := -1
	var best []LocatorEntry
	for _, src := range idx.sources {
		if !src.active {
			continue
		}
		for i := range src.entries {
			entry := src.entries[i]
			if !entry.Prefix.IsValid() {
				continue
			}
			if !entry.Prefix.Contains(sid) {
				continue
			}
			bits := entry.Prefix.Bits()
			if bits > bestBits {
				bestBits = bits
				best = append(best[:0], entry)
				continue
			}
			if bits == bestBits {
				best = append(best, entry)
			}
		}
	}

	if bestBits < 0 {
		return Result{
			Known:       false,
			ReasonClass: ReasonNoCoveringLocator,
			Reason:      "no advertised locator covers SID " + sid.String(),
		}
	}
	if len(best) == 1 {
		return resultFromEntry(best[0])
	}
	return mergeMatchingEntries(best)
}

// isNativeIPv6 reports whether addr is a native IPv6 address (excluding
// IPv4-mapped IPv6 addresses such as ::ffff:192.0.2.1).
func isNativeIPv6(addr netip.Addr) bool {
	if !addr.Is6() {
		return false
	}
	return !addr.Is4In6()
}

// MatchAddress compares one BGP address (such as an MP_REACH_NLRI next hop)
// against r. A matching IPv6 Source Router ID sub-TLV (RFC 7794 Section 2.2) is
// accepted as a known match unless the locator is anycast. Against a proven
// originator, an IPv4 or IPv4-mapped IPv6 address or an absent native IPv6
// address is a proven mismatch unless the originating LSP carries undecoded
// TLV 237 (MT IPv6 reachability) for native IPv6 comparisons.
func (r *Result) MatchAddress(addr netip.Addr) (MatchOutcome, ReasonClass, string) {
	if r.ReasonClass == ReasonAnycast {
		return MatchUnknown, ReasonAnycast, r.Reason
	}
	if !addr.IsValid() {
		return MatchUnknown, ReasonUnsupportedNextHopLen, "compared address is invalid"
	}
	if isNativeIPv6(addr) {
		if slices.Contains(r.SourceRouterIDs, addr) {
			return MatchKnown, ReasonNone, ""
		}
		if r.Known {
			if slices.Contains(r.Addresses, addr) {
				return MatchKnown, ReasonNone, ""
			}
			if r.HasUndecodedTLV237 {
				return MatchUnknown, ReasonUndecodedTLV237, "originator " + r.Originator + " LSP carries undecoded TLV 237 (MT IPv6 reachability)"
			}
			return MatchMismatch, ReasonNone, ""
		}
		return MatchUnknown, r.ReasonClass, r.Reason
	}
	// IPv4 and IPv4-mapped IPv6 addresses are never in an IPv6 originator set:
	// proven mismatch against a known originator, kept with warning when the
	// originator is not known.
	if r.Known {
		return MatchMismatch, ReasonNone, ""
	}
	return MatchUnknown, r.ReasonClass, r.Reason
}

// MatchPrefix compares an ISD route's Address/Prefix field against r. A native
// IPv6 host /128 prefix matches the originator's full IPv6 address set (TLV
// 140, TLV 232, local /128 TLV 236, or covering Source Router ID); a shorter
// native IPv6 prefix matches any local TLV 236 prefix advertised by the known
// originator (with undecoded TLV 237 guarding unmatched native IPv6 prefixes).
// A native IPv4 prefix matches the originator's advertised IPv4 prefixes (TLV
// 132/134 /32 or local TLV 135) or /32 IPv4 addresses. An IPv4-mapped IPv6
// prefix or unadvertised prefix against a proven originator is a proven
// mismatch.
func (r *Result) MatchPrefix(prefix netip.Prefix) (MatchOutcome, ReasonClass, string) {
	if r.ReasonClass == ReasonAnycast {
		return MatchUnknown, ReasonAnycast, r.Reason
	}
	if !prefix.IsValid() {
		return MatchUnknown, ReasonNoCoveringLocator, "compared prefix is invalid"
	}
	canonical := prefix.Masked()
	addr := canonical.Addr()
	if isNativeIPv6(addr) {
		if canonical.Bits() == 128 {
			return r.MatchAddress(addr)
		}
		if !r.Known {
			return MatchUnknown, r.ReasonClass, r.Reason
		}
		if slices.Contains(r.Prefixes, canonical) {
			return MatchKnown, ReasonNone, ""
		}
		if r.HasUndecodedTLV237 {
			return MatchUnknown, ReasonUndecodedTLV237, "originator " + r.Originator + " LSP carries undecoded TLV 237 (MT IPv6 reachability)"
		}
		return MatchMismatch, ReasonNone, ""
	}
	if !r.Known {
		return MatchUnknown, r.ReasonClass, r.Reason
	}
	if addr.Is4() {
		if slices.Contains(r.Prefixes, canonical) {
			return MatchKnown, ReasonNone, ""
		}
		if canonical.Bits() == 32 {
			if slices.Contains(r.Addresses, addr) {
				return MatchKnown, ReasonNone, ""
			}
		}
	}
	return MatchMismatch, ReasonNone, ""
}

func cloneEntries(in []LocatorEntry) []LocatorEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]LocatorEntry, 0, len(in))
	for i := range in {
		e := in[i]
		if !e.Prefix.IsValid() {
			continue
		}
		if !isNativeIPv6(e.Prefix.Addr()) {
			continue
		}
		e.Prefix = e.Prefix.Masked()
		e.Addresses = slices.Clone(e.Addresses)
		e.Prefixes = normalizePrefixes(e.Prefixes)
		e.SourceRouterIDs = slices.Clone(e.SourceRouterIDs)
		out = append(out, e)
	}
	return out
}

func normalizePrefixes(in []netip.Prefix) []netip.Prefix {
	if len(in) == 0 {
		return nil
	}
	out := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		if !p.IsValid() {
			continue
		}
		out = append(out, p.Masked())
	}
	return out
}

func resultFromEntry(e LocatorEntry) Result {
	return Result{
		Prefix:             e.Prefix,
		Known:              e.Known,
		Originator:         e.Originator,
		Addresses:          slices.Clone(e.Addresses),
		Prefixes:           slices.Clone(e.Prefixes),
		SourceRouterIDs:    slices.Clone(e.SourceRouterIDs),
		HasUndecodedTLV237: e.HasUndecodedTLV237,
		ReasonClass:        e.ReasonClass,
		Reason:             e.Reason,
	}
}

func mergeMatchingEntries(entries []LocatorEntry) Result {
	prefix := entries[0].Prefix
	for i := range entries {
		if entries[i].ReasonClass == ReasonAnycast {
			return Result{
				Prefix:      prefix,
				Known:       false,
				ReasonClass: ReasonAnycast,
				Reason:      entries[i].Reason,
			}
		}
	}

	var originator string
	var addrs []netip.Addr
	var prefixes []netip.Prefix
	var sourceIDs []netip.Addr
	hasTLV237 := false
	knownCount := 0
	var firstUnknown LocatorEntry
	hasUnknown := false

	for i := range entries {
		e := entries[i]
		for _, id := range e.SourceRouterIDs {
			if !slices.Contains(sourceIDs, id) {
				sourceIDs = append(sourceIDs, id)
			}
		}
		if !e.Known {
			if !hasUnknown {
				firstUnknown = e
				hasUnknown = true
			}
			continue
		}
		if knownCount == 0 {
			originator = e.Originator
			knownCount = 1
		} else if e.Originator != originator {
			return Result{
				Prefix:      prefix,
				Known:       false,
				ReasonClass: ReasonAnycast,
				Reason:      "locator " + prefix.String() + " advertised by multiple originators across sources",
			}
		}
		for _, a := range e.Addresses {
			if !slices.Contains(addrs, a) {
				addrs = append(addrs, a)
			}
		}
		for _, p := range e.Prefixes {
			if !slices.Contains(prefixes, p) {
				prefixes = append(prefixes, p)
			}
		}
		if e.HasUndecodedTLV237 {
			hasTLV237 = true
		}
	}

	if knownCount == 1 {
		return Result{
			Prefix:             prefix,
			Known:              true,
			Originator:         originator,
			Addresses:          addrs,
			Prefixes:           prefixes,
			SourceRouterIDs:    sourceIDs,
			HasUndecodedTLV237: hasTLV237,
			ReasonClass:        ReasonNone,
		}
	}

	return Result{
		Prefix:          prefix,
		Known:           false,
		Originator:      firstUnknown.Originator,
		SourceRouterIDs: sourceIDs,
		ReasonClass:     firstUnknown.ReasonClass,
		Reason:          firstUnknown.Reason,
	}
}
