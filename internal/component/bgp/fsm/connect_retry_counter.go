// Design: docs/architecture/behavior/fsm.md — BGP finite state machine
// RFC: rfc/short/rfc4271.md — mandatory session attributes (Section 8.1.1)
// Related: fsm.go — the handlers that increment and reset this counter

package fsm

import "sync/atomic"

// ConnectRetryCounter is RFC 4271 Section 8.1.1 mandatory session attribute 2.
//
// RFC 4271 Section 8.1.1: "The ConnectRetryCounter indicates the number of
// times a BGP peer has tried to establish a peer session."
//
// # Why it is a separate type, and not a field of FSM
//
// The RFC keeps one FSM per peer for the life of that peer, so a counter held
// inside the FSM would live exactly as long as the peer does. Ze does not:
// Peer.runOnce (internal/component/bgp/reactor/peer_run.go) builds a NEW
// Session, and therefore a NEW FSM, on every connection cycle, because the
// peer-level reconnect loop replaces the RFC's ConnectRetryTimer (see the
// ARCHITECTURAL NOTES in fsm.go). A counter stored in FSM state would reset on
// every retry and could never count one, which is the one thing it exists to
// do. So the counter is its own value, owned by the Peer, and handed to each
// cycle's FSM through FSM.SetConnectRetryCounter.
//
// # Concurrency
//
// The FSM mutates the counter under its own lock, but readers (the show path,
// the Prometheus refresh) run on other goroutines, so the value is atomic.
// Every method tolerates a nil receiver: an FSM with no counter wired (every
// pure-FSM unit test, and any future caller that does not care) mutates
// nothing rather than panicking.
type ConnectRetryCounter struct {
	n atomic.Uint32
}

// Increment adds one, per the RFC's "increments the ConnectRetryCounter by 1"
// clauses, and returns the new value. A nil counter counts nothing.
//
// Saturates at math.MaxUint32 rather than wrapping to zero: a wrap would read
// as "this peer has never retried", which is the opposite of what a counter at
// its maximum means. A peer that retries 4.29e9 times has a problem the
// counter should keep reporting.
func (c *ConnectRetryCounter) Increment() uint32 {
	if c == nil {
		return 0
	}
	for {
		cur := c.n.Load()
		if cur == ^uint32(0) {
			return cur
		}
		if c.n.CompareAndSwap(cur, cur+1) {
			return cur + 1
		}
	}
}

// Reset sets the counter to zero, per the RFC's "sets the ConnectRetryCounter
// to zero" clauses. A nil counter resets nothing.
func (c *ConnectRetryCounter) Reset() {
	if c == nil {
		return
	}
	c.n.Store(0)
}

// Load returns the current value. A nil counter reads zero.
func (c *ConnectRetryCounter) Load() uint32 {
	if c == nil {
		return 0
	}
	return c.n.Load()
}
