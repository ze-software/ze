// Design: docs/architecture/plugin/rib-storage-design.md — iterator pipeline for RIB show commands
// RFC: rfc/short/rfc4271.md -- route attributes surfaced by show pipelines
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestJSONTerminal)
// Related: rib_topology.go — graph terminal for AS path topology rendering
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: rib_nlri.go — NLRI wire format helpers
package rib

import (
	"encoding/json"
	"net/netip"
	"slices"
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
	JSON  string // set by json, prefix-summary, and graph terminals
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
// drain. PeerRIB iteration uses PeerRIB's own lock, and the downstream
// filters/terminals deref the captured InEntry pool handles -- the outer
// peerMu.RLock keeps those handles live against handleReceived, which releases
// them under peerMu.Lock (I2).
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
		s.items = s.items[:0]
		s.itemIdx = 0

		ref.peerRIB.IterateSorted(func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool {
			prefixStr := formatNLRIAsPrefix(fam, nlriBytes, ref.peerRIB.IsAddPath(fam))
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
		s.items = s.items[:0]
		s.itemIdx = 0

		protoPeers := s.r.ribInPool[s.protoID]
		peerRIB := protoPeers[peer]
		if peerRIB == nil {
			continue
		}

		peerRIB.IterateSorted(func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool {
			prefixStr := formatNLRIAsPrefix(fam, nlriBytes, peerRIB.IsAddPath(fam))
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

// outboundSource iterates over all adj-rib-out routes matching the peer selector.
// Lazy per-peer buffering: materializes one peer's routes at a time.
// Caller (showPipeline) holds r.peerMu.RLock across construction and the full
// drain, so reconstructRoute's pool-handle deref stays mutually exclusive with
// the writers that release those handles -- handleSent holds peerMu.Lock (I2).
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

		// Reconstruct under the caller's held peerMu.RLock: reconstructRoute
		// copies all wire bytes into an owned *Route, so the materialized
		// items remain valid for the rest of the drain. (I2)
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
		if len(f.buf) < f.limit {
			// Still filling: append grows the buffer lazily, bounded by limit.
			f.buf = append(f.buf, item)
		} else {
			// Full: overwrite the oldest element in O(1) and advance head.
			f.buf[f.head] = item
			f.head = (f.head + 1) % f.limit
			f.full = true
		}
	}
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

// prefixSummaryTerminal drains the upstream and counts routes by family and prefix length.
type prefixSummaryTerminal struct {
	upstream pipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newPrefixSummaryTerminal(upstream pipelineIterator) *prefixSummaryTerminal {
	return &prefixSummaryTerminal{upstream: upstream}
}

func (ps *prefixSummaryTerminal) Next() (RouteItem, bool) {
	if !ps.drained {
		ps.drain()
	}
	return RouteItem{}, false
}

func (ps *prefixSummaryTerminal) drain() {
	ps.drained = true

	// family -> prefix-length -> count
	summary := make(map[string]map[string]int)
	count := 0

	for {
		item, ok := ps.upstream.Next()
		if !ok {
			break
		}
		count++

		prefixLen := extractPrefixLength(item.Prefix)
		fam := item.Family.String()
		if item.Family == (family.Family{}) {
			fam = "unknown"
		}

		byLen, exists := summary[fam]
		if !exists {
			byLen = make(map[string]int)
			summary[fam] = byLen
		}
		byLen[prefixLen]++
	}

	ps.meta.Count = count
	data, _ := json.Marshal(map[string]any{"prefix-summary": summary, "count": count})
	ps.meta.JSON = string(data)
}

func (ps *prefixSummaryTerminal) Meta() PipelineMeta {
	if !ps.drained {
		ps.drain()
	}
	return ps.meta
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
	// Group by peer and direction
	type peerRoutes struct {
		received []map[string]any
		sent     []map[string]any
	}
	peers := make(map[string]*peerRoutes)

	count := 0
	for {
		item, ok := jt.upstream.Next()
		if !ok {
			break
		}
		count++

		pr, exists := peers[item.Peer]
		if !exists {
			pr = &peerRoutes{}
			peers[item.Peer] = pr
		}

		routeMap := serializeRouteItem(item)
		if item.Direction == rpc.DirectionReceived {
			pr.received = append(pr.received, routeMap)
		} else {
			pr.sent = append(pr.sent, routeMap)
		}
	}

	jt.meta.Count = count

	// Build JSON output
	result := make(map[string]any)

	// Add received routes (adj-rib-in)
	ribIn := make(map[string][]map[string]any)
	for peer, pr := range peers {
		if len(pr.received) > 0 {
			ribIn[peer] = pr.received
		}
	}
	if len(ribIn) > 0 {
		result["adj-rib-in"] = ribIn
	}

	// Add sent routes (adj-rib-out)
	ribOut := make(map[string][]map[string]any)
	for peer, pr := range peers {
		if len(pr.sent) > 0 {
			ribOut[peer] = pr.sent
		}
	}
	if len(ribOut) > 0 {
		result["adj-rib-out"] = ribOut
	}

	data, _ := json.Marshal(result)
	jt.meta.JSON = string(data)
}

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
// Holds r.peerMu.RLock across source construction AND the full drain: the
// sources carry pool handles (ribOut entries, adj-rib-in bundle handles) that
// filters/terminals dereference lazily, and the writers that release those
// handles (handleSent/handleReceived) hold peerMu.Lock, so the read lock must
// span the whole pipeline to keep handles live (I2).
func (r *RIBManager) showPipeline(selector string, args []string) any {
	scope, pipeSelector, stages, errMsg := parsePipelineArgs(args)
	if errMsg != "" {
		return map[string]any{"error": errMsg}
	}
	if pipeSelector != "" {
		selector = pipeSelector
	}

	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	// Create source based on scope
	var source pipelineIterator
	switch scope {
	case scopeReceived:
		source = newInboundSource(r, selector)
	case scopeSent:
		source = newOutboundSource(r, selector)
	case scopeSentReceived:
		source = newCombinedSource(r, selector)
	}

	// Apply filter stages
	current := source
	for _, stage := range stages {
		current = stage.apply(current)
	}

	// If no terminal was specified, default to json
	if !hasTerminal(stages) {
		jt := newJSONTerminal(current)
		meta := jt.Meta()
		return json.RawMessage(meta.JSON)
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
	case "prefix-summary":
		return newPrefixSummaryTerminal(upstream)
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
	"count":          true,
	"json":           true,
	"prefix-summary": true,
	"graph":          true,
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
