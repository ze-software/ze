package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// rcbRun drives PeerSession.run through n reconnect cycles and returns the delay run waited
// before each retry. establishOn names the 1-based cycles whose handshake reaches
// StateEstablished; every other cycle is ABANDONED at the IKE_AUTH stage.
//
// It measures the delay, not the code path. The whole defect was that a real loop asked for
// a real duration and got the wrong one, so the assertion has to be on the duration.
//
// The stub stands in for both waits a cycle performs, and it tells them apart by the SA's
// state. runInitiator waits on a live SA, so that call is where the handshake's fate is
// decided. Killing it there is what a refused IKE_AUTH response does from the dispatch
// goroutine, and it costs the cycle no retransmission at all. run's reconnect wait comes
// after, with the finished SA still attached (runInitiator leaves ps.sa in place on
// purpose), so its state is never the live one.
//
// An establishing cycle runs the REAL tail of runInitiator, which is where the counter is
// cleared, and then the real runEstablished. That call fails at once and returns: this SA
// never completed a handshake, so it holds no keys and createFirstChildSA refuses. The
// clearing has already happened by then, which is the point.
func rcbRun(t *testing.T, n int, establishOn map[int]bool) []time.Duration {
	t.Helper()
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

	var mu sync.Mutex
	var delays []time.Duration
	cycle := 0
	old := afterFunc
	afterFunc = func(d time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		if sa := ps.getSA(); sa != nil && sa.State != StateDead && sa.State != StateEstablished {
			mu.Lock()
			cycle++
			establish := establishOn[cycle]
			mu.Unlock()
			sa.RetransmitTime = time.Now().Add(-time.Millisecond)
			if establish {
				sa.State = StateEstablished
			} else {
				sa.State = StateDead
			}
			return ch
		}
		mu.Lock()
		delays = append(delays, d)
		enough := len(delays) >= n
		mu.Unlock()
		if enough {
			// Not ps.Stop(): that blocks on ps.done, and this runs on the very
			// goroutine that closes it.
			ps.stopOnce.Do(func() { close(ps.stopCh) })
		}
		return ch
	}
	t.Cleanup(func() { afterFunc = old })

	go ps.run(peer, testIKEGroup(), NewSATable(), nil, nil, log)
	select {
	case <-ps.done:
	case <-time.After(rtxArrive):
		ps.stopOnce.Do(func() { close(ps.stopCh) })
		<-ps.done
		t.Fatal("the reconnect loop never reached its verdict")
	}

	mu.Lock()
	defer mu.Unlock()
	out := make([]time.Duration, len(delays))
	copy(out, delays)
	return out
}

// VALIDATES: a handshake abandoned at the IKE_AUTH stage backs the reconnect off
// exponentially, up to reconnectMaxDelay.
//
// PREVENTS: the self-inflicted denial of service the errSADead exit introduced. That exit
// returns before the retransmit branch, which is the only writer of sa.RetransmitCount, and
// handleSAInitResponse zeroes that counter before IKE_AUTH. reconnectDelay read 0 and
// answered reconnectBase, so a peer that refuses authentication drew a fresh IKE_SA_INIT
// and a fresh Diffie-Hellman EVERY SECOND, indefinitely, against ze's own CPU and against
// the peer. Before the exit existed the cycle ran the whole retransmit schedule and the
// backoff reached reconnectMaxDelay.
//
// The delays are the assertion. A test on which error the cycle returned, or on which
// branch ran, is exactly the test that was already green while this was broken.
func TestRcbAuthStageFailureBacksOffExponentially(t *testing.T) {
	got := rcbRun(t, 8, nil)

	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		reconnectMaxDelay,
		reconnectMaxDelay,
	}
	if len(got) != len(want) {
		t.Fatalf("the loop waited %d times, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reconnect %d waited %v, want %v (full sequence %v)", i+1, got[i], want[i], got)
		}
	}

	// The headline claim, stated on its own so a future edit of the table above cannot
	// quietly restore the flat ramp.
	last := got[len(got)-1]
	if last <= reconnectBase {
		t.Errorf("after %d consecutive authentication failures the retry still waits %v, and "+
			"reconnectBase is %v; ze is re-running Diffie-Hellman against a peer that keeps "+
			"refusing", len(got), last, reconnectBase)
	}
}

// VALIDATES: a cycle that DOES establish resets the ramp, so a tunnel that came up and then
// went down retries at reconnectBase.
//
// This is the discriminator, and it is what stops the fix from being a counter that only
// ever climbs. The backoff must measure consecutive failures to ESTABLISH: a peer that has
// just proven it is reachable must not be made to wait a minute after one Delete. That is
// the behavior the old RetransmitCount reset gave, and it has to survive.
//
// It drives runInitiator's own establishment tail rather than assigning the counter, so
// deleting the clearing line reddens it. Cycle 4 establishes; the delay that FOLLOWS it is
// the measurement.
func TestRcbAnEstablishedCycleClearsTheBackoff(t *testing.T) {
	got := rcbRun(t, 5, map[int]bool{4: true})

	if len(got) != 5 {
		t.Fatalf("the loop waited %d times, want 5: %v", len(got), got)
	}
	// Cycles 1 to 3 fail, so the ramp is running before the establishment.
	if got[2] <= got[0] {
		t.Fatalf("the first three failures gave %v, so no ramp is under test here", got[:3])
	}
	if got[3] != reconnectBase {
		t.Errorf("the cycle after an established SA waited %v, want reconnectBase (%v); "+
			"the establishment did not reset the ramp, and a peer that has just proven it is "+
			"reachable is made to wait (full sequence %v)", got[3], reconnectBase, got)
	}
}

// VALIDATES: the retransmissions the last cycle really spent still raise the delay.
//
// reconnectDelay takes the larger of its two inputs, so adding the consecutive-failure
// count must not throw away what sa.RetransmitCount already measured: a peer that answers
// only after six repeats keeps the delay it earned instead of restarting the ramp.
func TestRcbRetransmitCountStillRaisesTheDelay(t *testing.T) {
	ps := &PeerSession{sa: &SA{RetransmitCount: 7}}
	ps.connectFailures = 1

	if d := reconnectDelay(ps); d != reconnectMaxDelay {
		t.Errorf("one cycle that spent 7 retransmissions waits %v, want reconnectMaxDelay (%v); "+
			"the transport measure was dropped when the failure count was added",
			d, reconnectMaxDelay)
	}
}
