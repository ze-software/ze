// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured keepalive.
// Related: network_phase.go -- keepaliveLoop reads interval and failures from sessionConfig.

package pppoeclient

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestKeepaliveEndsAtConfiguredFailures proves AC-2 at the keepalive loop:
// with interval 2 s and failures 2, and no Echo-Reply, the session ends on
// the second unanswered Echo-Request, 4 s in. synctest's clock makes the
// interval exact. A loop that kept the 10 s interval sends nothing at 2 s, and
// one that compared with > rather than >= stays up after the second request.
func TestKeepaliveEndsAtConfiguredFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, _ := w.session(t)
		s.cfg.keepaliveInterval = 2 * time.Second
		s.cfg.keepaliveFailures = 2
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		w.quiet(t)

		// sleep(timer): reach the first configured echo deadline.
		time.Sleep(2 * time.Second)
		first := w.take(t, ppp.ProtoLCP, ppp.LCPEchoRequest)
		if first[3] != 1 {
			t.Fatalf("first Echo Identifier = %d", first[3])
		}
		select {
		case <-done:
			t.Fatal("the session ended on the first unanswered Echo-Request, want the second")
		default:
		}

		// sleep(timer): reach the second configured echo deadline.
		time.Sleep(2 * time.Second)
		w.take(t, ppp.ProtoLCP, ppp.LCPEchoRequest)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("the session stayed up after the second unanswered Echo-Request")
		}
	})
}

// TestConnectedSessionRefusesMissingKeepalive proves the second check of the
// pair: a connected session built without keepalive values is refused, so a
// zero interval never reaches the echo ticker.
func TestConnectedSessionRefusesMissingKeepalive(t *testing.T) {
	for _, name := range []string{"interval", "failures"} {
		t.Run(name, func(t *testing.T) {
			w := newConnectedExchange()
			seed, initial := w.session(t)
			cfg := seed.cfg
			if name == "interval" {
				cfg.keepaliveInterval = 0
			}
			if name == "failures" {
				cfg.keepaliveFailures = 0
			}
			if _, err := newConnectedSession(w, initial, cfg, w.setMode); err == nil {
				t.Fatalf("a session with no keepalive %s was admitted", name)
			}
		})
	}
}

// TestDialRefusesInvalidTiming proves Dial checks the timing before it opens
// any socket: the source interface here does not exist, so only the timing
// check can produce the keepalive refusal.
func TestDialRefusesInvalidTiming(t *testing.T) {
	timing := testDialTiming
	timing.KeepaliveInterval = 0
	_, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{SourceInterface: "ze-no-such-if0", Timing: timing},
		make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("Dial accepted a zero keepalive interval")
	}
	if want := "keepalive interval"; !containsString(err.Error(), want) {
		t.Fatalf("Dial error = %q, want it to name %q", err, want)
	}
}

// containsString reports whether sub occurs in s.
func containsString(s, sub string) bool {
	return strings.Contains(s, sub)
}
