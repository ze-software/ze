// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected lifecycle admission.
// Related: connected_session.go -- constructor and invalid-value contract.

package pppoeclient

import (
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestConnectedSessionAdmission exercises each independent input invariant and
// proves refusals do no I/O. Valid construction must copy mutable CHAP state.
func TestConnectedSessionAdmission(t *testing.T) {
	for _, name := range []string{"nil", "typed-nil", "missing-reader", "zero-reader", "mode", "mtu-low", "mtu-high", "local-v6", "local-zero", "peer-v6", "peer-zero", "neg-low", "neg-high", "peer-mru", "valid"} {
		t.Run(name, func(t *testing.T) {
			w := newConnectedExchange()
			seed, initial := w.session(t)
			cfg := seed.cfg
			var transport io.ReadWriteCloser = w
			mode := w.setMode
			switch name {
			case "nil":
				transport = nil
			case "typed-nil":
				transport = (*connectedExchange)(nil)
			case "missing-reader":
				initial.reader = nil
			case "zero-reader":
				initial.reader = &frameReader{}
			case "mode":
				mode = nil
			case "mtu-low":
				cfg.mtu = sessionMTUMin - 1
			case "mtu-high":
				cfg.mtu = sessionMTUMax + 1
			case "local-v6":
				initial.localIP = netip.MustParseAddr("2001:db8::1")
			case "local-zero":
				initial.localIP = netip.IPv4Unspecified()
			case "peer-v6":
				initial.peerIP = netip.MustParseAddr("2001:db8::2")
			case "peer-zero":
				initial.peerIP = netip.IPv4Unspecified()
			case "neg-low":
				initial.negMTU = sessionMTUMin - 1
			case "neg-high":
				initial.negMTU = cfg.mtu + 1
			case "peer-mru":
				initial.peerMRU = initial.negMTU - 1
			case "valid":
			}
			s, err := newConnectedSession(transport, initial, cfg, mode)
			if name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if s.current.chap == initial.chap {
					t.Fatal("mutable CHAP alias retained")
				}
				initial.chap.pending = false
				if !s.current.chap.pending {
					t.Fatal("constructor did not retain CHAP snapshot")
				}
				if s.reader != initial.reader {
					t.Fatal("receive owner replaced")
				}
			} else {
				if err == nil {
					t.Fatal("invalid input admitted")
				}
				if s != nil {
					t.Fatal("failed construction published an owner")
				}
			}
			if len(w.events) != 0 {
				t.Fatalf("constructor performed I/O: %v", w.events)
			}
			if w.reads != 0 {
				t.Fatal("constructor started a reader")
			}
			if w.closes != 0 {
				t.Fatal("constructor closed borrowed transport")
			}
		})
	}
}

// TestConnectedRestartAdmission proves invalid lifecycle values fail before any
// mode operation or packet, and keepalive closes done for nil/zero owners.
func TestConnectedRestartAdmission(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, name := range []string{"nil", "zero", "missing-request", "zero-request"} {
		t.Run(name, func(t *testing.T) {
			w := newConnectedExchange()
			s, _ := w.session(t)
			var pending *lcpReentry
			switch name {
			case "nil":
				s = nil
			case "zero":
				s = &connectedSession{}
			case "missing-request":
			case "zero-request":
				pending = &lcpReentry{}
			}
			if err := s.restart(pending, make(chan struct{}), logger); err == nil {
				t.Fatal("invalid restart admitted")
			}
			if len(w.events) != 0 {
				t.Fatalf("invalid restart did I/O: %v", w.events)
			}
			switch name {
			case "nil", "zero":
				done := make(chan struct{})
				keepaliveLoop(s, done, make(chan struct{}), logger)
				select {
				case <-done:
				default:
					t.Fatal("invalid owner did not close done")
				}
			}
		})
	}
}

// TestConnectedTransitionDrain preserves the old drain's obligations at the
// replacement boundary: four premature unit packets are consumed in LCP, then
// the exact terminal error ends negotiation without admitting a later phase.
func TestConnectedTransitionDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// No connected unit is valid during initial establishment. A channel
		// read error must remain the result rather than a missing-unit refusal.
		control := make(chan readFrame)
		close(control)
		initial := testFrameReader(t, control, nil)
		var initialBuf [ppp.MaxFrameBufLen]byte
		_, initialErr := negotiateLCP(io.Discard, initial, initialBuf[:], sessionConfig{mtu: 1492}, clientMagic, nil, make(chan struct{}), slog.Default(), new(uint8))
		if !errors.Is(initialErr, io.EOF) {
			t.Fatalf("initial nil unit refused: %v", initialErr)
		}
		unit := make(chan readFrame, 5)
		for range 4 {
			unit <- readFrame{data: []byte{0x80, 0x21}}
		}
		unit <- readFrame{err: io.ErrUnexpectedEOF}
		reader := testFrameReader(t, make(chan readFrame), unit)
		w := &recordingRWC{}
		var buf [ppp.MaxFrameBufLen]byte
		// RFC 1661 Section 3.4: discard non-LCP input in establishment.
		_, err := negotiateLCP(w, reader, buf[:], sessionConfig{mtu: 1492}, clientMagic, nil, make(chan struct{}), slog.Default(), new(uint8))
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("read failure lost: %v", err)
		}
		if len(unit) != 0 {
			t.Fatal("transition retained premature unit frames")
		}
		packets := w.lcpPackets(t)
		if len(packets) != 1 {
			t.Fatalf("premature packets produced replies: %+v", packets)
		}
		if packets[0].Code != ppp.LCPConfigureRequest {
			t.Fatalf("initial request missing: %+v", packets)
		}
	})
}

// TestConnectedLinkModeAfterClose checks the lifecycle guard before both mode
// validation and the syscall: even a reused/invalid descriptor is never reached.
func TestConnectedLinkModeAfterClose(t *testing.T) {
	w := newConnectedExchange()
	closes := 0
	link := &sessionLink{channel: w, stopped: make(chan struct{}), closeTransport: func() { closes++ }}
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []ppp.NPMode{ppp.NPModeUnspecified, ppp.NPModePass, ppp.NPModeDrop, 255} {
		if err := link.setIPv4Mode(-1, mode); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("closed mode %d reached validation/ioctl: %v", mode, err)
		}
	}
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	if closes != 1 {
		t.Fatalf("transport closes = %d", closes)
	}
	if w.closes != 1 {
		t.Fatalf("channel closes = %d", w.closes)
	}
}

// TestConnectedLinkModeCloseRace races mode admission with owner shutdown. An
// invalid mode avoids a syscall before closure; after closure the lifecycle
// error must win. The race detector observes the shared closed flag and lock.
func TestConnectedLinkModeCloseRace(t *testing.T) {
	w := newConnectedExchange()
	closes := 0
	link := &sessionLink{channel: w, stopped: make(chan struct{}), closeTransport: func() { closes++ }}
	started, done := make(chan struct{}), make(chan error, 1)
	go connectedModeWorker(link, started, done)
	<-started
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := link.setIPv4Mode(-1, ppp.NPModePass); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("mode after concurrent close: %v", err)
	}
	if closes != 1 {
		t.Fatalf("transport closes = %d", closes)
	}
	if w.closes != 1 {
		t.Fatalf("channel closes = %d", w.closes)
	}
}

func connectedModeWorker(link *sessionLink, started chan<- struct{}, done chan<- error) {
	close(started)
	for range 64 {
		err := link.setIPv4Mode(-1, ppp.NPMode(255))
		if errors.Is(err, io.ErrClosedPipe) {
			continue
		}
		if err == nil {
			done <- errors.New("invalid mode admitted during shutdown")
			return
		}
		if !strings.Contains(err.Error(), "invalid IPv4 mode 255") {
			done <- err
			return
		}
	}
	done <- nil
}
