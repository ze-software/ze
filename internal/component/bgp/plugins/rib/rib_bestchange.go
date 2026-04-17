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
)

// bestChangeEntry is an alias for the exported event payload entry type so
// the per-prefix functions in this file keep their current signatures while
// still producing the exported payload shape. See ribevents.BestChangeEntry.
type bestChangeEntry = ribevents.BestChangeEntry

// bestChangeBatch is an alias for the exported event payload. The producer
// path builds one batch per (protocol, family) combination, then emits via
// the typed BestChange handle.
type bestChangeBatch = ribevents.BestChangeBatch

// bestPathRecord stores the previous best-path state for change detection.
// The prefix is not stored -- it is the key of the owning bestPrevStore entry
// and is formatted lazily from that key on the emission path.
//
// NextHop is a netip.Addr rather than a formatted string: comparison on the
// hot steady-state path is a value compare (no allocation), and the display
// string is produced only when a change event is actually emitted via
// nextHopString below. Prior to this shape, fmt.Sprintf inside formatNextHop
// accounted for ~15 MB of inuse allocation under the 1M-prefix stress.
type bestPathRecord struct {
	PeerAddr     string
	NextHop      netip.Addr
	Priority     int // admin distance: eBGP=20, iBGP=200
	Metric       uint32
	ProtocolType string // "ebgp" or "ibgp"
}

// bestPrevStore holds the previously-recorded best path per prefix for one
// family. It carries both a non-ADD-PATH trie-backed Store and an ADD-PATH
// map-backed Store so a family can host peers with mixed ADD-PATH capability
// without key collision -- AP peers advertise NLRIs with a 4-byte path-id
// prefix which the trie cannot key on, while non-AP peers use bare prefix
// bytes which the map would conflate with other path-ids.
type bestPrevStore struct {
	trie *storage.Store[bestPathRecord] // addPath=false backend
	ap   *storage.Store[bestPathRecord] // addPath=true backend
}

// newBestPrevStore creates a bestPrevStore for a family. Both backends are
// allocated eagerly so mixed-mode sessions (some peers ADD-PATH-capable, some
// not, for the same family) route each call to the correct key space without
// collision. The empty backend pays only a small idle cost (one map header
// or one empty trie root) regardless of which keys the family ends up using
// -- accepted trade-off for correctness on mixed sessions. See the
// rib-bart-bestprev design decision log entry D2.
func newBestPrevStore(fam family.Family) *bestPrevStore {
	return &bestPrevStore{
		trie: storage.NewStore[bestPathRecord](fam, false),
		ap:   storage.NewStore[bestPathRecord](fam, true),
	}
}

// pick returns the backend store for the given addPath flag.
func (s *bestPrevStore) pick(addPath bool) *storage.Store[bestPathRecord] {
	if addPath {
		return s.ap
	}
	return s.trie
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
// Returns (entry, true) when a change occurred; (zero, false) when unchanged or
// the NLRI is malformed. The bool form avoids heap-allocating *bestChangeEntry
// per update, which dominated GC work under stress.
func (r *RIBManager) checkBestPathChange(fam family.Family, nlriBytes []byte, addPath bool) (bestChangeEntry, bool) {
	// Gather candidates from all peers for this prefix.
	candidates := r.gatherCandidates(fam, nlriBytes)
	newBest := SelectBest(candidates)

	store := r.bestPrev[fam]
	if store == nil && newBest == nil {
		// Nothing to do -- no candidates and no previous record.
		return bestChangeEntry{}, false
	}
	if store == nil {
		store = newBestPrevStore(fam)
		r.bestPrev[fam] = store
	}
	backend := store.pick(addPath)

	prev, havePrev := backend.Lookup(nlriBytes)

	if newBest == nil {
		// No candidates remain -- withdraw if we had a previous best.
		if !havePrev {
			return bestChangeEntry{}, false
		}
		familyStr := fam.String()
		prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), familyStr)
		if prefix == "" {
			return bestChangeEntry{}, false
		}
		backend.Delete(nlriBytes)
		return bestChangeEntry{
			Action: ribevents.BestChangeWithdraw,
			Prefix: prefix,
		}, true
	}

	// Extract next-hop, priority, and protocol type from the new best.
	// NextHop is kept as netip.Addr for the hot comparison; the display
	// string is materialized only if we go on to emit a change below.
	nextHop := r.bestCandidateNextHopAddr(fam, nlriBytes, newBest)
	priority := r.adminDistance(newBest)
	protoType := r.protocolType(newBest)
	metric := newBest.MED

	if havePrev && prev.PeerAddr == newBest.PeerAddr && prev.NextHop == nextHop &&
		prev.Priority == priority && prev.Metric == metric {
		return bestChangeEntry{}, false // Same best, no change.
	}

	// Compute the display prefix and next-hop only now -- the unchanged
	// steady state above skips both entirely, saving a wirePrefixToString
	// alloc and a nextHop.String() alloc per update. The latter was the
	// ~15 MB fmt.Sprintf hotspot in the prior profile.
	familyStr := fam.String()
	prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), familyStr)
	if prefix == "" {
		return bestChangeEntry{}, false
	}
	nextHopStr := nextHopString(nextHop)

	backend.Insert(nlriBytes, bestPathRecord{
		PeerAddr:     newBest.PeerAddr,
		NextHop:      nextHop,
		Priority:     priority,
		Metric:       metric,
		ProtocolType: protoType,
	})
	action := ribevents.BestChangeAdd
	if havePrev {
		action = ribevents.BestChangeUpdate
	}
	return bestChangeEntry{
		Action:       action,
		Prefix:       prefix,
		NextHop:      nextHopStr,
		Priority:     priority,
		Metric:       metric,
		ProtocolType: protoType,
	}, true
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

// adminDistance returns the admin distance for a candidate.
// eBGP = 20, iBGP = 200. Uses protocolType to determine the session type.
func (r *RIBManager) adminDistance(c *Candidate) int {
	if r.protocolType(c) == protocolTypeEBGP {
		return 20
	}
	return 200 // iBGP
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
	for fam, store := range r.bestPrev {
		famStr := fam.String()
		// Replay is a cold path fired on late-subscriber replay-request.
		// Presize to the exact final length so a 1M-entry family commits
		// one ~80 MB allocation instead of paying ~20 geometric-growth
		// cycles (~30 MB transient peak, similar final size). Upfront
		// commitment is acceptable because the batch is emitted and
		// released in the same function; GC reclaims immediately after.
		changes := make([]bestChangeEntry, 0, store.trie.Len()+store.ap.Len())
		appendEntry := func(nlriBytes []byte, rec bestPathRecord, addPath bool) {
			prefix := wirePrefixToString(prefixBytesForDisplay(nlriBytes, addPath), famStr)
			if prefix == "" {
				return
			}
			changes = append(changes, bestChangeEntry{
				Action:       ribevents.BestChangeAdd,
				Prefix:       prefix,
				NextHop:      nextHopString(rec.NextHop),
				Priority:     rec.Priority,
				Metric:       rec.Metric,
				ProtocolType: rec.ProtocolType,
			})
		}
		store.trie.Iterate(func(nlriBytes []byte, rec bestPathRecord) bool {
			appendEntry(nlriBytes, rec, false)
			return true
		})
		store.ap.Iterate(func(nlriBytes []byte, rec bestPathRecord) bool {
			appendEntry(nlriBytes, rec, true)
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
