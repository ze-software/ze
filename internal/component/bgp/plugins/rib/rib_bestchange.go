// Design: docs/architecture/plugin/rib-storage-design.md -- best-path change tracking
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
	"net/netip"
	"sync"

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/store"
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

// MetricIdx returns the interner index for this record's MED value.
func (r bestPathRecord) MetricIdx() uint16 { return uint16(r >> shiftMetricIdx) }

// PeerIdx returns the interner index for this record's peer address.
func (r bestPathRecord) PeerIdx() uint16 { return uint16(r >> shiftPeerIdx) }

// NextHopIdx returns the interner index for this record's next-hop.
func (r bestPathRecord) NextHopIdx() uint16 { return uint16(r >> shiftNextHopIdx) }

// Flags returns the 16-bit flag field; bit 0 = isEBGP, bits 1-15 reserved.
func (r bestPathRecord) Flags() uint16 { return uint16(r) }

// IsEBGP reports whether the recorded best-path was learned from an eBGP peer.
func (r bestPathRecord) IsEBGP() bool { return r&flagEBGP != 0 }

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
func (r bestPathRecord) resolve(interner *bestPrevInterner, action bgptypes.RouteAction, prefix string, pathID uint32, addPath bool) bestChangeEntry {
	priority := 200
	protoType := bgptypes.BGPProtocolIBGP
	if r.IsEBGP() {
		priority = 20
		protoType = bgptypes.BGPProtocolEBGP
	}
	return bestChangeEntry{
		Action:       action,
		Prefix:       prefix,
		AddPath:      addPath,
		PathID:       pathID,
		NextHop:      nextHopString(interner.nextHopAt(r.NextHopIdx())),
		Priority:     priority,
		Metric:       interner.metricAt(r.MetricIdx()),
		ProtocolType: protoType,
	}
}

// bestPrevStore holds the previously-recorded best path per prefix for one
// family. It carries both a non-ADD-PATH BART-backed Store and an ADD-PATH
// BART-backed Store so a family can host peers with mixed ADD-PATH capability
// without key collision -- pathID=0 from a non-AP peer must not be conflated
// with a real AP-advertised pathID=0 from a different peer. Both backends are
// prefix-keyed (netip.Prefix); under AP the per-prefix value is a small
// path-id map (bestPrevSet), matching the pathSet pattern used by FamilyRIB.
type bestPrevStore struct {
	direct *store.Store[bestPathRecord] // non-AP: one record per prefix
	multi  *store.Store[bestPrevSet]    // AP: per-prefix path-id -> record map
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

// newBestPrevStore creates a bestPrevStore for a family. Both backends are
// allocated eagerly so mixed-mode sessions route each call to the correct
// key space without collision. The empty backend pays only a small idle cost
// (one empty BART root) regardless of which keys the family ends up using --
// accepted trade-off for correctness on mixed sessions.
func newBestPrevStore(fam family.Family) *bestPrevStore {
	return &bestPrevStore{
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
			// direct: collect prefixes to delete, then delete after Iterate.
			var directVictims []netip.Prefix
			sh.store.direct.Iterate(func(pfx netip.Prefix, rec bestPathRecord) bool {
				if rec.PeerIdx() == peerIdx {
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
					Prefix: pfx.String(),
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
					if e.rec.PeerIdx() == peerIdx {
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
						Prefix:  mv.prefix.String(),
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

// prefixBytesForDisplay returns the NLRI bytes suitable for wirePrefixToString.
// For ADD-PATH, strips the 4-byte path-ID prefix that nlrisplit includes.
func prefixBytesForDisplay(nlriBytes []byte, addPath bool) []byte {
	if addPath && len(nlriBytes) > 4 {
		return nlriBytes[4:]
	}
	return nlriBytes
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

// nextHopString produces the display form of a best-path next-hop for the
// BestChangeEntry JSON payload. Returns "" for the zero Addr (absent /
// malformed). IPv6 is emitted in canonical (RFC 5952) compressed form, which
// matches ze's other JSON paths in test/encode/*.ci and test/plugin/*.ci.
func nextHopString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
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
	newBest := SelectBest(candidates)

	// Resolve the nextHop and protocol class for the winner BEFORE we take
	// the shard lock. bestCandidateNextHopAddr acquires r.peerMu.RLock
	// internally; holding sh.mu across that call would put us on the
	// wrong side of a peerMu writer (e.g. purgeBestPrevForPeer running
	// under peerMu.Lock) and deadlock against it. Lock order contract:
	// r.peerMu -> shard.mu, never shard.mu -> r.peerMu.
	var (
		nextHop netip.Addr
		isEBGP  bool
	)
	if newBest != nil {
		nextHop = r.bestCandidateNextHopAddr(fam, nlriBytes, newBest)
		isEBGP = r.protocolType(newBest) == bgptypes.BGPProtocolEBGP
	}

	// Parse prefix once so we can route to the owning shard. Malformed
	// NLRI bails before touching any shard, regardless of newBest state
	// -- we have no way to key the stored record without a prefix.
	pathID, pfx, prefixOK := parsePrevKey(fam, nlriBytes, addPath)
	if !prefixOK {
		return bestChangeEntry{}, false
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
	sh := fs.shardFor(pfx)

	sh.mu.Lock()
	defer sh.mu.Unlock()

	prev, havePrev := sh.store.lookup(fam, nlriBytes, addPath)

	if newBest == nil {
		// No candidates remain -- withdraw if we had a previous best.
		if !havePrev {
			return bestChangeEntry{}, false
		}
		prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), fam.String())
		if prefix == "" {
			return bestChangeEntry{}, false
		}
		sh.store.delete(fam, nlriBytes, addPath)
		// Mirror the withdrawal into the shared Loc-RIB so non-BGP
		// consumers see one consistent view across protocols.
		if r.locRIB != nil {
			r.locRIB.Remove(fam, pfx, bgpProtocolID, pathID)
		}
		return bestChangeEntry{
			Action:  ribevents.BestChangeWithdraw,
			Prefix:  prefix,
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
	if havePrev {
		ir := r.bestPathInterner
		if ir.peerAt(prev.PeerIdx()) == newBest.PeerAddr &&
			ir.nextHopAt(prev.NextHopIdx()) == nextHop &&
			ir.metricAt(prev.MetricIdx()) == newBest.MED &&
			prev.IsEBGP() == isEBGP {
			return bestChangeEntry{}, false
		}
	}

	// The record has changed (or is brand new). Validate the display
	// prefix before any interner mutation so a malformed NLRI cannot
	// grow the reverse tables with unreferenced entries.
	prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), fam.String())
	if prefix == "" {
		return bestChangeEntry{}, false
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
	newRec := packBestPath(metricIdx, peerIdx, nhIdx, flags)

	sh.store.insert(fam, nlriBytes, addPath, newRec)
	// Mirror into the shared Loc-RIB. AdminDistance is the classical
	// Cisco/Juniper default (eBGP=20, iBGP=200) unless the operator
	// overrode it under bgp/admin-distance; Metric carries MED.
	if r.locRIB != nil {
		distance := uint8(r.adminDistanceIBGP.Load()) //nolint:gosec // YANG 1..255
		if isEBGP {
			distance = uint8(r.adminDistanceEBGP.Load()) //nolint:gosec // YANG 1..255
		}
		r.locRIB.InsertForward(fam, pfx, locrib.Path{
			Source:        bgpProtocolID,
			Instance:      pathID,
			NextHop:       nextHop,
			AdminDistance: distance,
			Metric:        newBest.MED,
		}, forward)
	}
	action := ribevents.BestChangeAdd
	if havePrev {
		action = ribevents.BestChangeUpdate
	}
	return newRec.resolve(r.bestPathInterner, action, prefix, pathID, addPath), true
}

// protocolType returns the protocol-type label for a candidate based on
// ASN comparison. When LocalASN is 0 (unknown, e.g. before OPEN negotiation
// completes), defaults to ebgp. This is intentional: routes learned before
// ASN negotiation are assumed external, which is the more common case.
func (r *RIBManager) protocolType(c *Candidate) bgptypes.BGPProtocolType {
	if c.LocalASN == 0 || c.PeerASN != c.LocalASN {
		return bgptypes.BGPProtocolEBGP
	}
	return bgptypes.BGPProtocolIBGP
}

// bestCandidateNextHopAddr extracts the next-hop for the winning candidate's
// route entry as a netip.Addr. Returns the zero Addr when missing. This is
// the zero-alloc equivalent of the former string-returning helper: the hot
// comparison in checkBestPathChange is a value compare against the stored
// bestPathRecord.NextHop, with string materialization deferred until the
// emission path.
// For IPv4, reads from the NEXT_HOP attribute (code 3).
// For IPv6 and other MP families, extracts from MP_REACH_NLRI (code 14) in OtherAttrs.
// Acquires r.peerMu.RLock internally for the brief ribInPool read; PeerRIB
// content reads (peerRIB.Lookup) use PeerRIB's own lock. Safe to call
// without any outer lock held.
func (r *RIBManager) bestCandidateNextHopAddr(fam family.Family, nlriBytes []byte, best *Candidate) netip.Addr {
	r.peerMu.RLock()
	peerRIB := r.ribInPool[best.PeerAddr]
	r.peerMu.RUnlock()
	if peerRIB == nil {
		return netip.Addr{}
	}
	entry, ok := peerRIB.Lookup(fam, nlriBytes)
	if !ok {
		return netip.Addr{}
	}

	// Try IPv4 NEXT_HOP attribute (code 3) first.
	if entry.HasNextHop() {
		data, err := pool.NextHop.Get(entry.NextHop)
		if err == nil {
			if a := parseNextHopAddr(data); a.IsValid() {
				return a
			}
		}
	}

	// For IPv6/multiprotocol: extract next-hop from MP_REACH_NLRI (code 14) in OtherAttrs.
	// MP_REACH wire format: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + reserved(1) + NLRIs.
	if entry.HasOtherAttrs() {
		return extractMPNextHopAddr(entry)
	}

	return netip.Addr{}
}

// extractMPNextHopAddr extracts the next-hop from MP_REACH_NLRI stored in
// OtherAttrs as a netip.Addr. Returns zero Addr on missing / malformed input.
// OtherAttrs format: [type(1)][flags(1)][length_16bit(2)][value(n)]...
// MP_REACH value: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + ...
func extractMPNextHopAddr(entry storage.RouteEntry) netip.Addr {
	data, err := pool.OtherAttrs.Get(entry.OtherAttrs)
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
			nhBytes := value[4 : 4+nhLen]
			// For 32-byte next-hop (IPv6 global + link-local), use the first 16 bytes.
			if nhLen == 32 {
				nhBytes = nhBytes[:16]
			}
			return parseNextHopAddr(nhBytes)
		}

		off += length
	}
	return netip.Addr{}
}

// replayBestPaths emits the entire current best-path table as one batch per
// family. Used when a downstream consumer (e.g. rib) sends
// (rib, replay-request). The Replay flag in the payload distinguishes a
// replay batch from a normal incremental change batch.
// Caller MUST NOT hold r.peerMu.
func (r *RIBManager) replayBestPaths() {
	eb := getEventBus()
	if eb == nil {
		return
	}

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
			total += sh.store.direct.Len()
			sh.store.multi.Iterate(func(_ netip.Prefix, ps bestPrevSet) bool {
				total += len(ps.entries)
				return true
			})
			sh.mu.RUnlock()
		}
		changes := make([]bestChangeEntry, 0, total)
		appendRec := func(rec bestPathRecord, prefix string, pathID uint32, addPath bool) {
			if prefix == "" {
				return
			}
			changes = append(changes, rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, prefix, pathID, addPath))
		}
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.RLock()
			sh.store.direct.Iterate(func(pfx netip.Prefix, rec bestPathRecord) bool {
				appendRec(rec, pfx.String(), 0, false)
				return true
			})
			sh.store.multi.Iterate(func(pfx netip.Prefix, ps bestPrevSet) bool {
				for i := range ps.entries {
					appendRec(ps.entries[i].rec, pfx.String(), ps.entries[i].pathID, true)
				}
				return true
			})
			sh.mu.RUnlock()
		}
		if len(changes) > 0 {
			changesByFamily[fam] = changes
		}
	}

	for famName, changes := range changesByFamily {
		batch := &bestChangeBatch{
			Protocol: "bgp",
			Family:   famName,
			Replay:   true,
			Changes:  changes,
		}
		if _, err := ribevents.BestChange.Emit(eb, batch); err != nil {
			logger().Warn("replay emit failed", "error", err)
		}
	}

	logger().Info("best-path replay published", "families", len(changesByFamily))
}

// publishBestChanges emits a best-change batch on the EventBus under
// (bgp-rib, best-change) via the typed BestChange handle. Called AFTER the
// RIB lock is released. In-process subscribers receive *BestChangeBatch
// directly; external plugin processes receive the JSON marshaling that the
// bus produces lazily (only when at least one external subscriber exists).
func publishBestChanges(changes []bestChangeEntry, fam family.Family) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	batch := &bestChangeBatch{
		Protocol: "bgp",
		Family:   fam,
		Changes:  changes,
	}
	if _, err := ribevents.BestChange.Emit(eb, batch); err != nil {
		logger().Warn("best-change emit failed", "error", err)
	}
}
