// Design: docs/guide/l2tp.md -- accepted proxy LCP and configured restart budgets.
// Related: session_run.go -- run, applyTransition and handleRestartTimeout.

package ppp

import (
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"testing/synctest"
	"time"
)

// TestProxyLCPRenegotiationUsesConfiguredRestartBudget starts through Driver
// with accepted proxy LCP, then counts the real pipe frames and each 500 ms
// expiry after a peer restarts LCP without answering the local requests.
// MUTATION: run omitting accepted-proxy Max-Configure initialization stops after one request.
// RFC requirement: RFC1661-4.6-3 positive -- accepted proxy LCP renegotiation sends the configured three/five Configure-Requests at 500 ms intervals and stops at the final expiry.
func TestProxyLCPRenegotiationUsesConfiguredRestartBudget(t *testing.T) {
	for _, maximum := range []int{3, 5} {
		t.Run(strconv.Itoa(maximum), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := startProxyLCPBudgetDriver(t, maximum)
				fixture.restart(t)
				started := time.Now()
				for sent := 1; sent <= maximum; sent++ {
					// A fake-clock nanosecond separates an unexpired request
					// from its retry or final expiry; no wall-clock margin is used.
					time.Sleep(500*time.Millisecond - time.Nanosecond)
					synctest.Wait()
					fixture.assertNegotiating(t, sent)
					time.Sleep(time.Nanosecond)
					synctest.Wait()
					if sent < maximum {
						fixture.assertNegotiating(t, sent+1)
						continue
					}
					select {
					case ev := <-fixture.driver.EventsOut():
						if _, ok := ev.(EventSessionDown); !ok {
							t.Fatalf("final expiry event = %T, want EventSessionDown", ev)
						}
					default:
						t.Fatal("no session-down event at the configured final expiry")
					}
				}
				if elapsed := time.Since(started); elapsed != time.Duration(maximum)*500*time.Millisecond {
					t.Fatalf("budget elapsed = %s, want %s", elapsed, time.Duration(maximum)*500*time.Millisecond)
				}
				if got := confRequestsIn(t, fixture.frames); got != maximum {
					t.Fatalf("Configure-Requests = %d, want configured maximum %d", got, maximum)
				}
				if got := fixture.session.currentState(); got != LCPStateStopped {
					t.Fatalf("final LCP state = %s, want Stopped", got)
				}
				if got := fixture.frames.count(); got != maximum+1 {
					t.Fatalf("wire frames = %d, want %d requests and one peer Configure-Ack", got, maximum)
				}
			})
		})
	}
}

// TestProxyLCPRenegotiationAnswerStopsRestartBudget answers the second local
// request, proving that an accepted proxy session can reopen and stop retries.
// MUTATION: run omitting accepted-proxy Max-Configure initialization ends before the second request.
// RFC requirement: RFC1661-4.6-3 negative -- answering the second Configure-Request reopens an accepted proxy LCP session and stops retries after exactly two requests.
func TestProxyLCPRenegotiationAnswerStopsRestartBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := startProxyLCPBudgetDriver(t, 3)
		fixture.restart(t)
		time.Sleep(500*time.Millisecond - time.Nanosecond)
		synctest.Wait()
		fixture.assertNegotiating(t, 1)
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		fixture.assertNegotiating(t, 2)
		request := lastLCPConfigureRequest(t, fixture.frames)
		writePeerLCP(t, fixture.peer, LCPConfigureAck, request.Identifier, request.Data)
		waitLCPUp(t, fixture.driver.EventsOut())
		awaitLifecycleUp(t, fixture.driver.EventsOut())
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if got := fixture.session.currentState(); got != LCPStateOpened {
			t.Fatalf("answering peer's LCP state = %s, want Opened", got)
		}
		if got := confRequestsIn(t, fixture.frames); got != 2 {
			t.Fatalf("answering peer received %d requests, want exactly two", got)
		}
		if got := fixture.frames.count(); got != 3 {
			t.Fatalf("wire frames = %d, want two requests and one peer Configure-Ack", got)
		}
		select {
		case ev := <-fixture.driver.EventsOut():
			t.Fatalf("unexpected event after LCP reopened: %T", ev)
		default:
		}
	})
}

type proxyLCPBudgetDriver struct {
	driver  *Driver
	peer    net.Conn
	frames  *frameRecorder
	session *pppSession
}

// startProxyLCPBudgetDriver uses the admitted settings and real run path.
// Its sole peer reader records complete pipe writes until cleanup closes it;
// cleanup stops the driver and joins that reader before the synctest ends.
func startProxyLCPBudgetDriver(t *testing.T, maximum int) proxyLCPBudgetDriver {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1201)
	frames := &frameRecorder{}
	readerDone := make(chan error, 1)
	go func() {
		buf := make([]byte, MaxFrameBufLen)
		for {
			n, err := pair.peerEnd.Read(buf)
			if err != nil {
				readerDone <- err
				return
			}
			if _, err := frames.Write(buf[:n]); err != nil {
				readerDone <- err
				return
			}
		}
	}()
	ops, _, _ := newFakeOps()
	driver := makeTestDriver(&fakeBackend{}, ops)
	t.Cleanup(func() {
		driver.Stop()
		closeConn(pair.peerEnd)
		err := <-readerDone
		if errors.Is(err, io.EOF) {
			return
		}
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("proxy peer reader ended: %v", err)
		}
	})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	lcp, err := NewLCPConfig(500*time.Millisecond, 2, maximum, 5)
	if err != nil {
		t.Fatal(err)
	}
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	driver.SessionsIn() <- StartSession{
		LCP:                 lcp,
		TunnelID:            1,
		SessionID:           42,
		ChanFD:              1201,
		UnitFD:              999,
		UnitNum:             7,
		LNSMode:             true,
		MaxMRU:              1500,
		EchoInterval:        time.Hour,
		DisableIPCP:         true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	up := waitLCPUp(t, driver.EventsOut())
	if up.ProxyLCP != ProxyLCPOutcomeAccepted {
		t.Fatalf("proxy outcome = %s, want accepted", up.ProxyLCP)
	}
	awaitLifecycleUp(t, driver.EventsOut())
	// Cross two configured periods while proxy LCP remains Opened. Neither
	// an initial request nor a Restart-timer retry may appear before RCR.
	time.Sleep(time.Second)
	synctest.Wait()
	if got := frames.count(); got != 0 {
		t.Fatalf("accepted proxy emitted %d initial wire frames, want none", got)
	}
	driver.mu.Lock()
	session := driver.sessions[sessionKey{1, 42}]
	driver.mu.Unlock()
	if session == nil {
		t.Fatal("accepted proxy session ended before peer renegotiation")
	}
	if got := session.currentState(); got != LCPStateOpened {
		t.Fatalf("accepted proxy state = %s, want Opened", got)
	}
	return proxyLCPBudgetDriver{driver: driver, peer: pair.peerEnd, frames: frames, session: session}
}

func (fixture proxyLCPBudgetDriver) restart(t *testing.T) {
	t.Helper()
	// RFC 1661 Section 4.1: Opened RCR+ runs TLD/SCR/SCA without IRC.
	writePeerLCP(t, fixture.peer, LCPConfigureRequest, 0x42, nil)
	ev := waitForEvent(t, fixture.driver.EventsOut(), time.Second)
	down, ok := ev.(EventLCPDown)
	if !ok {
		t.Fatalf("peer RCR event = %T, want EventLCPDown", ev)
	}
	down.Acknowledge()
	synctest.Wait()
	frames := decodeFrames(t, fixture.frames)
	if len(frames) != 2 {
		t.Fatalf("peer RCR emitted %d frames, want Configure-Request and Configure-Ack", len(frames))
	}
	if frames[0].Proto != ProtoLCP {
		t.Fatalf("request protocol = %#x, want LCP", frames[0].Proto)
	}
	if frames[0].Pkt.Code != LCPConfigureRequest {
		t.Fatalf("first code = %d, want Configure-Request", frames[0].Pkt.Code)
	}
	if frames[1].Proto != ProtoLCP {
		t.Fatalf("ack protocol = %#x, want LCP", frames[1].Proto)
	}
	if frames[1].Pkt.Code != LCPConfigureAck {
		t.Fatalf("second code = %d, want Configure-Ack", frames[1].Pkt.Code)
	}
	if frames[1].Pkt.Identifier != 0x42 {
		t.Fatalf("Configure-Ack identifier = %d, want 66", frames[1].Pkt.Identifier)
	}
	fixture.assertNegotiating(t, 1)
}

func (fixture proxyLCPBudgetDriver) assertNegotiating(t *testing.T, requests int) {
	t.Helper()
	if got := confRequestsIn(t, fixture.frames); got != requests {
		t.Fatalf("Configure-Requests = %d, want %d at this configured expiry", got, requests)
	}
	if got := fixture.session.currentState(); got != LCPStateAckSent {
		t.Fatalf("LCP state after %d requests = %s, want Ack-Sent", requests, got)
	}
	select {
	case ev := <-fixture.driver.EventsOut():
		t.Fatalf("unexpected event before budget exhaustion: %T", ev)
	default:
	}
}
