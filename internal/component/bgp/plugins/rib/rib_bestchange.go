// Design: docs/architecture/plugin/rib-storage-design.md -- best-path change tracking
// RFC: rfc/short/rfc4271.md -- best-path decision process (S9.1.2)
// RFC: rfc/short/rfc9252.md -- SRv6 SID extraction and transposition
// RFC: rfc/short/rfc9494.md -- LLGR stale depreference
// Overview: rib.go -- RIB plugin core types and event handlers
// Related: bestpath.go -- best-path selection algorithm (RFC 4271 S9.1.2)
// Related: rib_structured.go -- structured event handlers that trigger best-path checks
//
// Real-time best-path tracking and EventBus publishing.
// After each INSERT/REMOVE in handleReceivedStructured, the affected prefix is
// checked for best-path changes. Changes are collected into a batch under the
// RIB lock, then emitted on the EventBus after lock release.
package rib

import (
	"bytes"
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// bestChangeEntry is an alias for the exported event payload entry type so
// the per-prefix functions in this file keep their current signatures while
// still producing the exported payload shape. See ribevents.BestChangeEntry.
type bestChangeEntry = ribevents.BestChangeEntry

// bestChangeBatch is an alias for the exported event payload. The producer
// path builds one batch per (protocol, family) combination, then emits via
// the typed BestChange handle.
type bestChangeBatch = ribevents.BestChangeBatch

// Packed bestPathRecord layout: four 16-bit fields in a single uint64.
// High bits first so pack/unpack is a single shift-and-OR. Three of the
// four fields are indices into per-attribute reverse tables on the shared
// bestPrevInterner -- resolve() dereferences them only on the cold
// emission path. The fourth field is a Flags word whose bit 0 encodes
// eBGP vs iBGP; the rest is reserved. Zero GC-traceable pointers are
// stored per entry, so a BART fringe holding 1M of these is opaque to
// the GC mark phase (the primary motivation; see spec-rib-bestpath-pack.md).
const (
	shiftMetricIdx  = 48
	shiftPeerIdx    = 32
	shiftNextHopIdx = 16
	flagEBGP        = 0x0001
	flagHadSRv6SID  = 0x0002
	// flagBlackhole records that the previous best was stamped as an RFC 7999
	// discard route. It is in the packed record for the same reason
	// flagHadSRv6SID is: the same-best short circuit below compares the packed
	// state, and a prefix that turns into a discard with its peer, next-hop and
	// MED unchanged would otherwise be suppressed before the event-bus entry is
	// built. The Loc-RIB rail is safe without it (Path.Equal reads the route
	// type), and the event-bus rail is not.
	flagBlackhole = 0x0004
	// internerCap is the exclusive upper bound for any single interner
	// reverse table. uint16 cardinality is architecturally unreachable
	// (~2k peers at the largest Internet IXP); the cap exists only so
	// a mis-deployment degrades gracefully rather than corrupting indices.
	internerCap = 1 << 16 // 65536
)

// bestPathRecord stores the previous best-path state for change detection.
// The prefix is not stored -- it is the key of the owning bestPrevStore entry
// and is formatted lazily from that key on the emission path. Neither the
// peer address, next-hop, nor metric are stored directly: they are interned
// on the shared bestPrevInterner (held by RIBManager) and the three uint16
// indices are packed into this 8-byte value. The hot-path same-best check
// is a single uint64 equality comparison; the cold emission path calls
// resolve() to materialize the full bestChangeEntry from the reverse tables.
type bestPathRecord uint64

// packBestPath assembles a bestPathRecord from three interner indices plus a
// Flags word. Pure arithmetic; safe on any uint16 input.
func packBestPath(metricIdx, peerIdx, nextHopIdx, flags uint16) bestPathRecord {
	return bestPathRecord(uint64(metricIdx)<<shiftMetricIdx |
		uint64(peerIdx)<<shiftPeerIdx |
		uint64(nextHopIdx)<<shiftNextHopIdx |
		uint64(flags))
}

// metricIdx returns the interner index for this record's MED value.
func (r bestPathRecord) metricIdx() uint16 { return uint16(r >> shiftMetricIdx) }

// peerIdx returns the interner index for this record's peer address.
func (r bestPathRecord) peerIdx() uint16 { return uint16(r >> shiftPeerIdx) }

// nextHopIdx returns the interner index for this record's next-hop.
func (r bestPathRecord) nextHopIdx() uint16 { return uint16(r >> shiftNextHopIdx) }

// Flags returns the 16-bit flag field. Bit 0 = isEBGP, bit 1 = had an SRv6 SID,
// bit 2 = was stamped as an RFC 7999 discard. Bits 3-15 are reserved.
func (r bestPathRecord) Flags() uint16 { return uint16(r) }

// IsEBGP reports whether the recorded best-path was learned from an eBGP peer.
func (r bestPathRecord) IsEBGP() bool { return r&flagEBGP != 0 }

// isBlackhole reports whether the recorded best-path was stamped as an RFC 7999
// discard route. Unexported: the only reader is the same-best short circuit in
// this file.
func (r bestPathRecord) isBlackhole() bool { return r&flagBlackhole != 0 }

// bestPrevInterner maps per-attribute values (peer address, next-hop, MED)
// to dense uint16 indices shared across all families on a RIBManager. The
// forward map dedupes on insert; the reverse slice restores the original
// value at emission time. Realistic BGP deployments use <10^4 unique values
// per attribute (the largest IXP carries ~2k peers) -- uint16 gives >30x
// headroom, and the cap is defensive only.
//
// The three `*Overflowed` booleans are one-shot latches: the first time a
// given table saturates, the interner logs an slog.Error and flips the
// latch; subsequent saturated lookups return (0, false) silently. This
// avoids the per-UPDATE log flood a saturated deployment would otherwise
// produce while still surfacing the event once.
//
// Concurrency: safe for concurrent use. Each reverse table (peers, nextHops,
// metrics) is guarded by its own sync.RWMutex. Read paths (dedup-hit lookup,
// reverse lookup) take RLock; mutation paths (first sighting of a value)
// promote to Lock. The three locks are independent so unrelated tables do
// not serialize against each other.
type bestPrevInterner struct {
	peersMu         sync.RWMutex
	peers           []string
	peerIdx         map[string]uint16
	peersOverflowed bool
	// peersFree holds reverse-table indices freed by forgetPeer so a later
	// internPeer can reuse the slot without growing the reverse table.
	// Prevents unbounded peers[] growth across the cap under long
	// deployments with high peer churn (ISP-scale route-servers may see
	// thousands of distinct peer addresses over the life of a process).
	peersFree          []uint16
	nextHopsMu         sync.RWMutex
	nextHops           []netip.Addr
	nextHopIdx         map[netip.Addr]uint16
	nextHopsOverflowed bool
	metricsMu          sync.RWMutex
	metrics            []uint32
	metricIdx          map[uint32]uint16
	metricsOverflowed  bool
}

// newBestPrevInterner constructs an empty interner with modest initial
// capacity. The maps grow with unique values; the reverse slices share the
// same growth cadence.
func newBestPrevInterner() *bestPrevInterner {
	return &bestPrevInterner{
		peerIdx:    make(map[string]uint16),
		nextHopIdx: make(map[netip.Addr]uint16),
		metricIdx:  make(map[uint32]uint16),
	}
}

// peerIdxOf returns the uint16 index for v without mutating the reverse
// table. Returns (0, false) when v was never interned. Unlike internPeer,
// this never grows the table -- used by bestPrev purge paths that want to
// look up a peer that is about to depart without polluting the interner
// with a slot for a peer that will have no records.
func (b *bestPrevInterner) peerIdxOf(v string) (uint16, bool) {
	b.peersMu.RLock()
	idx, ok := b.peerIdx[v]
	b.peersMu.RUnlock()
	return idx, ok
}

// internPeer returns the uint16 index for v. On a first sighting, v is
// stored in a reclaimed slot when peersFree has one available, otherwise
// appended to the reverse table with a fresh index. Returns (0, false)
// only when the reverse table is saturated at 65536 entries AND the
// free-list is empty -- the caller must treat that as a degraded record
// and not store it. The first saturation logs an slog.Error; subsequent
// ones are silent.
func (b *bestPrevInterner) internPeer(v string) (uint16, bool) {
	b.peersMu.RLock()
	idx, ok := b.peerIdx[v]
	b.peersMu.RUnlock()
	if ok {
		return idx, true
	}
	b.peersMu.Lock()
	defer b.peersMu.Unlock()
	if idx, ok := b.peerIdx[v]; ok {
		return idx, true
	}
	if n := len(b.peersFree); n > 0 {
		idx = b.peersFree[n-1]
		// Defensive: peers[] never shrinks in any code path today, so
		// every free-list entry remains in bounds. The guard is here
		// so a future refactor that adds shrinking/compaction cannot
		// silently turn a reclaimed slot into an out-of-bounds write.
		if int(idx) < len(b.peers) {
			b.peersFree = b.peersFree[:n-1]
			b.peers[idx] = v
			b.peerIdx[v] = idx
			return idx, true
		}
		// Stale free-list entry (impossible today): drop it and fall
		// through to the normal append path rather than writing out of
		// bounds. Leaves a "hole" in accounting but is safe.
		b.peersFree = b.peersFree[:n-1]
	}
	if len(b.peers) >= internerCap {
		if !b.peersOverflowed {
			b.peersOverflowed = true
			logger().Error("best-path interner saturated", "table", "peers", "cap", internerCap)
		}
		return 0, false
	}
	idx = uint16(len(b.peers))
	b.peers = append(b.peers, v)
	b.peerIdx[v] = idx
	return idx, true
}

// forgetPeer releases v's reverse-table slot so a future internPeer can
// reuse it. Idempotent: called unconditionally at the end of
// purgeBestPrevForPeer whether or not any bestPrev records referenced
// the slot. A peer that was interned but never appeared in a best-path
// (connected, sent OPEN, went down without contributing a winning
// route) is still reclaimed here.
//
// Edge case: if an in-flight UPDATE Phase 3 for v completes after this
// forgetPeer call, it re-interns v (likely back into the same slot,
// because the slot just hit the free-list). Phase 3 then writes a new
// bestPrev record that will be the only record referencing that slot.
// A later forgetPeer(v) will reclaim it again. The only way two peers
// can end up sharing a reclaimed slot is if v's slot is popped by a
// different peer's internPeer between v's forgetPeer and v's Phase 3
// re-intern, which requires N back-to-back peer flaps colliding with
// Phase 3 pipelining -- rare, and at worst produces a spurious "no
// change" suppression on one prefix that self-corrects on the next
// UPDATE. Reference-counting would eliminate the window at the cost of
// per-insert/delete refcount maintenance; that is an intentional
// deferral (see handoff: rib-sharding, Option D a).
func (b *bestPrevInterner) forgetPeer(v string) {
	b.peersMu.Lock()
	defer b.peersMu.Unlock()
	idx, ok := b.peerIdx[v]
	if !ok {
		return
	}
	delete(b.peerIdx, v)
	// Defensive: idx was assigned by internPeer and peers[] never shrinks
	// today, so the write is always in bounds. Guard future refactors.
	if int(idx) < len(b.peers) {
		b.peers[idx] = ""
	}
	b.peersFree = append(b.peersFree, idx)
}

// internNextHop returns the uint16 index for v; see internPeer for contract.
// The zero netip.Addr (invalid / absent next-hop) is interned like any other
// value so resolve() round-trips it back to nextHopString("").
func (b *bestPrevInterner) internNextHop(v netip.Addr) (uint16, bool) {
	b.nextHopsMu.RLock()
	idx, ok := b.nextHopIdx[v]
	b.nextHopsMu.RUnlock()
	if ok {
		return idx, true
	}
	b.nextHopsMu.Lock()
	defer b.nextHopsMu.Unlock()
	if idx, ok := b.nextHopIdx[v]; ok {
		return idx, true
	}
	if len(b.nextHops) >= internerCap {
		if !b.nextHopsOverflowed {
			b.nextHopsOverflowed = true
			logger().Error("best-path interner saturated", "table", "nexthops", "cap", internerCap)
		}
		return 0, false
	}
	idx = uint16(len(b.nextHops))
	b.nextHops = append(b.nextHops, v)
	b.nextHopIdx[v] = idx
	return idx, true
}

// internMetric returns the uint16 index for v; see internPeer for contract.
func (b *bestPrevInterner) internMetric(v uint32) (uint16, bool) {
	b.metricsMu.RLock()
	idx, ok := b.metricIdx[v]
	b.metricsMu.RUnlock()
	if ok {
		return idx, true
	}
	b.metricsMu.Lock()
	defer b.metricsMu.Unlock()
	if idx, ok := b.metricIdx[v]; ok {
		return idx, true
	}
	if len(b.metrics) >= internerCap {
		if !b.metricsOverflowed {
			b.metricsOverflowed = true
			logger().Error("best-path interner saturated", "table", "metrics", "cap", internerCap)
		}
		return 0, false
	}
	idx = uint16(len(b.metrics))
	b.metrics = append(b.metrics, v)
	b.metricIdx[v] = idx
	return idx, true
}

// peerAt returns the original peer string for idx, or "" if idx is past the
// reverse-table bounds. A bounds-safe wrapper so emission and steady-state
// comparison do not panic if an index from an older interner lifetime (or a
// manually-constructed record in tests) outlives its backing table.
func (b *bestPrevInterner) peerAt(idx uint16) string {
	b.peersMu.RLock()
	defer b.peersMu.RUnlock()
	if int(idx) >= len(b.peers) {
		return ""
	}
	return b.peers[idx]
}

// nextHopAt returns the original netip.Addr for idx, or the zero Addr if idx
// is past the reverse-table bounds. See peerAt for rationale.
func (b *bestPrevInterner) nextHopAt(idx uint16) netip.Addr {
	b.nextHopsMu.RLock()
	defer b.nextHopsMu.RUnlock()
	if int(idx) >= len(b.nextHops) {
		return netip.Addr{}
	}
	return b.nextHops[idx]
}

// metricAt returns the original uint32 for idx, or 0 if idx is past the
// reverse-table bounds. See peerAt for rationale.
func (b *bestPrevInterner) metricAt(idx uint16) uint32 {
	b.metricsMu.RLock()
	defer b.metricsMu.RUnlock()
	if int(idx) >= len(b.metrics) {
		return 0
	}
	return b.metrics[idx]
}

// internerSize returns the current size of the named reverse table under its
// own read lock. Used by updateMetrics.
func (b *bestPrevInterner) internerSize() (peers, nextHops, metrics int) {
	b.peersMu.RLock()
	peers = len(b.peers)
	b.peersMu.RUnlock()
	b.nextHopsMu.RLock()
	nextHops = len(b.nextHops)
	b.nextHopsMu.RUnlock()
	b.metricsMu.RLock()
	metrics = len(b.metrics)
	b.metricsMu.RUnlock()
	return
}

// resolve materializes a bestChangeEntry from a packed record plus an action
// label and display prefix. The emitted payload priority (20 eBGP / 200 iBGP)
// and protocol-type ("ebgp"/"ibgp") derive from the packed Flags bit 0, so
// the single source of truth for protocol class is the stored record rather
// than a derivable pair of fields. The reverse tables are self-locked
// for reading (the reverse tables are mutated on insert).
//
// Reverse-table lookups go through the bounds-safe accessors, so a record
// whose indices outlive a reset interner emits zero-valued NextHop/Metric
// rather than panicking.
func (r bestPathRecord) resolve(interner *bestPrevInterner, action routeaction.Action, prefix netip.Prefix, pathID uint32, addPath bool) bestChangeEntry {
	priority := 200
	protoType := routeaction.ProtocolIBGP
	if r.IsEBGP() {
		priority = 20
		protoType = routeaction.ProtocolEBGP
	}
	return bestChangeEntry{
		Action:       action,
		Prefix:       prefix,
		AddPath:      addPath,
		PathID:       pathID,
		NextHop:      interner.nextHopAt(r.nextHopIdx()),
		Priority:     priority,
		Metric:       interner.metricAt(r.metricIdx()),
		ProtocolType: protoType,
	}
}

// bestPrevStore holds the previously-recorded best path per route for one
// family. It picks its backend from the family exactly as FamilyRIB does, so
// the best-prev record and the Adj-RIB-In route it tracks are keyed the same
// way:
//
//	                 | !addPath          | addPath
//	-----------------+-------------------+-----------------------
//	CIDR family      | direct (BART)     | multi (BART, bestPrevSet)
//	non-CIDR family  | opaque (map)      | opaque (map; path-id
//	                 |                   | baked into the wire key)
//
// A CIDR family carries both BART backends so it can host peers with mixed
// ADD-PATH capability without key collision -- pathID=0 from a non-AP peer
// must not be conflated with a real AP-advertised pathID=0 from a different
// peer. Under AP the per-prefix value is a small path-id list (bestPrevSet),
// matching the pathSet pattern used by FamilyRIB.
//
// A non-CIDR family (VPN, EVPN, MVPN, MUP, flowspec, VPLS, BGP-LS) has no
// netip.Prefix to key on: octet 0 of its NLRI is a total bit length counting
// a label stack and a Route Distinguisher, or a route type, so
// store.NLRIToPrefix rejects it. Those families key on the full wire bytes,
// which are unique per route and already carry the ADD-PATH path-id, so no
// bestPrevSet layer is needed. Before this backend existed, every such route
// failed to key and no best-path change was ever recorded or published for
// it (plan/journal/silent-fall-through.md).
//
// The record holds no pointer, so the map's string keys are the only
// GC-traceable memory here. That is why a CIDR family keeps BART rather than
// sharing the map: the packed bestPathRecord exists to keep a million-prefix
// fringe opaque to the GC mark phase.
type bestPrevStore struct {
	cidr   bool
	direct *store.Store[bestPathRecord] // cidr && non-AP: one record per prefix
	multi  *store.Store[bestPrevSet]    // cidr && AP: per-prefix path-id -> record map
	opaque map[string]bestPathRecord    // !cidr: one record per wire NLRI
}

// bestPrevSet holds the per-path-id bestPathRecord list for one prefix under
// ADD-PATH. Typically 1-4 entries; a linear scan beats a hash map at that
// size and keeps the BART fringe-node memory shape tight.
type bestPrevSet struct {
	entries []bestPrevEntry
}

type bestPrevEntry struct {
	pathID uint32
	rec    bestPathRecord
}

func (s *bestPrevSet) lookup(pathID uint32) (bestPathRecord, bool) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			return s.entries[i].rec, true
		}
	}
	return 0, false
}

func (s *bestPrevSet) upsert(pathID uint32, rec bestPathRecord) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			s.entries[i].rec = rec
			return
		}
	}
	s.entries = append(s.entries, bestPrevEntry{pathID: pathID, rec: rec})
}

func (s *bestPrevSet) remove(pathID uint32) bool {
	for i := range s.entries {
		if s.entries[i].pathID != pathID {
			continue
		}
		last := len(s.entries) - 1
		s.entries[i] = s.entries[last]
		s.entries = s.entries[:last]
		return true
	}
	return false
}

// newBestPrevStore creates a bestPrevStore for a family. For a CIDR family
// both BART backends are allocated eagerly so mixed-mode sessions route each
// call to the correct key space without collision. The empty backend pays
// only a small idle cost (one empty BART root) regardless of which keys the
// family ends up using -- accepted trade-off for correctness on mixed
// sessions. A non-CIDR family allocates the opaque map instead and leaves
// both BART pointers nil; every reader below branches on cidr first.
func newBestPrevStore(fam family.Family) *bestPrevStore {
	if !storage.IsCIDRFamily(fam) {
		return &bestPrevStore{opaque: make(map[string]bestPathRecord)}
	}
	return &bestPrevStore{
		cidr:   true,
		direct: store.NewStore[bestPathRecord](fam),
		multi:  store.NewStore[bestPrevSet](fam),
	}
}

// parsePrevKey splits wire NLRI bytes under the given addPath flag into
// (pathID, prefix). Returns ok=false when bytes are malformed.
func parsePrevKey(fam family.Family, nlriBytes []byte, addPath bool) (uint32, netip.Prefix, bool) {
	if addPath {
		if len(nlriBytes) < 4 {
			return 0, netip.Prefix{}, false
		}
		pathID := uint32(nlriBytes[0])<<24 |
			uint32(nlriBytes[1])<<16 |
			uint32(nlriBytes[2])<<8 |
			uint32(nlriBytes[3])
		pfx, ok := store.NLRIToPrefix(fam, nlriBytes[4:])
		return pathID, pfx, ok
	}
	pfx, ok := store.NLRIToPrefix(fam, nlriBytes)
	return 0, pfx, ok
}

// lookup returns the previously-recorded best path for (nlriBytes, addPath).
func (s *bestPrevStore) lookup(fam family.Family, nlriBytes []byte, addPath bool) (bestPathRecord, bool) {
	if !s.cidr {
		rec, ok := s.opaque[string(nlriBytes)]
		return rec, ok
	}
	pathID, pfx, ok := parsePrevKey(fam, nlriBytes, addPath)
	if !ok {
		return 0, false
	}
	if !addPath {
		return s.direct.Lookup(pfx)
	}
	ps, exists := s.multi.Lookup(pfx)
	if !exists {
		return 0, false
	}
	return ps.lookup(pathID)
}

// insert stores rec for (nlriBytes, addPath). Overwrites any previous record
// at the same key.
func (s *bestPrevStore) insert(fam family.Family, nlriBytes []byte, addPath bool, rec bestPathRecord) {
	if !s.cidr {
		s.opaque[string(nlriBytes)] = rec
		return
	}
	pathID, pfx, ok := parsePrevKey(fam, nlriBytes, addPath)
	if !ok {
		return
	}
	if !addPath {
		s.direct.Insert(pfx, rec)
		return
	}
	ps, _ := s.multi.Lookup(pfx)
	ps.upsert(pathID, rec)
	s.multi.Insert(pfx, ps)
}

// delete removes the record at (nlriBytes, addPath). Returns true when a
// record existed.
func (s *bestPrevStore) delete(fam family.Family, nlriBytes []byte, addPath bool) bool {
	if !s.cidr {
		key := string(nlriBytes)
		if _, exists := s.opaque[key]; !exists {
			return false
		}
		delete(s.opaque, key)
		return true
	}
	pathID, pfx, ok := parsePrevKey(fam, nlriBytes, addPath)
	if !ok {
		return false
	}
	if !addPath {
		return s.direct.Delete(pfx)
	}
	ps, exists := s.multi.Lookup(pfx)
	if !exists {
		return false
	}
	if !ps.remove(pathID) {
		return false
	}
	if len(ps.entries) == 0 {
		s.multi.Delete(pfx)
	} else {
		s.multi.Insert(pfx, ps)
	}
	return true
}

// purgeBestPrevForPeer walks every bestPrev shard across every family and
// drops records whose PeerIdx matches peerAddr. For each purged record
// the matching locrib entry is removed via r.locRIB.Remove so
// cross-protocol consumers see the withdrawal immediately, not on a
// delayed next-UPDATE-for-the-prefix trigger.
//
// Returns per-family batches of bestChangeEntry Withdraws. The CALLER
// MUST emit them on the EventBus AFTER releasing r.peerMu (via
// emitPurgedWithdraws). Emitting under the outer write lock would
// serialize every in-process subscriber that touches peer-keyed state
// behind that lock, risking deadlock against any subscriber that
// re-enters RIBManager methods.
//
// Caller MUST hold r.peerMu.Lock so no concurrent UPDATE processing for
// the departing peer can re-insert records while purge is walking. Purge
// does NOT acquire r.peerMu itself.
//
// Lock order: r.peerMu (outer, caller) -> bgp-rib shard.mu -> locrib
// shard.mu (via r.locRIB.Remove). Matches checkBestPathChange's ordering
// after the 2026-04-20 fix that moved bestCandidateNextHopAddr outside
// sh.mu so sh.mu never sits above r.peerMu.
//
// Safe with r.locRIB == nil (skips the mirror step).
//
// Cost: one shard.mu.Lock per (family, shard) pair, held across each
// shard's direct + multi Iterate. For a 1M-prefix table this is O(1M)
// serial reads across all shards -- call site expects a cold-path
// peer-down event, not the hot UPDATE path.
func (r *RIBManager) purgeBestPrevForPeer(peerAddr string) map[family.Family][]bestChangeEntry {
	peerIdx, ok := r.bestPathInterner.peerIdxOf(peerAddr)
	if !ok {
		// Peer was never interned, so no bestPrev record can reference it.
		return nil
	}
	// Reclaim the interner slot on the way out so peers[] stays bounded
	// by concurrent-peer count, not by total-peers-ever-seen. Runs even
	// when bestPrev is nil or no records reference the slot.
	defer r.bestPathInterner.forgetPeer(peerAddr)
	if r.bestPrev == nil {
		return nil
	}
	var pending map[family.Family][]bestChangeEntry
	for _, fam := range r.bestPrev.familyList() {
		fs := r.bestPrev.familyShards(fam, false)
		if fs == nil {
			continue
		}
		var changes []bestChangeEntry
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.Lock()
			if !sh.store.cidr {
				// Non-CIDR family: one map keyed by the wire NLRI. Deleting
				// during the range is defined in Go, so no victim list is
				// needed. No locrib.Remove: the mirror never ran for these
				// families (see mirrorToLocRIB).
				for key, rec := range sh.store.opaque {
					if rec.peerIdx() != peerIdx {
						continue
					}
					delete(sh.store.opaque, key)
					changes = append(changes, bestChangeEntry{
						Action: ribevents.BestChangeWithdraw,
						NLRI:   []byte(key),
					})
				}
				sh.mu.Unlock()
				continue
			}
			// direct: collect prefixes to delete, then delete after Iterate.
			var directVictims []netip.Prefix
			sh.store.direct.Iterate(func(pfx netip.Prefix, rec bestPathRecord) bool {
				if rec.peerIdx() == peerIdx {
					directVictims = append(directVictims, pfx)
				}
				return true
			})
			for _, pfx := range directVictims {
				sh.store.direct.Delete(pfx)
				// Direct entries are non-ADD-PATH by storage construction;
				// AddPath and PathID stay at their zero values.
				changes = append(changes, bestChangeEntry{
					Action: ribevents.BestChangeWithdraw,
					Prefix: pfx,
				})
				if r.locRIB != nil {
					r.locRIB.Remove(fam, pfx, bgpProtocolID, 0)
				}
			}

			// multi: collect (prefix, pathIDs) to remove.
			type multiVictim struct {
				prefix  netip.Prefix
				pathIDs []uint32
			}
			var multiVictims []multiVictim
			sh.store.multi.Iterate(func(pfx netip.Prefix, ps bestPrevSet) bool {
				var pathIDs []uint32
				for _, e := range ps.entries {
					if e.rec.peerIdx() == peerIdx {
						pathIDs = append(pathIDs, e.pathID)
					}
				}
				if len(pathIDs) > 0 {
					multiVictims = append(multiVictims, multiVictim{prefix: pfx, pathIDs: pathIDs})
				}
				return true
			})
			for _, mv := range multiVictims {
				sh.store.multi.Modify(mv.prefix, func(ps *bestPrevSet) {
					for _, pid := range mv.pathIDs {
						ps.remove(pid)
					}
				})
				// If every entry was the departing peer's, drop the prefix entry
				// from the multi store entirely; otherwise the modified pathSet
				// was already written back by Modify.
				if ps, exists := sh.store.multi.Lookup(mv.prefix); exists && len(ps.entries) == 0 {
					sh.store.multi.Delete(mv.prefix)
				}
				for _, pid := range mv.pathIDs {
					changes = append(changes, bestChangeEntry{
						Action:  ribevents.BestChangeWithdraw,
						Prefix:  mv.prefix,
						AddPath: true,
						PathID:  pid,
					})
					if r.locRIB != nil {
						r.locRIB.Remove(fam, mv.prefix, bgpProtocolID, pid)
					}
				}
			}
			sh.mu.Unlock()
		}
		if len(changes) > 0 {
			if pending == nil {
				pending = make(map[family.Family][]bestChangeEntry)
			}
			pending[fam] = changes
		}
	}
	return pending
}

// emitPurgedWithdraws publishes the per-family Withdraw batches returned
// by purgeBestPrevForPeer. MUST be called AFTER r.peerMu is released so
// in-process EventBus subscribers that re-enter RIBManager methods do
// not deadlock against the outer write lock.
func (r *RIBManager) emitPurgedWithdraws(pending map[family.Family][]bestChangeEntry) {
	for fam, changes := range pending {
		publishBestChanges(changes, fam)
	}
}

// parseNextHopAddr converts raw NEXT_HOP attribute bytes into a netip.Addr.
// Returns the zero Addr (IsValid()==false) on malformed input. Zero-alloc:
// netip.AddrFrom4 and AddrFrom16 are pure value constructors.
func parseNextHopAddr(data []byte) netip.Addr {
	switch len(data) {
	case 4:
		var a [4]byte
		copy(a[:], data)
		return netip.AddrFrom4(a)
	case 16:
		var a [16]byte
		copy(a[:], data)
		return netip.AddrFrom16(a)
	}
	return netip.Addr{}
}

// checkBestPathChange evaluates the best path for a prefix after an insert or remove.
// Compares with the previous best and returns a change entry if the best path changed.
// addPath indicates whether nlriBytes includes a 4-byte path-ID prefix.
// forward is an optional ForwardHandle for the source UPDATE wire bytes;
// propagated to locrib.InsertForward on the Insert branch so Change
// subscribers can forward the buffer without rebuilding. Pass nil when
// no handle is available. Remove-induced withdrawals bypass forward
// -- r.locRIB.Remove takes no handle because Remove carries no source
// buffer by design (see design-rib-rs-fastpath.md).
// Safe to call with no outer lock held. gatherCandidates and
// bestCandidateNextHopAddr take r.peerMu.RLock internally for their brief
// map reads; bestPrev has its own per-shard locks; bestPathInterner has
// its own per-table mutexes. Lock order: r.peerMu -> shard.mu.
//
// Returns (entry, true) when a change occurred; (zero, false) when unchanged,
// the NLRI is malformed, or an interner table is saturated. On saturation,
// the interner logs an slog.Error once (see bestPrevInterner), and the stored
// `prev` record is left in place: consumers continue to see the pre-saturation
// best path for that prefix rather than a spurious withdraw. Once saturated,
// the interner has no mechanism to recover within a process lifetime; a
// restart is required.
//
// Hot-path shape:
//  1. If there is a previous record, unpack its reverse-table entries and
//     compare against the winner's raw values. A match short-circuits with
//     no interner mutation and no prefix allocation.
//  2. Otherwise compute the display prefix (malformed NLRI bails without
//     mutation).
//  3. Intern the winner's fields, pack, store, and emit.
func (r *RIBManager) checkBestPathChange(fam family.Family, nlriBytes []byte, addPath bool, forward locrib.ForwardHandle) (bestChangeEntry, bool) {
	candidates := r.gatherCandidates(fam, nlriBytes)
	// SelectMultipath returns the same primary winner as SelectBest plus any
	// equal-cost siblings (rib-arch-4). When multipath is off (maximum-paths<=1,
	// the default) it returns nil siblings with no extra work, so the single-best
	// path is unchanged.
	newBest, siblings := SelectMultipath(candidates, r.maximumPaths.Load(), r.relaxASPath.Load())

	// Resolve the nextHop and protocol class for the winner BEFORE we take
	// the shard lock. bestCandidateNextHopAddr acquires r.peerMu.RLock
	// internally; holding sh.mu across that call would put us on the
	// wrong side of a peerMu writer (e.g. purgeBestPrevForPeer running
	// under peerMu.Lock) and deadlock against it. Lock order contract:
	// r.peerMu -> shard.mu, never shard.mu -> r.peerMu.
	var (
		nextHop      netip.Addr
		isEBGP       bool
		bestLabels   []uint32
		srv6SID      netip.Addr
		ecmpNextHops []netip.Addr
	)
	if newBest != nil {
		nextHop = r.bestCandidateNextHopAddr(fam, nlriBytes, newBest)
		isEBGP = r.protocolType(newBest) == routeaction.ProtocolEBGP
		if fam.SAFI == family.SAFIMPLSLabel {
			bestLabels = r.lookupLabelsForBest(fam, nlriBytes, newBest.PeerIP)
		}
		if fam.SAFI != family.SAFIMPLSLabel {
			srv6SID = r.lookupSRv6SIDForBest(fam, nlriBytes, addPath, newBest.PeerIP)
		}
		// Resolve the equal-cost multipath sibling next-hops so the Loc-RIB
		// carries the full ECMP set to the FIB (rib-arch-4). Each sibling
		// resolves via the same accessor as the primary; dedup against the
		// primary and each other. Resolved before the shard lock (the accessor
		// takes r.peerMu.RLock), preserving the r.peerMu -> shard.mu lock order.
		for _, s := range siblings {
			nh := r.bestCandidateNextHopAddr(fam, nlriBytes, s)
			if nh.IsValid() && nh != nextHop && !slices.Contains(ecmpNextHops, nh) {
				ecmpNextHops = append(ecmpNextHops, nh)
			}
		}
	}

	// Key the record. A CIDR family parses its NLRI into a (path-id, prefix)
	// pair and routes to the shard owning that prefix; malformed bytes bail
	// before touching any shard, regardless of newBest state, because there is
	// no way to key the stored record without a prefix.
	//
	// A non-CIDR family (VPN, EVPN, MVPN, MUP, flowspec, VPLS, BGP-LS) has no
	// prefix at all: its NLRI leads with a total bit length counting a label
	// stack and a Route Distinguisher, or with a route type. parsePrevKey
	// rejects every one of them, so keying through it published NO best-path
	// change for any of those families. The wire bytes are the key instead,
	// which is what the Adj-RIB-In already stores the route under.
	//
	// RFC 7911: for a non-CIDR family the path-id leads the wire bytes and
	// stays part of the key, so one record per (NLRI, path-id) pair falls out
	// of the bytes alone. Such an entry therefore leaves AddPath and PathID
	// zero and lets NLRI carry the whole identity: a second copy of the
	// path-id is a field the purge and replay paths would have to keep in
	// step, and neither can derive it from the store.
	cidr := storage.IsCIDRFamily(fam)
	var (
		pathID uint32
		pfx    netip.Prefix
	)
	if cidr {
		var prefixOK bool
		pathID, pfx, prefixOK = parsePrevKey(fam, nlriBytes, addPath)
		if !prefixOK {
			return bestChangeEntry{}, false
		}
	}

	// RFC 7999 Section 3.3: does this winner become a discard route? Resolved
	// here, before the shard lock, for the same reason the next-hop is: the
	// lookup takes r.peerMu.RLock, and the lock order is r.peerMu -> shard.mu.
	// Zero for every peer that stated no rule, which is every peer by default.
	//
	// Asked for CIDR families only. Section 3.3 authorizes a BLACKHOLE by the
	// covering IP prefix the operator configured, and a non-CIDR route names no
	// prefix to cover, so the question has no answer there rather than the
	// answer "not a discard".
	var blackholeType routetype.Type
	if newBest != nil && cidr {
		blackholeType = r.blackholeRouteTypeForBest(fam, nlriBytes, pfx, newBest.PeerIP)
	}

	// Skip family creation if there is nothing to record AND no previous
	// state could exist for this family.
	fs := r.bestPrev.familyShards(fam, false)
	if fs == nil && newBest == nil {
		return bestChangeEntry{}, false
	}
	if fs == nil {
		fs = r.bestPrev.familyShards(fam, true)
	}
	sh := fs.shardForNLRI(nlriBytes)
	if cidr {
		sh = fs.shardFor(pfx)
	}

	sh.mu.Lock()
	defer sh.mu.Unlock()

	prev, havePrev := sh.store.lookup(fam, nlriBytes, addPath)

	if newBest == nil {
		// No candidates remain -- withdraw if we had a previous best.
		if !havePrev {
			return bestChangeEntry{}, false
		}
		sh.store.delete(fam, nlriBytes, addPath)
		// The Loc-RIB is prefix-keyed and feeds the kernel FIB, so it takes
		// CIDR families only. See mirrorToLocRIB below for why.
		if r.locRIB != nil && cidr {
			r.locRIB.Remove(fam, pfx, bgpProtocolID, pathID)
		}
		if !cidr {
			return bestChangeEntry{
				Action: ribevents.BestChangeWithdraw,
				NLRI:   entryNLRI(cidr, nlriBytes),
			}, true
		}
		return bestChangeEntry{
			Action:  ribevents.BestChangeWithdraw,
			Prefix:  pfx,
			AddPath: addPath,
			PathID:  pathID,
		}, true
	}

	// Same-best short-circuit: compare raw winner values against the
	// previous record's unpacked reverse-table entries. Three slice
	// lookups + three value compares; no interner mutation; no prefix
	// allocation. If the bounds-safe accessors report a miss (stale
	// index from a reset interner), the comparison falls through and
	// the record is re-interned below.
	// Skip same-best suppression for labeled routes: the label stack is not
	// part of the interned prev record, so a relabel (same peer/next-hop/metric,
	// new label) would be wrongly suppressed and never reach the kernel. The cost
	// is a redundant event-bus entry when a labeled best is unchanged, but the
	// authoritative Loc-RIB path (the default consumer) still dedups via
	// Path.Equal, so there is no FIB churn.
	// mirrorToLocRIB writes the winning best path (plus its equal-cost multipath
	// set) into the shared Loc-RIB. Called on BOTH the same-best short-circuit and
	// the full best-change path so an ECMP-membership change is never lost when
	// the best next-hop itself is unchanged (the same-best test below compares the
	// best, not the sibling set); the Loc-RIB dedups a true no-op via Path.Equal.
	mirrorToLocRIB := func() {
		if r.locRIB == nil {
			return
		}
		// NOT MIRRORED for a non-CIDR family, and this is a deliberate limit
		// rather than an oversight. The Loc-RIB is keyed by netip.Prefix all
		// the way down (locrib.shardFor, its BART store, and sysrib's
		// prefixKey), and it exists to arbitrate what the kernel FIB installs.
		// A VPN or EVPN route has no such key: two VPN routes that differ only
		// in Route Distinguisher share one IP prefix and would overwrite each
		// other, and ze has no VRF plumbing to install them into anyway. The
		// event-bus rail carries them instead, identified by entry.NLRI.
		// Making the Loc-RIB carry them is a storage-shape change of its own.
		if !cidr {
			return
		}
		// AdminDistance is the classical Cisco/Juniper default (eBGP=20, iBGP=200)
		// unless the operator overrode it under rib/distance; Metric carries MED.
		// The DECLARATION decides, not this plugin's constant. locrib.selectBest
		// ranks paths on what is stamped here and runs before sysrib sees the
		// route, so the operator's `rib { distance { } }` has to reach this line
		// or it cannot change cross-protocol selection at all. The atomics are
		// the bootstrap value, reachable only before the first configure.
		proto, fallback := "ibgp", uint8(r.adminDistanceIBGP.Load()) //nolint:gosec // YANG 1..255
		if isEBGP {
			proto, fallback = "ebgp", uint8(r.adminDistanceEBGP.Load()) //nolint:gosec // YANG 1..255
		}
		distance := ribdistance.OrDefault(proto, fallback)
		r.locRIB.InsertForward(fam, pfx, locrib.Path{
			Source:        bgpProtocolID,
			Instance:      pathID,
			NextHop:       nextHop,
			AdminDistance: distance,
			// Carry the eBGP/iBGP class explicitly so the sysrib replay path
			// classifies the protocol type without re-deriving it from the
			// (operator-overridable) AdminDistance above.
			IsEBGP: isEBGP,
			Metric: newBest.MED,
			// Carry the label stack into the Loc-RIB so labeled-unicast routes
			// reach the kernel as MPLS push entries. sysrib prefers the Loc-RIB
			// path, so without this the labels are dropped and a plain IP route
			// is installed.
			Labels: bestLabels,
			// Carry the equal-cost multipath sibling next-hops so the Loc-RIB
			// emits Change.ECMP for a BGP multipath best (rib-arch-4); sysrib
			// expands it into an ECMP FIB entry. Nil when multipath is off.
			ECMP: ecmpNextHops,
			// Carry the RFC 7999 forwarding action. sysrib prefers the Loc-RIB
			// path, so without this a honored blackhole reaches the kernel as an
			// ordinary route on the default deployment. Zero unless the winning
			// peer agreed to honor BLACKHOLE and is authorized for a covering
			// prefix.
			RouteType: blackholeType,
		}, forward)
	}

	if havePrev && len(bestLabels) == 0 {
		ir := r.bestPathInterner
		if ir.peerAt(prev.peerIdx()) == newBest.PeerAddr &&
			ir.nextHopAt(prev.nextHopIdx()) == nextHop &&
			ir.metricAt(prev.metricIdx()) == newBest.MED &&
			prev.IsEBGP() == isEBGP &&
			prev.isBlackhole() == (blackholeType == routetype.Blackhole) &&
			!srv6SID.IsValid() && prev.Flags()&flagHadSRv6SID == 0 {
			// The best is unchanged, but the equal-cost multipath membership may
			// have changed (a sibling appeared or went away with the best next-hop
			// stable). Refresh the Loc-RIB Path so its Change.ECMP tracks the
			// current set; the Loc-RIB dedups a true no-op. Skip the expensive
			// re-intern + event-bus entry -- the best route itself did not change.
			mirrorToLocRIB()
			return bestChangeEntry{}, false
		}
	}

	peerIdx, ok := r.bestPathInterner.internPeer(newBest.PeerAddr)
	if !ok {
		return bestChangeEntry{}, false
	}
	nhIdx, ok := r.bestPathInterner.internNextHop(nextHop)
	if !ok {
		return bestChangeEntry{}, false
	}
	metricIdx, ok := r.bestPathInterner.internMetric(newBest.MED)
	if !ok {
		return bestChangeEntry{}, false
	}
	var flags uint16
	if isEBGP {
		flags |= flagEBGP
	}
	if srv6SID.IsValid() {
		flags |= flagHadSRv6SID
	}
	if blackholeType == routetype.Blackhole {
		flags |= flagBlackhole
	}
	newRec := packBestPath(metricIdx, peerIdx, nhIdx, flags)

	sh.store.insert(fam, nlriBytes, addPath, newRec)
	// Mirror the best path (and its equal-cost multipath set) into the shared
	// Loc-RIB via the same closure the same-best short-circuit uses.
	mirrorToLocRIB()
	action := ribevents.BestChangeAdd
	if havePrev {
		action = ribevents.BestChangeUpdate
	}
	// pfx, pathID and the ADD-PATH flag are all zero for a non-CIDR family:
	// the NLRI carries the whole identity there (see the keying comment above).
	entry := newRec.resolve(r.bestPathInterner, action, pfx, pathID, addPath && cidr)
	entry.NLRI = entryNLRI(cidr, nlriBytes)
	entry.Labels = bestLabels
	entry.SRv6SID = srv6SID
	// RFC 7999 Section 3.3. Zero for every route that is not a honored
	// blackhole, which leaves the FIB installing an ordinary route.
	entry.RouteType = blackholeType
	// Attach the winner's AS_PATH and origin AS for downstream consumers
	// (e.g. flow-export BGP enrichment). Cold path: once per best-path change,
	// not per packet. The AS_PATH bytes are already interned in the pool;
	// formatASPath turns them into a flat ASN slice.
	if newBest.ASPathHandle.IsValid() {
		if data, err := pool.ASPath.Get(newBest.ASPathHandle); err == nil {
			if asPath := formatASPath(data); len(asPath) > 0 {
				entry.ASPath = asPath
				entry.OriginAS = asPath[len(asPath)-1]
			}
		}
	}
	return entry, true
}

// entryNLRI returns the wire bytes a published bestChangeEntry names its route
// by, which is nothing for a CIDR family (Prefix names it) and an owned copy
// for every other one.
//
// The copy is not optional and it is not free. nlriBytes points into the
// WireUpdate buffer the reactor reuses, while the entry outlives this call:
// subscribers retain the batch and the bus marshals it lazily. Called only at
// the sites that emit an entry, never on the same-best path, which is most
// UPDATEs.
func entryNLRI(cidr bool, nlriBytes []byte) []byte {
	if cidr {
		return nil
	}
	return bytes.Clone(nlriBytes)
}

// lookupLabelsForBest retrieves MPLS labels from the winning peer's PeerRIB
// for a labeled unicast prefix. Caller must not hold r.peerMu.
func (r *RIBManager) lookupLabelsForBest(fam family.Family, nlriBytes []byte, peerAddr netip.Addr) []uint32 {
	r.peerMu.RLock()
	peerRIB := r.bgpPeers[peerAddr]
	r.peerMu.RUnlock()
	if peerRIB == nil {
		return nil
	}
	h := peerRIB.LookupLabels(fam, nlriBytes)
	return pool.ResolveLabels(h)
}

// lookupSRv6SIDForBest extracts the SRv6 SID from the PrefixSID attribute
// (code 40) stored in OtherAttrs of the winning peer's route entry, and
// reconstructs it when the sender transposed part of it into a label field.
// Returns an invalid Addr when the attribute is absent, is not SRv6, or
// names a transposition ze cannot undo -- reporting no SID rather than a
// partial one, because the partial one is not what the peer signaled.
// Caller must not hold r.peerMu.
func (r *RIBManager) lookupSRv6SIDForBest(fam family.Family, nlriBytes []byte, addPath bool, peerAddr netip.Addr) netip.Addr {
	r.peerMu.RLock()
	peerRIB := r.bgpPeers[peerAddr]
	r.peerMu.RUnlock()
	if peerRIB == nil {
		return netip.Addr{}
	}
	entry, ok := peerRIB.Lookup(fam, nlriBytes)
	if !ok {
		return netip.Addr{}
	}
	b := entry.GetBundle()
	if !b.HasOtherAttrs() {
		return netip.Addr{}
	}
	return srv6SIDFromResult(fam, nlriBytes, addPath, extractSRv6SIDResultFromOtherAttrs(b))
}

// srv6SIDFromResult reconstructs the SRv6 Service SID an UPDATE signaled.
//
// RFC 9252 Section 3.2.1 lets a sender take Transposition Length bits out of
// the SID starting at Transposition Offset and carry them in a label field:
// "The bits that have been shifted out MUST be set to 0 in the SID value."
// The SID in the attribute is therefore incomplete on its own, and the label
// field holds the rest. Reading only the attribute installs a SID with zeros
// where the Function part belongs, which is a different SID from the one the
// peer advertised.
//
// It answers an invalid address in three cases, and the route keeps its next
// hop rather than gaining a SID ze cannot vouch for: the attribute carried no
// SID; the transposition is wider than the label field that must carry it, so
// Section 7 says the SID value "is invalid"; or the family's label field is one
// ze cannot read, which nlrisplit.TranspositionLabel names.
//
// Marking such a path INELIGIBLE for best-path selection, which Section 7 also
// requires, is not done here. isSRv6Ineligible owns that question.
func srv6SIDFromResult(fam family.Family, nlriBytes []byte, addPath bool, result pool.SRv6SIDResult) netip.Addr {
	if !result.SID.IsValid() {
		return netip.Addr{}
	}
	if !result.HasTranspos {
		return result.SID
	}
	if result.TransposLen > labelWidthForSAFI(fam.SAFI) {
		return netip.Addr{}
	}
	label, ok := nlrisplit.TranspositionLabel(fam, nlriBytes, addPath)
	if !ok {
		return netip.Addr{}
	}
	return pool.ApplyTransposition(result.SID, label, result.TransposOffset, result.TransposLen, labelWidthForSAFI(fam.SAFI))
}

// labelWidthForSAFI returns the width in bits of the label field that carries
// transposed SRv6 SID bits for safi.
//
// RFC 9252 Sections 5.1 and 5.2 give the VPN families an RFC 8277 field with
// "the 20-bit Label Value set to the whole or a portion of the Function part
// of the SRv6 SID", and bound the transposition: "the Transposition Length
// MUST be less than or equal to 20". Sections 6.1.2, 6.2 and 6.5 give EVPN a
// three-octet field where "the value is set in the 24 bits", bounded at 24.
// Every other family has no such field, so nothing may be transposed into
// one; Section 7 requires the offset and length to be 0 there, and the
// 20 returned makes any non-zero length above it invalid.
func labelWidthForSAFI(safi family.SAFI) uint8 {
	if safi == family.SAFIEVPN {
		return 24
	}
	return 20
}

// isSRv6Ineligible reports whether a route entry is ineligible for best-path
// per RFC 9252 Section 5: a route with PrefixSID containing SRv6 Service TLVs
// (type 5 or 6) but no extractable valid SID MUST be excluded from best-path.
// Returns false (eligible) when no SRv6 TLVs are present or SID extraction succeeds.
func isSRv6Ineligible(entry storage.RouteEntry) bool {
	b := entry.GetBundle()
	if !b.HasOtherAttrs() {
		return false
	}
	data, err := pool.OtherAttrs.Get(b.OtherAttrs)
	if err != nil {
		return false
	}
	var hasSRv6TLV bool
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4
		if off+length > len(data) {
			break
		}
		if typeCode == 40 {
			if prefixSIDHasSRv6TLVs(data[off : off+length]) {
				hasSRv6TLV = true
				if sid := pool.ExtractSRv6SID(data[off : off+length]); sid.IsValid() {
					return false // Valid SID found, eligible.
				}
			}
			break
		}
		off += length
	}
	return hasSRv6TLV
}

// prefixSIDHasSRv6TLVs checks if PrefixSID attribute value contains any
// SRv6 Service TLVs (type 5 = L3 Service, type 6 = L2 Service).
func prefixSIDHasSRv6TLVs(prefixSIDValue []byte) bool {
	off := 0
	for off+3 <= len(prefixSIDValue) {
		tlvType := prefixSIDValue[off]
		tlvLen := int(prefixSIDValue[off+1])<<8 | int(prefixSIDValue[off+2])
		off += 3
		if off+tlvLen > len(prefixSIDValue) {
			break
		}
		if tlvType == 5 || tlvType == 6 {
			return true
		}
		off += tlvLen
	}
	return false
}

// extractSRv6SIDResultFromOtherAttrs finds PrefixSID (code 40) in OtherAttrs and
// extracts the SRv6 SID with transposition parameters.
// OtherAttrs format: [type(1)][flags(1)][length(2)][value(n)]...
func extractSRv6SIDResultFromOtherAttrs(b storage.Bundle) pool.SRv6SIDResult {
	data, err := pool.OtherAttrs.Get(b.OtherAttrs)
	if err != nil {
		return pool.SRv6SIDResult{}
	}
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4
		if off+length > len(data) {
			break
		}
		if typeCode == 40 {
			return pool.ExtractSRv6SIDFull(data[off : off+length])
		}
		off += length
	}
	return pool.SRv6SIDResult{}
}

// protocolType returns the protocol-type label for a candidate based on
// ASN comparison. When LocalASN is 0 (unknown, e.g. before OPEN negotiation
// completes), defaults to ebgp. This is intentional: routes learned before
// ASN negotiation are assumed external, which is the more common case.
func (r *RIBManager) protocolType(c *Candidate) routeaction.ProtocolType {
	if c.LocalASN == 0 || c.PeerASN != c.LocalASN {
		return routeaction.ProtocolEBGP
	}
	return routeaction.ProtocolIBGP
}

// bestCandidateNextHopAddr extracts the next-hop for the winning candidate's
// route entry as a netip.Addr. Returns the zero Addr when missing. This is
// the zero-alloc equivalent of the former string-returning helper: the hot
// comparison in checkBestPathChange is a value compare against the stored
// bestPathRecord.NextHop, with string materialization deferred until the
// emission path.
// For IPv4, reads from the NEXT_HOP attribute (code 3).
// For IPv6 and other MP families, extracts from MP_REACH_NLRI (code 14) in OtherAttrs.
// Acquires r.peerMu.RLock internally for the brief bgpPeers read; PeerRIB
// content reads (peerRIB.Lookup) use PeerRIB's own lock. Safe to call
// without any outer lock held.
func (r *RIBManager) bestCandidateNextHopAddr(fam family.Family, nlriBytes []byte, best *Candidate) netip.Addr {
	r.peerMu.RLock()
	peerRIB := r.bgpPeers[best.PeerIP]
	r.peerMu.RUnlock()
	if peerRIB == nil {
		return netip.Addr{}
	}
	entry, ok := peerRIB.Lookup(fam, nlriBytes)
	if !ok {
		return netip.Addr{}
	}
	return entryNextHopAddr(entry)
}

// entryNextHopAddr reads the next-hop a stored route entry advertises. Returns
// the zero Addr when the entry carries none.
//
// This is the ONE producer of that answer, so the winner's installed next hop
// (bestCandidateNextHopAddr) and the Section 5.1.3 eligibility test
// (gatherCandidatesLocked, rib_commands.go) cannot disagree about which address
// a route names. Reads pool handles only; no lock, no allocation.
func entryNextHopAddr(entry storage.RouteEntry) netip.Addr {
	// Try IPv4 NEXT_HOP attribute (code 3) first.
	b := entry.GetBundle()
	if b.HasNextHop() {
		data, err := pool.NextHop.Get(b.NextHop)
		if err == nil {
			if a := parseNextHopAddr(data); a.IsValid() {
				return a
			}
		}
	}

	// For IPv6/multiprotocol: extract next-hop from MP_REACH_NLRI (code 14) in OtherAttrs.
	// MP_REACH wire format: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + reserved(1) + NLRIs.
	if b.HasOtherAttrs() {
		return extractMPNextHopAddr(b)
	}

	return netip.Addr{}
}

// extractMPNextHopAddr extracts the next-hop from MP_REACH_NLRI stored in
// OtherAttrs as a netip.Addr. Returns zero Addr on missing / malformed input.
// OtherAttrs format: [type(1)][flags(1)][length_16bit(2)][value(n)]...
// MP_REACH value: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + ...
// The SAFI in that value selects the next-hop encoding; see mpNextHopAddr.
func extractMPNextHopAddr(b storage.Bundle) netip.Addr {
	data, err := pool.OtherAttrs.Get(b.OtherAttrs)
	if err != nil {
		return netip.Addr{}
	}

	// Walk OtherAttrs to find attribute type code 14 (MP_REACH_NLRI).
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4

		if off+length > len(data) {
			break
		}

		if typeCode == 14 { // MP_REACH_NLRI
			value := data[off : off+length]
			// AFI(2) + SAFI(1) + NH_len(1) = 4 bytes minimum.
			if len(value) < 4 {
				return netip.Addr{}
			}
			nhLen := int(value[3])
			if len(value) < 4+nhLen {
				return netip.Addr{}
			}
			return mpNextHopAddr(family.SAFI(value[2]), value[4:4+nhLen])
		}

		off += length
	}
	return netip.Addr{}
}

// mpNextHopAddr reads the address out of an MP_REACH_NLRI Network Address of
// Next Hop field of nhLen octets.
//
// RFC 4760 Section 3 leaves the encoding to the family, and the VPN families
// prefix it with a Route Distinguisher: RFC 4364 Section 6.1 says a PE's own
// address "is encoded as a VPN-IPv4 address with an RD of 0", 12 octets, and
// RFC 4659 Section 3.2.1.1 says the VPN-IPv6 form is "24 when only a global
// address is present, and 48 if a link-local address is also included". Read
// as a bare address those lengths match nothing and the next hop comes back
// invalid, which is what published a VPN best path with no next hop.
//
// A trailing link-local address is dropped for the same reason the 32-octet
// IPv6 unicast form drops it: the global address is the one to forward to.
func mpNextHopAddr(safi family.SAFI, nhBytes []byte) netip.Addr {
	if safi == family.SAFIVPN {
		// RD(8) + address. Anything shorter names no address.
		if len(nhBytes) < 8 {
			return netip.Addr{}
		}
		nhBytes = nhBytes[8:]
		// VPN-IPv6 global + link-local: RD(8)+IPv6(16) twice. The second RD
		// starts where the global address ends.
		if len(nhBytes) == 40 {
			nhBytes = nhBytes[:16]
		}
		return parseNextHopAddr(nhBytes)
	}
	// RFC 2545 Section 3: IPv6 global followed by link-local.
	if len(nhBytes) == 32 {
		nhBytes = nhBytes[:16]
	}
	return parseNextHopAddr(nhBytes)
}

// replayBestPaths emits the entire current best-path table as one batch per
// family. Used when a downstream consumer (e.g. sysrib) sends
// (bgp-rib, replay-request). This hop is broadcast, so the request's token is
// ignored except to stamp it onto the batches (replay.Broadcast), which makes
// IsReplay() report true and distinguishes a replay batch from an incremental
// one. Caller MUST NOT hold r.peerMu.
func (r *RIBManager) replayBestPaths(req *replay.Request) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	for famName, changes := range r.collectBestPaths() {
		batch := &bestChangeBatch{
			Protocol: protocolNameBGP,
			Family:   famName,
			ReplayID: req.ReplayID,
			Changes:  changes,
		}
		if _, err := ribevents.BestChange.Emit(eb, batch); err != nil {
			logger().Warn("replay emit failed", "error", err)
		}
	}
}

// replayRedistribute answers a redistribution replay request with the entire
// current best-path table, through the redistribution bridge alone.
//
// The redistribute orchestrator fires one when a consumer registers. Such a
// consumer would otherwise hold nothing this speaker learned before it existed.
// Startup order decides whether that happens, and nothing orders the plugin
// tiers.
//
// It does NOT emit on (bgp-rib, best-change). That hop has its own request
// vocabulary and its own subscriber, sysrib. Answering one request on both hops
// would hand sysrib a table it did not ask for.
//
// Caller MUST NOT hold r.peerMu.
func (r *RIBManager) replayRedistribute(req *redistevents.ReplayRequest) {
	eb := getEventBus()
	if eb == nil || req == nil || req.ReplayID == 0 {
		return
	}
	for famName, changes := range r.collectBestPaths() {
		bgpredist.EmitBestChange(eb, &bestChangeBatch{
			Protocol: protocolNameBGP,
			Family:   famName,
			ReplayID: req.ReplayID,
			Changes:  changes,
		})
	}
}

// collectBestPaths walks the whole best-path table and returns one add entry
// per stored path, keyed by family. A family holding nothing is absent rather
// than present and empty, so a caller emits no batch for it.
//
// It is the shared half of the two replay answers above, which differ only in
// which hop they emit on. Caller MUST NOT hold r.peerMu.
func (r *RIBManager) collectBestPaths() map[family.Family][]bestChangeEntry {
	families := r.bestPrev.familyList()
	changesByFamily := make(map[family.Family][]bestChangeEntry, len(families))
	for _, fam := range families {
		fs := r.bestPrev.familyShards(fam, false)
		if fs == nil {
			continue
		}
		// Sum direct + AP counts under each shard's read lock so the batch
		// preallocation is sized correctly. Replay is a cold path fired on
		// late-subscriber replay-request; the per-shard read locks are held
		// briefly in series.
		total := 0
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.RLock()
			if sh.store.cidr {
				total += sh.store.direct.Len()
				sh.store.multi.Iterate(func(_ netip.Prefix, ps bestPrevSet) bool {
					total += len(ps.entries)
					return true
				})
			} else {
				total += len(sh.store.opaque)
			}
			sh.mu.RUnlock()
		}
		changes := make([]bestChangeEntry, 0, total)
		appendRec := func(rec bestPathRecord, pfx netip.Prefix, pathID uint32, addPath bool) {
			if !pfx.IsValid() {
				return
			}
			changes = append(changes, rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, pfx, pathID, addPath))
		}
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.RLock()
			if !sh.store.cidr {
				// Non-CIDR family: the wire bytes name the route, and the
				// prefix stays zero. appendRec is the CIDR path and refuses a
				// zero prefix, so replay builds these entries directly.
				for key, rec := range sh.store.opaque {
					e := rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, netip.Prefix{}, 0, false)
					e.NLRI = []byte(key)
					changes = append(changes, e)
				}
				sh.mu.RUnlock()
				continue
			}
			sh.store.direct.Iterate(func(pfx netip.Prefix, rec bestPathRecord) bool {
				appendRec(rec, pfx, 0, false)
				return true
			})
			sh.store.multi.Iterate(func(pfx netip.Prefix, ps bestPrevSet) bool {
				for i := range ps.entries {
					appendRec(ps.entries[i].rec, pfx, ps.entries[i].pathID, true)
				}
				return true
			})
			sh.mu.RUnlock()
		}
		if len(changes) > 0 {
			changesByFamily[fam] = changes
		}
	}

	logger().Info("best-path replay collected", "families", len(changesByFamily))
	return changesByFamily
}

// publishBestChanges emits a best-change batch on the EventBus under
// (bgp-rib, best-change) via the typed BestChange handle. Called AFTER the
// RIB lock is released. In-process subscribers receive *BestChangeBatch
// directly; external plugin processes receive the JSON marshaling that the
// bus produces lazily (only when at least one external subscriber exists).
// reconcileBestPath runs best-path selection for a single prefix after a
// command-driven mutation (inject, withdraw). Must be called AFTER releasing
// peerMu so the internal peerMu.RLock in gatherCandidates does not deadlock.
// addPath=false because inject/withdraw build NLRI with pathID=0 and no
// ADD-PATH prefix; if those commands gain --path-id, this must change.
func (r *RIBManager) reconcileBestPath(fam family.Family, nlriBytes []byte) {
	change, ok := r.checkBestPathChange(fam, nlriBytes, false, nil)
	if ok {
		publishBestChanges([]bestChangeEntry{change}, fam)
	}
}

// reconcileBestPathBulk runs purgeBestPrevForPeer + emitPurgedWithdraws for
// each peer in the list. Used by bulk command mutations (empty, release)
// that clear an entire peer's RIB. Must be called AFTER releasing peerMu.
// Locks peerMu per peer (not once for all) so concurrent UPDATE processing
// is not blocked for the full sweep; re-insertion between iterations is safe
// because purgeBestPrevForPeer is idempotent.
func (r *RIBManager) reconcileBestPathBulk(peers []netip.Addr) {
	for _, peer := range peers {
		r.peerMu.Lock()
		// The interner is keyed by the canonical address string.
		pending := r.purgeBestPrevForPeer(peer.String())
		r.peerMu.Unlock()
		r.emitPurgedWithdraws(pending)
	}
}

func publishBestChanges(changes []bestChangeEntry, fam family.Family) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	batch := &bestChangeBatch{
		Protocol: protocolNameBGP,
		Family:   fam,
		Changes:  changes,
	}
	if _, err := ribevents.BestChange.Emit(eb, batch); err != nil {
		logger().Warn("best-change emit failed", "error", err)
	}
	bgpredist.EmitBestChange(eb, batch)
}
