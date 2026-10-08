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
	"encoding/binary"
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
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
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

// bestPathMetrics keeps the received AIGP value distinct from MED and from the
// distance subsequently added to the next hop. It is interned, not copied into
// every prefix record, and is replayed unchanged to recursive resolvers.
type bestPathMetrics struct {
	AIGP    uint64
	MED     uint32
	HasAIGP bool
}

// packBestPath assembles a bestPathRecord from three interner indices plus a
// Flags word. Pure arithmetic; safe on any uint16 input.
func packBestPath(metricIdx, peerIdx, nextHopIdx, flags uint16) bestPathRecord {
	return bestPathRecord(uint64(metricIdx)<<shiftMetricIdx |
		uint64(peerIdx)<<shiftPeerIdx |
		uint64(nextHopIdx)<<shiftNextHopIdx |
		uint64(flags))
}

// metricIdx returns the interner index for the received MED/AIGP values.
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
// Concurrency: safe for concurrent use. Each reverse table has its own
// sync.RWMutex. A changed election reserves its peer slot before releasing
// peerMu, then transfers that reference to its stored bestPrev record. Purge
// scans also pin their peer slot. No slot can be reused while an admitted
// publication, stored record or scan still names it.
type bestPrevInterner struct {
	peersMu         sync.RWMutex
	peers           []string
	peerRefs        []uint64
	peerIdx         map[string]uint16
	peersOverflowed bool
	// peersFree holds indices whose last owner called releasePeer, so a later
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
	metrics            []bestPathMetrics
	metricIdx          map[bestPathMetrics]uint16
	metricsOverflowed  bool
}

// newBestPrevInterner constructs an empty interner with modest initial
// capacity. The maps grow with unique values; the reverse slices share the
// same growth cadence.
func newBestPrevInterner() *bestPrevInterner {
	return &bestPrevInterner{
		peerIdx:    make(map[string]uint16),
		nextHopIdx: make(map[netip.Addr]uint16),
		metricIdx:  make(map[bestPathMetrics]uint16),
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

// internPeer acquires one reference to v's peer slot. The caller MUST transfer
// it to a stored bestPrev record or call releasePeer. Returns (0, false) when
// the bounded table has no free slot; no reference is acquired in that case.
func (b *bestPrevInterner) internPeer(v string) (uint16, bool) {
	b.peersMu.Lock()
	defer b.peersMu.Unlock()
	if idx, ok := b.peerIdx[v]; ok {
		b.peerRefs[idx]++
		return idx, true
	}
	if n := len(b.peersFree); n > 0 {
		idx := b.peersFree[n-1]
		// Defensive: peers[] never shrinks in any code path today, so
		// every free-list entry remains in bounds. The guard is here
		// so a future refactor that adds shrinking/compaction cannot
		// silently turn a reclaimed slot into an out-of-bounds write.
		if int(idx) < len(b.peers) {
			b.peersFree = b.peersFree[:n-1]
			b.peers[idx] = v
			b.peerRefs[idx] = 1
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
	idx := uint16(len(b.peers))
	b.peers = append(b.peers, v)
	b.peerRefs = append(b.peerRefs, 1)
	b.peerIdx[v] = idx
	return idx, true
}

// retainPeer pins an existing peer slot without creating an absent one. A
// successful caller MUST call releasePeer after its scan or borrowed use ends.
func (b *bestPrevInterner) retainPeer(v string) (uint16, bool) {
	b.peersMu.Lock()
	defer b.peersMu.Unlock()
	idx, ok := b.peerIdx[v]
	if ok {
		b.peerRefs[idx]++
	}
	return idx, ok
}

// releasePeer releases exactly one reference acquired by internPeer or
// retainPeer. A record owner MUST resolve any needed peer value before releasing
// its reference. The last release makes the slot available for reuse.
func (b *bestPrevInterner) releasePeer(idx uint16) {
	b.peersMu.Lock()
	defer b.peersMu.Unlock()
	if int(idx) >= len(b.peerRefs) {
		panic("BUG: releasing an unknown best-path peer slot")
	}
	if b.peerRefs[idx] == 0 {
		panic("BUG: releasing an unowned best-path peer slot")
	}
	b.peerRefs[idx]--
	if b.peerRefs[idx] != 0 {
		return
	}
	delete(b.peerIdx, b.peers[idx])
	b.peers[idx] = ""
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
func (b *bestPrevInterner) internMetric(v bestPathMetrics) (uint16, bool) {
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

// metricAt returns the received metrics for idx, or their zero value if idx is
// past the reverse-table bounds. See peerAt for rationale.
func (b *bestPrevInterner) metricAt(idx uint16) bestPathMetrics {
	b.metricsMu.RLock()
	defer b.metricsMu.RUnlock()
	if int(idx) >= len(b.metrics) {
		return bestPathMetrics{}
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
	metrics := interner.metricAt(r.metricIdx())
	return bestChangeEntry{
		Action:       action,
		Prefix:       prefix,
		AddPath:      addPath,
		PathID:       pathID,
		NextHop:      interner.nextHopAt(r.nextHopIdx()),
		Priority:     priority,
		Metric:       metrics.MED,
		AIGP:         metrics.AIGP,
		AIGPPresent:  metrics.HasAIGP,
		ProtocolType: protoType,
	}
}

// bestPrevStore holds the previously-recorded best path per route for one
// family. It picks its backend from the family exactly as FamilyRIB does:
//
//	CIDR family      | direct (BART): one record per PREFIX
//	non-CIDR family  | opaque (map):  one record per wire NLRI
//
// A CIDR family keeps ONE record per prefix whatever the ADD-PATH mode of the
// sessions that carry it. RFC 7911 Section 2 makes the Path Identifier name a
// path of a prefix, and RFC 8277 Section 3.1 makes two paths of one ADD-PATH
// session comparable, so selection elects one best per prefix and the record
// says which path won through its pathID and addPath fields.
//
// A non-CIDR family (VPN, EVPN, MVPN, MUP, flowspec, VPLS, BGP-LS) has no
// netip.Prefix to key on: octet 0 of its NLRI is a total bit length counting
// a label stack and a Route Distinguisher, or a route type, so
// store.NLRIToPrefix rejects it. Those families key on the full wire bytes.
// Before this backend existed, every such route failed to key and no best-path
// change was ever recorded or published for it
// (plan/journal/silent-fall-through.md).
//
// The CIDR record holds no pointer, keeping the million-prefix fringe opaque
// to the GC mark phase. Labeled families alone retain an owned snapshot of the
// last published label values beside those records; pool handles can be reused
// after a withdrawal and therefore cannot identify the previous publication.
type bestPrevStore struct {
	cidr   bool
	direct *store.Store[bestPrevRecord] // cidr: one record per prefix
	opaque map[string]opaqueBestPrev    // !cidr: one record per route key (routeIdentity)
	labels map[netip.Prefix][]uint32    // CIDR labels only; bounded by direct's live records
}

// bestPrevRecord is the stored best path of one route: the packed winner and
// the path it was received as. pathID and addPath are the winner's own, read
// from the candidate that won, so a best-change names the path the winning
// session sent. Both are zero for a winner received without ADD-PATH.
type bestPrevRecord struct {
	rec     bestPathRecord
	pathID  uint32
	addPath bool
}

// opaqueBestPrev is the stored best of one non-CIDR route: the record, and
// the winner's wire NLRI without its path identifier. The route key drops the
// labels (routeIdentity), so the wire route is kept beside the record: a
// best-change published for the route names it as the winner's session sent
// it, labels included. Kept out of bestPrevRecord so the CIDR store's records
// still hold no pointer.
type opaqueBestPrev struct {
	prev  bestPrevRecord
	route string
}

// newBestPrevStore creates a bestPrevStore for a family: a BART store for a
// CIDR family, the opaque map for every other one. Every reader below branches
// on cidr first.
func newBestPrevStore(fam family.Family) *bestPrevStore {
	if !storage.IsCIDRFamily(fam) {
		return &bestPrevStore{opaque: make(map[string]opaqueBestPrev)}
	}
	return &bestPrevStore{
		cidr:   true,
		direct: store.NewStore[bestPrevRecord](fam),
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

// lookup returns the previously-recorded best path: by pfx for a CIDR family,
// by route key for every other one, with the winner's wire route (empty for
// a CIDR prefix).
func (s *bestPrevStore) lookup(pfx netip.Prefix, routeKey []byte) (bestPrevRecord, string, bool) {
	if !s.cidr {
		best, ok := s.opaque[string(routeKey)]
		return best.prev, best.route, ok
	}
	rec, ok := s.direct.Lookup(pfx)
	return rec, "", ok
}

// insert stores rec, and for a non-CIDR route the winner's wire route, under
// the same key lookup reads. Overwrites any previous record at that key.
func (s *bestPrevStore) insert(pfx netip.Prefix, routeKey []byte, rec bestPrevRecord, route string) {
	if !s.cidr {
		s.opaque[string(routeKey)] = opaqueBestPrev{prev: rec, route: route}
		return
	}
	s.direct.Insert(pfx, rec)
}

// delete removes the record under the same key lookup reads.
func (s *bestPrevStore) delete(pfx netip.Prefix, nlriBytes []byte) {
	if !s.cidr {
		delete(s.opaque, string(nlriBytes))
		return
	}
	delete(s.labels, pfx)
	s.direct.Delete(pfx)
}

// purgeBestPrevForPeer walks every bestPrev shard across every family and
// drops records whose PeerIdx matches peerAddr. It publishes nothing and
// leaves the Loc-RIB alone: whether a purged route is withdrawn or passes to
// a surviving path is only known after the re-election emitPurgedWithdraws
// runs, so the Loc-RIB and the consumers hear about each route once, with
// its outcome, rather than a withdrawal followed by the survivor.
//
// Returns per-family batches of Withdraws. The caller MUST remove or mutate the
// peer under peerMu first, then release peerMu BEFORE calling this method and
// emitPurgedWithdraws. Shard acquisition MUST NOT happen while holding peerMu:
// elections and synchronous Loc-RIB subscribers take shard.mu before peerMu.
//
// A replacement session can publish during this scan. Every removed record
// releases its own peer-slot reference; the scan pins the slot so an unrelated
// peer cannot reuse its index before the final shard. A replacement that lands
// after its shard was scanned keeps its record and slot. One scanned afterward
// is re-elected by emitPurgedWithdraws before any Loc-RIB removal.
//
// Cost: one shard.mu.Lock per (family, shard) pair, held across each
// shard's Iterate. For a 1M-prefix table this is O(1M)
// serial reads across all shards -- call site expects a cold-path
// peer-down event, not the hot UPDATE path.
func (r *RIBManager) purgeBestPrevForPeer(peerAddr string) map[family.Family][]bestChangeEntry {
	peerIdx, ok := r.bestPathInterner.retainPeer(peerAddr)
	if !ok {
		// No stored record or admitted publication can reference this peer:
		// changed elections reserve their slot before releasing peerMu.
		return nil
	}
	defer r.bestPathInterner.releasePeer(peerIdx)
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
				for key, best := range sh.store.opaque {
					rec := best.prev
					if rec.rec.peerIdx() != peerIdx {
						continue
					}
					delete(sh.store.opaque, key)
					r.bestPathInterner.releasePeer(peerIdx)
					changes = append(changes, bestChangeEntry{
						Action:  ribevents.BestChangeWithdraw,
						NLRI:    framedRouteNLRI(nil, best.route, rec.pathID, rec.addPath),
						AddPath: rec.addPath,
						PathID:  rec.pathID,
					})
				}
				sh.mu.Unlock()
				continue
			}
			// Collect prefixes to delete, then delete after Iterate.
			type victim struct {
				prefix netip.Prefix
				rec    bestPrevRecord
			}
			var victims []victim
			sh.store.direct.Iterate(func(pfx netip.Prefix, rec bestPrevRecord) bool {
				if rec.rec.peerIdx() == peerIdx {
					victims = append(victims, victim{prefix: pfx, rec: rec})
				}
				return true
			})
			for _, v := range victims {
				sh.store.delete(v.prefix, nil)
				r.bestPathInterner.releasePeer(peerIdx)
				changes = append(changes, bestChangeEntry{
					Action:  ribevents.BestChangeWithdraw,
					Prefix:  v.prefix,
					AddPath: v.rec.addPath,
					PathID:  v.rec.pathID,
				})
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

// emitPurgedWithdraws re-elects every route purgeBestPrevForPeer dropped and
// publishes what each route became. MUST be called AFTER r.peerMu is released
// so in-process EventBus subscribers that re-enter RIBManager methods do not
// deadlock against the outer write lock.
//
// The election runs BEFORE anything about the route is published. A route
// another peer, or another path of an ADD-PATH session, still holds passes
// from the departed best to its survivor in one Update: publishing the
// Withdraw first would tell every consumer, and the Loc-RIB the kernel FIB
// reads, that the route is gone for the instant before the survivor's Add.
// Only a route left with no candidate is withdrawn, and only it leaves the
// Loc-RIB (withdrawIfUnheld).
//
// Each route is published as soon as its election answers, one batch per
// route, the way an UPDATE publishes its own election. Holding the family's
// changes until its last election would delay the first route's change by the
// whole table, and a later UPDATE's change for that route could reach the
// consumers first. Cold path, one election per purged route after a
// peer-down.
func (r *RIBManager) emitPurgedWithdraws(pending map[family.Family][]bestChangeEntry) {
	r.reconcileFlowSpecs()
	var keyBuf [cidrKeyOctetsMax]byte
	for fam, purged := range pending {
		for i := range purged {
			withdrawn := purged[i]
			route := withdrawn.NLRI
			if withdrawn.AddPath && len(route) >= 4 {
				route = route[4:]
			}
			if withdrawn.Prefix.IsValid() {
				route = store.PrefixToNLRIInto(withdrawn.Prefix, keyBuf[:])
			}
			if len(route) == 0 {
				continue
			}
			change, changed := r.checkBestPathChange(fam, route, false, nil)
			if changed {
				// The purge dropped the departed best's record, so the election
				// reads the survivor as a first best. For every consumer it
				// replaces the route the departed peer held.
				if change.Action == ribevents.BestChangeAdd {
					change.Action = ribevents.BestChangeUpdate
				}
				publishBestChanges([]bestChangeEntry{change}, fam)
				continue
			}
			// No change can mean an UPDATE that ran between the purge and this
			// election already re-elected the route and published it.
			if !r.withdrawIfUnheld(fam, withdrawn.Prefix, route) {
				continue
			}
			publishBestChanges([]bestChangeEntry{withdrawn}, fam)
		}
	}
}

// withdrawIfUnheld removes the route from the Loc-RIB unless a best is
// recorded for it, and reports whether it did: pfx for a CIDR family,
// otherwise the route key of route, an NLRI without a path identifier. The
// Loc-RIB is prefix-keyed and takes CIDR families only, so a non-CIDR route is
// only answered for. Takes the shard lock, so the caller MUST NOT hold it.
//
// The check and the removal run under ONE hold of the shard lock, the lock
// checkRouteBestChange records and mirrors a best under. A concurrent election
// for the route therefore either records its best first, and the route stays,
// or records it after, and its own mirror puts the route back. Checked under
// one hold and removed under another, the route could be recorded and
// mirrored in between, and the removal would then delete a best the bgp-rib
// still records from the Loc-RIB the kernel FIB reads.
//
// A family with no shards has never recorded a best, so nothing holds the
// route and the Loc-RIB holds nothing this RIB mirrored for it: the answer is
// "not held" with no removal, and no shards are created to give it.
func (r *RIBManager) withdrawIfUnheld(fam family.Family, pfx netip.Prefix, route []byte) bool {
	fs := r.bestPrev.familyShards(fam, false)
	if fs == nil {
		return true
	}
	var routeKey []byte
	var sh *bestPrevShard
	if pfx.IsValid() {
		sh = fs.shardFor(pfx)
	}
	if !pfx.IsValid() {
		var scratch [nlrisplit.PrefixKeyScratchSize]byte
		key, ok := routeIdentity(fam, route, false, false, scratch[:])
		if !ok {
			// No key names a record, so none can hold the route.
			return true
		}
		routeKey = key
		sh = fs.shardForNLRI(routeKey)
	}
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if _, _, held := sh.store.lookup(pfx, routeKey); held {
		return false
	}
	if pfx.IsValid() {
		if r.purgeRemoveHook != nil {
			r.purgeRemoveHook(fam, pfx)
		}
		r.removeLocRIB(fam, pfx)
	}
	return true
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
// Safe to call with no outer lock held. gatherPrefixCandidates, gatherKeyCandidates and
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
	return r.checkRouteBestChange(fam, nlriBytes, addPath, false, forward)
}

// checkRouteBestChange is checkBestPathChange for an NLRI whose framing the
// caller states: withdraw says nlriBytes was split from a received withdrawal,
// whose label field is a Compatibility field (RFC 8277 Section 2.4).
// Caller MUST NOT hold peerMu. Election order is shard.mu -> peerMu.RLock;
// peer writers MUST release peerMu before waiting for any bestPrev shard.
func (r *RIBManager) checkRouteBestChange(fam family.Family, nlriBytes []byte, addPath, withdraw bool, forward locrib.ForwardHandle) (bestChangeEntry, bool) {
	if ribevents.IsFlowSpec(fam) {
		r.reconcileFlowSpecs()
		return bestChangeEntry{}, false
	}
	if fam.SAFI == family.SAFIUnicast || fam.SAFI == family.SAFIVPN {
		defer r.reconcileFlowSpecs()
	}
	// Key the route before gathering. A CIDR family parses its NLRI into its
	// prefix, and the prefix alone keys the election and the stored record:
	// the path identifier names a path of the prefix (RFC 7911 Section 2) and
	// never partitions the selection. Malformed bytes bail before touching any
	// shard, because no record can be keyed without a prefix.
	//
	// A non-CIDR family (VPN, EVPN, MVPN, MUP, flowspec, VPLS, BGP-LS) has no
	// prefix at all: its NLRI leads with a total bit length counting a label
	// stack and a Route Distinguisher, or with a route type. Its route key, the
	// wire bytes without the path identifier, keys the election and the record
	// instead, for the same RFC 7911 reason: the Adj-RIB-In stores every path of
	// the route under that key (storage.FamilyRIB.opaqueRouteKey). The key
	// carries no label either (storage.RouteKey), so a relabel replaces the
	// route and a withdrawal's Compatibility field reaches it.
	//
	// The key is computed ONCE and the same key gathers the candidates and
	// keys the record, so a withdrawal's framing cannot gather one route and
	// look up another. Each key is computed only in its own branch: the route
	// key's scratch reaches the family's registered key operation, an indirect
	// call the compiler cannot see through, so it lives on the heap, and a CIDR
	// UPDATE, which is most of them, never pays for it.
	cidr := storage.IsCIDRFamily(fam)
	var (
		pfx        netip.Prefix
		routeKey   []byte
		candidates []*Candidate
	)
	if cidr {
		var prefixOK bool
		_, pfx, prefixOK = parsePrevKey(fam, nlriBytes, addPath)
		if !prefixOK {
			return bestChangeEntry{}, false
		}
	}
	if !cidr {
		var keyScratch [nlrisplit.PrefixKeyScratchSize]byte
		var keyOK bool
		routeKey, keyOK = routeIdentity(fam, nlriBytes, addPath, withdraw, keyScratch[:])
		if !keyOK {
			return bestChangeEntry{}, false
		}
	}
	// The shard serializes extraction with prior and later publication. There
	// can be no first candidate without a peer; avoid allocating a family table
	// for an empty-RIB withdrawal. A later insertion runs its own election.
	fs := r.bestPrev.familyShards(fam, false)
	if fs == nil {
		r.peerMu.RLock()
		havePeers := len(r.bgpPeers) != 0
		r.peerMu.RUnlock()
		if !havePeers {
			return bestChangeEntry{}, false
		}
		fs = r.bestPrev.familyShards(fam, true)
	}
	var sh *bestPrevShard
	if cidr {
		sh = fs.shardFor(pfx)
	} else {
		sh = fs.shardForNLRI(routeKey)
	}
	sh.mu.Lock()
	defer sh.mu.Unlock()
	prev, prevRoute, havePrev := sh.store.lookup(pfx, routeKey)
	var prevPeer string
	var prevNextHop netip.Addr
	var prevMetrics bestPathMetrics
	if havePrev {
		prevPeer = r.bestPathInterner.peerAt(prev.rec.peerIdx())
		prevNextHop = r.bestPathInterner.nextHopAt(prev.rec.nextHopIdx())
		prevMetrics = r.bestPathInterner.metricAt(prev.rec.metricIdx())
	}

	// Keep peer metadata intact against DOWN. Each gathered path independently
	// retains its storage revision against UPDATE replacement or deletion.
	// Reserve changed-winner publication before releasing peerMu; release all
	// candidate handles and peerMu before synchronous Loc-RIB callbacks.
	r.peerMu.RLock()
	if cidr {
		candidates = r.gatherPrefixCandidatesLocked(fam, pfx)
	} else {
		candidates = r.gatherKeyCandidatesLocked(fam, routeKey)
	}
	// SelectMultipath returns the same primary winner as SelectBest plus any
	// equal-cost siblings (rib-arch-4). When multipath is off (maximum-paths<=1,
	// the default) it returns nil siblings with no extra work, so the single-best
	// path is unchanged.
	newBest, siblings := SelectMultipath(candidates, r.maximumPaths.Load(), r.relaxASPath.Load())

	// Winner metadata belongs to the same retained revision as its candidacy.
	var (
		nextHop      netip.Addr
		isEBGP       bool
		bestLabels   []uint32
		srv6SID      netip.Addr
		ecmpNextHops []nexthop.NextHop
		// The winner's own path names publication; metadata comes from its
		// retained entry and label handles, never a second PeerRIB lookup.
		winnerPathID  uint32
		winnerAddPath bool
		winnerPath    storedPath
	)
	if newBest != nil {
		winnerPathID, winnerAddPath = newBest.PathID, newBest.AddPath
		winnerPath = candidatePath(newBest, cidr, pfx)
		nextHop = entryNextHopAddr(fam, newBest.entry)
		isEBGP = r.protocolType(newBest) == routeaction.ProtocolEBGP
		if fam.SAFI == family.SAFIMPLSLabel {
			bestLabels = pool.ResolveLabels(newBest.labelHandle)
		}
		if fam.SAFI != family.SAFIMPLSLabel {
			srv6SID = entrySRv6SID(fam, winnerPath, newBest.entry)
		}
		// Resolve the equal-cost multipath sibling next-hops so the Loc-RIB
		// carries the full ECMP set to the FIB (rib-arch-4). Each sibling
		// resolves from its retained revision, like the primary; dedup
		// against the primary and each other.
		for _, s := range siblings {
			// A BGP sibling names a gateway address and never a device or a
			// weight, so the group is unweighted: nexthop.NextHop's zero Weight
			// is what "share equally" is spelled as.
			nh := nexthop.NextHop{Addr: entryNextHopAddr(fam, s.entry)}
			if nh.Addr.IsValid() && nh.Addr != nextHop && !slices.Contains(ecmpNextHops, nh) {
				ecmpNextHops = append(ecmpNextHops, nh)
			}
		}
	}

	// RFC 7999 Section 3.3: resolve the forwarding action in the peer snapshot.
	// Zero for every peer that stated no rule, which is every peer by default.
	//
	// Asked for CIDR families only. Section 3.3 authorizes a BLACKHOLE by the
	// covering IP prefix the operator configured, and a non-CIDR route names no
	// prefix to cover, so the question has no answer there rather than the
	// answer "not a discard".
	var blackholeType routetype.Type
	if newBest != nil && cidr {
		blackholeType = r.blackholeRouteTypeForBest(pfx, newBest)
	}

	if newBest == nil {
		releaseCandidates(candidates)
		r.peerMu.RUnlock()
		// No candidates remain -- withdraw if we had a previous best.
		if !havePrev {
			return bestChangeEntry{}, false
		}
		sh.store.delete(pfx, routeKey)
		r.bestPathInterner.releasePeer(prev.rec.peerIdx())
		// The Loc-RIB is prefix-keyed and feeds the kernel FIB, so it takes
		// CIDR families only. See mirrorToLocRIB below for why.
		if cidr {
			r.removeLocRIB(fam, pfx)
		}
		if !cidr {
			return bestChangeEntry{
				Action:  ribevents.BestChangeWithdraw,
				NLRI:    framedRouteNLRI(nil, prevRoute, prev.pathID, prev.addPath),
				AddPath: prev.addPath,
				PathID:  prev.pathID,
			}, true
		}
		return bestChangeEntry{
			Action:  ribevents.BestChangeWithdraw,
			Prefix:  pfx,
			AddPath: prev.addPath,
			PathID:  prev.pathID,
		}, true
	}

	// All prior interner values were resolved before taking peerMu. An unchanged
	// route performs no interning, peer-slot reference changes or AS_PATH formatting.
	sameBest := havePrev && slices.Equal(sh.store.labels[pfx], bestLabels) &&
		prevPeer == newBest.PeerAddr &&
		prev.pathID == winnerPathID && prev.addPath == winnerAddPath &&
		prevRoute == winnerRoute(newBest, cidr) &&
		prevNextHop == nextHop &&
		prevMetrics == (bestPathMetrics{MED: newBest.MED, AIGP: newBest.AIGP, HasAIGP: newBest.HasAIGP}) &&
		prev.rec.IsEBGP() == isEBGP &&
		prev.rec.isBlackhole() == (blackholeType == routetype.Blackhole) &&
		!srv6SID.IsValid() && prev.rec.Flags()&flagHadSRv6SID == 0
	var peerIdx uint16
	if !sameBest {
		// DOWN acquires peerMu before looking up this slot. Reserve it before
		// releasing admission, so even a peer with no prior winning record
		// forces DOWN to wait for this publication's shard.
		var admitted bool
		peerIdx, admitted = r.bestPathInterner.internPeer(newBest.PeerAddr)
		if !admitted {
			releaseCandidates(candidates)
			r.peerMu.RUnlock()
			return bestChangeEntry{}, false
		}
	}
	var asPath []uint32
	if !sameBest && newBest.ASPathHandle.IsValid() {
		if data, err := pool.ASPath.Get(newBest.ASPathHandle); err == nil {
			asPath = formatASPath(data)
		}
	}
	releaseCandidates(candidates)
	r.peerMu.RUnlock()

	// mirrorToLocRIB writes the winning best path (plus its equal-cost multipath
	// set) into the shared Loc-RIB. Called on BOTH the same-best short-circuit and
	// the full best-change path so an ECMP-membership change is never lost when
	// the best next-hop itself is unchanged (the same-best test below compares the
	// best, not the sibling set); the Loc-RIB dedups a true no-op via Path.Equal.
	mirrorToLocRIB := func() {
		if r.locRIB.Load() == nil && r.forkRIB == nil {
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
		// No distance and Metric carries MED. The Loc-RIB ranks the path at the
		// distance `rib { distance { } }` declares for its class, ebgp or ibgp,
		// which IsEBGP selects, and re-ranks it when a reload changes that.
		r.insertLocRIB(fam, pfx, locrib.Path{
			Source:   bgpProtocolID,
			Instance: bgpLocRIBInstance,
			NextHop:  nextHop,
			// Carry the eBGP/iBGP class explicitly: the Loc-RIB picks the
			// declared ebgp or ibgp distance by it, and the sysrib replay path
			// classifies the protocol type by it.
			IsEBGP:      isEBGP,
			IsBGP:       true,
			AIGP:        newBest.AIGP,
			AIGPPresent: newBest.HasAIGP,
			Metric:      newBest.MED,
			// Carry the label stack into the Loc-RIB so labeled-unicast routes
			// reach the kernel as MPLS push entries. sysrib prefers the Loc-RIB
			// path, so without this the labels are dropped and a plain IP route
			// is installed.
			Labels:  bestLabels,
			SRv6SID: srv6SID,
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

	if sameBest {
		// A changed multipath set still reaches the Loc-RIB, which deduplicates
		// true no-ops, but the best itself needs no new record or event.
		mirrorToLocRIB()
		return bestChangeEntry{}, false
	}

	nhIdx, ok := r.bestPathInterner.internNextHop(nextHop)
	if !ok {
		r.bestPathInterner.releasePeer(peerIdx)
		return bestChangeEntry{}, false
	}
	metricIdx, ok := r.bestPathInterner.internMetric(bestPathMetrics{MED: newBest.MED, AIGP: newBest.AIGP, HasAIGP: newBest.HasAIGP})
	if !ok {
		r.bestPathInterner.releasePeer(peerIdx)
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

	// Transfer the admission reference to the new record before publication.
	sh.store.insert(pfx, routeKey, bestPrevRecord{rec: newRec, pathID: winnerPathID, addPath: winnerAddPath}, winnerRoute(newBest, cidr))
	if havePrev {
		r.bestPathInterner.releasePeer(prev.rec.peerIdx())
	}
	if len(bestLabels) > 0 {
		if sh.store.labels == nil {
			sh.store.labels = make(map[netip.Prefix][]uint32)
		}
		if !slices.Equal(sh.store.labels[pfx], bestLabels) {
			// Own the snapshot independently of both pool storage and the
			// publication payload retained by subscribers.
			sh.store.labels[pfx] = slices.Clone(bestLabels)
		}
	} else {
		delete(sh.store.labels, pfx)
	}
	// Mirror the best path (and its equal-cost multipath set) into the shared
	// Loc-RIB via the same closure the same-best short-circuit uses.
	mirrorToLocRIB()
	action := ribevents.BestChangeAdd
	if havePrev {
		action = ribevents.BestChangeUpdate
	}
	// pfx is zero for a non-CIDR family, whose NLRI names the route instead: the
	// winner's own wire NLRI, framed with its path identifier when its session
	// uses ADD-PATH, which AddPath and PathID then state.
	entry := newRec.resolve(r.bestPathInterner, action, pfx, winnerPathID, winnerAddPath)
	// Nil for a CIDR family, where Prefix names the route. For every other one
	// the winner's framed NLRI, which candidatePath built as an owned copy of
	// the stored route: the entry outlives this call, because subscribers retain
	// the batch and the bus marshals it lazily. Built only for a non-CIDR winner.
	entry.NLRI = winnerPath.nlri
	entry.Labels = bestLabels
	entry.SRv6SID = srv6SID
	// RFC 7999 Section 3.3. Zero for every route that is not a honored
	// blackhole, which leaves the FIB installing an ordinary route.
	entry.RouteType = blackholeType
	// The owned AS_PATH snapshot was made before releasing the peer admission;
	// DOWN may already have released the original pool handle.
	entry.ASPath = asPath
	if len(asPath) > 0 {
		entry.OriginAS = asPath[len(asPath)-1]
	}
	return entry, true
}

// bgpLocRIBInstance is the Loc-RIB Instance every BGP best path is mirrored
// under. The RIB elects ONE best path per prefix across every peer and every
// ADD-PATH path, so one BGP Path per prefix reaches the Loc-RIB, and the path
// id of the winner travels on the best-change, not in the Loc-RIB key. A
// per-path Instance would let two BGP paths of one prefix sit in the Loc-RIB,
// where locrib.selectBest ranks them by distance and metric alone and would
// override the RFC 4271 decision this RIB already made.
const bgpLocRIBInstance uint32 = 0

// cidrKeyOctetsMax bounds the wire key of a CIDR path: a 4-octet path
// identifier, a prefix-length octet and up to 16 address octets.
const cidrKeyOctetsMax = 4 + 1 + 16

// candidateNLRI writes into buf the key the candidate's own session stored its
// path under, the path identifier then the prefix under ADD-PATH and the prefix
// alone otherwise, and returns it. buf MUST hold cidrKeyOctetsMax octets.
//
// RFC 7911 Section 2: "a particular path for an address prefix can be
// identified by the combination of the address prefix and the Path
// Identifier". The winner's next hop, labels, SRv6 SID and blackhole answer
// are read through this key, so they belong to the path that won.
func candidateNLRI(c *Candidate, pfx netip.Prefix, buf []byte) []byte {
	head := 0
	if c.AddPath {
		binary.BigEndian.PutUint32(buf, c.PathID)
		head = 4
	}
	tail := store.PrefixToNLRIInto(pfx, buf[head:])
	if tail == nil {
		panic("BUG: candidateNLRI: a parsed prefix does not fit cidrKeyOctetsMax")
	}
	return buf[:head+len(tail)]
}

// opaqueKeyOctetsInline sizes the stack buffer a non-CIDR winner's NLRI is
// framed into: a 4-octet path identifier and a VPN, EVPN or MVPN key. A longer
// key (BGP-LS) spills to the heap once, on the append. It also holds every
// CIDR key, so it is never smaller than cidrKeyOctetsMax.
const opaqueKeyOctetsInline = 64

// routeIdentity returns the identity one election and one stored best are
// keyed by: for a CIDR family the NLRI without its path identifier (routeKeyOf),
// for every other family that route less its non-identifying fields, the
// labels among them (storage.RouteKey). withdraw says nlriBytes was split from
// a withdrawal. The result aliases nlriBytes or scratch, which MUST hold
// nlrisplit.PrefixKeyScratchSize octets. Returns false for an ADD-PATH NLRI
// too short to carry its identifier.
func routeIdentity(fam family.Family, nlriBytes []byte, addPath, withdraw bool, scratch []byte) ([]byte, bool) {
	route, ok := routeKeyOf(nlriBytes, addPath)
	if !ok {
		return nil, false
	}
	if storage.IsCIDRFamily(fam) {
		return route, true
	}
	return storage.RouteKey(fam, route, scratch, withdraw), true
}

// winnerRoute returns what bestPrevRecord.route holds for the winner: its
// wire route for a non-CIDR family, nothing for a CIDR prefix or no winner.
func winnerRoute(winner *Candidate, cidr bool) string {
	if winner == nil {
		return ""
	}
	if cidr {
		return ""
	}
	return winner.Route
}

// routeKeyOf returns the route key of nlriBytes, the NLRI without the 4-octet
// path identifier an ADD-PATH session frames it with (RFC 7911 Section 3).
// Returns false for an ADD-PATH NLRI too short to carry one.
func routeKeyOf(nlriBytes []byte, addPath bool) ([]byte, bool) {
	if !addPath {
		return nlriBytes, true
	}
	if len(nlriBytes) < 4 {
		return nil, false
	}
	return nlriBytes[4:], true
}

// framedRouteNLRI appends to dst the NLRI one session names a path of a
// non-CIDR route by: the path identifier then the wire route under ADD-PATH,
// the wire route alone otherwise. A nil dst gives an owned copy.
func framedRouteNLRI(dst []byte, route string, pathID uint32, addPath bool) []byte {
	if addPath {
		dst = binary.BigEndian.AppendUint32(dst, pathID)
	}
	return append(dst, route...)
}

// storedPath names one path in the RIB of the session that stored it, the way
// that RIB is keyed: a CIDR path by its prefix and path identifier, every other
// path by the wire NLRI its session framed it with.
//
// RFC 7911 Section 2: "a particular path for an address prefix can be
// identified by the combination of the address prefix and the Path
// Identifier". Every read of what the winner carries goes through this name, so
// it reads the path that won.
//
// The CIDR form carries no bytes, on purpose. A wire key reaches
// PeerRIB.Lookup, whose non-CIDR branch hands it to the family's registered key
// operation, an indirect call the compiler cannot see through, so any buffer
// that ever holds one moves to the heap. Asking by prefix keeps a CIDR
// election, which is most UPDATEs, off that path and off the heap.
type storedPath struct {
	nlri     []byte       // !byPrefix: the framed wire NLRI
	pfx      netip.Prefix // byPrefix: the prefix
	pathID   uint32       // byPrefix: the path identifier, zero without ADD-PATH
	addPath  bool         // the storing session frames the family with ADD-PATH
	byPrefix bool         // the family is CIDR, asked by prefix
}

// candidatePath names the path the candidate's own session stored. A non-CIDR
// path gets an owned copy of its framed NLRI (framedRouteNLRI), which the
// published entry may keep.
func candidatePath(c *Candidate, cidr bool, pfx netip.Prefix) storedPath {
	if cidr {
		return storedPath{pfx: pfx, pathID: c.PathID, addPath: c.AddPath, byPrefix: true}
	}
	return storedPath{nlri: framedRouteNLRI(nil, c.Route, c.PathID, c.AddPath), pathID: c.PathID, addPath: c.AddPath}
}

// entrySRv6SID extracts the SRv6 SID from the PrefixSID attribute (code 40)
// in the winning snapshot's OtherAttrs, reconstructing any transposed label bits.
// Returns an invalid Addr when the attribute is absent, is not SRv6, or
// names a transposition ze cannot undo -- reporting no SID rather than a
// partial one, because the partial one is not what the peer signaled.
// The caller MUST keep entry retained until this function returns.
//
// A path asked by prefix hands srv6SIDFromResult no NLRI. That is not a
// missing answer: only a VPN NLRI carries a label field a SID is transposed
// into (nlrisplit.TranspositionLabel), and a VPN route is never CIDR.
func entrySRv6SID(fam family.Family, p storedPath, entry storage.RouteEntry) netip.Addr {
	b := entry.GetBundle()
	if !b.HasOtherAttrs() {
		return netip.Addr{}
	}
	return srv6SIDFromResult(fam, p.nlri, p.addPath, extractSRv6SIDResultFromOtherAttrs(b))
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

// entryNextHopAddr reads the next-hop a stored route entry advertises. Returns
// the zero Addr when the entry carries none.
//
// This is the ONE producer of that answer, so the winner's installed next hop
// (checkRouteBestChange) and the Section 5.1.3 eligibility test
// (gatherCandidatesLocked, rib_commands.go) cannot disagree about which address
// a route names. Reads pool handles only; no lock, no allocation.
func entryNextHopAddr(fam family.Family, entry storage.RouteEntry) netip.Addr {
	// RFC 8955 Section 4: neither the MP next hop nor a legacy NEXT_HOP
	// carried for another family in the same UPDATE applies to FlowSpec.
	if ribevents.IsFlowSpec(fam) {
		return netip.Addr{}
	}
	b := entry.GetBundle()
	if fam == family.IPv4Unicast {
		if b.HasNextHop() {
			data, err := pool.NextHop.Get(b.NextHop)
			if err == nil {
				if a := parseNextHopAddr(data); a.IsValid() {
					return a
				}
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
	// RFC 8955 Section 4: the advertised next-hop address is ignored.
	if safi == family.SAFIFlowSpec || safi == family.SAFIFlowSpecVPN {
		return netip.Addr{}
	}
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
	r.replayFlowSpecs()
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
	if state := r.flowSpec.Load(); state != nil {
		for rule, winner := range state.best {
			if r.flowSpecEligible(winner.key, 0) {
				change := winner.change
				change.NLRI = bytes.Clone(change.NLRI)
				changesByFamily[rule.family] = append(changesByFamily[rule.family], change)
			}
		}
	}
	for _, fam := range families {
		fs := r.bestPrev.familyShards(fam, false)
		if fs == nil {
			continue
		}
		// Count under each shard's read lock so the batch preallocation is
		// sized correctly. Replay is a cold path fired on late-subscriber
		// replay-request; the per-shard read locks are held briefly in series.
		total := 0
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.RLock()
			if sh.store.cidr {
				total += sh.store.direct.Len()
			} else {
				total += len(sh.store.opaque)
			}
			sh.mu.RUnlock()
		}
		changes := make([]bestChangeEntry, 0, total)
		appendRec := func(pfx netip.Prefix, prev bestPrevRecord) {
			rec, pathID := prev.rec, prev.pathID
			if !pfx.IsValid() {
				return
			}
			if ribevents.ValidationEnabled() {
				peer, err := netip.ParseAddr(r.bestPathInterner.peerAt(rec.peerIdx()))
				if err != nil {
					return
				}
				if !ribevents.RouteEligible(ribevents.ValidationRoute{
					Peer: peer, Family: fam, Prefix: pfx, PathID: pathID,
				}, 0) {
					return
				}
			}
			changes = append(changes, rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, pfx, pathID, prev.addPath))
		}
		for i := range fs.shards {
			sh := &fs.shards[i]
			sh.mu.RLock()
			if !sh.store.cidr {
				// Non-CIDR family: the wire bytes name the route, and the
				// prefix stays zero. appendRec is the CIDR path and refuses a
				// zero prefix, so replay builds these entries directly.
				for _, best := range sh.store.opaque {
					rec := best.prev
					e := rec.rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, netip.Prefix{}, rec.pathID, rec.addPath)
					e.NLRI = framedRouteNLRI(nil, best.route, rec.pathID, rec.addPath)
					changes = append(changes, e)
				}
				sh.mu.RUnlock()
				continue
			}
			sh.store.direct.Iterate(func(pfx netip.Prefix, prev bestPrevRecord) bool {
				appendRec(pfx, prev)
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
// command-driven mutation (inject, withdraw). Caller MUST release peerMu first:
// checkRouteBestChange takes the owning shard before its peer read admission.
// addPath=false because inject/withdraw build NLRI with pathID=0 and no
// ADD-PATH prefix; if those commands gain --path-id, this must change.
func (r *RIBManager) reconcileBestPath(fam family.Family, nlriBytes []byte) {
	change, ok := r.checkBestPathChange(fam, nlriBytes, false, nil)
	if ok {
		publishBestChanges([]bestChangeEntry{change}, fam)
	}
}

// reconcileBestPathBulk purges and re-elects after bulk peer mutations. The
// caller MUST release peerMu first: purge takes shard locks, and an election
// holding a shard may be reading peer state or publishing a Loc-RIB callback.
func (r *RIBManager) reconcileBestPathBulk(peers []netip.Addr) {
	for _, peer := range peers {
		// The interner is keyed by the canonical address string.
		pending := r.purgeBestPrevForPeer(peer.String())
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
