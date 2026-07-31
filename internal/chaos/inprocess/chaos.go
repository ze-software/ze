// Design: docs/architecture/chaos-web-dashboard.md — in-process chaos scheduling

package inprocess

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/mocknet"
	"github.com/ze-software/ze/internal/chaos/route"
)

// establishedState tracks which peers are in Established state.
// Written by the event-processing goroutine, read by schedulers.
type establishedState struct {
	mu    sync.RWMutex
	peers []bool
}

func newEstablishedState(n int) *establishedState {
	return &establishedState{peers: make([]bool, n)}
}

func (es *establishedState) Set(idx int, val bool) {
	es.mu.Lock()
	es.peers[idx] = val
	es.mu.Unlock()
}

func (es *establishedState) Snapshot() []bool {
	es.mu.RLock()
	snap := make([]bool, len(es.peers))
	copy(snap, es.peers)
	es.mu.RUnlock()
	return snap
}

// Up reports whether the peer is Established right now.
func (es *establishedState) Up(idx int) bool {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return idx >= 0 && idx < len(es.peers) && es.peers[idx]
}

// waitSettled blocks until settled reports true. It returns early when ctx
// ends, and it does nothing when settle is false.
//
// Duration bounds the scenario. It does not bound teardown. The advance loop
// spends 60 seconds of virtual time in about 0.6 seconds of real time. A chaos
// disconnect in the final ticks therefore leaves a reconnect handshake in
// flight when the loop exits. The simulator cancel then closes that connection
// during the OPEN exchange. The run reports a disconnect that no reconnect
// follows, which is a scenario nobody wrote.
//
// Duration is therefore an earliest bound for teardown, in the same sense as
// RunConfig.DisconnectAt. The wait is on observed state and not on a duration.
//
// Chaos scheduling has already stopped when this runs, because the advance loop
// is the only feeder of the chaos tick channel. The window therefore cannot
// extend the scenario. ctx bounds a peer that never returns, which is a product
// regression rather than the normal case.
//
// The caller composes settled from every signal that shows a perturbation is
// still in flight. Read the call site in Run for the ones it uses.
func waitSettled(ctx context.Context, settle bool, settled func() bool, poll time.Duration) {
	if !settle {
		return
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for !settled() {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// chaosProgress tracks how far each peer's chaos has got. It counts actions
// dispatched against actions applied. It also records whether the peer has come
// back up since the first action hit it.
//
// A dispatched action that has not been executed is a perturbation. The run has
// committed to it and has not yet applied it. It sits on the peer's channel, or
// it runs inside the simulator. Neither place shows in peer state, and neither
// shows in the event stream. ActionConnectionCollision holds the simulator for
// 500 milliseconds of real time before it closes anything. The peer reads as
// healthy for all of it.
//
// wentDown and recovered are latches, and they bound the settle wait. The run
// owes the scenario ONE observable recovery for each peer that chaos knocked
// down. It does not owe a healthy peer at the final instant. Chaos rate 1.0
// leaves the peer down when the window closes. A wait for that last flap waits
// for something the run never promised.
//
// A peer that chaos perturbed, and did not knock down, owes nothing. A malformed
// UPDATE changes no session state. No further establishment is ever coming, and
// a demand for one hangs until ctx ends.
type chaosProgress struct {
	mu         sync.Mutex
	dispatched []int
	executed   []int
	perturbed  []bool
	expectFall []bool
	wentDown   []bool
	recovered  []bool
}

func newChaosProgress(n int) *chaosProgress {
	return &chaosProgress{
		dispatched: make([]int, n),
		executed:   make([]int, n),
		perturbed:  make([]bool, n),
		expectFall: make([]bool, n),
		wentDown:   make([]bool, n),
		recovered:  make([]bool, n),
	}
}

// endsSession reports whether an action is certain to end the peer's session.
//
// The first four end it in the simulator, which reports Disconnected. The fifth
// ends it in the reactor. ActionHoldTimerExpiry only stops the keepalives, so
// the session survives until the hold timer expires. That is 20 seconds of
// VIRTUAL time later. Nothing about the peer changes in between. The settle gate
// therefore cannot read that consequence off peer state, and it must know in
// advance that the consequence is coming.
//
// ActionConnectionCollision is absent on purpose. RFC 4271 Section 6.8 permits
// the reactor to reject the second connection and keep the session. A fall is
// therefore not certain, and a demand for one hangs when the reactor is right.
func endsSession(t engine.ActionType) bool {
	switch t { //nolint:exhaustive // every other action leaves the session up
	case engine.ActionTCPDisconnect,
		engine.ActionNotificationCease,
		engine.ActionDisconnectDuringBurst,
		engine.ActionReconnectStorm,
		engine.ActionHoldTimerExpiry:
		return true
	}
	return false
}

// Dispatched records that an action reached the peer's channel.
func (c *chaosProgress) Dispatched(idx int, action engine.ActionType) {
	c.mu.Lock()
	c.dispatched[idx]++
	if endsSession(action) {
		c.expectFall[idx] = true
	}
	c.mu.Unlock()
}

// Executed records that the simulator finished applying an action.
func (c *chaosProgress) Executed(idx int) {
	c.mu.Lock()
	if idx >= 0 && idx < len(c.executed) {
		c.executed[idx]++
		c.perturbed[idx] = true
	}
	c.mu.Unlock()
}

// WentDown latches that the peer lost its session after chaos hit it.
func (c *chaosProgress) WentDown(idx int) {
	c.mu.Lock()
	if idx >= 0 && idx < len(c.wentDown) && c.perturbed[idx] {
		c.wentDown[idx] = true
	}
	c.mu.Unlock()
}

// Recovered latches that the peer reached Established after chaos knocked it
// down. It ignores an establishment from before the peer went down.
func (c *chaosProgress) Recovered(idx int) {
	c.mu.Lock()
	if idx >= 0 && idx < len(c.recovered) && c.wentDown[idx] {
		c.recovered[idx] = true
	}
	c.mu.Unlock()
}

// Owed reports whether the peer still owes the scenario a recovery. It owes one
// after chaos has knocked it down. It also owes one after the dispatch of an
// action that is certain to knock it down.
func (c *chaosProgress) Owed(idx int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx < 0 || idx >= len(c.wentDown) {
		return false
	}
	return (c.wentDown[idx] || c.expectFall[idx]) && !c.recovered[idx]
}

// Quiet reports whether every dispatched action has been applied.
func (c *chaosProgress) Quiet() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, d := range c.dispatched {
		if c.executed[i] < d {
			return false
		}
	}
	return true
}

// AwaitingFirstFall reports whether chaos has been applied to the peer and has
// not yet knocked it down. The run must read such a peer as up before it ends,
// because the session CAN be dying at that instant. After the peer has fallen
// and recovered it owes nothing, and its state at the end carries no meaning.
func (c *chaosProgress) AwaitingFirstFall(idx int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx < 0 || idx >= len(c.perturbed) {
		return false
	}
	return c.perturbed[idx] && !c.wentDown[idx]
}

// closeNotifier wraps a connection so its owner learns when the REACTOR closes
// it. RFC 4271 Section 6.8 rejection (Reactor.rejectConnectionCollision) writes
// a NOTIFICATION and closes the connection it refuses, so that close IS the
// collision verdict: an observable state transition to wait on instead of a
// guessed duration.
type closeNotifier struct {
	net.Conn
	once   sync.Once
	closed chan struct{}
}

// newCloseNotifier wraps conn so Closed() reports when it has been closed.
func newCloseNotifier(conn net.Conn) *closeNotifier {
	return &closeNotifier{Conn: conn, closed: make(chan struct{})}
}

// Close signals Closed() and closes the wrapped connection.
func (c *closeNotifier) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

// Closed is closed once the wrapped connection has been closed, whichever side
// closed it. Safe for concurrent use.
func (c *closeNotifier) Closed() <-chan struct{} { return c.closed }

// reconnectDialer creates mock TCP connection pairs on demand for chaos
// reconnection in in-process mode. Each DialContext call creates a new
// pair, wraps the reactor end with proper TCP addresses, and queues it
// on the MockListener.
type reconnectDialer struct {
	cpm       *mocknet.ConnPairManager
	ml        *mocknet.MockListener
	localAddr *net.TCPAddr
	peerIP    net.IP
}

func (rd *reconnectDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	peerEnd, reactorEnd, err := rd.cpm.NewPair()
	if err != nil {
		return nil, err
	}
	remoteTCPAddr := &net.TCPAddr{IP: rd.peerIP, Port: 0}
	wrappedEnd := mocknet.NewConnWithAddr(reactorEnd, rd.localAddr, remoteTCPAddr)
	rd.ml.QueueConn(wrappedEnd)
	return peerEnd, nil
}

// chaosSchedulerLoop ticks the chaos scheduler using virtual time and
// dispatches actions to per-peer channels. Runs until ctx is canceled.
func chaosSchedulerLoop(ctx context.Context, sched *engine.Scheduler, g *guard.Guard, es *establishedState, channels []chan engine.ChaosAction, tickCh <-chan time.Time, inFlight *chaosProgress) {
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tickCh:
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, _ := g.AllowChaos(a.PeerIndex, a.Action.Type); !ok {
					continue
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					// Counted only on a successful send. The drop below never
					// reaches the simulator, so it is not in flight.
					inFlight.Dispatched(a.PeerIndex, a.Action.Type)
					if a.Action.Type == engine.ActionHoldTimerExpiry {
						g.OnHoldTimerExpiry(a.PeerIndex)
					}
				default: // non-blocking: drop action if peer is busy processing previous one
				}
			}
		}
	}
}

// routeSchedulerLoop ticks the route scheduler using virtual time and
// dispatches actions to per-peer channels. Runs until ctx is canceled.
func routeSchedulerLoop(ctx context.Context, sched *route.Scheduler, g *guard.Guard, es *establishedState, channels []chan route.Action, tickCh <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tickCh:
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, _ := g.AllowRoute(a.PeerIndex, a.Action.Type); !ok {
					continue
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					if a.Action.Type == route.ActionFullWithdraw {
						g.OnFullWithdraw(a.PeerIndex)
					}
				default: // non-blocking: drop action if peer is busy processing previous one
				}
			}
		}
	}
}
