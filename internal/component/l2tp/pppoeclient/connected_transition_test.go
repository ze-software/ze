// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- unit queue phase boundaries.
// Related: phases.go -- ordered dispatch between LCP, authentication and IPCP.

package pppoeclient

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// transitionInput inserts one reader event at a phase-completion log, on the
// worker itself. This controls the boundary without racing a packet against
// select. The fixture owns the handler; WithAttrs/WithGroup retain its injection.
type transitionInput struct {
	message  string
	input    readFrame
	unit     chan<- readFrame
	injected int
}

func (h *transitionInput) Enabled(context.Context, slog.Level) bool { return true }
func (h *transitionInput) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *transitionInput) WithGroup(string) slog.Handler            { return h }
func (h *transitionInput) Handle(_ context.Context, record slog.Record) error { //nolint:gocritic // slog.Handler requires the Record parameter by value.
	if record.Message == h.message {
		h.unit <- h.input
		h.injected++
	}
	return nil
}

// TestConnectedTransitionReadErrors proves each phase boundary propagates
// reader failure rather than enabling IPv4 or publishing partial state.
func TestConnectedTransitionReadErrors(t *testing.T) {
	for _, phase := range []string{"entry", "lcp", "auth"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				s, initial := w.session(t)
				before := s.current
				if phase == "entry" {
					w.unit <- readFrame{err: io.ErrUnexpectedEOF}
					pending, ok := requestLCPReentry(ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 91, Data: exchangePeerOptions(0)})
					if !ok {
						t.Fatal("valid restart request refused")
					}
					err := s.restart(pending, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
					if !errors.Is(err, io.ErrUnexpectedEOF) {
						t.Fatalf("entry drain: %v", err)
					}
					// Restart dispatches scr/sca before requesting the first read.
					w.take(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
					ack := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
					if !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 91, exchangePeerOptions(0))) {
						t.Fatalf("entry error: wrong pending Ack %x", ack)
					}
					w.quiet(t)
				} else {
					message := "pppoe-client: LCP opened"
					if phase == "auth" {
						message = "pppoe-client: CHAP auth success"
					}
					h := &transitionInput{message: message, unit: w.unit, input: readFrame{err: io.ErrUnexpectedEOF}}
					done, stop := make(chan struct{}), make(chan struct{})
					defer close(stop)
					go keepaliveLoop(s, done, stop, slog.New(h))
					w.openLCP(t, ppp.ProtoCHAP, initial.magic)
					if phase == "auth" {
						w.authenticate(t, ppp.ProtoCHAP, true)
					}
					synctest.Wait()
					if h.injected != 1 {
						t.Fatalf("phase injection count = %d", h.injected)
					}
					select {
					case <-done:
					default:
						t.Fatal("transition error did not end worker")
					}
				}
				if s.current != before {
					t.Fatal("transition error published partial result")
				}
				if len(w.modes) != 1 {
					t.Fatalf("transition error resumed forwarding: %v", w.modes)
				}
				if w.modes[0] != ppp.NPModeDrop {
					t.Fatalf("transition failed to suspend: %v", w.modes)
				}
			})
		})
	}
}

// TestConnectedAuthQueueDiscard consumes IPCP termination while authentication
// is still pending, then completes authentication and fresh IPCP. Input queued
// only after Success is not premature; the eager test covers that distinct case.
func TestConnectedAuthQueueDiscard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer func() { close(stop); <-done }()
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w.openLCP(t, ppp.ProtoCHAP, initial.magic)
		w.send(ppp.ProtoCHAP, 1, 63, []byte{4, 9, 8, 7, 6, 'a', 'c'})
		response := w.take(t, ppp.ProtoCHAP, 2)
		digest := md5.Sum([]byte{63, 's', 'e', 'c', 'r', 'e', 't', 9, 8, 7, 6}) //nolint:gosec // CHAP-MD5.
		want := append([]byte{16}, digest[:]...)
		want = append(want, "subscriber"...)
		if !bytes.Equal(response, exchangeFrame(ppp.ProtoCHAP, 2, 63, want)) {
			t.Fatalf("CHAP response: %x", response)
		}
		w.send(ppp.ProtoIPCP, 5, 33, nil)
		if len(w.unit) != 0 {
			t.Fatal("premature authentication input was not consumed")
		}
		w.quiet(t)
		if len(s.changes) != 0 {
			t.Fatal("premature IPCP published a result")
		}
		if w.passes() != 0 {
			t.Fatal("premature IPCP resumed forwarding")
		}
		w.send(ppp.ProtoCHAP, 3, 63, nil)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		synctest.Wait()
		if w.passes() != 0 {
			t.Fatalf("IPCP passed IPv4 before the interface applied the result: %v", w.modes)
		}
		applyPending(t, s)
		if w.passes() != 1 {
			t.Fatalf("IPCP did not reopen: %v", w.modes)
		}
	})
}
