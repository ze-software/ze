// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- restart failure ownership.
// Related: connected_exchange_test.go -- scripted peer; dialer.go -- supervisor.

package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestConnectedRestartFailures checks the actual supervisor's termination and
// stable-result ownership for failures at each restart boundary. No test owns a
// real unit: kernel reservation/release remains the later integration proof.
func TestConnectedRestartFailures(t *testing.T) {
	for _, name := range []string{"drop", "pass", "write", "chap", "pap", "lcp-timeout", "auth-timeout", "ipcp-timeout", "unit-error", "stop-lcp", "stop-auth", "stop-ipcp"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				seed, initial := w.session(t)
				transportCloses := 0
				link := &sessionLink{channel: w, stopped: make(chan struct{}), closeTransport: func() { transportCloses++ }}
				s, err := newConnectedSession(link, initial, seed.cfg, w.setMode)
				if err != nil {
					t.Fatal(err)
				}
				before := s.current
				var log bytes.Buffer
				done := make(chan struct{})
				if name == "drop" {
					w.modeFailure = ppp.NPModeDrop
				}
				if name == "pass" {
					w.modeFailure = ppp.NPModePass
				}
				if name == "write" {
					w.writeFailure = io.ErrShortWrite
				}
				go superviseNetworkPhase(link, s, done, slog.New(slog.NewTextHandler(&log, nil)))
				synctest.Wait()
				want := ""
				switch name {
				case "drop", "write", "lcp-timeout", "stop-lcp":
					w.send(ppp.ProtoLCP, 1, 91, exchangePeerOptions(0))
					switch name {
					case "drop":
						want = "cannot suspend IPv4"
						w.quiet(t)
					case "write":
						want = "short write"
					case "lcp-timeout":
						want = "LCP timeout"
						// sleep(timer): advance the production LCP deadline in fake time.
						time.Sleep(lcpNegotiationTimeout)
					case "stop-lcp":
						want = "stopped during LCP"
						if err := link.Close(); err != nil {
							t.Fatal(err)
						}
					}
				case "chap", "pap", "auth-timeout", "stop-auth":
					auth := ppp.ProtoCHAP
					if name == "pap" {
						auth = ppp.ProtoPAP
					}
					w.openLCP(t, auth, initial.magic)
					switch name {
					case "chap":
						want = "CHAP auth failed"
						w.authenticate(t, auth, false)
					case "pap":
						want = "PAP auth rejected"
						w.authenticate(t, auth, false)
					case "auth-timeout":
						want = "auth timeout"
						// sleep(timer): no CHAP challenge arrives before the auth deadline.
						time.Sleep(authTimeout)
					case "stop-auth":
						want = "stopped during auth"
						if err := link.Close(); err != nil {
							t.Fatal(err)
						}
					}
				case "pass", "ipcp-timeout", "unit-error", "stop-ipcp":
					w.openLCP(t, 0, initial.magic)
					switch name {
					case "pass":
						want = "cannot resume IPv4"
						w.openIPCP(t, initial.ipcpIdentifier, 0)
					case "ipcp-timeout":
						want = "IPCP timeout"
						// sleep(timer): advance the production IPCP deadline in fake time.
						time.Sleep(ncpTimeout)
					case "unit-error":
						want = "unexpected EOF"
						w.unit <- readFrame{err: io.ErrUnexpectedEOF}
					case "stop-ipcp":
						want = "stopped during IPCP"
						if err := link.Close(); err != nil {
							t.Fatal(err)
						}
					}
				}
				synctest.Wait()
				select {
				case <-done:
				default:
					t.Fatal("failure did not end network worker")
				}
				if !strings.Contains(log.String(), want) {
					t.Fatalf("failure log %q, want %q", log.String(), want)
				}
				if s.current != before {
					t.Fatal("failure published a partial negotiated result")
				}
				passes := 0
				if name == "pass" {
					passes = 1
				}
				if w.passes() != passes {
					t.Fatalf("failure enabled IPv4: %v", w.modes)
				}
				select {
				case <-link.stopped:
				default:
					t.Fatal("supervisor did not publish session Done")
				}
				if w.closes != 1 {
					t.Fatalf("supervisor channel closes = %d", w.closes)
				}
				if transportCloses != 1 {
					t.Fatalf("supervisor transport closes = %d", transportCloses)
				}
				if err := link.Close(); err != nil {
					t.Fatal(err)
				}
				if w.closes != 1 {
					t.Fatalf("channel closes = %d", w.closes)
				}
				if transportCloses != 1 {
					t.Fatalf("transport closes = %d", transportCloses)
				}
				if _, err := link.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("post-close write: %v", err)
				}
				if err := link.setIPv4Mode(-1, ppp.NPModePass); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("post-close mode touched unit: %v", err)
				}
			})
		})
	}
}

// TestConnectedRestartReentry keeps DROP across interrupted authentication and
// IPCP, then publishes only the final completed exchange on the same worker.
func TestConnectedRestartReentry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		magic := w.openLCP(t, ppp.ProtoCHAP, initial.magic)
		w.send(ppp.ProtoCHAP, 1, 63, []byte{4, 9, 8, 7, 6})
		response := w.take(t, ppp.ProtoCHAP, 2)
		if response[3] != 63 {
			t.Fatal("initial challenge not answered")
		}
		// Reopening CHAP must forget the successfully written old Response.
		magic = w.openLCP(t, ppp.ProtoCHAP, magic)
		w.send(ppp.ProtoCHAP, 3, 63, nil)
		w.quiet(t)
		// No new authentication verdict: the next request interrupts CHAP again.
		magic = w.openLCP(t, 0, magic)
		request := w.take(t, ppp.ProtoIPCP, 1)
		if request[3] != 41 {
			t.Fatalf("first IPCP Identifier = %d", request[3])
		}
		// No IPCP verdict: the next request interrupts the unit's exchange.
		magic = w.openLCP(t, ppp.ProtoPAP, magic)
		w.authenticate(t, ppp.ProtoPAP, true)
		if s.current.magic != initial.magic {
			t.Fatal("interrupted result published")
		}
		w.openIPCP(t, 41, 0)
		if s.current.magic != magic {
			t.Fatal("final result not published")
		}
		if len(w.modes) != 2 {
			t.Fatalf("interruption changed forwarding: %v", w.modes)
		}
		if w.modes[0] != ppp.NPModeDrop {
			t.Fatalf("first mode: %v", w.modes)
		}
		if w.modes[1] != ppp.NPModePass {
			t.Fatalf("last mode: %v", w.modes)
		}
		w.send(ppp.ProtoLCP, 9, 99, []byte{0, 0, 0, 1})
		w.take(t, ppp.ProtoLCP, 10)
	})
}

// TestConnectedRestartEchoDeadline holds authentication across an old echo tick.
// The next keepalive must wait a full interval after IPCP, with Identifier one.
func TestConnectedRestartEchoDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		// sleep(timer): position restart just before the old echo deadline.
		time.Sleep(testKeepaliveInterval - time.Second)
		w.openLCP(t, ppp.ProtoCHAP, initial.magic)
		// sleep(timer): cross that deadline while authentication owns the transport.
		time.Sleep(2 * time.Second)
		w.quiet(t)
		w.authenticate(t, ppp.ProtoCHAP, true)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		// sleep(timer): no keepalive is due until a full fresh interval elapsed.
		time.Sleep(testKeepaliveInterval - time.Nanosecond)
		w.quiet(t)
		// sleep(timer): reach the fresh ticker's first deadline exactly.
		time.Sleep(time.Nanosecond)
		echo := w.take(t, ppp.ProtoLCP, 9)
		if echo[3] != 1 {
			t.Fatalf("first post-restart echo Identifier = %d", echo[3])
		}
	})
}
