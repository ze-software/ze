// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client LCP Restart timer and counter.
// Related: session.go -- negotiateLCP, the producer these tests drive.

package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// timedLCPWriter records each frame negotiateLCP writes with the bubble's
// clock, and fails every LCP frame whose Code is failCode. Safe for
// concurrent use: negotiateLCP writes from its goroutine while the test reads.
type timedLCPWriter struct {
	mu       sync.Mutex
	frames   [][]byte
	at       []time.Time
	failCode uint8
	failErr  error
}

func (w *timedLCPWriter) Write(frame []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failErr != nil {
		if len(frame) > 2 {
			if frame[2] == w.failCode {
				return 0, w.failErr
			}
		}
	}
	w.frames = append(w.frames, bytes.Clone(frame))
	w.at = append(w.at, time.Now())
	return len(frame), nil
}

// requests answers the Configure-Requests written so far, with their times.
func (w *timedLCPWriter) requests() ([][]byte, []time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var frames [][]byte
	var at []time.Time
	for i, frame := range w.frames {
		if len(frame) < 4 {
			continue
		}
		if frame[2] != ppp.LCPConfigureRequest {
			continue
		}
		frames = append(frames, frame)
		at = append(at, w.at[i])
	}
	return frames, at
}

type lcpRunResult struct {
	err     error
	elapsed time.Duration
}

// startLCP runs negotiateLCP in the bubble and reports its error and how long
// it ran. The caller MUST close stop or let the negotiation end.
func startLCP(t *testing.T, w *timedLCPWriter, frames <-chan readFrame, stop <-chan struct{}) <-chan lcpRunResult {
	t.Helper()
	return startConfiguredLCP(t, w, frames, stop, sessionConfig{lcpRestartTimer: lcpRestartInterval, lcpMaxConfigure: lcpConfigureMax, mtu: 1492})
}

// startConfiguredLCP retains the same virtual clock and worker lifetime as
// startLCP while supplying explicit session timing.
func startConfiguredLCP(t *testing.T, w *timedLCPWriter, frames <-chan readFrame, stop <-chan struct{}, cfg sessionConfig) <-chan lcpRunResult {
	t.Helper()
	reader := testFrameReader(t, frames, nil)
	done := make(chan lcpRunResult, 1)
	go func() {
		start := time.Now()
		var buf [ppp.MaxFrameBufLen]byte
		_, err := negotiateLCP(w, reader, buf[:], cfg, clientMagic, nil, stop,
			slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		done <- lcpRunResult{err: err, elapsed: time.Since(start)}
	}()
	return done
}

// TestLCPConfiguredBudget observes requests, their exact virtual-clock gaps
// and the final timeout. The long budget also catches a fixed 60-second cap.
func TestLCPConfiguredBudget(t *testing.T) {
	for _, interval := range []time.Duration{500 * time.Millisecond, 60 * time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &timedLCPWriter{}
				stop := make(chan struct{})
				defer close(stop)
				cfg := sessionConfig{mtu: 1492, lcpRestartTimer: interval, lcpMaxConfigure: 3}
				got := <-startConfiguredLCP(t, w, make(chan readFrame), stop, cfg)
				if got.err == nil {
					t.Fatal("LCP opened without a reply")
				}
				if got.elapsed != 3*interval {
					t.Fatalf("LCP ended after %v, want configured budget %v", got.elapsed, 3*interval)
				}
				requests, at := w.requests()
				if len(requests) != 3 {
					t.Fatalf("requests = %d, want configured limit 3", len(requests))
				}
				for i := 1; i < len(requests); i++ {
					if gap := at[i].Sub(at[i-1]); gap != interval {
						t.Fatalf("request gap = %v, want configured timer %v", gap, interval)
					}
				}
			})
		})
	}
}

func TestLCPConfiguredBudgetAllowsSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 2)
		stop := make(chan struct{})
		defer close(stop)
		cfg := sessionConfig{mtu: 1492, lcpRestartTimer: time.Second, lcpMaxConfigure: 1}
		done := startConfiguredLCP(t, w, frames, stop, cfg)
		synctest.Wait()
		requests, _ := w.requests()
		if len(requests) != 1 {
			t.Fatalf("initial requests = %d, want 1", len(requests))
		}
		frames <- serverFrame(ppp.LCPConfigureAck, requests[0][3], bytes.Clone(requests[0][6:]))
		frames <- serverFrame(ppp.LCPConfigureRequest, 77, []byte{ppp.LCPOptMRU, 4, 0x05, 0xd4})
		got := <-done
		if got.err != nil {
			t.Fatalf("answering peer failed with Max-Configure 1: %v", got.err)
		}
	})
}

func TestLCPConfiguredConvergenceDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		defer close(stop)
		cfg := sessionConfig{mtu: 1492, lcpRestartTimer: 5 * time.Second, lcpMaxConfigure: 10}
		done := startConfiguredLCP(t, w, frames, stop, cfg)
		var got lcpRunResult
		for ended := false; !ended; {
			// sleep(test-clock): deliver each Nak before the configured five-second Restart timer expires.
			time.Sleep(4900 * time.Millisecond)
			synctest.Wait()
			select {
			case got = <-done:
				ended = true
				continue
			default:
			}
			requests, _ := w.requests()
			frames <- mruNak(requests[len(requests)-1])
		}
		if got.err == nil {
			t.Fatal("LCP opened while every request was Naked")
		}
		if got.elapsed != 100*time.Second {
			t.Fatalf("LCP ended after %v, want twice the configured 50s request budget", got.elapsed)
		}
		if !strings.Contains(got.err.Error(), (100 * time.Second).String()) {
			t.Fatalf("error = %v, want the configured convergence deadline", got.err)
		}
	})
}

func TestLCPRefusesMissingTimingBeforeWriting(t *testing.T) {
	for _, missing := range []string{"restart-timer", "max-configure"} {
		t.Run(missing, func(t *testing.T) {
			w := &timedLCPWriter{}
			cfg := sessionConfig{mtu: 1492, lcpRestartTimer: time.Second, lcpMaxConfigure: 3}
			if missing == "restart-timer" {
				cfg.lcpRestartTimer = 0
			}
			if missing == "max-configure" {
				cfg.lcpMaxConfigure = 0
			}
			var buf [ppp.MaxFrameBufLen]byte
			_, err := negotiateLCP(w, nil, buf[:], cfg, clientMagic, nil, make(chan struct{}), slog.Default(), new(uint8))
			if err == nil {
				t.Fatalf("LCP accepted missing %s", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Fatalf("error = %v, want missing leaf %s", err, missing)
			}
			if requests, _ := w.requests(); len(requests) != 0 {
				t.Fatal("invalid timing wrote a Configure-Request")
			}
		})
	}
}

// mruNak answers request with a Configure-Nak suggesting MRU 1400.
func mruNak(request []byte) readFrame {
	return serverFrame(ppp.LCPConfigureNak, request[3], []byte{ppp.LCPOptMRU, 4, 0x05, 0x78})
}

// TestLCPRestartTimerFollowsLastRequest proves the Restart timer restarts on
// every Configure-Request: a Nak at 2.9 seconds sends the revised request at
// once, and its retransmission follows three seconds after it (5.9 seconds),
// never on a fixed three-second cadence from the first request.
// Break: drop restart.Reset from negotiateLCP's sent closure; a retransmission
// then leaves at 3 seconds and the count at 5.899 seconds is three.
func TestLCPRestartTimerFollowsLastRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := startLCP(t, w, frames, stop)
		synctest.Wait()
		requests, _ := w.requests()
		if len(requests) != 1 {
			t.Fatalf("initial requests = %d, want 1", len(requests))
		}
		time.Sleep(2900 * time.Millisecond)
		frames <- mruNak(requests[0])
		synctest.Wait()
		requests, _ = w.requests()
		if len(requests) != 2 {
			t.Fatalf("requests after the Nak = %d, want the revised one", len(requests))
		}
		time.Sleep(2999 * time.Millisecond)
		synctest.Wait()
		if got, _ := w.requests(); len(got) != 2 {
			t.Fatalf("requests 2.999s after the revised one = %d, want 2: the timer did not restart on send", len(got))
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		requests, at := w.requests()
		if len(requests) != 3 {
			t.Fatalf("requests 3s after the revised one = %d, want its retransmission", len(requests))
		}
		if !bytes.Equal(requests[2], requests[1]) {
			t.Fatalf("retransmission % x, want the revised request % x", requests[2], requests[1])
		}
		if gap := at[2].Sub(at[1]); gap != lcpRestartInterval {
			t.Fatalf("retransmission %v after the revised request, want %v", gap, lcpRestartInterval)
		}
		close(stop)
		<-done
	})
}

// TestLCPRestartCounterEndsSilentPeer proves Max-Configure: a peer that never
// answers gets exactly ten identical Configure-Requests three seconds apart,
// and LCP ends with an LCP timeout when the tenth one's timer expires (TO-).
// Break: remove the configureRemaining check in negotiateLCP's restart case;
// requests then continue to the 60-second deadline (20 of them).
func TestLCPRestartCounterEndsSilentPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		stop := make(chan struct{})
		defer close(stop)
		got := <-startLCP(t, w, make(chan readFrame), stop)
		if got.err == nil {
			t.Fatal("LCP opened with a silent peer")
		}
		if !strings.Contains(got.err.Error(), "LCP timeout") {
			t.Fatalf("error = %v, want an LCP timeout", got.err)
		}
		requests, at := w.requests()
		if len(requests) != lcpConfigureMax {
			t.Fatalf("Configure-Requests = %d, want Max-Configure %d", len(requests), lcpConfigureMax)
		}
		for i := 1; i < len(requests); i++ {
			if !bytes.Equal(requests[i], requests[0]) {
				t.Fatalf("request %d = % x, want retransmission % x", i, requests[i], requests[0])
			}
			if gap := at[i].Sub(at[i-1]); gap != lcpRestartInterval {
				t.Fatalf("request %d left %v after the previous one, want %v", i, gap, lcpRestartInterval)
			}
		}
		if want := lcpConfigureMax * lcpRestartInterval; got.elapsed != want {
			t.Fatalf("LCP ended after %v, want %v", got.elapsed, want)
		}
	})
}

// TestLCPReplyRenewsSpentRestartCounter spends most of the budget before a
// valid Nak or Reject, then leaves the peer silent so the next timer expiry
// observes the renewed counter. Both Req-Sent and Ack-Sent carry irc on RCN.
// Break: remove configureRemaining = lcpConfigureMax in the Nak/Reject
// branch. The revised request then spends the last count, and LCP ends at
// 27.5 seconds after ten requests instead of 54.5 seconds after nineteen.
func TestLCPReplyRenewsSpentRestartCounter(t *testing.T) {
	for _, ackSent := range []bool{false, true} {
		state := "ReqSent"
		if ackSent {
			state = "AckSent"
		}
		for _, code := range []uint8{ppp.LCPConfigureNak, ppp.LCPConfigureReject} {
			reply := "Nak"
			if code == ppp.LCPConfigureReject {
				reply = "Reject"
			}
			t.Run(state+"/"+reply, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					w := &timedLCPWriter{}
					frames := make(chan readFrame, 1)
					stop := make(chan struct{})
					defer close(stop)
					done := startLCP(t, w, frames, stop)
					synctest.Wait()
					if ackSent {
						frames <- serverFrame(ppp.LCPConfigureRequest, 77, []byte{ppp.LCPOptMRU, 4, 0x05, 0xd4})
						synctest.Wait()
					}
					// sleep(test-clock): spend eight retransmissions, then reply before the ninth timer expires.
					time.Sleep(24500 * time.Millisecond)
					synctest.Wait()
					requests, _ := w.requests()
					if len(requests) != 9 {
						t.Fatalf("requests before the reply = %d, want 9", len(requests))
					}
					last := requests[len(requests)-1]
					answer := mruNak(last)
					if code == ppp.LCPConfigureReject {
						answer = serverFrame(code, last[3], bytes.Clone(last[6:10]))
					}
					frames <- answer
					synctest.Wait()
					got := <-done
					if got.err == nil {
						t.Fatal("LCP opened without an Ack")
					}
					if !strings.Contains(got.err.Error(), "LCP timeout") {
						t.Fatalf("error = %v, want an LCP timeout", got.err)
					}
					if got.elapsed != 54500*time.Millisecond {
						t.Fatalf("LCP ended after %v, want 54.5s: the reply must renew the spent budget", got.elapsed)
					}
					requests, at := w.requests()
					if len(requests) != 19 {
						t.Fatalf("Configure-Requests = %d, want nine before the reply and ten after it", len(requests))
					}
					for i := 10; i < len(requests); i++ {
						if !bytes.Equal(requests[i], requests[9]) {
							t.Fatalf("request %d = % x, want revised request retransmission % x", i, requests[i], requests[9])
						}
						if gap := at[i].Sub(at[i-1]); gap != lcpRestartInterval {
							t.Fatalf("request %d left %v after the previous one, want %v", i, gap, lcpRestartInterval)
						}
					}
				})
			})
		}
	}
}

// TestLCPDeadlineEndsNakLoop proves the higher-level deadline: a peer that
// Naks every request before the Restart timer expires still cannot keep LCP
// negotiating beyond the 60-second convergence bound.
// Break: remove the deadline case from negotiateLCP; the negotiation never ends.
func TestLCPDeadlineEndsNakLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		defer close(stop)
		done := startLCP(t, w, frames, stop)
		var got lcpRunResult
		for ended := false; !ended; {
			time.Sleep(2900 * time.Millisecond)
			synctest.Wait()
			select {
			case got = <-done:
				ended = true
				continue
			default:
			}
			requests, _ := w.requests()
			frames <- mruNak(requests[len(requests)-1])
		}
		if got.err == nil {
			t.Fatal("LCP opened while the peer Naks every request")
		}
		if !strings.Contains(got.err.Error(), "LCP timeout") {
			t.Fatalf("error = %v, want an LCP timeout", got.err)
		}
		if got.elapsed != lcpNegotiationTimeout {
			t.Fatalf("LCP ended after %v, want the %v deadline", got.elapsed, lcpNegotiationTimeout)
		}
		if requests, _ := w.requests(); len(requests) <= lcpConfigureMax {
			t.Fatalf("Configure-Requests = %d, want more than Max-Configure before the convergence deadline", len(requests))
		}
	})
}

// TestLCPNakWriteFailureEndsLCP proves a failed Configure-Nak write ends LCP
// with that write error instead of waiting for the peer's retransmission:
// the peer's Magic-Number of zero earns a Nak, and only that frame fails.
// Break: discard sendLCPOptionReply's error at the Nak call in negotiateLCP;
// LCP then waits and ends on stop, not on the injected error.
func TestLCPNakWriteFailureEndsLCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		injected := errors.New("injected Nak write failure")
		w := &timedLCPWriter{failCode: ppp.LCPConfigureNak, failErr: injected}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := startLCP(t, w, frames, stop)
		synctest.Wait()
		frames <- serverFrame(ppp.LCPConfigureRequest, 77, []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0})
		synctest.Wait()
		var got lcpRunResult
		select {
		case got = <-done:
		default:
			close(stop)
			got = <-done
			t.Fatalf("LCP kept negotiating after the Nak write failed; it ended with %v", got.err)
		}
		close(stop)
		if !errors.Is(got.err, injected) {
			t.Fatalf("error = %v, want the injected Nak write failure", got.err)
		}
	})
}
