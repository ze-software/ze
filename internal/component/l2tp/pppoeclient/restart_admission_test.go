// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- restart result admission.
// Related: connected_exchange_test.go -- complete scripted exchanges.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestInitialMTUAdmission checks final result admission with valid config and
// separately checks the wire boundary. MRU 66 receives Nak, then peer EOF ends
// negotiation before Connect. MRU 68 completes IPCP and reaches the fake ioctl.
// TestNegotiatedResultMinimum supplies the independent guard-removal red proof.
func TestInitialMTUAdmission(t *testing.T) {
	for _, mtu := range []uint16{66, 68} {
		t.Run(strconv.Itoa(int(mtu)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				w.unit = w.frames // Initial IPCP arrives on the unconnected channel.
				result := make(chan error, 1)
				stop := make(chan struct{})
				defer close(stop)
				link := &sessionLink{channel: w}
				go func() {
					_, err := negotiateSession(link, testFrameReader(t, w.frames, nil), -1, 0,
						sessionConfig{mtu: 1492, chanFD: -1}, stop,
						slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
					result <- err
				}()
				request := w.take(t, ppp.ProtoLCP, 1)
				options := exchangePeerOptions(0)
				binary.BigEndian.PutUint16(options[2:], mtu)
				w.send(ppp.ProtoLCP, 1, 91, options)
				if mtu == 66 {
					nak := w.take(t, ppp.ProtoLCP, 3)
					if !bytes.Equal(nak, exchangeFrame(ppp.ProtoLCP, 3, 91, []byte{1, 4, 0, 68})) {
						t.Fatalf("LCP Nak: %x", nak)
					}
					w.send(ppp.ProtoLCP, 2, request[3], request[6:])
					w.quiet(t)
					w.frames <- readFrame{err: io.EOF}
					if err := <-result; !errors.Is(err, io.EOF) {
						t.Fatalf("uncorrected peer = %v, want EOF before Connect", err)
					}
					// The wire can no longer deliver 66 to final admission. Supply
					// that raw result directly, with valid config and addresses.
					refused, err := newNegotiatedSession(sessionResult{
						localIP: netip.MustParseAddr("192.0.2.2"), peerIP: netip.MustParseAddr("192.0.2.1"),
						negMTU: mtu, peerMRU: mtu,
					}, sessionConfig{mtu: 1492})
					if !errors.Is(err, errSessionMTURange) {
						t.Fatalf("initial admission = %v, want MTU refusal", err)
					}
					if refused != (negotiatedSession{}) {
						t.Fatal("invalid raw result became publishable")
					}
					return
				}
				ack := w.take(t, ppp.ProtoLCP, 2)
				if !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 91, options)) {
					t.Fatalf("LCP Ack: %x", ack)
				}
				w.send(ppp.ProtoLCP, 2, request[3], request[6:])
				w.openIPCP(t, 0, 0)
				err := <-result
				if err == nil {
					t.Fatal("fake descriptor unexpectedly connected")
				}
				if !strings.Contains(err.Error(), "PPPIOCCONNECT") {
					t.Fatalf("boundary did not reach connect: %v", err)
				}
			})
		})
	}
}

// TestRestartEchoSequence acknowledges an Echo before restart, then requires the
// next Identifier and newly negotiated Magic-Number on the same worker.
func TestRestartEchoSequence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		// sleep(timer): advance the real worker's ticker in synctest fake time.
		time.Sleep(testKeepaliveInterval)
		first := w.take(t, ppp.ProtoLCP, 9)
		if first[3] != 1 {
			t.Fatalf("first Echo Identifier = %d", first[3])
		}
		if binary.BigEndian.Uint32(first[6:]) != initial.magic {
			t.Fatalf("initial Echo magic: %x", first)
		}
		w.send(ppp.ProtoLCP, 10, first[3], first[6:])
		magic := w.openLCP(t, 0, initial.magic)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		// The restart completes only once the interface applied its result.
		applyPending(t, s)
		// sleep(timer): reach the first tick after the completed restart.
		time.Sleep(testKeepaliveInterval)
		next := w.take(t, ppp.ProtoLCP, 9)
		if next[3] != 2 {
			t.Fatalf("post-restart Echo Identifier = %d, want 2", next[3])
		}
		if binary.BigEndian.Uint32(next[6:]) != magic {
			t.Fatalf("post-restart Echo magic: %x, want %08x", next, magic)
		}
	})
}

// TestRestartMTUAdmission checks invalid raw-result admission with valid config,
// then the supervisor's failure ordering when a peer does not correct MRU 66.
// The direct constructor proof and wire failure proof are separate: no claim is
// made that valid wire negotiation still produces a completed result of 66.
func TestRestartMTUAdmission(t *testing.T) {
	for _, mtu := range []uint16{66, 68} {
		t.Run(strconv.Itoa(int(mtu)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				seed, initial := w.session(t)
				link := &sessionLink{channel: w, stopped: make(chan struct{}), closeTransport: func() {}}
				s, err := newConnectedSession(link, initial, seed.cfg, w.setMode)
				if err != nil {
					t.Fatal(err)
				}
				before := s.current
				var log bytes.Buffer
				done := make(chan struct{})
				defer link.Close() //nolint:errcheck // Test cleanup after lifecycle assertions.
				go superviseNetworkPhase(link, s, done, slog.New(slog.NewTextHandler(&log, nil)))
				options := exchangePeerOptions(0)
				binary.BigEndian.PutUint16(options[2:], mtu)
				w.send(ppp.ProtoLCP, 1, 91, options)
				request := w.take(t, ppp.ProtoLCP, 1)
				if mtu == 68 {
					ack := w.take(t, ppp.ProtoLCP, 2)
					if !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 91, options)) {
						t.Fatalf("LCP Ack: %x", ack)
					}
					w.send(ppp.ProtoLCP, 2, request[3], request[6:])
					w.openIPCP(t, initial.ipcpIdentifier, 0)
					synctest.Wait()
					if len(s.changes) != 1 {
						t.Fatal("boundary did not publish result")
					}
					if w.passes() != 0 {
						t.Fatalf("IPv4 passed before the interface applied the result: %v", w.modes)
					}
					if got := applyPending(t, s); got.NegMTU != mtu {
						t.Fatalf("published MTU = %d, want %d", got.NegMTU, mtu)
					}
					if w.passes() != 1 {
						t.Fatalf("boundary did not enable IPv4: %v", w.modes)
					}
					if s.current.negMTU != mtu {
						t.Fatalf("MTU = %d, want %d", s.current.negMTU, mtu)
					}
					return
				}
				nak := w.take(t, ppp.ProtoLCP, 3)
				if !bytes.Equal(nak, exchangeFrame(ppp.ProtoLCP, 3, 91, []byte{1, 4, 0, 68})) {
					t.Fatalf("LCP Nak: %x", nak)
				}
				w.send(ppp.ProtoLCP, 2, request[3], request[6:])
				w.quiet(t)
				// Independently reach final result admission with VALID config.
				raw := initial
				raw.negMTU, raw.peerMRU = mtu, mtu
				refused, err := newNegotiatedSession(raw, s.cfg)
				if !errors.Is(err, errSessionMTURange) {
					t.Fatalf("restart result admission = %v, want MTU refusal", err)
				}
				if refused != (negotiatedSession{}) {
					t.Fatal("invalid raw result became publishable")
				}
				// This EOF proves supervisor propagation, not constructor wiring.
				w.frames <- readFrame{err: io.EOF}
				synctest.Wait()
				if w.passes() != 0 {
					t.Fatalf("invalid MTU enabled IPv4: %v", w.modes)
				}
				if s.current != before {
					t.Fatal("invalid result replaced current")
				}
				if len(s.changes) != 0 {
					t.Fatal("invalid result published")
				}
				select {
				case <-link.stopped:
				default:
					t.Fatal("invalid result did not publish session Done")
				}
				if !strings.Contains(log.String(), io.EOF.Error()) {
					t.Fatalf("missing admission error: %s", log.String())
				}
			})
		})
	}
}

// TestRestartConvergenceBound interrupts each IPCP exchange with fresh LCP.
// The eleventh request must end the supervisor without PASS or a result change.
func TestRestartConvergenceBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		seed, initial := w.session(t)
		link := &sessionLink{channel: w, stopped: make(chan struct{}), closeTransport: func() {}}
		s, err := newConnectedSession(link, initial, seed.cfg, w.setMode)
		if err != nil {
			t.Fatal(err)
		}
		before := s.current
		var log bytes.Buffer
		done := make(chan struct{})
		defer link.Close() //nolint:errcheck // Test cleanup after lifecycle assertions.
		go superviseNetworkPhase(link, s, done, slog.New(slog.NewTextHandler(&log, nil)))
		magic := initial.magic
		for range 10 {
			magic = w.openLCP(t, 0, magic)
			w.take(t, ppp.ProtoIPCP, 1)
		}
		w.send(ppp.ProtoLCP, 1, 91, exchangePeerOptions(0))
		select {
		case <-link.stopped:
		default:
			t.Fatal("repeated reentry did not exhaust the attempt")
		}
		if !strings.Contains(log.String(), "restart limit") {
			t.Fatalf("missing exhaustion diagnostic: %s", log.String())
		}
		if w.passes() != 0 {
			t.Fatalf("exhausted attempt enabled IPv4: %v", w.modes)
		}
		if s.current != before {
			t.Fatal("exhausted attempt replaced current")
		}
		if len(s.changes) != 0 {
			t.Fatal("exhausted attempt published a change")
		}
		if _, err := link.Write(nil); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("transport remains writable: %v", err)
		}
	})
}
