// Design: docs/architecture/plugin/rib-storage-design.md — iterator pipeline for RIB show commands
// RFC: rfc/short/rfc4271.md -- route attributes surfaced by show pipelines
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestPathRows)
// Related: rib_topology.go — graph terminal for AS path topology rendering
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: rib_nlri.go — NLRI wire format helpers
package rib

import (
	"encoding/json"
	"iter"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// RouteItem is a single route yielded by the pipeline iterator.
// Carries enough to filter and serialize without re-reading the RIB.
type RouteItem struct {
	Peer      string
	Family    family.Family
	Prefix    string
	Direction rpc.MessageDirection // DirectionReceived / DirectionSent

	// Exactly one of these is set, depending on Direction.
	HasInEntry bool               // true when InEntry is populated
	InEntry    storage.RouteEntry // pool-based entry for adj-rib-in
	OutRoute   *Route             // parsed route for adj-rib-out

	// MultipathPeers lists the peer addresses of additional equal-cost
	// paths that share the multipath set with Peer (the primary best).
	// Populated only by the best-path source when bgp/multipath/maximum-paths
	// is > 1 and more than one candidate ties through RFC 4271 §9.1.2
	// steps 1-5. nil when multipath is disabled or no siblings exist.
	// Consumers that don't care about ECMP can ignore this field.
	MultipathPeers []string
}

// PipelineMeta holds pipeline result metadata.
type PipelineMeta struct {
	Count int
	JSON  string // set by json, histogram, and graph terminals
}

// pipelineIterator is the pull-based iterator interface for pipeline stages.
type pipelineIterator interface {
	Next() (RouteItem, bool)
	Meta() PipelineMeta
}

// --- Source iterators ---

// inboundPeerRef is a snapshot of a peer name and its PeerRIB pointer,
// captured under peerMu so iteration can proceed without holding peerMu.
type inboundPeerRef struct {
	peer    string
	peerRIB *storage.PeerRIB
}

// inboundSource iterates over all adj-rib-in routes matching the peer selector.
// Caller (showPipeline) holds r.peerMu.RLock across construction and the full
// drain, and PeerRIB iteration uses PeerRIB's own lock.
//
// WHAT THAT LOCK DOES NOT DO. The downstream filters and terminals dereference
// the captured InEntry pool handles, and peerMu does NOT keep those handles
// live. peerMu protects the peer-keyed maps only (RIBManager.peerMu, rib.go):
// handleReceivedStructured gives it back before its phase 2 removes anything
// (rib_structured.go), and PeerRIB.Remove releases an entry's handles under
// PeerRIB's own lock alone (FamilyRIB.Remove, storage/familyrib.go). So a route
// withdrawn under this walk can leave a row reading a released slot, which a
// release build may already have re-interned with other bytes.
//
// The cover this needs is a REFERENCE, which is what PeerRIB.LookupRetained
// takes for the converted best-path walk (bestSource.Next,
// rib_pipeline_best.go). This walk is not converted, so it does not have one.
// The measurement and the reason it is recorded rather than repaired here are in
// plan/journal/false-synchronization-claim.md.
// releaseBufferedItems gives back the pool references a source's buffer holds.
//
// A source that BUFFERS a peer's routes and then yields them one at a time is
// holding pool handles across the yield, and PeerRIB.Remove releases an entry's
// handles under the PeerRIB lock alone. A released slot goes on its shard's free
// list and a release build re-interns it, so a buffered row read afterwards
// carries another route's attributes rather than an error
// (storage.PeerRIB.LookupRetained states the same rule from the read side).
//
// So the buffer takes a reference when it fills and gives it back when it is
// discarded. The cost is one peer's table of references at a time, which is
// what makes it affordable where retaining a whole walk would not be.
func releaseBufferedItems(items []RouteItem) {
	for i := range items {
		if items[i].HasInEntry {
			items[i].InEntry.Release()
		}
	}
}

// retainEntry takes a reference to a route entry for a buffer that will outlive
// the lock the entry was read under. It answers false when the entry is already
// dead, and the caller then drops the row rather than buffering a handle
// nothing holds.
func retainEntry(entry *storage.RouteEntry) bool {
	return entry.AddRef() == nil
}

type inboundSource struct {
	peers   []inboundPeerRef
	peerIdx int
	items   []RouteItem
	itemIdx int
	count   int
}

func newInboundSource(r *RIBManager, selectorStr string) *inboundSource {
	sel := selector.ParseDefault(selectorStr)
	// Caller holds r.peerMu.RLock (see type doc); read r.bgpPeers under it.
	// RouteItem.Peer is the JSON output string; PeerRIB caches the
	// canonical form so no conversion happens here.
	var peers []inboundPeerRef
	for peer, peerRIB := range r.bgpPeers {
		if sel.Matches(peer) {
			peers = append(peers, inboundPeerRef{peer: peerRIB.PeerAddr(), peerRIB: peerRIB})
		}
	}
	// Sorted so the WALK is deterministic. r.bgpPeers is a map, so the order it
	// yields peers in differs between runs, and a streaming answer cannot sort
	// its rows afterwards without holding them all, which is the thing
	// streaming exists to avoid. Ordering the peer list costs one sort of a
	// small slice and makes the whole answer reproducible: peers in address
	// order, and each peer's routes in the order IterateSorted yields them.
	sort.Slice(peers, func(i, j int) bool { return peers[i].peer < peers[j].peer })
	return &inboundSource{peers: peers}
}

func (s *inboundSource) Next() (RouteItem, bool) {
	for {
		if s.itemIdx < len(s.items) {
			item := s.items[s.itemIdx]
			s.itemIdx++
			s.count++
			return item, true
		}

		if s.peerIdx >= len(s.peers) {
			return RouteItem{}, false
		}

		ref := s.peers[s.peerIdx]
		s.peerIdx++
		releaseBufferedItems(s.items)
		s.items = s.items[:0]
		s.itemIdx = 0

		// The ADD-PATH flags are read BEFORE the iteration. IterateSorted runs
		// its callback under the PeerRIB read lock and IsAddPath takes that same
		// lock, so asking inside the callback deadlocks against any writer that
		// arrives between the two (PeerRIB.IsAddPath).
		addPath := ref.peerRIB.AddPathFamilies()
		ref.peerRIB.IterateSorted(func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool {
			// Retained INSIDE the iteration, which runs under the same PeerRIB
			// lock Remove releases handles under. Taking the reference here is
			// what makes the buffer safe to read after the walk gives that lock
			// back and yields.
			if !retainEntry(&entry) {
				return true
			}
			prefixStr := formatNLRIAsPrefix(fam, nlriBytes, addPath[fam])
			s.items = append(s.items, RouteItem{
				Peer:       ref.peer,
				Family:     fam,
				Prefix:     prefixStr,
				Direction:  rpc.DirectionReceived,
				HasInEntry: true,
				InEntry:    entry,
			})
			return true
		})
	}
}

func (s *inboundSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// release gives back the references the last buffered peer still holds. A
// caller that drains to exhaustion has already released every earlier peer's
// buffer; this covers the tail and every early exit.
func (s *inboundSource) release() {
	releaseBufferedItems(s.items)
	s.items = s.items[:0]
}

// protocolInboundSource iterates over adj-rib-in routes for a single protocol's peers.
// Caller must hold at least RLock on RIBManager.
type protocolInboundSource struct {
	r        *RIBManager
	protoID  redistevents.ProtocolID
	selector string
	peers    []string
	peerIdx  int
	items    []RouteItem
	itemIdx  int
	count    int
}

func newProtocolInboundSource(r *RIBManager, protoID redistevents.ProtocolID, selectorStr string) *protocolInboundSource {
	sel := selector.ParseDefault(selectorStr)
	protoPeers := r.ribInPool[protoID]
	var peers []string
	for peer := range protoPeers {
		if sel.MatchesPeerKey(peer) {
			peers = append(peers, peer)
		}
	}
	// Sorted for the reason newInboundSource states: a streamed answer cannot
	// order its rows after the fact.
	sort.Strings(peers)
	return &protocolInboundSource{r: r, protoID: protoID, selector: selectorStr, peers: peers}
}

func (s *protocolInboundSource) Next() (RouteItem, bool) {
	for {
		if s.itemIdx < len(s.items) {
			item := s.items[s.itemIdx]
			s.itemIdx++
			s.count++
			return item, true
		}

		if s.peerIdx >= len(s.peers) {
			return RouteItem{}, false
		}

		peer := s.peers[s.peerIdx]
		s.peerIdx++
		releaseBufferedItems(s.items)
		s.items = s.items[:0]
		s.itemIdx = 0

		// r.ribInPool is one of the peer-keyed maps peerMu protects, and a
		// streaming walk gives that lock back between rows. The lookup takes it
		// for itself rather than relying on a caller holding it across the whole
		// drain, which is what a walk that yields cannot do.
		s.r.peerMu.RLock()
		peerRIB := s.r.ribInPool[s.protoID][peer]
		s.r.peerMu.RUnlock()
		if peerRIB == nil {
			continue
		}

		// Read the ADD-PATH flags before the iteration, for the reason
		// PeerRIB.IsAddPath states: asking inside the callback takes the read
		// lock the iteration already holds.
		addPath := peerRIB.AddPathFamilies()
		peerRIB.IterateSorted(func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool {
			// Retained under the iteration's own lock: see inboundSource.Next.
			if !retainEntry(&entry) {
				return true
			}
			prefixStr := formatNLRIAsPrefix(fam, nlriBytes, addPath[fam])
			s.items = append(s.items, RouteItem{
				Peer:       peer,
				Family:     fam,
				Prefix:     prefixStr,
				Direction:  rpc.DirectionReceived,
				HasInEntry: true,
				InEntry:    entry,
			})
			return true
		})
	}
}

func (s *protocolInboundSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// release gives back the references the last buffered peer still holds.
func (s *protocolInboundSource) release() {
	releaseBufferedItems(s.items)
	s.items = s.items[:0]
}

// outboundSource iterates over all adj-rib-out routes matching the peer selector.
// Lazy per-peer buffering: materializes one peer's routes at a time.
// Caller (showPipeline) holds r.peerMu.RLock across construction and the full
// drain, so reconstructRoute's pool-handle deref stays mutually exclusive with
// handleSent, which writes r.ribOut under peerMu.Lock (rib.go).
//
// That is a claim about the ADJ-RIB-OUT half alone. The adj-rib-in half of the
// same pipeline does NOT have it: peerMu never covered a PeerRIB entry's handles
// (inboundSource, above, and plan/journal/false-synchronization-claim.md).
type outboundSource struct {
	r       *RIBManager
	peers   []netip.Addr
	peerIdx int
	items   []RouteItem
	itemIdx int
	count   int
}

func newOutboundSource(r *RIBManager, selectorStr string) *outboundSource {
	sel := selector.ParseDefault(selectorStr)
	var peers []netip.Addr
	for peer := range r.ribOut {
		if sel.Matches(peer) {
			peers = append(peers, peer)
		}
	}
	// Sorted for the reason newInboundSource states.
	sort.Slice(peers, func(i, j int) bool { return peers[i].Less(peers[j]) })
	return &outboundSource{r: r, peers: peers}
}

func (s *outboundSource) Next() (RouteItem, bool) {
	for {
		if s.itemIdx < len(s.items) {
			item := s.items[s.itemIdx]
			s.itemIdx++
			s.count++
			return item, true
		}

		if s.peerIdx >= len(s.peers) {
			return RouteItem{}, false
		}

		peer := s.peers[s.peerIdx]
		s.peerIdx++
		s.items = s.items[:0]
		s.itemIdx = 0

		// RouteItem.Peer is the JSON output string: one conversion per peer
		// (not per route), on this cold show-command path.
		peerStr := peer.String()

		// r.ribOut is written by handleSent under peerMu.Lock, so the read
		// takes peerMu for itself: a streaming walk gives the lock back between
		// rows and cannot rely on a caller holding it across the drain.
		//
		// The lock covers the MAP READ and the reconstruction, and nothing
		// else. reconstructRoute copies every wire byte into an owned *Route,
		// so the materialized items carry no pool handle and stay valid once
		// the lock is gone. That is why this half needs no reference where the
		// inbound halves do.
		s.r.peerMu.RLock()
		for fam, familyRoutes := range s.r.ribOut[peer] {
			for key, entry := range familyRoutes {
				rt := reconstructRoute(entry, fam, key, s.r.ribOutSourcePeer(fam, key))
				s.items = append(s.items, RouteItem{
					Peer:      peerStr,
					Family:    fam,
					Prefix:    rt.Prefix,
					Direction: rpc.DirectionSent,
					OutRoute:  rt,
				})
			}
		}
		s.r.peerMu.RUnlock()

		// The source has already materialized this peer's bounded route
		// population, so order it here before yielding. Sorting the complete
		// answer at a terminal would make streamed and buffered paths disagree
		// and would require retaining every peer's routes.
		sort.Slice(s.items, func(i, j int) bool {
			left := &s.items[i]
			right := &s.items[j]
			if left.Family != right.Family {
				return family.FamilyLess(left.Family, right.Family)
			}
			if left.Prefix != right.Prefix {
				return left.Prefix < right.Prefix
			}
			return left.OutRoute.PathID < right.OutRoute.PathID
		})
	}
}

func (s *outboundSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// combinedSource chains inbound then outbound sources.
type combinedSource struct {
	inbound  *inboundSource
	outbound *outboundSource
	inDone   bool
	count    int
}

func newCombinedSource(r *RIBManager, selector string) *combinedSource {
	return &combinedSource{
		inbound:  newInboundSource(r, selector),
		outbound: newOutboundSource(r, selector),
	}
}

// release gives back what either half still holds.
func (s *combinedSource) release() {
	s.inbound.release()
}

func (s *combinedSource) Next() (RouteItem, bool) {
	if !s.inDone {
		item, ok := s.inbound.Next()
		if ok {
			s.count++
			return item, true
		}
		s.inDone = true
	}
	item, ok := s.outbound.Next()
	if ok {
		s.count++
	}
	return item, ok
}

func (s *combinedSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// --- AS path matching ---

// matchASPath tests whether asPath matches the given pattern.
// Pattern syntax:
//   - "64501" — single AS exists anywhere in path
//   - "64501,64502" — contiguous subsequence
//   - "^64501" — anchored at start
//   - "^64501,64502" — anchored contiguous sequence starting at index 0
//   - "" — always matches (no filter)
func matchASPath(asPath []uint32, pattern string) bool {
	if pattern == "" {
		return true
	}

	anchored := false
	p := pattern
	if strings.HasPrefix(p, "^") {
		anchored = true
		p = p[1:]
	}

	// Parse pattern ASNs.
	parts, count := stringsx.SplitCount(p, ",")
	needles := make([]uint32, 0, count)
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		asn, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return false
		}
		needles = append(needles, uint32(asn))
	}

	if len(needles) == 0 {
		return true
	}

	if anchored {
		// Must match starting at index 0
		if len(asPath) < len(needles) {
			return false
		}
		for i, n := range needles {
			if asPath[i] != n {
				return false
			}
		}
		return true
	}

	// Contiguous subsequence search
	if len(needles) > len(asPath) {
		return false
	}
	for i := 0; i <= len(asPath)-len(needles); i++ {
		found := true
		for j, n := range needles {
			if asPath[i+j] != n {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// validatePathPattern checks that every ASN in a path pattern is a valid uint32.
// Returns an error message if invalid, empty string if valid.
func validatePathPattern(pattern string) string {
	p := strings.TrimPrefix(pattern, "^")
	for s := range strings.SplitSeq(p, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := strconv.ParseUint(s, 10, 32); err != nil {
			var tb textbuf.Buffer
			return tb.Str("invalid ASN in path pattern: ").Str(s).String()
		}
	}
	return ""
}

// --- Filter stages ---

// pathFilter filters routes by AS path pattern.
type pathFilter struct {
	upstream pipelineIterator
	pattern  string
	count    int
}

func newPathFilter(upstream pipelineIterator, pattern string) *pathFilter {
	return &pathFilter{upstream: upstream, pattern: pattern}
}

func (f *pathFilter) Next() (RouteItem, bool) {
	for {
		item, ok := f.upstream.Next()
		if !ok {
			return RouteItem{}, false
		}
		asPath := f.getASPath(item)
		if matchASPath(asPath, f.pattern) {
			f.count++
			return item, true
		}
	}
}

func (f *pathFilter) getASPath(item RouteItem) []uint32 {
	return extractASPathFromItem(item)
}

// extractASPathFromItem extracts the AS path from a RouteItem as []uint32.
// For InEntry (adj-rib-in): reads from pool storage via formatASPath.
// For OutRoute (adj-rib-out): reads the ASPath field directly.
func extractASPathFromItem(item RouteItem) []uint32 {
	if item.OutRoute != nil {
		return item.OutRoute.ASPath
	}
	if item.HasInEntry && item.InEntry.HasASPath() {
		if data, err := pool.ASPath.Get(item.InEntry.ASPath); err == nil {
			return formatASPath(data)
		}
	}
	return nil
}

func (f *pathFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

// familyFilter filters routes by address family.
type familyFilter struct {
	upstream pipelineIterator
	family   family.Family
	match    string // original pattern for unregistered/fallback matching
	count    int
}

func newFamilyFilter(upstream pipelineIterator, familyPattern string) *familyFilter {
	f, _ := family.LookupFamily(familyPattern)
	return &familyFilter{upstream: upstream, family: f, match: familyPattern}
}

func (f *familyFilter) Next() (RouteItem, bool) {
	for {
		item, ok := f.upstream.Next()
		if !ok {
			return RouteItem{}, false
		}
		if item.Family == f.family || item.Family.String() == f.match {
			f.count++
			return item, true
		}
	}
}

func (f *familyFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

// prefixFilter filters routes by prefix string match.
type prefixFilter struct {
	upstream pipelineIterator
	pattern  string
	count    int
}

func newPrefixFilter(upstream pipelineIterator, pattern string) *prefixFilter {
	return &prefixFilter{upstream: upstream, pattern: pattern}
}

func (f *prefixFilter) Next() (RouteItem, bool) {
	for {
		item, ok := f.upstream.Next()
		if !ok {
			return RouteItem{}, false
		}
		if strings.HasPrefix(item.Prefix, f.pattern) {
			f.count++
			return item, true
		}
	}
}

func (f *prefixFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

// communityFilter filters routes containing a specific community.
type communityFilter struct {
	upstream  pipelineIterator
	community attribute.Community
	count     int
}

func newCommunityFilter(upstream pipelineIterator, communityStr string) *communityFilter {
	v, _ := attribute.ParseCommunity(communityStr)
	return &communityFilter{upstream: upstream, community: attribute.Community(v)}
}

func (f *communityFilter) Next() (RouteItem, bool) {
	for {
		item, ok := f.upstream.Next()
		if !ok {
			return RouteItem{}, false
		}
		if f.hasCommunity(item) {
			f.count++
			return item, true
		}
	}
}

func (f *communityFilter) hasCommunity(item RouteItem) bool {
	if item.OutRoute != nil {
		return slices.Contains(item.OutRoute.Communities, f.community)
	}
	if item.HasInEntry {
		ib := item.InEntry.GetBundle()
		if ib.HasCommunities() {
			if data, err := pool.Communities.Get(ib.Communities); err == nil {
				return poolContainsCommunity(data, f.community)
			}
		}
	}
	return false
}

func poolContainsCommunity(data []byte, target attribute.Community) bool {
	t := uint32(target)
	for i := 0; i+4 <= len(data); i += 4 {
		v := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		if v == t {
			return true
		}
	}
	return false
}

func (f *communityFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

// matchFilter filters routes by case-insensitive substring match on field values.
type matchFilter struct {
	upstream pipelineIterator
	pattern  string
	count    int
}

func newMatchFilter(upstream pipelineIterator, pattern string) *matchFilter {
	return &matchFilter{upstream: upstream, pattern: strings.ToLower(pattern)}
}

func (f *matchFilter) Next() (RouteItem, bool) {
	for {
		item, ok := f.upstream.Next()
		if !ok {
			return RouteItem{}, false
		}
		if f.matches(item) {
			f.count++
			return item, true
		}
	}
}

func (f *matchFilter) matches(item RouteItem) bool {
	// Check core fields
	if strings.Contains(strings.ToLower(item.Prefix), f.pattern) {
		return true
	}
	if strings.Contains(strings.ToLower(item.Peer), f.pattern) {
		return true
	}
	if strings.Contains(strings.ToLower(item.Family.String()), f.pattern) {
		return true
	}

	if item.OutRoute != nil {
		return f.matchOutRoute(item.OutRoute)
	}
	if item.HasInEntry {
		return f.matchInEntry(item.InEntry)
	}
	return false
}

// matchOutRoute checks OutRoute fields: next-hop, origin, AS-path, communities, MED, local-pref.
func (f *matchFilter) matchOutRoute(rt *Route) bool {
	if strings.Contains(strings.ToLower(rt.NextHop), f.pattern) {
		return true
	}
	if rt.Origin != nil {
		if s := rt.Origin.LowerString(); s != "" && strings.Contains(s, f.pattern) {
			return true
		}
	}
	// AS-path as space-separated ASNs
	for _, asn := range rt.ASPath {
		if strings.Contains(strconv.FormatUint(uint64(asn), 10), f.pattern) {
			return true
		}
	}
	// Communities
	for _, c := range rt.Communities {
		if strings.Contains(strings.ToLower(c.String()), f.pattern) {
			return true
		}
	}
	// MED
	if rt.MED != nil {
		if strings.Contains(strconv.FormatUint(uint64(*rt.MED), 10), f.pattern) {
			return true
		}
	}
	// LOCAL_PREF
	if rt.LocalPreference != nil {
		if strings.Contains(strconv.FormatUint(uint64(*rt.LocalPreference), 10), f.pattern) {
			return true
		}
	}
	return false
}

// matchInEntry checks InEntry pool attributes: next-hop, origin, AS-path, communities, MED, local-pref.
func (f *matchFilter) matchInEntry(entry storage.RouteEntry) bool {
	b := entry.GetBundle()
	if b.HasNextHop() {
		if data, err := pool.NextHop.Get(b.NextHop); err == nil {
			if strings.Contains(strings.ToLower(formatNextHop(data)), f.pattern) {
				return true
			}
		}
	}
	if b.HasOrigin() {
		if data, err := pool.Origin.Get(b.Origin); err == nil {
			if strings.Contains(strings.ToLower(formatOrigin(data)), f.pattern) {
				return true
			}
		}
	}
	// AS-path as space-separated ASNs
	if entry.HasASPath() {
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			for _, asn := range formatASPath(data) {
				if strings.Contains(strconv.FormatUint(uint64(asn), 10), f.pattern) {
					return true
				}
			}
		}
	}
	if b.HasCommunities() {
		if data, err := pool.Communities.Get(b.Communities); err == nil {
			for _, c := range formatCommunities(data) {
				if strings.Contains(strings.ToLower(c), f.pattern) {
					return true
				}
			}
		}
	}
	if b.HasMED() {
		if data, err := pool.MED.Get(b.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				if strings.Contains(strconv.FormatUint(uint64(v), 10), f.pattern) {
					return true
				}
			}
		}
	}
	if b.HasLocalPref() {
		if data, err := pool.LocalPref.Get(b.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				if strings.Contains(strconv.FormatUint(uint64(v), 10), f.pattern) {
					return true
				}
			}
		}
	}
	return false
}

func (f *matchFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

type firstFilter struct {
	upstream pipelineIterator
	limit    int
	seen     int
}

func newFirstFilter(upstream pipelineIterator, arg string) *firstFilter {
	n, _ := strconv.Atoi(arg)
	if n <= 0 {
		n = 1
	}
	return &firstFilter{upstream: upstream, limit: n}
}

func (f *firstFilter) Next() (RouteItem, bool) {
	if f.seen >= f.limit {
		return RouteItem{}, false
	}
	item, ok := f.upstream.Next()
	if !ok {
		return RouteItem{}, false
	}
	f.seen++
	return item, true
}

func (f *firstFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.seen}
}

// lastFilterInitialCap bounds the initial buffer allocation for `last N` so a
// huge user-supplied N does not preallocate gigabytes up front. The buffer
// grows lazily via append (capped by the actual item count), so peak memory is
// min(N, items received), never N alone. See I3 / spec-rib-show-bounded-dump.
const lastFilterInitialCap = 1024

type lastFilter struct {
	upstream pipelineIterator
	limit    int
	buf      []RouteItem // ring buffer; len grows lazily to at most limit
	head     int         // index of the oldest element once the ring is full
	full     bool        // true once buf reached limit and started overwriting
	drained  bool
	idx      int // emission cursor over the drained items

	// retain says this filter takes a reference of its own to the pool entry
	// of every item it keeps. It is set only by a chain whose SOURCE hands an
	// item's reference back on the next pull (bestSource.Next,
	// rib_pipeline_best.go): this filter holds items past that pull, so the row
	// built from one later would otherwise read attributes no reference covers.
	// The caller that sets it MUST call release when it stops pulling.
	retain bool
}

func newLastFilter(upstream pipelineIterator, arg string) *lastFilter {
	// parsePipelineArgs validates N (positive integer) before construction; the
	// clamp is a defensive fallback for any direct caller.
	n, _ := strconv.Atoi(arg)
	if n <= 0 {
		n = 1
	}
	return &lastFilter{upstream: upstream, limit: n}
}

func (f *lastFilter) Next() (RouteItem, bool) {
	if !f.drained {
		f.drain()
	}
	if f.idx >= len(f.buf) {
		return RouteItem{}, false
	}
	// Emit oldest-first. When the ring wrapped, the oldest item is at head.
	pos := f.idx
	if f.full {
		pos = (f.head + f.idx) % f.limit
	}
	f.idx++
	return f.buf[pos], true
}

func (f *lastFilter) drain() {
	f.drained = true
	initCap := min(f.limit, lastFilterInitialCap)
	f.buf = make([]RouteItem, 0, initCap)
	for {
		item, ok := f.upstream.Next()
		if !ok {
			break
		}
		item = f.keep(item)
		if len(f.buf) < f.limit {
			// Still filling: append grows the buffer lazily, bounded by limit.
			f.buf = append(f.buf, item)
		} else {
			// Full: the oldest element leaves the ring, so its reference goes
			// back before it is overwritten.
			f.drop(f.buf[f.head])
			f.buf[f.head] = item
			f.head = (f.head + 1) % f.limit
			f.full = true
		}
	}
}

// keep takes this filter's own reference to item's pool entry, so the entry
// survives the next pull from the source. An entry the RIB has already released
// cannot be referenced, and the item then carries the prefix and its peer with
// no attributes, which is the same answer a route withdrawn mid-walk produces
// (bestSource.Next).
//
// It does nothing when the chain's source keeps its items alive by other means
// (retain), so `show bgp rib` is unchanged.
func (f *lastFilter) keep(item RouteItem) RouteItem {
	if !f.retain || !item.HasInEntry {
		return item
	}
	if err := item.InEntry.AddRef(); err != nil {
		item.HasInEntry = false
		item.InEntry = storage.RouteEntry{}
	}
	return item
}

// drop gives back the reference keep took, for an item leaving the ring.
func (f *lastFilter) drop(item RouteItem) {
	if !f.retain || !item.HasInEntry {
		return
	}
	item.InEntry.Release()
}

// release gives back every reference this filter still holds. It MUST be called
// by the caller that set retain, on every way out of the walk, and it is safe to
// call more than once and safe to call before anything was pulled.
func (f *lastFilter) release() {
	for i := range f.buf {
		f.drop(f.buf[i])
	}
	f.buf = nil
}

func (f *lastFilter) Meta() PipelineMeta {
	if !f.drained {
		f.drain()
	}
	return PipelineMeta{Count: len(f.buf)}
}

// --- Terminal stages ---

// countTerminal drains the upstream and records count in metadata.
// It never yields items.
type countTerminal struct {
	upstream pipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newCountTerminal(upstream pipelineIterator) *countTerminal {
	return &countTerminal{upstream: upstream}
}

func (ct *countTerminal) Next() (RouteItem, bool) {
	if !ct.drained {
		ct.drain()
	}
	return RouteItem{}, false
}

func (ct *countTerminal) drain() {
	ct.drained = true
	count := 0
	for {
		if _, ok := ct.upstream.Next(); !ok {
			break
		}
		count++
	}
	ct.meta.Count = count
}

func (ct *countTerminal) Meta() PipelineMeta {
	if !ct.drained {
		ct.drain()
	}
	return ct.meta
}

// histogramTerminal drains the upstream and counts routes by family and prefix length.
type histogramTerminal struct {
	upstream pipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newHistogramTerminal(upstream pipelineIterator) *histogramTerminal {
	return &histogramTerminal{upstream: upstream}
}

func (h *histogramTerminal) Next() (RouteItem, bool) {
	if !h.drained {
		h.drain()
	}
	return RouteItem{}, false
}

func (h *histogramTerminal) drain() {
	h.drained = true

	// family -> prefix-length -> count
	histogram := make(map[string]map[string]int)
	count := 0

	for {
		item, ok := h.upstream.Next()
		if !ok {
			break
		}
		count++

		prefixLen := extractPrefixLength(item.Prefix)
		fam := item.Family.String()
		if item.Family == (family.Family{}) {
			fam = "unknown"
		}

		byLen, exists := histogram[fam]
		if !exists {
			byLen = make(map[string]int)
			histogram[fam] = byLen
		}
		byLen[prefixLen]++
	}

	h.meta.Count = count
	data, _ := json.Marshal(map[string]any{"histogram": histogram, "count": count})
	h.meta.JSON = string(data)
}

func (h *histogramTerminal) Meta() PipelineMeta {
	if !h.drained {
		h.drain()
	}
	return h.meta
}

// extractPrefixLength returns the "/N" suffix from a prefix string like "10.0.0.0/24".
func extractPrefixLength(prefix string) string {
	idx := strings.LastIndexByte(prefix, '/')
	if idx < 0 {
		return "unknown"
	}
	return prefix[idx+1:]
}

// jsonTerminal drains the upstream, serializes all items to JSON, and records metadata.
type jsonTerminal struct {
	upstream pipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newJSONTerminal(upstream pipelineIterator) *jsonTerminal {
	return &jsonTerminal{upstream: upstream}
}

func (jt *jsonTerminal) Next() (RouteItem, bool) {
	if !jt.drained {
		jt.drain()
	}
	return RouteItem{}, false
}

func (jt *jsonTerminal) drain() {
	jt.drained = true

	rows := make([]map[string]any, 0)
	count := 0
	for {
		item, ok := jt.upstream.Next()
		if !ok {
			break
		}
		count++

		row := serializeRouteItem(item)
		row[rowKeyPeer] = item.Peer
		row[rowKeyDirection] = item.Direction.String()
		rows = append(rows, row)
	}

	jt.meta.Count = count

	// The rows are NOT re-sorted here. They arrive in the sources' own order,
	// which is deterministic because each source sorts its PEER LIST at
	// construction, and that is the same order the streaming path yields. A
	// sort here would give one command two orderings, depending only on whether
	// `| json` was typed, and the streaming path cannot match it: sorting a
	// stream means holding every row, which is the thing streaming avoids.

	data, _ := json.Marshal(map[string]any{"routes": rows})
	jt.meta.JSON = string(data)
}

// Row fields that say which peer a route belongs to and which way it went.
// They are FIELDS rather than levels of an envelope, which is what lets a
// filter select on them: `show bgp rib | peer 10.0.0.1 | direction in` cannot
// be expressed against an object keyed by direction and then by peer.
const (
	rowKeyPeer      = "peer"
	rowKeyDirection = "direction"
)

func (jt *jsonTerminal) Meta() PipelineMeta {
	if !jt.drained {
		jt.drain()
	}
	return jt.meta
}

// serializeRouteItem converts a RouteItem to a JSON-serializable map.
func serializeRouteItem(item RouteItem) map[string]any {
	routeMap := map[string]any{
		"family": item.Family,
		"prefix": item.Prefix,
	}

	if item.HasInEntry {
		enrichRouteMapFromEntry(routeMap, item.InEntry)
	} else if item.OutRoute != nil {
		if item.OutRoute.NextHop != "" {
			routeMap["next-hop"] = item.OutRoute.NextHop
		}
		if item.OutRoute.PathID != 0 {
			routeMap["path-id"] = item.OutRoute.PathID
		}
		enrichRouteMapFromRoute(routeMap, item.OutRoute)
	}

	return routeMap
}

// --- Pipeline builder ---

// Scope keywords for rib show.
const (
	scopeSent         = "sent"
	scopeReceived     = "received"
	scopeSentReceived = "sent-received"
)

// filterPath is the pipeline keyword for AS-path filtering.
const filterPath = "path"

// showPipeline builds and executes a pipeline from command args.
// Called by handleCommand for "show bgp rib" with optional scope + filter stages.
// Holds r.peerMu.RLock across source construction AND the full drain. The whole
// answer is built under it and returned, so nothing is written to a transport
// while it is held (AC-5 of spec-record-answers-3-zero-alloc).
//
// The read lock covers the peer-keyed maps and the adj-rib-out entries
// handleSent writes under peerMu.Lock. It does NOT cover an adj-rib-in entry's
// pool handles, which PeerRIB.Remove releases under PeerRIB's own lock alone:
// see inboundSource and plan/journal/false-synchronization-claim.md. A converted
// walk covers those with a reference instead (bestSource.Next,
// rib_pipeline_best.go), and this one is not converted.
func (r *RIBManager) showPipeline(selector string, args []string) any {
	scope, pipeSelector, stages, errMsg := parsePipelineArgs(args)
	if errMsg != "" {
		return map[string]any{"error": errMsg}
	}
	if pipeSelector != "" {
		selector = pipeSelector
	}

	// A walk with no terminal STREAMS: it answers a row generator and the
	// daemon never holds the whole table as one document
	// (spec-record-answers-3-zero-alloc AC-4).
	if !hasTerminal(stages) {
		return sdk.Records{Key: showRowsEnvelopeKey, Rows: r.showRows(selector, scope, stages)}
	}

	// A terminal builds its whole answer here, so nothing is written to a
	// transport while this runs and the sources may be drained in one go.
	source, release := r.newShowSource(selector, scope)
	defer release()

	// Apply filter stages
	current := source
	for _, stage := range stages {
		current = stage.apply(current)
	}

	// Execute terminal — drain it and return metadata
	meta := current.Meta()

	if meta.JSON != "" {
		if hasTerminalKind(stages, "graph") {
			return meta.JSON
		}
		return json.RawMessage(meta.JSON)
	}

	// count terminal
	return map[string]any{"count": meta.Count}
}

// showRowsEnvelopeKey names the list `show bgp rib` answers with. One envelope,
// one row per route: the rows carry `peer` and `direction` as fields.
const showRowsEnvelopeKey = "routes"

// newShowSource builds the source for a scope and reports the release it owes.
//
// peerMu is held for CONSTRUCTION alone. Every source reads the peer-keyed maps
// to build its peer list, and each one takes peerMu again for itself when it
// materializes a peer inside Next. Holding it across the drain as well would
// nest a reader inside a reader, which deadlocks the moment a writer queues
// between them: Go's RWMutex makes a later RLock wait behind a waiting writer.
func (r *RIBManager) newShowSource(selector, scope string) (pipelineIterator, func()) {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	switch scope {
	case scopeReceived:
		src := newInboundSource(r, selector)
		return src, src.release
	case scopeSent:
		return newOutboundSource(r, selector), func() {}
	default:
		src := newCombinedSource(r, selector)
		return src, src.release
	}
}

// showRows answers the rows of `show bgp rib` one at a time.
//
// No lock is held across a yield. peerMu covers source construction and each
// per-peer materialization inside Next, and the inbound halves hold a pool
// REFERENCE for the rows they have buffered, because PeerRIB.Remove releases an
// entry's handles under the PeerRIB lock alone and a released slot is
// re-interned in a release build.
func (r *RIBManager) showRows(selector, scope string, stages []pipelineStage) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		source, release := r.newShowSource(selector, scope)
		defer release()

		current := source
		for _, stage := range stages {
			current = stage.apply(current)
		}

		for {
			item, ok := current.Next()
			if !ok {
				return
			}
			row := serializeRouteItem(item)
			row[rowKeyPeer] = item.Peer
			row[rowKeyDirection] = item.Direction.String()

			encoded, err := json.Marshal(row)
			if err != nil {
				// The walk continues: one row that cannot be encoded is a fault
				// about that row, not a reason to stop answering the rest.
				var tb textbuf.Buffer
				tb.Str(`{"message":"route row could not be encoded","peer":`).
					Quoted(item.Peer).
					Str(`,"prefix":`).
					Quoted(item.Prefix).
					Byte('}')
				if !yield(rpc.RowRecord{Fault: rpc.RawRow(tb.String())}) {
					return
				}
				continue
			}
			if !yield(rpc.RowRecord{Item: rpc.RawRow(string(encoded))}) {
				return
			}
		}
	}
}

// pipelineStage represents a parsed pipeline stage (filter or terminal).
type pipelineStage struct {
	kind     string
	arg      string
	terminal bool
}

func (s pipelineStage) apply(upstream pipelineIterator) pipelineIterator {
	switch s.kind {
	case filterPath, "aspath":
		return newPathFilter(upstream, s.arg)
	case "prefix":
		return newPrefixFilter(upstream, s.arg)
	case "community":
		return newCommunityFilter(upstream, s.arg)
	case "family":
		return newFamilyFilter(upstream, s.arg)
	case "match":
		return newMatchFilter(upstream, s.arg)
	case "first":
		return newFirstFilter(upstream, s.arg)
	case "last":
		return newLastFilter(upstream, s.arg)
	case "count":
		return newCountTerminal(upstream)
	case "json":
		return newJSONTerminal(upstream)
	case "histogram":
		return newHistogramTerminal(upstream)
	case "graph":
		return newGraphTerminal(upstream)
	}
	// parsePipelineArgs validates all keywords before reaching here,
	// so this is unreachable in normal operation.
	return upstream
}

// filterKeywords are pipeline stage keywords that require a value argument.
var filterKeywords = map[string]bool{
	filterPath:  true,
	"aspath":    true,
	"prefix":    true,
	"community": true,
	"family":    true,
	"match":     true,
	"first":     true,
	"last":      true,
}

// terminalKeywords are pipeline terminal keywords that take no value.
var terminalKeywords = map[string]bool{
	"count":     true,
	"json":      true,
	"histogram": true,
	"graph":     true,
}

// scopeKeywords are positional scope keywords (must appear first).
var scopeKeywords = map[string]string{
	"advertised":    scopeSent,
	"sent":          scopeSent,
	"received":      scopeReceived,
	"sent-received": scopeSentReceived,
}

// parsePipelineArgs parses args into scope, selector, and ordered stage list.
// Returns (scope, peerSelector, stages, errorMessage).
// Validates ordering: filters must precede terminals, and at most one terminal is allowed.
func parsePipelineArgs(args []string) (string, string, []pipelineStage, string) {
	scope := scopeSentReceived
	selector := ""
	var stages []pipelineStage

	i := 0
	sawTerminal := false
	scopeName := ""

	// Check for optional scope keyword at position 0
	if i < len(args) {
		if s, ok := scopeKeywords[args[i]]; ok {
			scope = s
			scopeName = args[i]
			i++
		}
	}

	// Parse remaining args as filter/terminal stages
	for i < len(args) {
		keyword := args[i]
		if _, ok := scopeKeywords[keyword]; ok {
			if sawTerminal {
				var tb textbuf.Buffer
				return "", "", nil, tb.Str("filter after terminal: ").Str(keyword).String()
			}
			if scopeName != "" {
				var tb textbuf.Buffer
				return "", "", nil, tb.Str("multiple route direction filters: ").Str(scopeName).Str(" and ").Str(keyword).String()
			}
			scope = scopeKeywords[keyword]
			scopeName = keyword
			i++
			continue
		}
		if keyword == "peer" {
			if sawTerminal {
				return "", "", nil, "filter after terminal: peer"
			}
			if selector != "" {
				return "", "", nil, "duplicate peer filter"
			}
			i++
			if i >= len(args) {
				return "", "", nil, "peer requires a value"
			}
			selector = args[i]
			i++
			continue
		}

		if filterKeywords[keyword] {
			if sawTerminal {
				var tb textbuf.Buffer
				return "", "", nil, tb.Str("filter after terminal: ").Str(keyword).String()
			}
			i++
			if i >= len(args) {
				var tb textbuf.Buffer
				return "", "", nil, tb.Str(keyword).Str(" requires a value").String()
			}
			if keyword == filterPath {
				if errMsg := validatePathPattern(args[i]); errMsg != "" {
					return "", "", nil, errMsg
				}
			}
			// first/last take a positive count. The client-side ValidatePipes
			// check is bypassed when these are folded into the command for the
			// server-side fast path, so validate here too: reject non-numeric
			// and <= 0 rather than silently clamping (and never preallocate N).
			if keyword == "first" || keyword == "last" {
				if n, err := strconv.Atoi(args[i]); err != nil || n <= 0 {
					var tb textbuf.Buffer
					return "", "", nil, tb.Str(keyword).Str(" requires a positive number").String()
				}
			}
			stages = append(stages, pipelineStage{kind: keyword, arg: args[i]})
			i++
			continue
		}

		if terminalKeywords[keyword] {
			if sawTerminal {
				return "", "", nil, "multiple terminals not allowed"
			}
			sawTerminal = true
			stages = append(stages, pipelineStage{kind: keyword, terminal: true})
			i++
			continue
		}

		var tb textbuf.Buffer
		return "", "", nil, tb.Str("unknown keyword: ").Str(keyword).String()
	}

	return scope, selector, stages, ""
}

// hasTerminal returns true if any stage is a terminal.
func hasTerminal(stages []pipelineStage) bool {
	for _, s := range stages {
		if s.terminal {
			return true
		}
	}
	return false
}

func hasTerminalKind(stages []pipelineStage, kind string) bool {
	for _, s := range stages {
		if s.terminal && s.kind == kind {
			return true
		}
	}
	return false
}
