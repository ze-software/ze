// Design: docs/architecture/api/json-format.md -- the signal event
// Related: peer_fsm_events.go -- fsmEventPeerInfo names the peer the same way
// Related: ../server/events.go -- deliverPeerLine delivers the event

package reactor

import (
	"github.com/ze-software/ze/internal/core/daemonevents"
)

// subscribeDaemonSignals tells every configured peer's processes about each
// operating-system signal the daemon takes. The hub owns the signals and
// publishes them (daemonevents.Signal); the reactor only fans each one out, so
// a signal reaches a process by one path whichever way the daemon was built.
// Must be called after SetEventBus, like subscribeInterfaceEvents.
func (r *Reactor) subscribeDaemonSignals() {
	if r.eventBus == nil {
		return
	}
	unsubscribe := daemonevents.Signal.Subscribe(r.eventBus, r.onDaemonSignal)
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubscribe)
}

// onDaemonSignal delivers one `signal` event per configured peer, which is
// what ExaBGP does: its reactor walks every neighbor and calls
// `processes.signal(neighbor, number)` for each (5.0.1 reactor/loop.py,
// `_signalled`). A process that did not subscribe to `signal` for a peer is
// told nothing (deliverPeerLine).
//
// It runs on the EventBus delivery path, synchronously with the hub's publish,
// and holds reactor.mu only to copy the peer list.
func (r *Reactor) onDaemonSignal(received daemonevents.Received) {
	dispatcher := r.eventDispatcher
	if dispatcher == nil {
		return
	}
	for _, peer := range r.Peers() {
		peerInfo := fsmEventPeerInfo(peer)
		dispatcher.OnPeerSignal(&peerInfo, received.Number, received.Name)
	}
}
