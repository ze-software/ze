// Package web implements a live HTMX dashboard for ze-bgp-chaos.
package web

import (
	"fmt"
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// ControlCommand represents a command from the web dashboard to the orchestrator.
type ControlCommand struct {
	// Type identifies the command: "pause", "resume", "rate", "trigger", "stop".
	Type string

	// Rate is the new chaos rate for "rate" commands (0.0-1.0).
	Rate float64

	// Trigger holds details for "trigger" commands.
	Trigger *ManualTrigger
}

// ManualTrigger describes a manually-triggered chaos action.
type ManualTrigger struct {
	// ActionType is the kebab-case action name (e.g., "tcp-disconnect").
	ActionType string

	// Peers is the list of target peer indices. Empty means random selection.
	Peers []int

	// Params holds action-specific parameters (e.g., "count": "500").
	Params map[string]string
}

// ControlState tracks the current control status for UI rendering.
type ControlState struct {
	Paused           bool
	Rate             float64
	Status           string // "running", "paused", "stopped", "restarting"
	RestartAvailable bool   // true when restart channel is configured
}

// ControlLogger logs dashboard control events to the NDJSON event log.
// Implemented by report.JSONLog. When nil, control events are not logged.
type ControlLogger interface {
	LogControl(command, value string, t time.Time)
}

// PropertyBadge holds a property result for dashboard display.
type PropertyBadge struct {
	Name       string
	Pass       bool
	Violations []string // Human-readable violation messages.
}

// PeerStatus represents the current state of a simulated peer.
type PeerStatus int

const (
	// PeerIdle means the peer has not connected yet.
	PeerIdle PeerStatus = iota
	// PeerUp means the peer has an established BGP session.
	PeerUp
	// PeerDown means the peer's session is closed.
	PeerDown
	// PeerReconnecting means the peer is reconnecting after chaos.
	PeerReconnecting
)

// String returns a human-readable status label.
func (s PeerStatus) String() string {
	switch s {
	case PeerIdle:
		return "idle"
	case PeerUp:
		return "up"
	case PeerDown:
		return "down"
	case PeerReconnecting:
		return "reconnecting"
	}
	return "idle"
}

// CSSClass returns the CSS class for status coloring.
func (s PeerStatus) CSSClass() string {
	switch s {
	case PeerIdle:
		return "status-idle"
	case PeerUp:
		return "status-up"
	case PeerDown:
		return "status-down"
	case PeerReconnecting:
		return "status-reconnecting"
	}
	return "status-idle"
}

// PeerState holds the current state and counters for a single peer.
type PeerState struct {
	Index       int
	Status      PeerStatus
	RoutesSent  int
	RoutesRecv  int
	Missing     int
	LastEvent   peer.EventType
	LastEventAt time.Time
	ChaosCount  int
	Reconnects  int
	Events      *RingBuffer[peer.Event]
}

// NewPeerState creates a PeerState with the given index and event buffer size.
func NewPeerState(index, bufSize int) *PeerState {
	return &PeerState{
		Index:  index,
		Events: NewRingBuffer[peer.Event](bufSize),
	}
}

// RingBuffer is a fixed-size circular buffer.
type RingBuffer[T any] struct {
	items []T
	head  int // next write position
	count int
	cap   int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer[T]{
		items: make([]T, capacity),
		cap:   capacity,
	}
}

// Push adds an item, overwriting the oldest if full.
func (r *RingBuffer[T]) Push(item T) {
	r.items[r.head] = item
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

// Len returns the number of items currently stored.
func (r *RingBuffer[T]) Len() int {
	return r.count
}

// Cap returns the capacity.
func (r *RingBuffer[T]) Cap() int {
	return r.cap
}

// All returns all items in insertion order (oldest first).
func (r *RingBuffer[T]) All() []T {
	if r.count == 0 {
		return nil
	}
	result := make([]T, r.count)
	start := (r.head - r.count + r.cap) % r.cap
	for i := range r.count {
		result[i] = r.items[(start+i)%r.cap]
	}
	return result
}

// Latest returns the most recently added item and true, or zero value and false if empty.
func (r *RingBuffer[T]) Latest() (T, bool) {
	if r.count == 0 {
		var zero T
		return zero, false
	}
	idx := (r.head - 1 + r.cap) % r.cap
	return r.items[idx], true
}

// ActiveSetEntry tracks a peer's presence in the active set.
type ActiveSetEntry struct {
	PeerIndex  int
	Pinned     bool
	PromotedAt time.Time
	LastActive time.Time
	Priority   PromotionPriority
}

// PromotionPriority indicates why a peer was promoted to the active set.
type PromotionPriority int

const (
	// PriorityManual means the user manually added or pinned the peer.
	PriorityManual PromotionPriority = iota
	// PriorityLow is for minor events (missing routes).
	PriorityLow
	// PriorityMedium is for notable events (chaos, reconnecting, withdrawals).
	PriorityMedium
	// PriorityHigh is for critical events (disconnected, error).
	PriorityHigh
)

// ActiveSet manages the visible subset of peers in the dashboard table.
type ActiveSet struct {
	MaxVisible int
	entries    map[int]*ActiveSetEntry // peerIndex -> entry
}

// NewActiveSet creates an active set with the given capacity.
func NewActiveSet(maxVisible int) *ActiveSet {
	if maxVisible < 10 {
		maxVisible = 10
	}
	return &ActiveSet{
		MaxVisible: maxVisible,
		entries:    make(map[int]*ActiveSetEntry),
	}
}

// Promote adds or refreshes a peer in the active set with the given priority.
// Returns true if the peer was newly added (not already present).
func (a *ActiveSet) Promote(peerIndex int, priority PromotionPriority, now time.Time) bool {
	if e, ok := a.entries[peerIndex]; ok {
		e.LastActive = now
		if priority > e.Priority {
			e.Priority = priority
		}
		return false
	}

	// Evict oldest non-pinned peer if at capacity.
	if len(a.entries) >= a.MaxVisible {
		evicted := a.findEvictionCandidate(now)
		if evicted < 0 {
			return false // All pinned, can't evict.
		}
		delete(a.entries, evicted)
	}

	a.entries[peerIndex] = &ActiveSetEntry{
		PeerIndex:  peerIndex,
		PromotedAt: now,
		LastActive: now,
		Priority:   priority,
	}
	return true
}

// findEvictionCandidate returns the peer index of the best eviction target,
// or -1 if all peers are pinned.
func (a *ActiveSet) findEvictionCandidate(_ time.Time) int {
	var (
		candidate = -1
		oldest    time.Time
	)
	for idx, e := range a.entries {
		if e.Pinned {
			continue
		}
		if candidate < 0 || e.LastActive.Before(oldest) {
			candidate = idx
			oldest = e.LastActive
		}
	}
	return candidate
}

// Pin marks a peer as pinned. If not in the active set, promotes it first.
func (a *ActiveSet) Pin(peerIndex int, now time.Time) {
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = true
		return
	}
	a.Promote(peerIndex, PriorityManual, now)
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = true
	}
}

// Unpin removes the pin from a peer, making it subject to decay.
func (a *ActiveSet) Unpin(peerIndex int) {
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = false
	}
}

// IsPinned returns true if the peer is pinned.
func (a *ActiveSet) IsPinned(peerIndex int) bool {
	e, ok := a.entries[peerIndex]
	return ok && e.Pinned
}

// Contains returns true if the peer is in the active set.
func (a *ActiveSet) Contains(peerIndex int) bool {
	_, ok := a.entries[peerIndex]
	return ok
}

// Decay removes expired non-pinned peers from the active set.
// Returns the indices of removed peers.
func (a *ActiveSet) Decay(now time.Time) []int {
	ttl := a.adaptiveTTL()
	var removed []int
	for idx, e := range a.entries {
		if e.Pinned {
			continue
		}
		if now.Sub(e.LastActive) > ttl {
			removed = append(removed, idx)
		}
	}
	for _, idx := range removed {
		delete(a.entries, idx)
	}
	return removed
}

// adaptiveTTL returns the current decay TTL based on fill ratio.
func (a *ActiveSet) adaptiveTTL() time.Duration {
	fill := float64(len(a.entries)) / float64(a.MaxVisible)
	switch {
	case fill > 0.8:
		return 5 * time.Second
	case fill > 0.5:
		return 30 * time.Second
	default:
		return 120 * time.Second
	}
}

// AdaptiveTTL returns the current decay TTL (exported for testing/display).
func (a *ActiveSet) AdaptiveTTL() time.Duration {
	return a.adaptiveTTL()
}

// Len returns the number of peers in the active set.
func (a *ActiveSet) Len() int {
	return len(a.entries)
}

// Indices returns all peer indices in the active set.
func (a *ActiveSet) Indices() []int {
	result := make([]int, 0, len(a.entries))
	for idx := range a.entries {
		result = append(result, idx)
	}
	return result
}

// Entry returns the entry for a peer, or nil if not in the active set.
func (a *ActiveSet) Entry(peerIndex int) *ActiveSetEntry {
	return a.entries[peerIndex]
}

// PromotionPriorityForEvent returns the priority for auto-promoting a peer
// based on the event type.
func PromotionPriorityForEvent(evType peer.EventType) (PromotionPriority, bool) {
	switch evType {
	case peer.EventDisconnected, peer.EventError:
		return PriorityHigh, true
	case peer.EventChaosExecuted, peer.EventReconnecting, peer.EventWithdrawalSent:
		return PriorityMedium, true
	case peer.EventRouteWithdrawn:
		return PriorityLow, true
	case peer.EventEstablished, peer.EventRouteSent, peer.EventRouteReceived, peer.EventEORSent:
		return 0, false
	}
	return 0, false
}

// ConvergenceBucket defines a latency bucket for the convergence histogram.
type ConvergenceBucket struct {
	Label string        // Human-readable label (e.g., "0-5ms").
	Min   time.Duration // Inclusive lower bound.
	Max   time.Duration // Exclusive upper bound (0 means unbounded).
	Count int           // Number of routes in this bucket.
}

// convergenceBucketDefs defines the 9 histogram bucket ranges.
var convergenceBucketDefs = []struct {
	Label string
	Min   time.Duration
	Max   time.Duration
}{
	{"0-5ms", 0, 5 * time.Millisecond},
	{"5-10ms", 5 * time.Millisecond, 10 * time.Millisecond},
	{"10-25ms", 10 * time.Millisecond, 25 * time.Millisecond},
	{"25-50ms", 25 * time.Millisecond, 50 * time.Millisecond},
	{"50-100ms", 50 * time.Millisecond, 100 * time.Millisecond},
	{"100-250ms", 100 * time.Millisecond, 250 * time.Millisecond},
	{"250-500ms", 250 * time.Millisecond, 500 * time.Millisecond},
	{"500ms-1s", 500 * time.Millisecond, time.Second},
	{">1s", time.Second, 0},
}

// ConvergenceHistogram tracks route propagation latency distribution.
type ConvergenceHistogram struct {
	Buckets   [9]ConvergenceBucket
	Total     int
	Sum       time.Duration // For computing average.
	Min       time.Duration
	Max       time.Duration
	SlowCount int // Routes exceeding 1s.
}

// NewConvergenceHistogram creates an initialized histogram with the 9 bucket definitions.
func NewConvergenceHistogram() *ConvergenceHistogram {
	h := &ConvergenceHistogram{}
	for i, def := range convergenceBucketDefs {
		h.Buckets[i] = ConvergenceBucket{
			Label: def.Label,
			Min:   def.Min,
			Max:   def.Max,
		}
	}
	return h
}

// Record adds a latency measurement to the appropriate bucket and updates stats.
func (h *ConvergenceHistogram) Record(latency time.Duration) {
	h.Total++
	h.Sum += latency

	if h.Total == 1 || latency < h.Min {
		h.Min = latency
	}
	if latency > h.Max {
		h.Max = latency
	}
	if latency >= time.Second {
		h.SlowCount++
	}

	for i := range h.Buckets {
		b := &h.Buckets[i]
		if latency >= b.Min && (b.Max == 0 || latency < b.Max) {
			b.Count++
			return
		}
	}
}

// Avg returns the average latency, or 0 if no measurements.
func (h *ConvergenceHistogram) Avg() time.Duration {
	if h.Total == 0 {
		return 0
	}
	return h.Sum / time.Duration(h.Total)
}

// MaxCount returns the highest count across all buckets (for scaling bar heights).
func (h *ConvergenceHistogram) MaxCount() int {
	max := 0
	for _, b := range h.Buckets {
		if b.Count > max {
			max = b.Count
		}
	}
	return max
}

// ChaosHistoryEntry records a single chaos action for the timeline.
type ChaosHistoryEntry struct {
	Time      time.Time
	PeerIndex int
	Action    string
}

// PeerStateTransition records a peer status change for the timeline.
type PeerStateTransition struct {
	Time   time.Time
	Status PeerStatus
}

// maxPrefixTracking is the maximum number of prefixes tracked in routeOrigins
// and sentTimes. When exceeded, both maps are cleared to reclaim memory.
// Cumulative cell counters are unaffected — only new source correlations are
// temporarily lost until re-populated by subsequent EventRouteSent events.
const maxPrefixTracking = 100_000

// RouteMatrix tracks source→destination route counts for the heatmap.
// Uses a sparse map to handle 200+ peers without allocating a 200×200 array.
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
	}
}

// RecordSent records that a peer announced a prefix (for source inference).
// Evicts all prefix tracking data when the map exceeds maxPrefixTracking to
// bound memory. Cumulative cell counters are preserved.
func (m *RouteMatrix) RecordSent(peerIndex int, prefix netip.Prefix, t time.Time) {
	if len(m.routeOrigins) >= maxPrefixTracking {
		m.routeOrigins = make(map[netip.Prefix]int, maxPrefixTracking/2)
		m.sentTimes = make(map[netip.Prefix]time.Time, maxPrefixTracking/2)
	}
	m.routeOrigins[prefix] = peerIndex
	m.sentTimes[prefix] = t
}

// RecordReceived records that a peer received a prefix and updates the matrix.
// Returns whether the source was found and the propagation latency (zero if
// no send time is available). Callers use the latency to feed the convergence
// histogram.
func (m *RouteMatrix) RecordReceived(destPeer int, prefix netip.Prefix, t time.Time) (found bool, latency time.Duration) {
	source, ok := m.routeOrigins[prefix]
	if !ok {
		return false, 0
	}
	key := [2]int{source, destPeer}
	m.cells[key]++
	m.peerTotals[source]++
	m.peerTotals[destPeer]++

	// Track per-family counts.
	fam := prefixFamily(prefix)
	fc, exists := m.familyCells[fam]
	if !exists {
		fc = make(map[[2]int]int)
		m.familyCells[fam] = fc
	}
	fc[key]++

	// Track latency if we have the send time.
	if sentAt, haveSent := m.sentTimes[prefix]; haveSent {
		latency = t.Sub(sentAt)
		if latency >= 0 {
			m.cellLatencySum[key] += latency
			m.cellLatencyCount[key]++
		} else {
			latency = 0
		}
	}
	return true, latency
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

// DashboardState holds all mutable state for the web dashboard, protected by a RWMutex.
type DashboardState struct {
	mu sync.RWMutex

	// Per-peer state, indexed by peer index.
	Peers map[int]*PeerState

	// Active set for table visibility.
	Active *ActiveSet

	// Global counters.
	TotalAnnounced  int
	TotalReceived   int
	TotalMissing    int
	TotalChaos      int
	TotalReconnects int
	TotalWithdrawn  int // Withdrawals received (EventRouteWithdrawn).
	TotalWdrawSent  int // Withdrawals sent by chaos peers (EventWithdrawalSent).
	PeersUp         int

	// Run metadata.
	StartTime time.Time
	PeerCount int

	// Event ring buffer (global, for event stream tab).
	GlobalEvents *RingBuffer[peer.Event]

	// Convergence histogram for route propagation latency.
	Convergence *ConvergenceHistogram

	// Chaos history for timeline visualization.
	ChaosHistory []ChaosHistoryEntry

	// Per-peer state transitions for timeline visualization.
	PeerTransitions map[int][]PeerStateTransition

	// Route flow matrix: routeMatrix[source][dest] = count of routes received.
	RouteMatrix *RouteMatrix

	// Control state for the UI.
	Control ControlState

	// Property badges for display.
	Properties []PropertyBadge

	// Warmup duration for chaos timeline rendering.
	WarmupDuration time.Duration

	// ConvergenceDeadline for histogram deadline marker.
	ConvergenceDeadline time.Duration

	// Dirty flags for SSE — set by ProcessEvent, read by SSE goroutine.
	dirtyPeers  map[int]bool
	dirtyGlobal bool
}

// NewDashboardState creates a new dashboard state.
func NewDashboardState(peerCount, maxVisible, eventBufSize int) *DashboardState {
	peers := make(map[int]*PeerState, peerCount)
	for i := range peerCount {
		peers[i] = NewPeerState(i, 100) // 100 events per peer ring buffer
	}
	return &DashboardState{
		Peers:           peers,
		Active:          NewActiveSet(maxVisible),
		StartTime:       time.Now(),
		PeerCount:       peerCount,
		GlobalEvents:    NewRingBuffer[peer.Event](eventBufSize),
		Convergence:     NewConvergenceHistogram(),
		PeerTransitions: make(map[int][]PeerStateTransition, peerCount),
		RouteMatrix:     NewRouteMatrix(),
		dirtyPeers:      make(map[int]bool),
	}
}

// RLock acquires a read lock on the state.
func (s *DashboardState) RLock() { s.mu.RLock() }

// RUnlock releases the read lock.
func (s *DashboardState) RUnlock() { s.mu.RUnlock() }

// MarkDirty sets the dirty flag for a peer (and global).
func (s *DashboardState) MarkDirty(peerIndex int) {
	s.dirtyPeers[peerIndex] = true
	s.dirtyGlobal = true
}

// ConsumeDirty returns which peers are dirty and resets the flags.
// Must be called under write lock.
func (s *DashboardState) ConsumeDirty() (peers map[int]bool, global bool) {
	peers = s.dirtyPeers
	global = s.dirtyGlobal
	s.dirtyPeers = make(map[int]bool)
	s.dirtyGlobal = false
	return peers, global
}

// FormatDuration formats a duration in a compact human-readable form.
// Reimplemented here to avoid coupling with unexported report.formatDuration.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(time.Millisecond).String()
}
