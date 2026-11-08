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
	reader := testFrameReader(t, frames, nil)
	done := make(chan lcpRunResult, 1)
	go func() {
		start := time.Now()
		var buf [ppp.MaxFrameBufLen]byte
		_, err := negotiateLCP(w, reader, buf[:], sessionConfig{mtu: 1492}, clientMagic, nil, stop,
			slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		done <- lcpRunResult{err: err, elapsed: time.Since(start)}
	}()
	return done
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

// TestLCPDeadlineEndsNakLoop proves the higher-level deadline: a peer that
// Naks every request 2.9 seconds after it resets the Restart counter each
// time, so only the 60-second negotiation deadline ends LCP.
// Break: remove the irc (configureRemaining = lcpConfigureMax) in the
// Nak/Reject branch; the counter then ends LCP after ten requests, near 26
// seconds instead of 60.
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
			t.Fatalf("Configure-Requests = %d, want more than Max-Configure: each Nak restarts the counter", len(requests))
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
