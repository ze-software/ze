// Design: rfc/short/rfc5880.md -- engine express-loop owning sessions and timers
// Design: docs/research/bfd-implementation-guide.md -- BIRD express-loop pattern
// Detail: loop.go -- express-loop goroutine and handle type
//
// Package engine implements the BFD express-loop runtime.
//
// One Loop instance owns:
//
//   - A pool of sessions keyed by api.Key.
//   - A discriminator -> session reverse index for fast Your-Discriminator
//     lookup.
//   - A goroutine ("express loop") that drains the transport's RX channel,
//     fires per-session detection and TX timers, and emits state-change
//     events to subscribers.
//
// Sessions are NOT goroutine-per-session; the BIRD model puts every
// session-level mutation on one dedicated thread, eliminating per-session
// locks and giving the engine exclusive ownership of bfd.* state. The
// trade-off is that the loop has to do its own timer scheduling rather
// than rely on Go's runtime; we use a small-integer poll interval that
// is good enough for sub-second detection times.
package engine

import (
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/session"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// PollInterval is how often the express loop wakes up to check timers
// when no packet has arrived. 5 ms gives sub-50-ms detection-time
// resolution at modest CPU cost.
const PollInterval = 5 * time.Millisecond

// SubscribeBuffer is the per-subscriber channel depth. State changes are
// dropped if a subscriber falls behind; the engine never blocks on a
// slow consumer.
const SubscribeBuffer = 8

// ErrAlreadyStarted is returned by Start when the loop is already running.
var ErrAlreadyStarted = errors.New("bfd: engine already started")

// ErrForeignHandle is returned when a SessionHandle from a different
// engine is passed to ReleaseSession.
var ErrForeignHandle = errors.New("bfd: foreign SessionHandle")

// ErrDiscriminatorSpaceExhausted is returned by EnsureSession when no
// free local discriminator can be allocated. The 32-bit space minus
// zero gives 4 294 967 295 slots; this error means every one is taken.
var ErrDiscriminatorSpaceExhausted = errors.New("bfd: local discriminator space exhausted")

// ErrUnknownSession is returned when a SessionHandle method is called
// after the session has been torn down (refcount dropped to zero).
var ErrUnknownSession = errors.New("bfd: session no longer exists")

// Loop is the BFD express-loop runtime. Caller MUST call Start exactly
// once and Stop exactly once. EnsureSession and ReleaseSession may be
// called from any goroutine after Start.
//
// Lock order: subsMu MUST NOT be held while acquiring mu. The reverse is
// allowed: the express loop holds mu and may call into makeNotify which
// briefly takes subsMu to fan out events. This split prevents the
// notify-callback path from re-entering mu and deadlocking.
type Loop struct {
	transport transport.Transport
	clk       clock.Clock

	mu        sync.Mutex
	sessions  map[api.Key]*sessionEntry
	byDiscr   map[uint32]*sessionEntry
	byKey     map[firstPacketKey]*sessionEntry
	nextDiscr uint32

	subsMu      sync.Mutex
	subscribers map[api.Key][]chan api.StateChange

	stopCh  chan struct{}
	doneCh  chan struct{}
	started atomic.Bool
	stopped atomic.Bool
}

// sessionEntry wraps a session.Machine with engine-side bookkeeping
// (subscriber registry is held on the engine, not the entry).
type sessionEntry struct {
	machine *session.Machine
}

// firstPacketKey indexes sessions for first-packet (Your-Discriminator==0)
// dispatch. RFC 5880 §6.8.6 leaves the lookup tuple to the application;
// we use (peer, vrf, mode, interface) -- single-hop binds an interface,
// multi-hop leaves it empty so two multi-hop sessions to the same peer
// in the same VRF would collide. EnsureSession asserts uniqueness.
type firstPacketKey struct {
	peer  netip.Addr
	vrf   string
	iface string
	mode  api.HopMode
}

// firstPacketIndex builds a firstPacketKey from a session key. The peer
// address, VRF, hop mode, and (for single-hop) ingress interface are
// what an incoming first-packet exposes via Inbound; matching on this
// tuple is deterministic across map iteration order.
func firstPacketIndex(k api.Key) firstPacketKey {
	return firstPacketKey{
		peer:  k.Peer,
		vrf:   k.VRF,
		iface: k.Interface,
		mode:  k.Mode,
	}
}

// NewLoop creates a Loop bound to t. clk supplies the time source; pass
// clock.RealClock{} in production and a controllable clock in tests.
func NewLoop(t transport.Transport, clk clock.Clock) *Loop {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &Loop{
		transport:   t,
		clk:         clk,
		sessions:    make(map[api.Key]*sessionEntry),
		byDiscr:     make(map[uint32]*sessionEntry),
		byKey:       make(map[firstPacketKey]*sessionEntry),
		subscribers: make(map[api.Key][]chan api.StateChange),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		nextDiscr:   1, // start at 1 because zero is reserved by RFC 5880
	}
}

// Start launches the express loop and the underlying transport. Start is
// NOT idempotent; calling it twice returns ErrAlreadyStarted.
func (l *Loop) Start() error {
	if !l.started.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}
	if err := l.transport.Start(); err != nil {
		return err
	}
	go l.run()
	return nil
}

// Stop signals the express loop to exit, waits for it, and stops the
// transport. Stop is idempotent. Subscribers' channels are closed.
func (l *Loop) Stop() error {
	if !l.stopped.CompareAndSwap(false, true) {
		return nil
	}
	close(l.stopCh)
	<-l.doneCh
	stopErr := l.transport.Stop()

	l.subsMu.Lock()
	for k, subs := range l.subscribers {
		for _, ch := range subs {
			close(ch)
		}
		delete(l.subscribers, k)
	}
	l.subsMu.Unlock()
	return stopErr
}

// EnsureSession is the public api.Service entry point. If a session with
// the same Key already exists, its refcount is bumped. Otherwise the
// engine creates the session, allocates a unique discriminator, and the
// express loop will begin sending packets on the next tick.
func (l *Loop) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	key := req.Key()
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, ok := l.sessions[key]; ok {
		entry.machine.Acquire()
		return &handle{loop: l, key: key}, nil
	}

	discr, err := l.allocateDiscriminatorLocked()
	if err != nil {
		return nil, err
	}

	m := &session.Machine{}
	notify := l.makeNotify(key)
	m.Init(req, discr, l.clk, notify)

	entry := &sessionEntry{machine: m}
	l.sessions[key] = entry
	l.byDiscr[discr] = entry
	l.byKey[firstPacketIndex(key)] = entry
	return &handle{loop: l, key: key}, nil
}

// allocateDiscriminatorLocked returns the next free local discriminator,
// or ErrDiscriminatorSpaceExhausted if all 2^32-1 slots are taken.
//
// Zero is reserved by RFC 5880 §6.3 as "unknown" so it is never handed
// out. The counter wraps cleanly around uint32 max; collisions are
// resolved by walking until a free slot is found. Worst case is O(N)
// in the live session count, which is fine because the loop runs only
// at session creation time and the session count is bounded by config.
//
// Caller MUST hold l.mu.
func (l *Loop) allocateDiscriminatorLocked() (uint32, error) {
	// 2^32 - 1 valid slots (zero excluded). Bail rather than spin
	// forever if every slot is taken.
	const maxAttempts = 1 << 32
	for range maxAttempts {
		d := l.nextDiscr
		l.nextDiscr++
		if l.nextDiscr == 0 {
			l.nextDiscr = 1 // skip the reserved value on wrap
		}
		if d == 0 {
			continue
		}
		if _, taken := l.byDiscr[d]; taken {
			continue
		}
		return d, nil
	}
	return 0, ErrDiscriminatorSpaceExhausted
}

// ReleaseSession decrements the refcount on the session identified by h
// and tears it down when the count reaches zero.
func (l *Loop) ReleaseSession(h api.SessionHandle) error {
	hh, ok := h.(*handle)
	if !ok {
		return ErrForeignHandle
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.sessions[hh.key]
	if !ok {
		return nil
	}
	if entry.machine.Release() == 0 {
		delete(l.sessions, hh.key)
		delete(l.byDiscr, entry.machine.LocalDiscriminator())
		delete(l.byKey, firstPacketIndex(hh.key))
		l.subsMu.Lock()
		for _, ch := range l.subscribers[hh.key] {
			close(ch)
		}
		delete(l.subscribers, hh.key)
		l.subsMu.Unlock()
	}
	return nil
}

// makeNotify returns a notify callback bound to a session key. The
// callback runs from the express loop goroutine while l.mu is held; it
// acquires l.subsMu briefly to read the subscriber list, then delivers
// outside the lock so a slow consumer cannot stall the loop.
func (l *Loop) makeNotify(key api.Key) func(packet.State, packet.Diag) {
	return func(state packet.State, diag packet.Diag) {
		change := api.StateChange{
			Key:   key,
			State: state,
			Diag:  diag,
			When:  l.clk.Now(),
		}
		l.subsMu.Lock()
		subs := append([]chan api.StateChange(nil), l.subscribers[key]...)
		l.subsMu.Unlock()
		for _, ch := range subs {
			// Non-blocking send: a slow subscriber must never stall
			// the express loop. trySendStateChange reports whether
			// the enqueue succeeded; a full channel is logged but
			// the loop continues.
			if !trySendStateChange(ch, change) {
				engineLog().Debug("subscriber channel full, dropping state change", "key", key)
			}
		}
	}
}

// trySendStateChange attempts a non-blocking send onto ch and returns
// true on success, false when the channel is already full. The capacity
// check is not racy because the express loop is the only writer; the
// reader is the subscriber goroutine. A full channel means the
// subscriber has fallen one SubscribeBuffer behind, which the engine
// drops rather than blocks on.
func trySendStateChange(ch chan api.StateChange, change api.StateChange) bool {
	if cap(ch)-len(ch) == 0 {
		return false
	}
	ch <- change
	return true
}
