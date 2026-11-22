// Design: docs/architecture/route-selection.md -- next-hop admission visibility and runtime doctor checks
// Related: rib.go, rib_next_hop.go, rib_remote.go, doctor_codes.go -- runtime doctor wiring and next-hop admission
package rib

import (
	"cmp"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	unresolvedNextHopDoctorName         = "bgp-rib-unresolved-next-hop"
	nextHopMetricsDoctorName            = "bgp-rib-next-hop-metrics"
	unresolvedNextHopDoctorFix          = "ze show bgp rib | match next-hop-unresolvable"
	doctorSeverityWarning               = "warning"
	maxTrackedUnresolvedNextHops        = 4096
	maxUnresolvedDoctorSamples          = 5
	overflowSeenWords                   = 64
	overflowSeenMask             uint64 = overflowSeenWords*64 - 1
	fnvOffset64                  uint64 = 14695981039346656037
	fnvPrime64                   uint64 = 1099511628211
)

// unresolvedRouteKey identifies one route (CIDR prefix or non-CIDR route key)
// in the bounded unresolved-next-hop tracker.
type unresolvedRouteKey struct {
	fam    family.Family
	prefix netip.Prefix
	opaque string
}

// unresolvedEntryKey identifies one excluded path for a route by its owning
// source and ADD-PATH path identifier so same-address peer sessions stay
// isolated on purge.
type unresolvedEntryKey struct {
	route    unresolvedRouteKey
	owner    uint64
	peerAddr string
	pathID   uint32
	addPath  bool
}

// unresolvedEntryValue stores the operator-facing prefix and next hop for one
// tracked unresolved path.
type unresolvedEntryValue struct {
	displayPrefix string
	nextHop       netip.Addr
}

// unresolvedCandidate carries one forwarding candidate excluded by
// candidateNextHopResolved into unresolvedNextHopTracker.reconcileRoute.
type unresolvedCandidate struct {
	owner         uint64
	peerAddr      string
	pathID        uint32
	addPath       bool
	displayPrefix string
	nextHop       netip.Addr
}

// unresolvedSample is a sortable snapshot entry formatted for doctor output.
type unresolvedSample struct {
	displayPrefix string
	nextHop       netip.Addr
	peerAddr      string
	owner         uint64
	pathID        uint32
}

// unresolvedNextHopTracker records BGP routes currently excluded from Phase 2
// best-path selection by an unresolvable next hop.
//
// Concurrency: safe for concurrent use. mu is a leaf mutex: callers may take it
// while holding peerMu or a bestPrev shard lock, and methods on this tracker
// NEVER acquire peerMu, remote.mu, a shard lock, or perform SDK/engine calls.
// Memory is strictly bounded to maxTrackedUnresolvedNextHops individual entries;
// exclusions beyond the cap are counted per peer owner using a fixed 4096-bit
// deduplication filter (512 bytes) so repeated re-announcements of an
// overflowed route do not inflate the count without growing entries.
type unresolvedNextHopTracker struct {
	total          atomic.Int64
	overflow       atomic.Int64
	mu             sync.Mutex
	entries        map[unresolvedEntryKey]unresolvedEntryValue
	byRoute        map[unresolvedRouteKey][]unresolvedEntryKey
	overflowByPeer map[bestPeer]int
	overflowOther  int
	overflowTotal  int
	overflowSeen   [overflowSeenWords]uint64
}

// hasOverflow reports whether any route exclusion is currently recorded beyond
// maxTrackedUnresolvedNextHops. Lock-free fast path for withdrawal cleanup.
func (t *unresolvedNextHopTracker) hasOverflow() bool {
	return t.overflow.Load() != 0
}

// reconcilePrefixRoute replaces the tracked unresolved set for a CIDR route.
func (t *unresolvedNextHopTracker) reconcilePrefixRoute(fam family.Family, prefix netip.Prefix, excluded []unresolvedCandidate) {
	if len(excluded) == 0 {
		if t.total.Load() == 0 {
			return
		}
	}
	t.reconcileRoute(unresolvedRouteKey{fam: fam, prefix: prefix}, excluded)
}

// reconcileOpaqueRoute replaces the tracked unresolved set for a non-CIDR route.
func (t *unresolvedNextHopTracker) reconcileOpaqueRoute(fam family.Family, routeKey []byte, excluded []unresolvedCandidate) {
	if len(excluded) == 0 {
		if t.total.Load() == 0 {
			return
		}
	}
	t.reconcileRoute(unresolvedRouteKey{fam: fam, opaque: string(routeKey)}, excluded)
}

func (t *unresolvedNextHopTracker) reconcileRoute(route unresolvedRouteKey, excluded []unresolvedCandidate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	oldKeys := t.byRoute[route]
	if len(excluded) == 0 {
		if len(oldKeys) == 0 {
			return
		}
		for _, k := range oldKeys {
			delete(t.entries, k)
		}
		delete(t.byRoute, route)
		t.syncTotalLocked()
		return
	}

	// Remove paths of this route that resolved or were withdrawn before
	// inserting replacements so a route changing next hop or peer reuses its
	// own tracker slot rather than overflowing unnecessarily.
	for _, oldKey := range oldKeys {
		if !containsExcludedKey(excluded, route, oldKey) {
			delete(t.entries, oldKey)
		}
	}

	if t.entries == nil {
		t.entries = make(map[unresolvedEntryKey]unresolvedEntryValue)
	}
	if t.byRoute == nil {
		t.byRoute = make(map[unresolvedRouteKey][]unresolvedEntryKey)
	}

	newKeys := make([]unresolvedEntryKey, 0, len(excluded))
	for _, ex := range excluded {
		k := unresolvedEntryKey{
			route:    route,
			owner:    ex.owner,
			peerAddr: ex.peerAddr,
			pathID:   ex.pathID,
			addPath:  ex.addPath,
		}
		val := unresolvedEntryValue{
			displayPrefix: ex.displayPrefix,
			nextHop:       ex.nextHop,
		}
		if _, exists := t.entries[k]; exists {
			t.entries[k] = val
			newKeys = append(newKeys, k)
			continue
		}
		if len(t.entries) < maxTrackedUnresolvedNextHops {
			t.entries[k] = val
			newKeys = append(newKeys, k)
			continue
		}
		t.recordOverflowLocked(k)
	}

	if len(newKeys) == 0 {
		delete(t.byRoute, route)
	} else {
		t.byRoute[route] = newKeys
	}
	t.syncTotalLocked()
}

func (t *unresolvedNextHopTracker) recordOverflowLocked(k unresolvedEntryKey) {
	bit := hashUnresolvedEntryKey(k) & overflowSeenMask
	word := bit >> 6
	mask := uint64(1) << (bit & 63)
	if t.overflowSeen[word]&mask != 0 {
		return
	}
	t.overflowSeen[word] |= mask

	peer := bestPeer{owner: k.owner, address: k.peerAddr}
	if t.overflowByPeer == nil {
		t.overflowByPeer = make(map[bestPeer]int)
	}
	if _, exists := t.overflowByPeer[peer]; exists {
		t.overflowByPeer[peer]++
		t.overflowTotal++
		return
	}
	if len(t.overflowByPeer) < maxTrackedUnresolvedNextHops {
		t.overflowByPeer[peer] = 1
		t.overflowTotal++
		return
	}
	t.overflowOther++
	t.overflowTotal++
}

// purgeByOwner removes all tracked and overflow entries belonging to peer.
// Same-address sibling BGP owners remain isolated because non-zero owners match
// strictly by owner ID.
func (t *unresolvedNextHopTracker) purgeByOwner(peer bestPeer) {
	if t.total.Load() == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	for k := range t.entries {
		if !entryMatchesOwner(k, peer) {
			continue
		}
		delete(t.entries, k)
		t.removeKeyFromRouteIndexLocked(k)
	}
	for p, n := range t.overflowByPeer {
		if !peerMatchesOwner(p, peer) {
			continue
		}
		t.overflowTotal -= n
		delete(t.overflowByPeer, p)
	}
	if len(t.entries) == 0 {
		if len(t.overflowByPeer) == 0 {
			t.overflowOther = 0
			t.overflowTotal = 0
		}
	}
	if t.overflowTotal == 0 {
		t.overflowSeen = [overflowSeenWords]uint64{}
	}
	t.syncTotalLocked()
}

// purgeByPeerAddr removes all tracked and overflow entries matching peerAddr
// across every owner (BGP and manual).
func (t *unresolvedNextHopTracker) purgeByPeerAddr(peerAddr string) {
	if t.total.Load() == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	for k := range t.entries {
		if k.peerAddr != peerAddr {
			continue
		}
		delete(t.entries, k)
		t.removeKeyFromRouteIndexLocked(k)
	}
	for p, n := range t.overflowByPeer {
		if p.address != peerAddr {
			continue
		}
		t.overflowTotal -= n
		delete(t.overflowByPeer, p)
	}
	if len(t.entries) == 0 {
		if len(t.overflowByPeer) == 0 {
			t.overflowOther = 0
			t.overflowTotal = 0
		}
	}
	if t.overflowTotal == 0 {
		t.overflowSeen = [overflowSeenWords]uint64{}
	}
	t.syncTotalLocked()
}

// clearAll removes every tracked entry and overflow counter when no BGP peer or
// manual BGP table remains in the RIB.
func (t *unresolvedNextHopTracker) clearAll() {
	if t.total.Load() == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	clear(t.entries)
	clear(t.byRoute)
	clear(t.overflowByPeer)
	t.overflowOther = 0
	t.overflowTotal = 0
	t.overflowSeen = [overflowSeenWords]uint64{}
	t.syncTotalLocked()
}

// clearOverflow resets overflow counters at the start of a full-table
// reselection while keeping tracked entries intact so concurrent doctor reads
// never flicker empty mid-scan.
func (t *unresolvedNextHopTracker) clearOverflow() {
	if t.overflow.Load() == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.overflowTotal == 0 {
		return
	}
	clear(t.overflowByPeer)
	t.overflowOther = 0
	t.overflowTotal = 0
	t.overflowSeen = [overflowSeenWords]uint64{}
	t.syncTotalLocked()
}

func (t *unresolvedNextHopTracker) snapshot() ([]unresolvedSample, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.entries) == 0 {
		if t.overflowTotal == 0 {
			return nil, 0
		}
	}
	samples := make([]unresolvedSample, 0, len(t.entries))
	for k, v := range t.entries {
		samples = append(samples, unresolvedSample{
			displayPrefix: v.displayPrefix,
			nextHop:       v.nextHop,
			peerAddr:      k.peerAddr,
			owner:         k.owner,
			pathID:        k.pathID,
		})
	}
	return samples, t.overflowTotal
}

func (t *unresolvedNextHopTracker) removeKeyFromRouteIndexLocked(k unresolvedEntryKey) {
	keys := t.byRoute[k.route]
	for i := range keys {
		if keys[i] != k {
			continue
		}
		last := len(keys) - 1
		keys[i] = keys[last]
		keys = keys[:last]
		if len(keys) == 0 {
			delete(t.byRoute, k.route)
			return
		}
		t.byRoute[k.route] = keys
		return
	}
}

func (t *unresolvedNextHopTracker) syncTotalLocked() {
	t.overflow.Store(int64(t.overflowTotal))
	t.total.Store(int64(len(t.entries) + t.overflowTotal))
}

func hashMixByte(h uint64, b byte) uint64 {
	return (h ^ uint64(b)) * fnvPrime64
}

func hashMixUint64(h, v uint64) uint64 {
	for range 8 {
		h = hashMixByte(h, byte(v))
		v >>= 8
	}
	return h
}

func hashMixString(h uint64, s string) uint64 {
	for i := range len(s) {
		h = hashMixByte(h, s[i])
	}
	return hashMixByte(h, 0xff)
}

func hashUnresolvedEntryKey(k unresolvedEntryKey) uint64 {
	h := fnvOffset64
	h = hashMixUint64(h, uint64(k.route.fam.AFI)<<8|uint64(k.route.fam.SAFI))
	if k.route.prefix.IsValid() {
		addr := k.route.prefix.Addr()
		if addr.Is4() {
			for _, octet := range addr.As4() {
				h = hashMixByte(h, octet)
			}
		} else {
			for _, octet := range addr.As16() {
				h = hashMixByte(h, octet)
			}
		}
		if bits := k.route.prefix.Bits(); bits >= 0 && bits <= 128 {
			h = hashMixByte(h, byte(bits))
		}
	}
	h = hashMixString(h, k.route.opaque)
	h = hashMixUint64(h, k.owner)
	h = hashMixString(h, k.peerAddr)
	h = hashMixUint64(h, uint64(k.pathID))
	if k.addPath {
		h = hashMixByte(h, 1)
	} else {
		h = hashMixByte(h, 0)
	}
	// SplitMix64 finalizer spreads adjacent prefixes across the 4096-bit index.
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	h ^= h >> 31
	return h
}

func containsExcludedKey(excluded []unresolvedCandidate, route unresolvedRouteKey, k unresolvedEntryKey) bool {
	if k.route != route {
		return false
	}
	for _, ex := range excluded {
		if ex.owner != k.owner {
			continue
		}
		if ex.peerAddr != k.peerAddr {
			continue
		}
		if ex.pathID != k.pathID {
			continue
		}
		if ex.addPath != k.addPath {
			continue
		}
		return true
	}
	return false
}

func entryMatchesOwner(k unresolvedEntryKey, peer bestPeer) bool {
	if peer.owner != 0 {
		return k.owner == peer.owner
	}
	if k.owner != 0 {
		return false
	}
	return k.peerAddr == peer.address
}

func peerMatchesOwner(candidate, target bestPeer) bool {
	if target.owner != 0 {
		return candidate.owner == target.owner
	}
	if candidate.owner != 0 {
		return false
	}
	return candidate.address == target.address
}

func compareUnresolvedSamples(a, b unresolvedSample) int {
	if c := cmp.Compare(a.displayPrefix, b.displayPrefix); c != 0 {
		return c
	}
	if c := a.nextHop.Compare(b.nextHop); c != 0 {
		return c
	}
	if c := cmp.Compare(a.peerAddr, b.peerAddr); c != 0 {
		return c
	}
	if c := cmp.Compare(a.owner, b.owner); c != 0 {
		return c
	}
	return cmp.Compare(a.pathID, b.pathID)
}

// unresolvedNextHopDoctorDeclaration declares the runtime doctor check that
// reports BGP routes currently excluded from Phase 2 by an unresolved next hop.
func unresolvedNextHopDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name:  unresolvedNextHopDoctorName,
		Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{codeDoctorBGPNextHopUnresolvable},
	}
}

// nextHopMetricsDoctorDeclaration declares the runtime doctor check that
// reports an active engine route-metrics lookup failure in forked placement.
func nextHopMetricsDoctorDeclaration() rpc.DoctorCheckDecl {
	return rpc.DoctorCheckDecl{
		Name:  nextHopMetricsDoctorName,
		Phase: rpc.DoctorPhaseRuntime,
		Codes: []string{codeDoctorBGPNextHopMetrics},
	}
}

// doctorCheck dispatches Stage 1 declared runtime doctor checks for this RIB.
// Safe for concurrent use; checks read in-memory state and make no engine RPCs.
func (r *RIBManager) doctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	switch name {
	case advertiseDoctorName:
		return r.advertiseDoctorCheck(name)
	case unresolvedNextHopDoctorName:
		return r.unresolvedNextHopDoctorCheck(), nil
	case nextHopMetricsDoctorName:
		return r.nextHopMetricsDoctorCheck(), nil
	default:
		return nil, opererr.New("The BGP RIB has no doctor check named "+strconv.Quote(name),
			"The requested check is not declared by this BGP RIB.", "ze show doctor")
	}
}

// unresolvedNextHopDoctorCheck reports BGP routes currently excluded from
// Phase 2 best-path selection by an unresolved next hop.
func (r *RIBManager) unresolvedNextHopDoctorCheck() []rpc.DoctorCheckDiagnostic {
	samples, overflow := r.unresolvedNextHops.snapshot()
	total := len(samples) + overflow
	if total == 0 {
		return nil
	}

	slices.SortFunc(samples, compareUnresolvedSamples)

	var tb textbuf.Buffer
	tb.Int(int64(total))
	if total == 1 {
		tb.Str(" BGP route is excluded from best-path selection because its next hop is unresolvable")
	} else {
		tb.Str(" BGP routes are excluded from best-path selection because their next hops are unresolvable")
	}
	sampleCount := min(len(samples), maxUnresolvedDoctorSamples)
	if sampleCount > 0 {
		if overflow > 0 {
			tb.Str(" (tracking cap ").Int(maxTrackedUnresolvedNextHops).Str(" reached)")
		}
		tb.Str(": ")
		for i := range sampleCount {
			if i > 0 {
				tb.Str(", ")
			}
			tb.Str(samples[i].displayPrefix).Str(" via ").Str(samples[i].nextHop.String())
		}
		remaining := total - sampleCount
		if remaining > 0 {
			tb.Str(" (+").Int(int64(remaining)).Str(" more)")
		}
		tb.Byte('.')
	} else {
		tb.Str(" (beyond the ").Int(maxTrackedUnresolvedNextHops).Str("-entry sample cap).")
	}

	return []rpc.DoctorCheckDiagnostic{{
		Code:     codeDoctorBGPNextHopUnresolvable,
		Severity: doctorSeverityWarning,
		Message:  tb.String(),
		Fix:      unresolvedNextHopDoctorFix,
	}}
}

// nextHopMetricsDoctorCheck reports an active engine route-metrics failure in
// forked placement so show doctor reads live state directly.
func (r *RIBManager) nextHopMetricsDoctorCheck() []rpc.DoctorCheckDiagnostic {
	if !r.forkRIB.metricFailureActive() {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     codeDoctorBGPNextHopMetrics,
		Severity: doctorSeverityWarning,
		Message:  remoteMetricProblem,
		Fix:      remoteMetricFix,
	}}
}
