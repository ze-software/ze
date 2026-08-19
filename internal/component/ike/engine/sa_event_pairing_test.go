// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-3 and AC-4 -- an initiator tunnel
// that establishes and then goes down emits exactly one SADown for its one SAUp, and the
// two counts stay equal however many times the tunnel reconnects.
// PREVENTS: the unbounded lifecycle drift the initiator path carried. runInitiator emitted
// SAUp at establishment and returned runEstablished's result with no pair, while
// runResponder emitted both. The reconnect-on-peer-Delete path re-enters runInitiator, so
// a tunnel that flaps added one unanswered SAUp per cycle: every subscriber counting SAs
// up against SAs down (a `show` view, a metric, a fleet dashboard) drifted for as long as
// the daemon ran.

package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// sepBus records the event types the engine emits, in the order it emits them. The
// ORDER is part of the assertion: one up followed by one down is the pairing, and two
// downs after one up is the double-emit the fix has to avoid.
type sepBus struct {
	mu    sync.Mutex
	types []string
}

func (b *sepBus) Emit(namespace, eventType string, _ any) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.types = append(b.types, namespace+"/"+eventType)
	return 0, nil
}

func (b *sepBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// saLifecycle reports the sa-up / sa-down subsequence, dropping every other event
// (child-up and its kin) so a change in what else the engine emits cannot move this
// assertion.
func (b *sepBus) saLifecycle() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.types))
	for _, ev := range b.types {
		if ev == Namespace+"/sa-up" || ev == Namespace+"/sa-down" {
			out = append(out, ev)
		}
	}
	return out
}

func TestInitiatorEmitsSADownWhenEstablishedLoopReturns(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "pairing-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:  "ze",
		peerCfg:   iniPeer,
		ikeGroup:  ikeGroup,
		espGroup:  espGroup,
		stopCh:    make(chan struct{}),
		inbound:   make(chan transport.Packet, inboundQueueDepth),
		supersede: make(chan struct{}, 1),
	}
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	table := NewSATable()
	far := &icyFarEnd{}
	bus := &sepBus{}

	// The far end answers on the session goroutine, as it does in initiator_cycle_test.
	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(_ time.Duration) <-chan time.Time {
		if cur := ps.getSA(); cur != nil {
			far.advance(cur, respPS, table, log)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	done := make(chan struct{})
	go func() {
		_ = ps.runInitiator(iniPeer, ikeGroup, table, nil, bus, log)
		close(done)
	}()
	icyWaitFor(t, "the owner loop adopting the established SA", func() bool {
		return ps.ownedSA.Load() != nil
	})

	farEnd, err := far.get()
	if err != nil {
		close(ps.stopCh)
		<-done
		t.Fatalf("the driven handshake failed: %v", err)
	}

	// The peer tears the tunnel down (RFC 7296 Section 1.4). This is the ordinary way
	// an established initiator tunnel ends, and the way that repeats on every
	// reconnect. An established initiator SA has never cached a response, so the
	// peer's first request rides Message ID zero.
	ps.inbound <- transport.Packet{Data: lcyRequest(t, farEnd, 0, rteIKEDeleteChain())}

	select {
	case <-done:
	case <-time.After(rtxArrive):
		close(ps.stopCh)
		<-done
		t.Fatal("the initiator cycle never ended after the peer Delete")
	}

	got := bus.saLifecycle()
	want := []string{Namespace + "/sa-up", Namespace + "/sa-down"}
	if len(got) != len(want) {
		t.Fatalf("the tunnel emitted %v, want %v: one established initiator tunnel that went down owes exactly one up and one down", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d is %s, want %s (full sequence %v)", i, got[i], want[i], got)
		}
	}

	// The second half of the pairing. TerminatePeerSA, TerminateAllSAs and
	// reconcilePeers each emit SADown for the SA the session still holds, and each
	// reads it after ps.Stop has joined this goroutine, so an SA left in place would
	// answer this one up with a second down.
	if sa := ps.getSA(); sa != nil {
		t.Errorf("the ended cycle still holds SA ispi=%s: the operator teardown paths would emit a second sa-down for it",
			SPIHex(sa.InitiatorSPI))
	}
}

func TestSAUpAndSADownBalanceAcrossReconnects(t *testing.T) {
	const cycles = 4

	log := slogutil.DiscardLogger()
	peer := testPeer()
	peer.LocalAddress = "127.0.0.1"
	peer.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  peer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	bus := &sepBus{}

	// One stub stands in for both waits a cycle performs, and tells them apart by the
	// SA's state, as rcbRun (reconnect_backoff_test.go) does. On the handshake wait the
	// SA is live and the peer answers, so the cycle establishes and runs the REAL
	// establishment tail of runInitiator. runEstablished then fails at once (this SA
	// completed no key exchange, so createFirstChildSA refuses) and returns, which is
	// the tunnel-down path under test. The reconnect wait that follows counts the
	// cycle, and stops the session once enough of them have run.
	var mu sync.Mutex
	ran := 0
	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(_ time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		if sa := ps.getSA(); sa != nil && sa.State != StateDead && sa.State != StateEstablished {
			sa.RetransmitTime = time.Now().Add(-time.Millisecond)
			sa.State = StateEstablished
			return ch
		}
		mu.Lock()
		ran++
		enough := ran >= cycles
		mu.Unlock()
		if enough {
			// Not ps.Stop(): that blocks on ps.done, and this runs on the very
			// goroutine that closes it.
			ps.stopOnce.Do(func() { close(ps.stopCh) })
		}
		return ch
	}

	go ps.run(peer, testIKEGroup(), NewSATable(), nil, bus, log)
	select {
	case <-ps.done:
	case <-time.After(rtxArrive):
		ps.stopOnce.Do(func() { close(ps.stopCh) })
		<-ps.done
		t.Fatal("the reconnect loop never reached its verdict")
	}

	got := bus.saLifecycle()
	ups, downs := 0, 0
	for _, ev := range got {
		if ev == Namespace+"/sa-up" {
			ups++
		} else {
			downs++
		}
	}
	if ups != cycles {
		t.Fatalf("%d cycles established but %d sa-up events were emitted, so the balance below is not under test (sequence %v)", cycles, ups, got)
	}
	if downs != ups {
		t.Errorf("%d reconnect cycles emitted %d sa-up and %d sa-down: a subscriber counting SAs up against SAs down drifts by %d and never recovers",
			cycles, ups, downs, ups-downs)
	}
	for i := 0; i+1 < len(got); i += 2 {
		if got[i] != Namespace+"/sa-up" || got[i+1] != Namespace+"/sa-down" {
			t.Errorf("cycle %d emitted %v, want one up then one down (full sequence %v)", i/2+1, got[i:i+2], got)
			break
		}
	}
}
