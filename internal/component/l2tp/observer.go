// Design: docs/architecture/l2tp/l2tp-9-observer.md -- observer, event ring, ring pool
// Related: cqm.go -- CQM bucket types and sample ring
// Related: events/events.go -- typed event handles subscribed by observer

package l2tp

import (
	"errors"
	"slices"
	"sync"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const stateUnknown = "unknown"

// ObserverEventType identifies the kind of event stored in an event ring.
type ObserverEventType uint8

const (
	ObserverEventTunnelUp ObserverEventType = iota + 1
	ObserverEventTunnelDown
	ObserverEventSessionUp
	ObserverEventSessionDown
	ObserverEventSessionIPAssigned
	ObserverEventEchoRTT
	ObserverEventDisconnectRequested
)

func (t ObserverEventType) String() string {
	switch t {
	case 0:
		return stateUnknown
	case ObserverEventTunnelUp:
		return "tunnel-up"
	case ObserverEventTunnelDown:
		return "tunnel-down"
	case ObserverEventSessionUp:
		return "session-up"
	case ObserverEventSessionDown:
		return "session-down"
	case ObserverEventSessionIPAssigned:
		return "session-ip-assigned"
	case ObserverEventEchoRTT:
		return "echo-rtt"
	case ObserverEventDisconnectRequested:
		return "disconnect-requested"
	default:
		panic("BUG: unknown L2TP observer event type")
	}
}

// ObserverEvent is one record in a per-session event ring.
type ObserverEvent struct {
	Birth     subscriberid.L2TPBirth `json:"subscriber-id,omitzero"`
	Timestamp time.Time
	Type      ObserverEventType
	TunnelID  uint16
	SessionID uint16
	RTT       time.Duration
	Actor     string
	Reason    string
	Cause     uint32
}

// eventRing is a circular buffer of ObserverEvent records.
type eventRing struct {
	events []ObserverEvent
	head   int
	count  int
}

func newEventRing(capacity int) *eventRing {
	return &eventRing{events: make([]ObserverEvent, capacity)}
}

func (r *eventRing) append(ev ObserverEvent) {
	r.events[r.head] = ev
	r.head = (r.head + 1) % len(r.events)
	if r.count < len(r.events) {
		r.count++
	}
}

func (r *eventRing) snapshot() []ObserverEvent {
	if r.count == 0 {
		return nil
	}
	result := make([]ObserverEvent, r.count)
	start := (r.head - r.count + len(r.events)) % len(r.events)
	for i := range r.count {
		result[i] = r.events[(start+i)%len(r.events)]
	}
	return result
}

func (r *eventRing) reset() {
	for i := range r.events {
		r.events[i] = ObserverEvent{}
	}
	r.head = 0
	r.count = 0
}

// eventRingPool is a pre-allocated free list of identically-sized event rings.
type eventRingPool struct {
	free     []*eventRing
	capacity int
}

func newEventRingPool(poolSize, ringCapacity int) *eventRingPool {
	p := &eventRingPool{
		free:     make([]*eventRing, 0, poolSize),
		capacity: ringCapacity,
	}
	for range poolSize {
		p.free = append(p.free, newEventRing(ringCapacity))
	}
	return p
}

func (p *eventRingPool) acquire() *eventRing {
	n := len(p.free)
	if n == 0 {
		return nil
	}
	r := p.free[n-1]
	p.free = p.free[:n-1]
	return r
}

func (p *eventRingPool) release(r *eventRing) {
	r.reset()
	p.free = append(p.free, r)
}

// sampleRingPool is a pre-allocated free list of sample rings.
type sampleRingPool struct {
	free     []*sampleRing
	capacity int
}

func newSampleRingPool(poolSize, ringCapacity int) *sampleRingPool {
	p := &sampleRingPool{
		free:     make([]*sampleRing, 0, poolSize),
		capacity: ringCapacity,
	}
	for range poolSize {
		p.free = append(p.free, newSampleRing(ringCapacity))
	}
	return p
}

func (p *sampleRingPool) acquire() *sampleRing {
	n := len(p.free)
	if n == 0 {
		return nil
	}
	r := p.free[n-1]
	p.free = p.free[:n-1]
	return r
}

func (p *sampleRingPool) release(r *sampleRing) {
	r.reset()
	p.free = append(p.free, r)
}

// lruEntry tracks a login's sample ring and position in the LRU list.
type lruEntry struct {
	login        string
	ring         *sampleRing
	current      CQMBucket
	state        BucketState
	contributor  subscriberid.L2TPBirth
	downConsumed bool
	series       cqmSeriesLease
	metrics      *l2tpMetrics
	prev         *lruEntry
	next         *lruEntry
}

// observerMetricRefusal records a login whose CQM series lease could not be
// retained during late metric attachment, reported after locks are released.
type observerMetricRefusal struct {
	birth subscriberid.L2TPBirth
	login string
	cause error
}

// Observer manages per-session event rings and per-login CQM sample
// rings. All public methods are safe for concurrent use.
type Observer struct {
	mu sync.Mutex

	eventPool  *eventRingPool
	samplePool *sampleRingPool

	sessions   map[subscriberid.L2TPBirth]*eventRing
	generation l2tpevents.ObservationGeneration
	admission  observerAdmission
	metrics    *l2tpMetrics

	logins  map[string]*lruEntry
	lruHead *lruEntry
	lruTail *lruEntry

	unsubs []func()

	// Zero preserves the disabled contract for non-positive direct inputs.
	echoInterval            time.Duration
	expectedEchoesPerBucket uint64
}

// observerConfig holds observer construction parameters.
type observerConfig struct {
	Generation    l2tpevents.ObservationGeneration
	Metrics       *l2tpMetrics
	MaxSessions   int
	EventRingSize int
	MaxLogins     int
	BucketCount   int
	EchoInterval  time.Duration
}

// newObserver creates an observer with pre-allocated ring pools.
func newObserver(cfg observerConfig) (*Observer, error) {
	if err := validateObserverConfig(cfg); err != nil {
		return nil, err
	}
	var expected uint64
	if cfg.EchoInterval > 0 {
		expected = uint64(BucketInterval / cfg.EchoInterval)
	}
	return &Observer{
		eventPool:               newEventRingPool(cfg.MaxSessions, cfg.EventRingSize),
		samplePool:              newSampleRingPool(cfg.MaxLogins, cfg.BucketCount),
		echoInterval:            max(cfg.EchoInterval, 0),
		expectedEchoesPerBucket: expected,
		sessions:                make(map[subscriberid.L2TPBirth]*eventRing),
		logins:                  make(map[string]*lruEntry),
		generation:              cfg.Generation,
		admission:               observerOpen,
		metrics:                 cfg.Metrics,
	}, nil
}

// releaseSessionLocked requires Observer.mu. Its caller MUST already admit
// the original delivery; this pool operation MUST never select by wire labels.
func (o *Observer) releaseSessionLocked(birth subscriberid.L2TPBirth) {
	ring := o.sessions[birth]
	if ring == nil {
		return
	}
	delete(o.sessions, birth)
	o.eventPool.release(ring)
}

// SessionEvents returns a snapshot of the per-session event ring.
func (o *Observer) SessionEvents(birth subscriberid.L2TPBirth) []ObserverEvent {
	if o == nil || !birth.IsValid() {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	ring := o.sessions[birth]
	if ring == nil {
		return nil
	}
	return ring.snapshot()
}

// LoginSamples returns a snapshot of the per-login CQM sample ring.
// Returns non-nil (possibly empty) when the login exists, nil when not.
func (o *Observer) LoginSamples(login string) []CQMBucket {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry, ok := o.logins[login]
	if !ok {
		return nil
	}
	snap := entry.ring.snapshot()
	if snap == nil {
		return []CQMBucket{}
	}
	return snap
}

// SessionSummary is a compact view of one session's event ring.
type SessionSummary struct {
	Birth         subscriberid.L2TPBirth `json:"subscriber-id,omitzero"`
	TunnelID      uint16                 `json:"tunnel-id"`
	SessionID     uint16
	EventCount    int
	LastEventType string
	LastEventTime time.Time
}

// SessionSummaries returns a summary of every active session event ring.
func (o *Observer) SessionSummaries() []SessionSummary {
	o.mu.Lock()
	defer o.mu.Unlock()

	out := make([]SessionSummary, 0, len(o.sessions))
	for birth, ring := range o.sessions {
		s := SessionSummary{Birth: birth, EventCount: ring.count}
		if ring.count > 0 {
			last := ring.events[(ring.head-1+len(ring.events))%len(ring.events)]
			s.SessionID = last.SessionID
			s.TunnelID = last.TunnelID
			s.LastEventType = last.Type.String()
			s.LastEventTime = last.Timestamp
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b SessionSummary) int {
		if a.SessionID != b.SessionID {
			return int(a.SessionID) - int(b.SessionID)
		}
		return int(a.TunnelID) - int(b.TunnelID)
	})
	return out
}

// LoginSummary is a compact view of one login's CQM state.
type LoginSummary struct {
	Login       string
	BucketCount int
	LastState   string
	EchoCount   uint64
	AvgRTT      time.Duration
}

// LoginSummaries returns a summary of every tracked login.
func (o *Observer) LoginSummaries() []LoginSummary {
	o.mu.Lock()
	defer o.mu.Unlock()

	out := make([]LoginSummary, 0, len(o.logins))
	for login, entry := range o.logins {
		s := LoginSummary{
			Login:       login,
			BucketCount: entry.ring.count,
			LastState:   entry.current.State.String(),
			EchoCount:   entry.current.EchoCount,
			AvgRTT:      entry.current.AvgRTT(),
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b LoginSummary) int {
		if a.Login < b.Login {
			return -1
		}
		if a.Login > b.Login {
			return 1
		}
		return 0
	})
	return out
}

// LoginEchoState returns the current echo state for a login.
type LoginEchoState struct {
	Login   string
	LastRTT time.Duration
	// LossRatio is nil when the original interval yields no expected echoes.
	// A computed zero remains present and must not be treated as unavailable.
	LossRatio    *float64
	EchoInterval time.Duration
	BucketState  string
}

// EchoState returns the current echo state for a login derived from CQM.
// Returns nil when the login is unknown.
func (o *Observer) EchoState(login string) *LoginEchoState {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry, ok := o.logins[login]
	if !ok {
		return nil
	}
	state := &LoginEchoState{
		Login:        login,
		EchoInterval: o.echoInterval,
		BucketState:  entry.current.State.String(),
	}
	if entry.current.EchoCount > 0 {
		state.LastRTT = entry.current.AvgRTT()
	}
	if o.expectedEchoesPerBucket > 0 {
		received := float64(entry.current.EchoCount)
		expected := float64(o.expectedEchoesPerBucket)
		if expected > 0 {
			loss := 1.0 - (received / expected)
			if loss < 0 {
				loss = 0
			}
			state.LossRatio = &loss
		}
	}
	return state
}

// addUnsub registers an unsubscribe function to be called on Stop.
func (o *Observer) addUnsub(fn func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.unsubs = append(o.unsubs, fn)
}

// RetainKnownSessions seals lifecycle admission for this original generation.
// Its owner MUST call it before retiring roots; known terminal updates MUST remain useful.
func (o *Observer) RetainKnownSessions() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.admission == observerOpen {
		o.admission = observerRetained
	}
}

// Stop closes admission before unsubscribe so copied callbacks are revoked.
// The owner MUST call it after its final root retires; readers MUST retain
// their immutable history while actual native series leases are released once.
func (o *Observer) Stop() {
	if o == nil {
		return
	}
	o.mu.Lock()
	if o.admission == observerClosed {
		o.mu.Unlock()
		return
	}
	o.admission = observerClosed
	unsubs := o.unsubs
	o.unsubs = nil
	for _, entry := range o.logins {
		entry.series.Release()
	}
	o.mu.Unlock()
	for _, fn := range unsubs {
		fn()
	}
}

// attachMetrics binds a late-arriving metrics registry to an observer that
// started before any registry existed. It never replaces an existing binding
// (preserving original physical registration custody) and is a no-op once
// admission has sealed. Already-tracked logins retain their series leases on m;
// any login whose retention fails keeps its CQM history without publishing
// unleased samples, and its refusal is returned for reporting outside locks.
func (o *Observer) attachMetrics(m *l2tpMetrics) []observerMetricRefusal {
	if o == nil {
		return nil
	}
	if m == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.metrics != nil {
		return nil
	}
	if o.admission != observerOpen {
		return nil
	}
	o.metrics = m
	var refusals []observerMetricRefusal
	for login, entry := range o.logins {
		lease, err := m.retainCQMSeries(login)
		if err != nil {
			refusals = append(refusals, observerMetricRefusal{
				birth: entry.contributor, login: login, cause: err,
			})
			continue
		}
		entry.series = lease
		entry.metrics = m
	}
	return refusals
}

// errLoginUntracked is touchLogin's answer when no sample ring is free and no
// tracked login can be evicted for one. The event then carries no quality
// history for that login. It is a bound reached, not a failure: callers MUST
// drop the event and return nil for it.
var errLoginUntracked = errors.New("l2tp observer: no sample ring for the login")

// touchLogin answers the tracked entry for login, creating it when a ring is
// available. It answers errLoginUntracked when none is.
func (o *Observer) touchLogin(login string, now time.Time, birth subscriberid.L2TPBirth) (*lruEntry, error) {
	if entry := o.logins[login]; entry != nil {
		o.promoteEntry(entry)
		return entry, nil
	}
	if len(o.samplePool.free) == 0 && o.lruTail == nil {
		return nil, errLoginUntracked
	}
	// Retain every original physical series before evicting useful history.
	// A refusal MUST leave that history intact; partial custody MUST be released.
	series, err := o.metrics.retainCQMSeries(login)
	if err != nil {
		return nil, err
	}
	ring := o.samplePool.acquire()
	if ring == nil {
		o.evictLRU()
		ring = o.samplePool.acquire()
	}
	if ring == nil {
		series.Release()
		return nil, errLoginUntracked
	}
	entry := &lruEntry{
		login: login, ring: ring, state: BucketStateNegotiating,
		contributor: birth, series: series, metrics: o.metrics,
		current: CQMBucket{Start: now, State: BucketStateNegotiating},
	}
	o.logins[login] = entry
	o.promoteEntry(entry)
	return entry, nil
}

func (o *Observer) maybeCloseBucket(entry *lruEntry, now time.Time) {
	if entry.current.Start.IsZero() {
		entry.current.Start = now
		entry.current.State = entry.state
		return
	}
	// Cap iterations at ring capacity+1 to avoid spinning when the
	// time gap is large (e.g. system suspend). Beyond the cap, empty
	// buckets would just overwrite each other in the ring anyway.
	ringCap := len(entry.ring.buckets) + 1
	for i := 0; now.Sub(entry.current.Start) >= BucketInterval && i < ringCap; i++ {
		entry.metrics.observeCQMBucket(entry.login, entry.current, o.expectedEchoesPerBucket)
		entry.ring.append(entry.current)
		entry.current = CQMBucket{
			Start: entry.current.Start.Add(BucketInterval),
			State: entry.state,
		}
	}
	if now.Sub(entry.current.Start) >= BucketInterval {
		skip := now.Sub(entry.current.Start) / BucketInterval
		entry.current.Start = entry.current.Start.Add(skip * BucketInterval)
	}
}

func (o *Observer) evictLRU() {
	victim := o.lruTail
	if victim == nil {
		return
	}
	o.removeEntry(victim)
	delete(o.logins, victim.login)
	victim.series.Release()
	o.samplePool.release(victim.ring)
}

func (o *Observer) promoteEntry(entry *lruEntry) {
	if o.lruHead == entry {
		return
	}
	o.removeEntry(entry)
	entry.prev = nil
	entry.next = o.lruHead
	if o.lruHead != nil {
		o.lruHead.prev = entry
	}
	o.lruHead = entry
	if o.lruTail == nil {
		o.lruTail = entry
	}
}

func (o *Observer) removeEntry(entry *lruEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else if o.lruHead == entry {
		o.lruHead = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else if o.lruTail == entry {
		o.lruTail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}
