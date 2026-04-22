// Design: plan/spec-l2tp-9-observer.md -- observer, event ring, ring pool
// Related: cqm.go -- CQM bucket types and sample ring
// Related: events/events.go -- typed event handles subscribed by observer

package l2tp

import (
	"sync"
	"time"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
)

func (t ObserverEventType) String() string {
	switch t {
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
	default:
		return stateUnknown
	}
}

// ObserverEvent is one record in a per-session event ring.
type ObserverEvent struct {
	Timestamp time.Time
	Type      ObserverEventType
	TunnelID  uint16
	SessionID uint16
	RTT       time.Duration
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
	login   string
	ring    *sampleRing
	current CQMBucket
	state   BucketState
	prev    *lruEntry
	next    *lruEntry
}

// Observer manages per-session event rings and per-login CQM sample
// rings. All public methods are safe for concurrent use.
type Observer struct {
	mu sync.Mutex

	eventPool  *eventRingPool
	samplePool *sampleRingPool

	sessions map[uint16]*eventRing

	logins  map[string]*lruEntry
	lruHead *lruEntry
	lruTail *lruEntry

	unsubs []func()
}

// ObserverConfig holds observer construction parameters.
type ObserverConfig struct {
	MaxSessions   int
	EventRingSize int
	MaxLogins     int
	BucketCount   int
}

// NewObserver creates an observer with pre-allocated ring pools.
func NewObserver(cfg ObserverConfig) *Observer {
	return &Observer{
		eventPool:  newEventRingPool(cfg.MaxSessions, cfg.EventRingSize),
		samplePool: newSampleRingPool(cfg.MaxLogins, cfg.BucketCount),
		sessions:   make(map[uint16]*eventRing),
		logins:     make(map[string]*lruEntry),
	}
}

// RecordEvent appends an event to the per-session event ring.
func (o *Observer) RecordEvent(ev ObserverEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	ring := o.sessions[ev.SessionID]
	if ring == nil {
		ring = o.eventPool.acquire()
		if ring == nil {
			return
		}
		o.sessions[ev.SessionID] = ring
	}
	ring.append(ev)
}

// ReleaseSession returns a session's event ring to the pool.
func (o *Observer) ReleaseSession(sessionID uint16) {
	o.mu.Lock()
	defer o.mu.Unlock()

	ring := o.sessions[sessionID]
	if ring == nil {
		return
	}
	delete(o.sessions, sessionID)
	o.eventPool.release(ring)
}

// RecordEcho folds one echo RTT sample into the current CQM bucket.
func (o *Observer) RecordEcho(login string, now time.Time, rtt time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := o.touchLogin(login, now)
	if entry == nil {
		return
	}
	o.maybeCloseBucket(entry, now)
	entry.current.addEcho(rtt)
}

// SetLoginState updates the CQM bucket state for a login, creating
// the entry if it does not yet exist (e.g. SessionIPAssigned arrives
// before the first echo).
func (o *Observer) SetLoginState(login string, state BucketState) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := o.touchLogin(login, time.Now())
	if entry == nil {
		return
	}
	entry.state = state
	entry.current.State = state
}

// SessionEvents returns a snapshot of the per-session event ring.
func (o *Observer) SessionEvents(sessionID uint16) []ObserverEvent {
	o.mu.Lock()
	defer o.mu.Unlock()

	ring := o.sessions[sessionID]
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

// AddUnsub registers an unsubscribe function to be called on Stop.
func (o *Observer) AddUnsub(fn func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.unsubs = append(o.unsubs, fn)
}

// Stop unsubscribes all EventBus handlers.
func (o *Observer) Stop() {
	o.mu.Lock()
	unsubs := o.unsubs
	o.unsubs = nil
	o.mu.Unlock()

	for _, fn := range unsubs {
		fn()
	}
}

func (o *Observer) touchLogin(login string, now time.Time) *lruEntry {
	if entry, ok := o.logins[login]; ok {
		o.promoteEntry(entry)
		return entry
	}

	ring := o.samplePool.acquire()
	if ring == nil {
		if o.lruTail == nil {
			return nil
		}
		o.evictLRU()
		ring = o.samplePool.acquire()
		if ring == nil {
			return nil
		}
	}

	entry := &lruEntry{
		login: login,
		ring:  ring,
		state: BucketStateNegotiating,
		current: CQMBucket{
			Start: now,
			State: BucketStateNegotiating,
		},
	}
	o.logins[login] = entry
	o.promoteEntry(entry)
	return entry
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

// wireObserverSubscriptions subscribes the observer to L2TP EventBus
// events. Called from Subsystem.Start when CQM is enabled.
func (s *Subsystem) wireObserverSubscriptions(bus ze.EventBus) {
	obs := s.observer

	obs.AddUnsub(l2tpevents.SessionUp.Subscribe(bus, func(p *l2tpevents.SessionUpPayload) {
		obs.RecordEvent(ObserverEvent{
			Timestamp: time.Now(),
			Type:      ObserverEventSessionUp,
			TunnelID:  p.TunnelID,
			SessionID: p.SessionID,
		})
	}))

	obs.AddUnsub(l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		obs.RecordEvent(ObserverEvent{
			Timestamp: time.Now(),
			Type:      ObserverEventSessionDown,
			TunnelID:  p.TunnelID,
			SessionID: p.SessionID,
		})
		obs.ReleaseSession(p.SessionID)
		if p.Username != "" {
			obs.SetLoginState(p.Username, BucketStateDown)
		}
	}))

	obs.AddUnsub(l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) {
		obs.RecordEvent(ObserverEvent{
			Timestamp: time.Now(),
			Type:      ObserverEventSessionIPAssigned,
			TunnelID:  p.TunnelID,
			SessionID: p.SessionID,
		})
		if p.Username != "" {
			obs.SetLoginState(p.Username, BucketStateEstablished)
		}
	}))

	obs.AddUnsub(l2tpevents.EchoRTT.Subscribe(bus, func(p *l2tpevents.EchoRTTPayload) {
		now := time.Now()
		obs.RecordEvent(ObserverEvent{
			Timestamp: now,
			Type:      ObserverEventEchoRTT,
			TunnelID:  p.TunnelID,
			SessionID: p.SessionID,
			RTT:       p.RTT,
		})
		if p.Username != "" {
			obs.RecordEcho(p.Username, now, p.RTT)
		}
	}))
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
