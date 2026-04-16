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
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/auth"
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

// JitterMaxFraction is the upper bound on per-packet TX interval reduction
// from RFC 5880 Section 6.8.7 ("jittered on a per-packet basis by up to
// 25%"). Exclusive upper bound so the reduction never reaches exactly 25%.
const JitterMaxFraction = 0.25

// JitterMinFractionDetectMultOne is the lower bound on the reduction when
// bfd.DetectMult == 1. RFC 5880 Section 6.8.7: "If bfd.DetectMult is equal
// to 1, the interval ... MUST be no more than 90% of the negotiated
// transmission interval, and MUST be no less than 75%..." -- a reduction
// of at least 10% is required so the receiver cannot time out before the
// next packet arrives.
const JitterMinFractionDetectMultOne = 0.10

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

// MetricsHook is the Loop-level notification channel for the BFD
// plugin's Prometheus metrics. The engine calls the hook's methods
// from the express loop goroutine (except OnStateChange, which is
// called from makeNotify while l.mu is held). Implementations MUST
// be cheap and non-blocking; the hook runs inside the hot path.
type MetricsHook interface {
	OnStateChange(from, to packet.State, diag packet.Diag, mode, vrf string)
	OnTxPacket(mode string)
	OnRxPacket(mode string)
	OnAuthFailure(mode string)
	OnEchoTx(mode string)
	OnEchoRx(mode string)
	OnEchoRTT(mode string, rtt time.Duration)
}

// Loop is the BFD express-loop runtime. Caller MUST call Start exactly
// once and Stop exactly once. EnsureSession and ReleaseSession may be
// called from any goroutine after Start.
//
// Lock order: subsMu MUST NOT be held while acquiring mu. The reverse is
// allowed: the express loop holds mu and may call into makeNotify which
// briefly takes subsMu to fan out events. This split prevents the
// notify-callback path from re-entering mu and deadlocking.
type Loop struct {
	transport     transport.Transport
	echoTransport transport.Transport
	clk           clock.Clock

	mu        sync.Mutex
	sessions  map[api.Key]*sessionEntry
	byDiscr   map[uint32]*sessionEntry
	byKey     map[firstPacketKey]*sessionEntry
	nextDiscr uint32

	subsMu      sync.Mutex
	subscribers map[api.Key][]chan api.StateChange

	metricsHook atomic.Pointer[MetricsHook]

	stopCh  chan struct{}
	doneCh  chan struct{}
	started atomic.Bool
	stopped atomic.Bool
}

// SetMetricsHook installs a Prometheus notification hook. Passing nil
// clears the hook. Safe for concurrent use; installed atomically.
func (l *Loop) SetMetricsHook(h MetricsHook) {
	if h == nil {
		l.metricsHook.Store(nil)
		return
	}
	l.metricsHook.Store(&h)
}

// sessionEntry wraps a session.Machine with engine-side bookkeeping
// (subscriber registry is held on the engine, not the entry).
//
// transitions is a bounded ring buffer of recent state changes; the
// engine overwrites the oldest entry once the ring is full. profile
// records the profile name this session inherited its timer parameters
// from at config load time; the engine never consults it but Snapshot
// surfaces it so `show bfd sessions` can show which profile applies.
// txPackets / rxPackets are incremented inside the express loop and
// exported via Snapshot + Prometheus counters.
type sessionEntry struct {
	machine     *session.Machine
	profile     string
	createdAt   time.Time
	txPackets   uint64
	rxPackets   uint64
	transitions []api.TransitionRecord
	lastState   packet.State
}

// recordTransition appends a new TransitionRecord to the ring buffer.
// When the ring is already at api.TransitionHistoryDepth entries, the
// oldest entry is discarded via slice reslice; the buffer never grows
// beyond the depth. Called from the makeNotify closure while l.mu is
// held, so no additional synchronization is required.
func (e *sessionEntry) recordTransition(from, to packet.State, diag packet.Diag, when time.Time) {
	rec := api.TransitionRecord{
		When: when,
		From: from.String(),
		To:   to.String(),
		Diag: diag.String(),
	}
	if len(e.transitions) < api.TransitionHistoryDepth {
		e.transitions = append(e.transitions, rec)
		return
	}
	// Shift left by one and write the new record at the tail.
	copy(e.transitions, e.transitions[1:])
	e.transitions[len(e.transitions)-1] = rec
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

// NewLoopWithEcho creates a Loop with a second transport bound to
// the RFC 5881 echo port. The echo transport is lifecycle-managed
// by the Loop: Start binds both sockets, Stop closes both. Passing
// a nil echo transport is equivalent to NewLoop.
//
// Only single-hop Loops should attach an echo transport; RFC 5883
// Section 4 prohibits multi-hop echo and the plugin config parser
// rejects it at load time.
func NewLoopWithEcho(t, echo transport.Transport, clk clock.Clock) *Loop {
	l := NewLoop(t, clk)
	l.echoTransport = echo
	return l
}

// applyJitter returns an RFC 5880 Section 6.8.7 TX interval reduction for
// the given base interval and detect multiplier. The reduction is always
// non-negative and strictly less than base so the next-TX deadline never
// moves backwards.
//
// For DetectMult >= 2 the reduction is drawn from [0%, 25%) of base. For
// DetectMult == 1 the reduction is drawn from [10%, 25%) of base so the
// transmitted interval sits in the RFC's [75%, 90%] window.
//
// Uses the package-level math/rand/v2 source (auto-seeded ChaCha8, safe
// for concurrent use). The engine does not need a per-Loop RNG because
// jitter is purely statistical; operators do not reason about individual
// draws, only about the aggregate distribution across a session's packet
// train.
func (l *Loop) applyJitter(base time.Duration, detectMult uint8) time.Duration {
	if base <= 0 {
		return 0
	}
	var f float64
	if detectMult == 1 {
		// [JitterMinFractionDetectMultOne, JitterMaxFraction)
		span := JitterMaxFraction - JitterMinFractionDetectMultOne
		f = JitterMinFractionDetectMultOne + rand.Float64()*span //nolint:gosec // BFD TX jitter is not a security decision
	} else {
		// [0, JitterMaxFraction)
		f = rand.Float64() * JitterMaxFraction //nolint:gosec // BFD TX jitter is not a security decision
	}
	return time.Duration(float64(base) * f)
}

// Start launches the express loop and the underlying transport. Start is
// NOT idempotent; calling it twice returns ErrAlreadyStarted.
//
// If the transport fails to start, Start reverts the `started` flag so
// a subsequent Stop() does not block on a doneCh that will never close.
// Without the revert the partial-failure path would leave the Loop in a
// half-started state where Stop deadlocks on `<-l.doneCh`.
func (l *Loop) Start() error {
	if !l.started.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}
	if err := l.transport.Start(); err != nil {
		l.started.Store(false)
		return err
	}
	if l.echoTransport != nil {
		if err := l.echoTransport.Start(); err != nil {
			// Control started successfully; roll it back so the
			// kernel sees a clean shutdown before reporting the
			// echo-bind failure upstream.
			if stopErr := l.transport.Stop(); stopErr != nil {
				engineLog().Debug("bfd control stop after echo start failure",
					"err", stopErr)
			}
			l.started.Store(false)
			return err
		}
	}
	go l.run()
	return nil
}

// Stop signals the express loop to exit, waits for it, and stops the
// transport. Stop is idempotent. Subscribers' channels are closed.
//
// Any session still pinned at shutdown has its auth persister closed
// so the Meticulous Keyed TX sequence reaches disk before the process
// exits. ReleaseSession handles this for refcount-driven teardown;
// Stop covers the runtimeState.stopAll path where loops are torn down
// while pinned handles are still live.
func (l *Loop) Stop() error {
	if !l.stopped.CompareAndSwap(false, true) {
		return nil
	}
	close(l.stopCh)
	<-l.doneCh

	l.mu.Lock()
	for key, entry := range l.sessions {
		if err := entry.machine.CloseAuth(); err != nil {
			engineLog().Debug("bfd auth persister close failed", "key", key, "err", err)
		}
	}
	l.mu.Unlock()

	stopErr := l.transport.Stop()
	if l.echoTransport != nil {
		if err := l.echoTransport.Stop(); err != nil && stopErr == nil {
			stopErr = err
		}
	}

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
	entry := &sessionEntry{
		machine:   m,
		profile:   req.Profile,
		createdAt: l.clk.Now(),
		lastState: packet.StateDown,
	}
	notify := l.makeNotify(key, entry)
	m.Init(req, discr, l.clk, notify)

	// RFC 5880 §6.7: install the authentication pair before the
	// session sees any packets. Build signer + verifier from the
	// request's AuthSettings and, for Meticulous variants with a
	// configured PersistDir, attach a SeqPersister so the TX
	// sequence survives a restart.
	if req.Auth != nil {
		pair, pairErr := buildAuthPair(req, key)
		if pairErr != nil {
			delete(l.sessions, key)
			delete(l.byDiscr, discr)
			delete(l.byKey, firstPacketIndex(key))
			return nil, pairErr
		}
		m.SetAuth(pair)
	}

	l.sessions[key] = entry
	l.byDiscr[discr] = entry
	l.byKey[firstPacketIndex(key)] = entry
	return &handle{loop: l, key: key}, nil
}

// buildAuthPair converts api.AuthSettings into a session.AuthPair,
// including an optional SeqPersister for Meticulous variants. The
// caller owns the pair and must call Close via Machine.CloseAuth on
// session teardown.
func buildAuthPair(req api.SessionRequest, key api.Key) (*session.AuthPair, error) {
	cfg := auth.Settings{
		Type:       req.Auth.Type,
		KeyID:      req.Auth.KeyID,
		Secret:     req.Auth.Secret,
		Meticulous: req.Auth.Meticulous,
	}
	signer, err := auth.NewSigner(cfg)
	if err != nil {
		return nil, err
	}
	verifier, err := auth.NewVerifier(cfg)
	if err != nil {
		return nil, err
	}
	pair := &session.AuthPair{Signer: signer, Verifier: verifier}
	if req.Auth.Meticulous && req.PersistDir != "" {
		keyStr := key.Peer.String() + "-" + key.VRF + "-" + key.Mode.String()
		p, perr := auth.NewSeqPersister(req.PersistDir, keyStr)
		if perr != nil {
			return nil, perr
		}
		pair.Persister = p
	}
	return pair, nil
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
		if err := entry.machine.CloseAuth(); err != nil {
			engineLog().Debug("bfd auth persister close failed", "key", hh.key, "err", err)
		}
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

// makeNotify returns a notify callback bound to a session key and its
// engine-side bookkeeping entry. The callback runs from the express
// loop goroutine while l.mu is held; it appends a TransitionRecord to
// the entry's ring buffer, forwards the event to any metrics hook, and
// acquires l.subsMu briefly to read the subscriber list, then delivers
// outside the lock so a slow consumer cannot stall the loop.
func (l *Loop) makeNotify(key api.Key, entry *sessionEntry) func(packet.State, packet.Diag) {
	return func(state packet.State, diag packet.Diag) {
		now := l.clk.Now()
		from := entry.lastState
		entry.recordTransition(from, state, diag, now)
		entry.lastState = state

		if hook := l.metricsHook.Load(); hook != nil {
			(*hook).OnStateChange(from, state, diag, key.Mode.String(), key.VRF)
		}

		change := api.StateChange{
			Key:   key,
			State: state,
			Diag:  diag,
			When:  now,
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
// true on success, false when the channel is full or closed. The
// capacity check is not racy for the full-channel case because the
// express loop is the only writer.
//
// The recover guard handles a concurrent Unsubscribe closing the
// channel between the makeNotify snapshot (taken under subsMu) and
// this send (running outside subsMu). Without the guard the express
// loop panics on send-to-closed-channel when a subscriber tears
// down while a state transition is in flight.
func trySendStateChange(ch chan api.StateChange, change api.StateChange) (sent bool) {
	defer func() {
		if r := recover(); r != nil {
			sent = false
		}
	}()
	if cap(ch)-len(ch) == 0 {
		return false
	}
	ch <- change
	return true
}
