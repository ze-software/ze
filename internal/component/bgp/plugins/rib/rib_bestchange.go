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

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
// Concurrency: NOT safe for concurrent use. Callers hold RIBManager.mu for
// all intern* and reverse-lookup calls.
type bestPrevInterner struct {
	peers              []string
	peerIdx            map[string]uint16
	peersOverflowed    bool
	nextHops           []netip.Addr
	nextHopIdx         map[netip.Addr]uint16
	nextHopsOverflowed bool
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

// internPeer returns the uint16 index for v. On a first sighting, v is
// appended to the reverse table and assigned the next index. Returns
// (0, false) only when the reverse table is saturated at 65536 entries --
// the caller must treat that as a degraded record and not store it. The
// first saturation logs an slog.Error; subsequent ones are silent.
func (b *bestPrevInterner) internPeer(v string) (uint16, bool) {
	if idx, ok := b.peerIdx[v]; ok {
		return idx, true
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
	b.peerIdx[v] = idx
	return idx, true
}

// internNextHop returns the uint16 index for v; see internPeer for contract.
// The zero netip.Addr (invalid / absent next-hop) is interned like any other
// value so resolve() round-trips it back to nextHopString("").
func (b *bestPrevInterner) internNextHop(v netip.Addr) (uint16, bool) {
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
	idx := uint16(len(b.nextHops))
	b.nextHops = append(b.nextHops, v)
	b.nextHopIdx[v] = idx
	return idx, true
}

// internMetric returns the uint16 index for v; see internPeer for contract.
func (b *bestPrevInterner) internMetric(v uint32) (uint16, bool) {
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
	idx := uint16(len(b.metrics))
	b.metrics = append(b.metrics, v)
	b.metricIdx[v] = idx
	return idx, true
}

// peerAt returns the original peer string for idx, or "" if idx is past the
// reverse-table bounds. A bounds-safe wrapper so emission and steady-state
// comparison do not panic if an index from an older interner lifetime (or a
// manually-constructed record in tests) outlives its backing table.
func (b *bestPrevInterner) peerAt(idx uint16) string {
	if int(idx) >= len(b.peers) {
		return ""
	}
	return b.peers[idx]
}

// nextHopAt returns the original netip.Addr for idx, or the zero Addr if idx
// is past the reverse-table bounds. See peerAt for rationale.
func (b *bestPrevInterner) nextHopAt(idx uint16) netip.Addr {
	if int(idx) >= len(b.nextHops) {
		return netip.Addr{}
	}
	return b.nextHops[idx]
}

// metricAt returns the original uint32 for idx, or 0 if idx is past the
// reverse-table bounds. See peerAt for rationale.
func (b *bestPrevInterner) metricAt(idx uint16) uint32 {
	if int(idx) >= len(b.metrics) {
		return 0
	}
	return b.metrics[idx]
}

// resolve materializes a bestChangeEntry from a packed record plus an action
// label and display prefix. The emitted payload priority (20 eBGP / 200 iBGP)
// and protocol-type ("ebgp"/"ibgp") derive from the packed Flags bit 0, so
// the single source of truth for protocol class is the stored record rather
// than a derivable pair of fields. Caller MUST hold at least RIBManager.mu
// for reading (the reverse tables are mutated on insert).
//
// Reverse-table lookups go through the bounds-safe accessors, so a record
// whose indices outlive a reset interner emits zero-valued NextHop/Metric
// rather than panicking.
func (r bestPathRecord) resolve(interner *bestPrevInterner, action, prefix string) bestChangeEntry {
	priority := 200
	protoType := protocolTypeIBGP
	if r.IsEBGP() {
		priority = 20
		protoType = protocolTypeEBGP
	}
	return bestChangeEntry{
		Action:       action,
		Prefix:       prefix,
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

// prefixBytesForDisplay returns the NLRI bytes suitable for wirePrefixToString.
// For ADD-PATH, strips the 4-byte path-ID prefix that splitNLRIs includes.
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
// Caller MUST hold r.mu (write lock).
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
func (r *RIBManager) checkBestPathChange(fam family.Family, nlriBytes []byte, addPath bool) (bestChangeEntry, bool) {
	candidates := r.gatherCandidates(fam, nlriBytes)
	newBest := SelectBest(candidates)

	prevStore := r.bestPrev[fam]
	if prevStore == nil && newBest == nil {
		return bestChangeEntry{}, false
	}
	if prevStore == nil {
		prevStore = newBestPrevStore(fam)
		r.bestPrev[fam] = prevStore
	}

	prev, havePrev := prevStore.lookup(fam, nlriBytes, addPath)

	if newBest == nil {
		// No candidates remain -- withdraw if we had a previous best.
		if !havePrev {
			return bestChangeEntry{}, false
		}
		prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), fam.String())
		if prefix == "" {
			return bestChangeEntry{}, false
		}
		prevStore.delete(fam, nlriBytes, addPath)
		return bestChangeEntry{
			Action: ribevents.BestChangeWithdraw,
			Prefix: prefix,
		}, true
	}

	nextHop := r.bestCandidateNextHopAddr(fam, nlriBytes, newBest)
	isEBGP := r.protocolType(newBest) == protocolTypeEBGP

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

	prevStore.insert(fam, nlriBytes, addPath, newRec)
	action := ribevents.BestChangeAdd
	if havePrev {
		action = ribevents.BestChangeUpdate
	}
	return newRec.resolve(r.bestPathInterner, action, prefix), true
}

// protocolType returns the protocol-type label for a candidate based on
// ASN comparison. When LocalASN is 0 (unknown, e.g. before OPEN negotiation
// completes), defaults to ebgp. This is intentional: routes learned before
// ASN negotiation are assumed external, which is the more common case.
func (r *RIBManager) protocolType(c *Candidate) string {
	if c.LocalASN == 0 || c.PeerASN != c.LocalASN {
		return protocolTypeEBGP
	}
	return protocolTypeIBGP
}

// bestCandidateNextHopAddr extracts the next-hop for the winning candidate's
// route entry as a netip.Addr. Returns the zero Addr when missing. This is
// the zero-alloc equivalent of the former string-returning helper: the hot
// comparison in checkBestPathChange is a value compare against the stored
// bestPathRecord.NextHop, with string materialization deferred until the
// emission path.
// For IPv4, reads from the NEXT_HOP attribute (code 3).
// For IPv6 and other MP families, extracts from MP_REACH_NLRI (code 14) in OtherAttrs.
// Caller MUST hold r.mu (at least read lock).
func (r *RIBManager) bestCandidateNextHopAddr(fam family.Family, nlriBytes []byte, best *Candidate) netip.Addr {
	peerRIB := r.ribInPool[best.PeerAddr]
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
// Caller MUST NOT hold r.mu.
func (r *RIBManager) replayBestPaths() {
	eb := getEventBus()
	if eb == nil {
		return
	}

	r.mu.RLock()
	changesByFamily := make(map[string][]bestChangeEntry, len(r.bestPrev))
	for fam, prevStore := range r.bestPrev {
		famStr := fam.String()
		// Replay is a cold path fired on late-subscriber replay-request.
		// Count AP entries (one per path-id) before allocating so a 1M-entry
		// family commits one allocation instead of paying multiple geometric-
		// growth cycles. Upfront commitment is acceptable because the batch is
		// emitted and released in the same function; GC reclaims immediately.
		apCount := 0
		prevStore.multi.Iterate(func(_ netip.Prefix, ps bestPrevSet) bool {
			apCount += len(ps.entries)
			return true
		})
		changes := make([]bestChangeEntry, 0, prevStore.direct.Len()+apCount)
		appendRec := func(rec bestPathRecord, prefix string) {
			if prefix == "" {
				return
			}
			changes = append(changes, rec.resolve(r.bestPathInterner, ribevents.BestChangeAdd, prefix))
		}
		prevStore.direct.Iterate(func(pfx netip.Prefix, rec bestPathRecord) bool {
			appendRec(rec, pfx.String())
			return true
		})
		prevStore.multi.Iterate(func(pfx netip.Prefix, ps bestPrevSet) bool {
			for i := range ps.entries {
				appendRec(ps.entries[i].rec, pfx.String())
			}
			return true
		})
		if len(changes) > 0 {
			changesByFamily[famStr] = changes
		}
	}
	r.mu.RUnlock()

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
func publishBestChanges(changes []bestChangeEntry, family string) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	batch := &bestChangeBatch{
		Protocol: "bgp",
		Family:   family,
		Changes:  changes,
	}
	if _, err := ribevents.BestChange.Emit(eb, batch); err != nil {
		logger().Warn("best-change emit failed", "error", err)
	}
}
