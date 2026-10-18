// Design: docs/architecture/behavior/fsm.md -- Reactor Notification: the down follows the buffered deliveries
// Related: peer_session_end.go -- the leave arm and runOnce's down
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, real sessions over net.Pipe
package reactor

import (
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
)

// downOrderReceiver wraps the reactor's receiver. For the held source it
// blocks each batch until release closes, then publishes it and counts the
// UPDATEs it published. The delivery worker is the only caller.
type downOrderReceiver struct {
	MessageReceiver
	source    netip.Addr
	entered   chan struct{}
	enterOnce sync.Once
	release   chan struct{}
	published atomic.Int64
}

func (receiver *downOrderReceiver) OnMessageBatchReceived(peer *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	if peer.Address != receiver.source {
		return receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
	}
	receiver.enterOnce.Do(func() { close(receiver.entered) })
	<-receiver.release
	counts := receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
	receiver.published.Add(int64(len(messages)))
	return counts
}

// downOrderObserver records each down with what the test needs to judge its
// place: how many UPDATEs were published when it arrived, the peer state and
// the session's up phase at that moment.
type downOrderObserver struct {
	mu        sync.Mutex
	receiver  *downOrderReceiver
	downs     []downOrderRecord
	downAdded chan struct{}
}

type downOrderRecord struct {
	published int64
	state     PeerState
	closing   bool
	reason    string
}

func (observer *downOrderObserver) OnPeerEstablished(*Peer) {}

func (observer *downOrderObserver) OnPeerClosed(peer *Peer, closed sessionClose) {
	record := downOrderRecord{state: peer.State(), reason: closed.reason}
	if observer.receiver != nil {
		record.published = observer.receiver.published.Load()
	}
	peer.mu.RLock()
	record.closing = peer.upPhaseLocked(peer.session) == sessionUpClosing
	peer.mu.RUnlock()
	observer.mu.Lock()
	observer.downs = append(observer.downs, record)
	observer.mu.Unlock()
	select {
	case observer.downAdded <- struct{}{}:
	default: // The test reads the records; the signal only wakes it.
	}
}

func (observer *downOrderObserver) records() []downOrderRecord {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return append([]downOrderRecord(nil), observer.downs...)
}

// TestSessionDownFollowsBufferedDeliveries proves that a session's down is
// announced only after every UPDATE it buffered was published, and after the
// FSM leave arm marked the session closing and left PeerStateEstablished.
//
// Method: one real session over net.Pipe. The reactor's receiver is held, so
// the delivery worker blocks on the first batch while the remote sends the
// rest. The remote then closes the connection without a NOTIFICATION. The
// leave arm runs (the closing mark is set, State() left Established) while no
// down has arrived; once the receiver is released, exactly one down arrives,
// carrying every published UPDATE, and the history row for leaving
// Established carries the close reason. Red at HEAD, where the leave
// callback announced the down before the held UPDATEs were published.
//
// VALIDATES: the down follows the buffered deliveries (spec Data Flow 5).
// PREVENTS: a plugin hearing a session down before that session's UPDATEs.
func TestSessionDownFollowsBufferedDeliveries(t *testing.T) {
	const updates = 3
	peers := lowLiveRouter(t, lowLiveSettings("10.0.0.1", 65000, 65002))
	v := peers[0]
	p := v.peer
	r := p.reactor

	r.mu.RLock()
	prior := r.messageReceiver
	r.mu.RUnlock()
	receiver := &downOrderReceiver{MessageReceiver: prior, source: p.Settings().Address,
		entered: make(chan struct{}), release: make(chan struct{})}
	r.setMessageReceiver(receiver)
	t.Cleanup(func() { r.setMessageReceiver(prior) })
	observer := &downOrderObserver{receiver: receiver, downAdded: make(chan struct{}, 1)}
	r.addPeerObserver(observer)

	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	attrs := firstASAttrs(4, 65002) // RFC 6793 Section 3.
	for i := range updates {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, byte(i), 0}), 24)
		ownershipLiveSend(t, v, nil, attrs, prefix) // RFC 4271 Section 4.3.
	}
	select {
	case <-receiver.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery worker never reached the held receiver")
	}
	// Every UPDATE was accepted by the read goroutine before the close.
	lowEventually(t, func() bool {
		publication := p.receivePublication.Load()
		return publication != nil && publication.accepted.Load() >= uint64(updates)
	}, "all UPDATEs accepted")

	_ = v.remote.Close() // The remote ends the session with no NOTIFICATION.

	lowEventually(t, func() bool { return p.State() != PeerStateEstablished }, "leave arm ran")
	p.mu.RLock()
	closing := p.upPhaseLocked(session) == sessionUpClosing
	p.mu.RUnlock()
	if !closing {
		t.Fatal("the leave arm returned without marking the session closing")
	}
	time.Sleep(50 * time.Millisecond) // A down that overtakes the held UPDATEs lands here.
	if got := observer.records(); len(got) != 0 {
		t.Fatalf("down announced while %d UPDATEs were still held: %+v", updates, got)
	}

	close(receiver.release)
	select {
	case <-observer.downAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("no down after the held UPDATEs were released")
	}
	lowEventually(t, func() bool { return p.currentSession() != session }, "runOnce returned")
	downs := observer.records()
	if len(downs) != 1 {
		t.Fatalf("downs = %d, want exactly 1: %+v", len(downs), downs)
	}
	if downs[0].published != updates {
		t.Fatalf("down arrived after %d published UPDATEs, want %d", downs[0].published, updates)
	}
	if downs[0].state == PeerStateEstablished {
		t.Fatal("down arrived while State() still read Established")
	}
	if !downs[0].closing {
		t.Fatal("down arrived without the closing mark")
	}
	sessionDownHistoryRow(t, p, downs[0].reason)
}

// sessionDownHistoryRow asserts that the newest history row leaving
// Established carries the down's reason.
func sessionDownHistoryRow(t *testing.T, p *Peer, reason string) {
	t.Helper()
	rows := p.history.snapshot()
	for _, row := range slices.Backward(rows) {
		if row.From != fsm.StateEstablished.String() {
			continue
		}
		if row.Reason != reason {
			t.Fatalf("leave-Established history reason = %q, want %q", row.Reason, reason)
		}
		return
	}
	t.Fatalf("no leave-Established history row in %+v", rows)
}
