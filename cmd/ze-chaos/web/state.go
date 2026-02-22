// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI
// Related: state_activeset.go — active set peer visibility management
// Related: state_routematrix.go — route flow matrix and heatmap tracking
//
// Package web implements a live HTMX dashboard for ze-chaos.
package web

import (
	"fmt"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
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

	// Route dynamics control state.
	RoutePaused bool
	RouteRate   float64
	RouteStatus string // "running", "paused", "stopped", "disabled"

	// Speed control state (in-process mode only).
	SpeedFactor    int  // Current speed factor (1, 10, 100, 1000).
	SpeedAvailable bool // true when speed control is enabled.
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
		return cssReconnecting
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

	// Families is the list of negotiated address families for this peer.
	// Set when EventEstablished is received.
	Families []string

	// FamilySent tracks route-sent counts per address family.
	FamilySent map[string]int

	// FamilyRecv tracks route-received counts per address family.
	FamilyRecv map[string]int

	// FamilySentTarget holds the expected route count per family for this peer.
	// Computed from profile data at init: unicast families get full RouteCount,
	// non-unicast (VPN, EVPN, FlowSpec) get RouteCount/4.
	FamilySentTarget map[string]int
}

// NewPeerState creates a PeerState with the given index and event buffer size.
func NewPeerState(index, bufSize int) *PeerState {
	return &PeerState{
		Index:      index,
		Events:     NewRingBuffer[peer.Event](bufSize),
		FamilySent: make(map[string]int),
		FamilyRecv: make(map[string]int),
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

// ConvergenceBucket defines a latency bucket for the convergence histogram.
type ConvergenceBucket struct {
	Label string        // Human-readable label (e.g., "0-5ms").
	Min   time.Duration // Inclusive lower bound.
	Max   time.Duration // Exclusive upper bound (0 means unbounded).
	Count int           // Number of routes in this bucket.
}

// convergenceBucketDefs defines the 13 histogram bucket ranges.
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
	{"1-2s", time.Second, 2 * time.Second},
	{"2-5s", 2 * time.Second, 5 * time.Second},
	{"5-10s", 5 * time.Second, 10 * time.Second},
	{"10-30s", 10 * time.Second, 30 * time.Second},
	{">30s", 30 * time.Second, 0},
}

// convergenceBucketCount is the number of histogram buckets.
const convergenceBucketCount = 13

// ConvergenceHistogram tracks route propagation latency distribution.
type ConvergenceHistogram struct {
	Buckets   [convergenceBucketCount]ConvergenceBucket
	Total     int
	Sum       time.Duration // For computing average.
	Min       time.Duration
	Max       time.Duration
	SlowCount int // Routes exceeding 1s.
}

// NewConvergenceHistogram creates an initialized histogram with the bucket definitions.
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
	for _, b := range &h.Buckets {
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

// DashboardState holds all mutable state for the web dashboard, protected by a RWMutex.
type DashboardState struct {
	mu sync.RWMutex

	// Per-peer state, indexed by peer index.
	Peers map[int]*PeerState

	// Active set for table visibility.
	Active *ActiveSet

	// Global counters.
	TotalAnnounced    int
	TotalReceived     int
	TotalMissing      int
	TotalChaos        int
	TotalReconnects   int
	TotalWithdrawn    int // Withdrawals received (EventRouteWithdrawn).
	TotalWdrawSent    int // Withdrawals sent by chaos peers (EventWithdrawalSent).
	TotalRouteActions int // Route dynamics actions executed (EventRouteAction).
	TotalDropped      int // Events silently dropped by readLoop (EventDroppedEvents).
	PeersUp           int

	// Run metadata.
	Seed      uint64
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

	// AllFamilies is the set of all address families seen across all peers.
	// Used to render per-family columns with red cross for non-negotiated.
	AllFamilies map[string]bool

	// Dirty flags for SSE — set by ProcessEvent, read by SSE goroutine.
	dirtyPeers    map[int]bool
	newlyPromoted map[int]bool // peers promoted since last broadcast
	dirtyGlobal   bool
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
		AllFamilies:     make(map[string]bool),
		dirtyPeers:      make(map[int]bool),
		newlyPromoted:   make(map[int]bool),
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

// ConsumeDirty returns which peers are dirty, which were newly promoted,
// and whether global state changed. Resets all flags.
// Must be called under write lock.
func (s *DashboardState) ConsumeDirty() (peers, promoted map[int]bool, global bool) {
	peers = s.dirtyPeers
	promoted = s.newlyPromoted
	global = s.dirtyGlobal
	s.dirtyPeers = make(map[int]bool)
	s.newlyPromoted = make(map[int]bool)
	s.dirtyGlobal = false
	return peers, promoted, global
}

// SortedFamilies returns AllFamilies as a sorted slice for deterministic rendering.
func (s *DashboardState) SortedFamilies() []string {
	fams := make([]string, 0, len(s.AllFamilies))
	for f := range s.AllFamilies {
		fams = append(fams, f)
	}
	sortStringSlice(fams)
	return fams
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

// FormatElapsed formats a duration for "time ago" display.
// Less precise than FormatDuration — sub-second precision is noise for elapsed times.
func FormatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s %= 60
	if m < 60 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := m / 60
	m %= 60
	return fmt.Sprintf("%dh%dm", h, m)
}
