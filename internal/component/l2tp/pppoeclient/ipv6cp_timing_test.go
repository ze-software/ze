// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured IPv6CP timing.
// Related: ipv6cp_client.go -- one Restart timer follows the automaton.

package pppoeclient

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// timedIPv6CP supplies explicit timing; no default is invented by the constructor.
func timedIPv6CP(t *testing.T, interval time.Duration) *clientIPv6CP {
	t.Helper()
	c, err := newClientIPv6CP(ipv6cpTestMAC, 1492, interval)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.restart.Stop() })
	return c
}

// TestIPv6CPRestartRefusesInvalidPeriod checks the constructor boundary before
// a timer can receive a zero, fractional or out-of-range period.
func TestIPv6CPRestartRefusesInvalidPeriod(t *testing.T) {
	for _, period := range []time.Duration{0, 99 * time.Millisecond, 60001 * time.Millisecond, 100*time.Millisecond + time.Nanosecond} {
		if _, err := newClientIPv6CP(ipv6cpTestMAC, 1492, period); err == nil {
			t.Fatalf("invalid Restart period %s was accepted", period)
		}
	}
}

// TestIPv6CPConfiguredSilentBudget keeps the configured 60-second interval
// usable beyond IPCP's 30-second negotiation bound and spends exactly ten
// requests. This automaton may carry on after IPCP opened or stopped.
func TestIPv6CPConfiguredSilentBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := timedIPv6CP(t, 60*time.Second)
		w := &ipv6cpWire{}
		var buf [ppp.MaxFrameBufLen]byte
		if err := c.start(w, buf[:]); err != nil {
			t.Fatal(err)
		}
		began := time.Now()
		for expiry := 1; expiry <= ipv6cpConfigureMax; expiry++ {
			<-c.restart.C
			if elapsed := time.Since(began); elapsed != time.Duration(expiry)*60*time.Second {
				t.Fatalf("expiry %d at %s, want %s", expiry, elapsed, time.Duration(expiry)*60*time.Second)
			}
			if err := c.tick(w, buf[:], slogutil.DiscardLogger()); err != nil {
				t.Fatal(err)
			}
		}
		if got := len(w.frames); got != 10 {
			t.Fatalf("requests %d, want exactly 10", got)
		}
		if c.state != ppp.LCPStateStopped {
			t.Fatalf("state %v after silent budget, want Stopped", c.state)
		}
	})
}

// TestIPv6CPConfiguredRestartDuringIPCP observes the configured deadline while
// IPCP remains silent. Its independent timer must not resend IPCP at 500 ms.
// MUTATION: give runIPCP the fixed IPCP timer channel for ipv6Restart.
func TestIPv6CPConfiguredRestartDuringIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop, ended := make(chan struct{}), make(chan error, 1)
		c := timedIPv6CP(t, 500*time.Millisecond)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, err := negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], 1, 1492, nil,
				new(uint8), stop, slogutil.DiscardLogger(), new(uint8), c)
			ended <- err
		}()
		synctest.Wait()
		if got := len(w.snapshot()); got != 2 {
			t.Fatalf("initial frames %d, want one request for each NCP", got)
		}
		// sleep(timer): approach the configured IPv6CP deadline without reaching it.
		time.Sleep(499 * time.Millisecond)
		synctest.Wait()
		if got := len(w.snapshot()); got != 2 {
			t.Fatalf("frames before Restart expiry %d, want 2", got)
		}
		// sleep(timer): expire IPv6CP's 500 ms timer while IPCP's 3 s timer is pending.
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if got := len(w.snapshot()); got != 3 {
			t.Fatalf("frames at Restart expiry %d, want 3", got)
		}
		proto, packet := recordedNCP(t, w, 2)
		if proto != ppp.ProtoIPv6CP || packet.Code != ppp.LCPConfigureRequest {
			t.Fatalf("retry protocol %#x code %d, want only IPv6CP Configure-Request", proto, packet.Code)
		}
		close(stop)
		if err := <-ended; err == nil {
			t.Fatal("stopped IPCP returned success")
		}
	})
}

// TestIPv6CPRestartTimerFollowsRequest renews a partly spent interval with a
// Nak-induced request. The old deadline must not retransmit early.
// MUTATION: remove restart.Reset from clientIPv6CP.sendRequest.
func TestIPv6CPRestartTimerFollowsRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := timedIPv6CP(t, 500*time.Millisecond)
		w := &ipv6cpWire{}
		var buf [ppp.MaxFrameBufLen]byte
		if err := c.start(w, buf[:]); err != nil {
			t.Fatal(err)
		}
		// sleep(timer): spend most of the first request's interval before the Nak.
		time.Sleep(400 * time.Millisecond)
		packet := lastIPv6CP(t, w)
		nak := ppp.LCPPacket{Code: ppp.LCPConfigureNak, Identifier: packet.Identifier,
			Data: ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 2})}
		if _, err := c.handle(w, buf[:], nak, slogutil.DiscardLogger()); err != nil {
			t.Fatal(err)
		}
		// sleep(timer): the first request's deadline has passed, the new one's has not.
		time.Sleep(499 * time.Millisecond)
		select {
		case <-c.restart.C:
			t.Fatal("Restart expired before the replacement request's interval")
		default:
		}
		// sleep(timer): reach exactly 500 ms after the replacement request.
		time.Sleep(time.Millisecond)
		select {
		case <-c.restart.C:
		default:
			t.Fatal("Restart did not expire at the replacement request's deadline")
		}
	})
}

// TestIPv6CPRestartSurvivesPhaseHandoff hands a timer with only 100 ms left
// from IPCP negotiation to keepalive, which must use that same pending expiry.
// MUTATION: create a fresh timer in keepaliveLoop instead of borrowing restart.C.
func TestIPv6CPRestartSurvivesPhaseHandoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, _ := w.session(t)
		c := timedIPv6CP(t, 500*time.Millisecond)
		var buf [ppp.MaxFrameBufLen]byte
		if err := c.start(w, buf[:]); err != nil {
			t.Fatal(err)
		}
		w.take(t, ppp.ProtoIPv6CP, ppp.LCPConfigureRequest)
		// sleep(timer): hand off an already partly spent Restart interval.
		time.Sleep(400 * time.Millisecond)
		if err := s.enableIPv6CP(c, acceptTestIPv4Mode, slogutil.DiscardLogger()); err != nil {
			t.Fatal(err)
		}
		done, stop := make(chan struct{}), make(chan struct{})
		go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
		synctest.Wait()
		// sleep(timer): expire the original deadline, not a new phase-local interval.
		time.Sleep(100 * time.Millisecond)
		w.take(t, ppp.ProtoIPv6CP, ppp.LCPConfigureRequest)
		close(stop)
		<-done
	})
}

// TestIPv6CPRestartStopsOnOpenAndLowerDown completes both directions, then
// restarts and lowers LCP; neither inactive state may retain a pending timeout.
func TestIPv6CPRestartStopsOnOpenAndLowerDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := timedIPv6CP(t, 500*time.Millisecond)
		w := &ipv6cpWire{}
		var buf [ppp.MaxFrameBufLen]byte
		if err := c.start(w, buf[:]); err != nil {
			t.Fatal(err)
		}
		ours := lastIPv6CP(t, w)
		peer := ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 8,
			Data: ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 2})}
		if _, err := c.handle(w, buf[:], peer, slogutil.DiscardLogger()); err != nil {
			t.Fatal(err)
		}
		ack := ppp.LCPPacket{Code: ppp.LCPConfigureAck, Identifier: ours.Identifier, Data: ours.Data}
		if event, err := c.handle(w, buf[:], ack, slogutil.DiscardLogger()); err != nil || event != ipv6cpOpened {
			t.Fatalf("open event %d error %v", event, err)
		}
		// sleep(timer): a stopped Opened timer must not deliver an expiry.
		time.Sleep(500 * time.Millisecond)
		select {
		case <-c.restart.C:
			t.Fatal("Opened retained a Restart timeout")
		default:
		}
		if err := c.start(w, buf[:]); err != nil {
			t.Fatal(err)
		}
		c.lowerDown()
		// sleep(timer): LCP Down must also cancel the new negotiation's timer.
		time.Sleep(500 * time.Millisecond)
		select {
		case <-c.restart.C:
			t.Fatal("LCP Down retained a Restart timeout")
		default:
		}
	})
}
