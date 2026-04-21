// Design: docs/architecture/plugin/rib-storage-design.md — iterator pipeline for RIB show commands
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestJSONTerminal)
// Related: rib_topology.go — graph terminal for AS path topology rendering
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: rib_nlri.go — NLRI wire format helpers
package rib

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

// PipelineIterator is the pull-based iterator interface for pipeline stages.
type PipelineIterator interface {
	Next() (RouteItem, bool)
	Meta() PipelineMeta
}

// --- Source iterators ---

// inboundSource iterates over all adj-rib-in routes matching the peer selector.
// Caller must hold at least RLock on RIBManager.
type inboundSource struct {
	r        *RIBManager
	selector string
	peers    []string
	peerIdx  int
	items    []RouteItem // buffered items from current peer
	itemIdx  int
	count    int
}

func newInboundSource(r *RIBManager, selector string) *inboundSource {
	peers := make([]string, 0, len(r.ribInPool))
	for peer := range r.ribInPool {
		if matchesPeer(peer, selector) {
			peers = append(peers, peer)
		}
	}
	return &inboundSource{r: r, selector: selector, peers: peers}
}

func (s *inboundSource) Next() (RouteItem, bool) {
	for {
		// Return buffered items first
		if s.itemIdx < len(s.items) {
			item := s.items[s.itemIdx]
			s.itemIdx++
			s.count++
			return item, true
		}

		// Load next peer
		if s.peerIdx >= len(s.peers) {
			return RouteItem{}, false
		}

		peer := s.peers[s.peerIdx]
		s.peerIdx++
		s.items = s.items[:0]
		s.itemIdx = 0

		peerRIB := s.r.ribInPool[peer]
		if peerRIB == nil {
			continue
		}

		peerRIB.Iterate(func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool {
			prefixStr := formatNLRIAsPrefix(fam, nlriBytes)
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

func (s *inboundSource) Meta() PipelineMeta {
	return PipelineMeta{Count: s.count}
}

// outboundSource iterates over all adj-rib-out routes matching the peer selector.
// Caller must hold at least RLock on RIBManager.
type outboundSource struct {
	r        *RIBManager
	selector string
	items    []RouteItem
	idx      int
	count    int
}

func newOutboundSource(r *RIBManager, selector string) *outboundSource {
	var items []RouteItem
	for peer, peerFamilies := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		for _, familyRoutes := range peerFamilies {
			for _, rt := range familyRoutes {
				items = append(items, RouteItem{
					Peer:      peer,
					Family:    rt.Family,
					Prefix:    rt.Prefix,
					Direction: rpc.DirectionSent,
					OutRoute:  rt,
				})
			}
		}
	}
	return &outboundSource{r: r, selector: selector, items: items}
}

func (s *outboundSource) Next() (RouteItem, bool) {
	if s.idx >= len(s.items) {
		return RouteItem{}, false
	}
	item := s.items[s.idx]
	s.idx++
	s.count++
	return item, true
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

	// Parse pattern ASNs
	parts := strings.Split(p, ",")
	needles := make([]uint32, 0, len(parts))
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
			return fmt.Sprintf("invalid ASN in path pattern: %s", s)
		}
	}
	return ""
}

// --- Filter stages ---

// pathFilter filters routes by AS path pattern.
type pathFilter struct {
	upstream PipelineIterator
	pattern  string
	count    int
}

func newPathFilter(upstream PipelineIterator, pattern string) *pathFilter {
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
	upstream PipelineIterator
	family   family.Family
	match    string // original pattern for unregistered/fallback matching
	count    int
}

func newFamilyFilter(upstream PipelineIterator, familyPattern string) *familyFilter {
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
	upstream PipelineIterator
	pattern  string
	count    int
}

func newPrefixFilter(upstream PipelineIterator, pattern string) *prefixFilter {
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
	upstream  PipelineIterator
	community string
	count     int
}

func newCommunityFilter(upstream PipelineIterator, community string) *communityFilter {
	return &communityFilter{upstream: upstream, community: community}
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
	if item.HasInEntry && item.InEntry.HasCommunities() {
		if data, err := pool.Communities.Get(item.InEntry.Communities); err == nil {
			return slices.Contains(formatCommunities(data), f.community)
		}
	}
	return false
}

func (f *communityFilter) Meta() PipelineMeta {
	return PipelineMeta{Count: f.count}
}

// matchFilter filters routes by case-insensitive substring match on field values.
type matchFilter struct {
	upstream PipelineIterator
	pattern  string
	count    int
}

func newMatchFilter(upstream PipelineIterator, pattern string) *matchFilter {
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
	if s := rt.Origin.LowerString(); s != "" && strings.Contains(s, f.pattern) {
		return true
	}
	// AS-path as space-separated ASNs
	for _, asn := range rt.ASPath {
		if strings.Contains(strconv.FormatUint(uint64(asn), 10), f.pattern) {
			return true
		}
	}
	// Communities
	for _, c := range rt.Communities {
		if strings.Contains(strings.ToLower(c), f.pattern) {
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
	// Next-hop
	if entry.HasNextHop() {
		if data, err := pool.NextHop.Get(entry.NextHop); err == nil {
			if strings.Contains(strings.ToLower(formatNextHop(data)), f.pattern) {
				return true
			}
		}
	}
	// Origin
	if entry.HasOrigin() {
		if data, err := pool.Origin.Get(entry.Origin); err == nil {
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
	// Communities
	if entry.HasCommunities() {
		if data, err := pool.Communities.Get(entry.Communities); err == nil {
			for _, c := range formatCommunities(data) {
				if strings.Contains(strings.ToLower(c), f.pattern) {
					return true
				}
			}
		}
	}
	// MED
	if entry.HasMED() {
		if data, err := pool.MED.Get(entry.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				if strings.Contains(strconv.FormatUint(uint64(v), 10), f.pattern) {
					return true
				}
			}
		}
	}
	// LOCAL_PREF
	if entry.HasLocalPref() {
		if data, err := pool.LocalPref.Get(entry.LocalPref); err == nil {
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

// --- Terminal stages ---

// countTerminal drains the upstream and records count in metadata.
// It never yields items.
type countTerminal struct {
	upstream PipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newCountTerminal(upstream PipelineIterator) *countTerminal {
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
	upstream PipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newPrefixSummaryTerminal(upstream PipelineIterator) *prefixSummaryTerminal {
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
	upstream PipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newJSONTerminal(upstream PipelineIterator) *jsonTerminal {
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
// Called by handleCommand for "bgp rib show" with optional scope + filter stages.
// Returns JSON string result.
func (r *RIBManager) showPipeline(selector string, args []string) string {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	scope, stages, errMsg := parsePipelineArgs(args)
	if errMsg != "" {
		data, _ := json.Marshal(map[string]any{"error": errMsg})
		return string(data)
	}

	// Create source based on scope
	var source PipelineIterator
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
		return meta.JSON
	}

	// Execute terminal — drain it and return metadata
	meta := current.Meta()

	if meta.JSON != "" {
		return meta.JSON
	}

	// count terminal
	data, _ := json.Marshal(map[string]any{"count": meta.Count})
	return string(data)
}

// pipelineStage represents a parsed pipeline stage (filter or terminal).
type pipelineStage struct {
	kind     string
	arg      string
	terminal bool
}

func (s pipelineStage) apply(upstream PipelineIterator) PipelineIterator {
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
	"sent":          scopeSent,
	"received":      scopeReceived,
	"sent-received": scopeSentReceived,
}

// parsePipelineArgs parses args into scope + ordered stage list.
// Returns (scope, stages, errorMessage).
// Validates ordering: filters must precede terminals, and at most one terminal is allowed.
func parsePipelineArgs(args []string) (string, []pipelineStage, string) {
	scope := scopeSentReceived
	var stages []pipelineStage

	i := 0
	sawTerminal := false

	// Check for optional scope keyword at position 0
	if i < len(args) {
		if s, ok := scopeKeywords[args[i]]; ok {
			scope = s
			i++
		}
	}

	// Parse remaining args as filter/terminal stages
	for i < len(args) {
		keyword := args[i]

		if filterKeywords[keyword] {
			if sawTerminal {
				return "", nil, fmt.Sprintf("filter after terminal: %s", keyword)
			}
			i++
			if i >= len(args) {
				return "", nil, fmt.Sprintf("%s requires a value", keyword)
			}
			if keyword == filterPath {
				if errMsg := validatePathPattern(args[i]); errMsg != "" {
					return "", nil, errMsg
				}
			}
			stages = append(stages, pipelineStage{kind: keyword, arg: args[i]})
			i++
			continue
		}

		if terminalKeywords[keyword] {
			if sawTerminal {
				return "", nil, "multiple terminals not allowed"
			}
			sawTerminal = true
			stages = append(stages, pipelineStage{kind: keyword, terminal: true})
			i++
			continue
		}

		return "", nil, fmt.Sprintf("unknown keyword: %s", keyword)
	}

	return scope, stages, ""
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
