// Design: docs/architecture/chaos-web-dashboard.md — route flow matrix and heatmap
// Related: state.go — DashboardState references RouteMatrix

package web

import (
	"net/netip"
	"time"
)

// maxPrefixTracking is the maximum number of prefixes tracked in routeOrigins
// and sentTimes. When exceeded, both maps are cleared to reclaim memory.
// Cumulative cell counters are unaffected — only new source correlations are
// temporarily lost until re-populated by subsequent EventRouteSent events.
const maxPrefixTracking = 100_000

// RouteMatrix tracks cumulative traffic volume between peers for the heatmap.
// Uses a sparse map to handle 200+ peers without allocating a 200×200 array.
//
// Matching strategy (two tiers):
//  1. Prefix match (exact): routeOrigins maps prefix→sender. Provides latency.
//  2. Credit match (fallback): per-family send counts allow approximate source
//     inference when the prefix isn't in routeOrigins. No latency, but ensures
//     every receive contributes to cell volume.
type RouteMatrix struct {
	// cells maps (source, dest) to route count.
	cells map[[2]int]int

	// cellLatencySum tracks cumulative latency per cell for averaging.
	cellLatencySum map[[2]int]time.Duration

	// cellLatencyCount tracks number of latency samples per cell.
	cellLatencyCount map[[2]int]int

	// familyCells tracks per-family route counts: family → (src,dst) → count.
	familyCells map[string]map[[2]int]int

	// routeOrigins maps each announced prefix to the peer that sent it.
	// Populated on EventRouteSent, queried on EventRouteReceived.
	// Evicted when size exceeds maxPrefixTracking.
	routeOrigins map[netip.Prefix]int

	// sentTimes maps each prefix to when it was sent (for latency computation).
	// Evicted when size exceeds maxPrefixTracking.
	sentTimes map[netip.Prefix]time.Time

	// peerTotals tracks total route involvement per peer (for top-N sorting).
	peerTotals map[int]int

	// familySent maps family → sender → total routes sent.
	// Unified tracking for ALL families (unicast and non-unicast).
	// Used by credit-based matching when prefix match fails.
	familySent map[string]map[int]int

	// familyCredits maps family → (src,dst) → routes credited so far.
	// Each credit represents one credit-matched send→receive. Credits
	// for a pair never exceed the sender's send count for that family.
	familyCredits map[string]map[[2]int]int

	// Diagnostic counters for the matching pipeline.
	statSentCalls   int // total send calls (unicast + non-unicast)
	statRecvCalls   int // total RecordReceived calls
	statDirectMatch int // receives matched via prefix (exact)
	statCreditMatch int // receives matched via family credit (fallback)
	statUnmatched   int // receives with no available sender
}

// NewRouteMatrix creates an empty route matrix.
func NewRouteMatrix() *RouteMatrix {
	return &RouteMatrix{
		cells:            make(map[[2]int]int),
		cellLatencySum:   make(map[[2]int]time.Duration),
		cellLatencyCount: make(map[[2]int]int),
		familyCells:      make(map[string]map[[2]int]int),
		routeOrigins:     make(map[netip.Prefix]int),
		sentTimes:        make(map[netip.Prefix]time.Time),
		peerTotals:       make(map[int]int),
		familySent:       make(map[string]map[int]int),
		familyCredits:    make(map[string]map[[2]int]int),
	}
}

// RecordSent records that a peer announced a prefix (for source inference).
// Evicts prefix tracking data when the map exceeds maxPrefixTracking to
// bound memory. Also tracks per-family per-peer send counts for the credit
// fallback mechanism. Cumulative cell counters are preserved across evictions.
func (m *RouteMatrix) RecordSent(peerIndex int, prefix netip.Prefix, t time.Time) {
	m.statSentCalls++
	if len(m.routeOrigins) >= maxPrefixTracking {
		m.routeOrigins = make(map[netip.Prefix]int, maxPrefixTracking/2)
		m.sentTimes = make(map[netip.Prefix]time.Time, maxPrefixTracking/2)
	}
	m.routeOrigins[prefix] = peerIndex
	m.sentTimes[prefix] = t

	// Track per-family per-peer send count for credit fallback.
	fam := prefixFamily(prefix)
	m.trackFamilySent(peerIndex, fam)
}

// RecordReceived records that a peer received a prefix and updates the matrix.
// Returns the propagation latency (zero if unavailable). Two-tier matching:
//  1. Prefix match: exact source + latency from routeOrigins/sentTimes.
//  2. Credit match: approximate source from family send counts. No latency.
func (m *RouteMatrix) RecordReceived(destPeer int, prefix netip.Prefix, t time.Time) time.Duration {
	m.statRecvCalls++
	fam := prefixFamily(prefix)

	// Tier 1: exact prefix match.
	if source, ok := m.routeOrigins[prefix]; ok {
		m.statDirectMatch++
		return m.recordMatch(source, destPeer, fam, m.sentTimes[prefix], t)
	}

	// Tier 2: credit-based fallback (volume only, no latency).
	if m.creditMatch(destPeer, fam) {
		m.statCreditMatch++
		return 0
	}
	m.statUnmatched++
	return 0
}

// RecordNonUnicastSent records that a peer sent a non-unicast route in the
// given family. Uses the unified family send tracking.
func (m *RouteMatrix) RecordNonUnicastSent(peerIndex int, family string) {
	m.statSentCalls++
	m.trackFamilySent(peerIndex, family)
}

// RecordNonUnicastReceived records that a peer received a non-unicast route
// in the given family. Uses credit-based matching to infer the source.
func (m *RouteMatrix) RecordNonUnicastReceived(destPeer int, family string) {
	m.statRecvCalls++
	if m.creditMatch(destPeer, family) {
		m.statCreditMatch++
		return
	}
	m.statUnmatched++
}

// trackFamilySent increments the per-family per-peer send counter.
func (m *RouteMatrix) trackFamilySent(peerIndex int, family string) {
	fm := m.familySent[family]
	if fm == nil {
		fm = make(map[int]int)
		m.familySent[family] = fm
	}
	fm[peerIndex]++
}

// creditMatch finds the sender with the most uncredited routes for destPeer
// in the given family, increments the cell, and returns true. Returns false
// if no sender has available credits. Picks the sender with the highest
// remaining budget (sent - credited) for deterministic attribution regardless
// of Go map iteration order. Ties broken by lowest peer index.
func (m *RouteMatrix) creditMatch(destPeer int, family string) bool {
	senders := m.familySent[family]
	if len(senders) == 0 {
		return false
	}
	credits := m.familyCredits[family]
	if credits == nil {
		credits = make(map[[2]int]int)
		m.familyCredits[family] = credits
	}

	// Pick sender with highest remaining budget for stable attribution.
	bestSender := -1
	bestRemaining := 0
	for sender, sent := range senders {
		if sender == destPeer {
			continue
		}
		key := [2]int{sender, destPeer}
		remaining := sent - credits[key]
		if remaining > 0 && (remaining > bestRemaining || (remaining == bestRemaining && (bestSender < 0 || sender < bestSender))) {
			bestSender = sender
			bestRemaining = remaining
		}
	}
	if bestSender < 0 {
		return false
	}

	key := [2]int{bestSender, destPeer}
	credits[key]++
	m.cells[key]++
	m.peerTotals[bestSender]++
	m.peerTotals[destPeer]++
	fc := m.familyCells[family]
	if fc == nil {
		fc = make(map[[2]int]int)
		m.familyCells[family] = fc
	}
	fc[key]++
	return true
}

// recordMatch updates cells, family counts, and latency for a prefix-matched
// source→dest route. Only called from RecordReceived on exact prefix match.
// Also consumes one credit from the family budget so that credit-based
// fallback doesn't double-count routes already matched by prefix.
func (m *RouteMatrix) recordMatch(source, destPeer int, family string, sentAt, recvAt time.Time) time.Duration {
	if source == destPeer {
		return 0 // self-receive; consistent with creditMatch skip
	}

	key := [2]int{source, destPeer}
	m.cells[key]++
	m.peerTotals[source]++
	m.peerTotals[destPeer]++

	// Track per-family counts.
	fc, exists := m.familyCells[family]
	if !exists {
		fc = make(map[[2]int]int)
		m.familyCells[family] = fc
	}
	fc[key]++

	// Consume one credit so credit-based fallback stays consistent.
	credits := m.familyCredits[family]
	if credits == nil {
		credits = make(map[[2]int]int)
		m.familyCredits[family] = credits
	}
	credits[key]++

	// Track latency if we have the send time.
	if sentAt.IsZero() {
		return 0
	}
	latency := recvAt.Sub(sentAt)
	if latency >= 0 {
		m.cellLatencySum[key] += latency
		m.cellLatencyCount[key]++
		return latency
	}
	return 0
}

// RouteMatrixStats holds diagnostic counters for the matching pipeline.
type RouteMatrixStats struct {
	SentCalls   int // total send calls (unicast + non-unicast)
	RecvCalls   int // total receive calls (unicast + non-unicast)
	DirectMatch int // receives matched via exact prefix lookup
	CreditMatch int // receives matched via family credit fallback
	Unmatched   int // receives with no available sender credit
}

// Stats returns diagnostic counters for the matching pipeline.
func (m *RouteMatrix) Stats() RouteMatrixStats {
	return RouteMatrixStats{
		SentCalls:   m.statSentCalls,
		RecvCalls:   m.statRecvCalls,
		DirectMatch: m.statDirectMatch,
		CreditMatch: m.statCreditMatch,
		Unmatched:   m.statUnmatched,
	}
}

// prefixFamily infers the address family from a prefix.
func prefixFamily(p netip.Prefix) string {
	if p.Addr().Is6() {
		return "ipv6/unicast"
	}
	return "ipv4/unicast"
}

// Families returns the list of address families seen in the matrix.
func (m *RouteMatrix) Families() []string {
	fams := make([]string, 0, len(m.familyCells))
	for f := range m.familyCells {
		fams = append(fams, f)
	}
	sortStringSlice(fams)
	return fams
}

// GetByFamily returns the route count for a specific family from source to dest.
// Returns the total count if family is empty.
func (m *RouteMatrix) GetByFamily(source, dest int, family string) int {
	if family == "" {
		return m.cells[[2]int{source, dest}]
	}
	fc := m.familyCells[family]
	if fc == nil {
		return 0
	}
	return fc[[2]int{source, dest}]
}

// sortStringSlice sorts a string slice in ascending order.
func sortStringSlice(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

// AvgLatency returns the average latency for a cell, or 0 if no data.
func (m *RouteMatrix) AvgLatency(source, dest int) time.Duration {
	key := [2]int{source, dest}
	count := m.cellLatencyCount[key]
	if count == 0 {
		return 0
	}
	return m.cellLatencySum[key] / time.Duration(count)
}

// MaxAvgLatency returns the maximum average latency across all cells (for scaling).
func (m *RouteMatrix) MaxAvgLatency() time.Duration {
	var max time.Duration
	for key, count := range m.cellLatencyCount {
		if count == 0 {
			continue
		}
		avg := m.cellLatencySum[key] / time.Duration(count)
		if avg > max {
			max = avg
		}
	}
	return max
}

// Get returns the route count from source to dest.
func (m *RouteMatrix) Get(source, dest int) int {
	return m.cells[[2]int{source, dest}]
}

// TopNPeers returns the top N peer indices sorted by total route involvement.
func (m *RouteMatrix) TopNPeers(n int) []int {
	type peerCount struct {
		index int
		count int
	}
	peers := make([]peerCount, 0, len(m.peerTotals))
	for idx, count := range m.peerTotals {
		peers = append(peers, peerCount{idx, count})
	}

	// Sort descending by count, ascending by index for ties.
	for i := 1; i < len(peers); i++ {
		key := peers[i]
		j := i - 1
		for j >= 0 && (peers[j].count < key.count || (peers[j].count == key.count && peers[j].index > key.index)) {
			peers[j+1] = peers[j]
			j--
		}
		peers[j+1] = key
	}

	if n > len(peers) {
		n = len(peers)
	}
	result := make([]int, n)
	for i := range n {
		result[i] = peers[i].index
	}
	// Sort selected peers by index for stable display order.
	for i := 1; i < len(result); i++ {
		key := result[i]
		j := i - 1
		for j >= 0 && result[j] > key {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = key
	}
	return result
}

// MaxCell returns the maximum cell value across all cells (for scaling colors).
func (m *RouteMatrix) MaxCell() int {
	max := 0
	for _, v := range m.cells {
		if v > max {
			max = v
		}
	}
	return max
}

// Len returns the number of non-zero cells.
func (m *RouteMatrix) Len() int {
	return len(m.cells)
}
