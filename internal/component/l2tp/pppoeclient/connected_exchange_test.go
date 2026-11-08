// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected restart exchanges.
// Related: connected_session.go -- publication; network_phase.go -- worker lifecycle.

package pppoeclient

import (
	"bytes"
	"crypto/md5" //nolint:gosec // Independent CHAP-MD5 wire oracle.
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// connectedExchange records a single worker's bounded output. Tests inspect it
// only after synctest.Wait or worker completion; it is not safe for concurrent use.
// Its session constructor starts one production receive owner over both inputs.
// Read is forbidden: restart must reuse that owner, not read the transport again.
type connectedExchange struct {
	frames       chan readFrame
	unit         chan readFrame
	writes       chan []byte
	modes        []ppp.NPMode
	events       []string
	modeFailure  ppp.NPMode
	writeFailure error
	closes       int
	reads        int
}

func newConnectedExchange() *connectedExchange {
	return &connectedExchange{
		frames: make(chan readFrame, 16), unit: make(chan readFrame, 16),
		writes: make(chan []byte, 128),
	}
}

func (w *connectedExchange) Read([]byte) (int, error) {
	w.reads++
	return 0, errors.New("unexpected second transport reader")
}

func (w *connectedExchange) Write(frame []byte) (int, error) {
	if len(w.writes) == 128 {
		return 0, errors.New("script output limit")
	}
	w.writes <- bytes.Clone(frame)
	w.events = append(w.events, "write")
	if w.writeFailure != nil {
		return 0, w.writeFailure
	}
	return len(frame), nil
}

func (w *connectedExchange) Close() error {
	w.closes++
	return nil
}

func (w *connectedExchange) setMode(mode ppp.NPMode) error {
	w.modes = append(w.modes, mode)
	if mode == ppp.NPModeDrop {
		w.events = append(w.events, "drop")
	} else {
		w.events = append(w.events, "pass")
	}
	if mode == w.modeFailure {
		return io.ErrClosedPipe
	}
	return nil
}

func (w *connectedExchange) passes() int {
	count := 0
	for _, mode := range w.modes {
		if mode == ppp.NPModePass {
			count++
		}
	}
	return count
}

func (w *connectedExchange) session(t *testing.T) (*connectedSession, sessionResult) {
	t.Helper()
	cfg := sessionConfig{mtu: 1492, username: "subscriber", password: "secret", keepaliveInterval: testKeepaliveInterval, keepaliveFailures: testKeepaliveFailures}
	initial := sessionResult{
		reader: testFrameReader(t, w.frames, w.unit), localIP: netip.MustParseAddr("192.0.2.2"),
		peerIP: netip.MustParseAddr("192.0.2.1"), negMTU: 1492, peerMRU: 1492,
		magic: 0x12345678, ipcpIdentifier: 40,
		chap: &networkPhaseCHAP{cfg: cfg, id: 77, pending: true},
	}
	s, err := newConnectedSession(w, initial, cfg, w.setMode)
	if err != nil {
		t.Fatal(err)
	}
	return s, initial
}

// exchangeFrame uses literal control-header offsets so the oracle does not use
// the production packet encoders. All test payloads are below one PPP MRU.
func exchangeFrame(proto uint16, code, id uint8, data []byte) []byte {
	frame := make([]byte, 6+len(data))
	binary.BigEndian.PutUint16(frame, proto)
	frame[2], frame[3] = code, id
	binary.BigEndian.PutUint16(frame[4:], uint16(4+len(data)))
	copy(frame[6:], data)
	return frame
}

func (w *connectedExchange) send(proto uint16, code, id uint8, data []byte) {
	source := w.frames
	if proto == ppp.ProtoIPCP {
		source = w.unit
	}
	source <- readFrame{data: exchangeFrame(proto, code, id, data)}
	synctest.Wait()
}

func (w *connectedExchange) take(t *testing.T, proto uint16, code uint8) []byte {
	t.Helper()
	synctest.Wait()
	if len(w.writes) == 0 {
		t.Fatalf("missing protocol %04x code %d", proto, code)
	}
	frame := <-w.writes
	if len(frame) < 6 {
		t.Fatalf("short output %x", frame)
	}
	if binary.BigEndian.Uint16(frame) != proto {
		t.Fatalf("protocol: got %x, want %04x", frame, proto)
	}
	if frame[2] != code {
		t.Fatalf("code: got %x, want %d", frame, code)
	}
	if int(binary.BigEndian.Uint16(frame[4:])) != len(frame)-2 {
		t.Fatalf("length: %x", frame)
	}
	return frame
}

func (w *connectedExchange) quiet(t *testing.T) {
	t.Helper()
	synctest.Wait()
	if len(w.writes) != 0 {
		t.Fatalf("unexpected output %x", <-w.writes)
	}
}

func exchangePeerOptions(auth uint16) []byte {
	options := []byte{1, 4, 5, 120, 5, 6, 0x55, 0x66, 0x77, 0x88} // MRU 1400.
	if auth == ppp.ProtoCHAP {
		return append(options, 3, 5, 0xc2, 0x23, 5)
	}
	if auth == ppp.ProtoPAP {
		return append(options, 3, 4, 0xc0, 0x23)
	}
	return options
}

// openLCP checks scr-before-sca, the exact peer Ack and fresh local magic.
// It also consumes premature IPCP termination before LCP opens: leakage
// into the new IPCP exchange would end it instead of publishing new addresses.
func (w *connectedExchange) openLCP(t *testing.T, auth uint16, oldMagic uint32) uint32 {
	t.Helper()
	options := exchangePeerOptions(auth)
	w.send(ppp.ProtoLCP, 1, 91, options)
	request := w.take(t, ppp.ProtoLCP, 1)
	if len(request) != 16 {
		t.Fatalf("LCP request shape: %x", request)
	}
	if !bytes.Equal(request[6:12], []byte{1, 4, 5, 212, 5, 6}) {
		t.Fatalf("LCP options: %x", request)
	}
	magic := binary.BigEndian.Uint32(request[12:])
	if magic == 0 {
		t.Fatal("zero restart magic")
	}
	if magic == oldMagic {
		t.Fatal("restart reused magic")
	}
	ack := w.take(t, ppp.ProtoLCP, 2)
	if !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 91, options)) {
		t.Fatalf("peer LCP Ack: %x", ack)
	}
	w.unit <- readFrame{data: exchangeFrame(ppp.ProtoIPCP, 5, 33, nil)}
	synctest.Wait()
	if len(w.unit) != 0 {
		t.Fatal("premature unit packet was not consumed during LCP")
	}
	w.quiet(t)
	w.send(ppp.ProtoLCP, 2, request[3], request[6:])
	return magic
}

// authenticate checks credentials independently of buildCHAPResponse and PAP's
// encoder, and leaves the worker waiting for the authentication verdict.
func (w *connectedExchange) authenticate(t *testing.T, auth uint16, success bool) {
	t.Helper()
	if auth == ppp.ProtoCHAP {
		w.send(ppp.ProtoCHAP, 1, 63, []byte{4, 9, 8, 7, 6, 'a', 'c'})
		response := w.take(t, ppp.ProtoCHAP, 2)
		digest := md5.Sum([]byte{63, 's', 'e', 'c', 'r', 'e', 't', 9, 8, 7, 6}) //nolint:gosec // CHAP-MD5.
		want := append([]byte{16}, digest[:]...)
		want = append(want, "subscriber"...)
		if !bytes.Equal(response, exchangeFrame(ppp.ProtoCHAP, 2, 63, want)) {
			t.Fatalf("CHAP response: %x", response)
		}
		code := uint8(4)
		if success {
			code = 3
		}
		w.send(ppp.ProtoCHAP, code, 63, nil)
	}
	if auth == ppp.ProtoPAP {
		request := w.take(t, ppp.ProtoPAP, 1)
		if !bytes.Equal(request[6:], []byte("\x0asubscriber\x06secret")) {
			t.Fatalf("PAP credentials: %x", request)
		}
		code := uint8(3)
		if success {
			code = 2
		}
		w.send(ppp.ProtoPAP, code, request[3], []byte{0})
	}
}

// openIPCP withholds our Ack until the peer direction is acknowledged. PASS
// must remain absent until the last packet, not merely until the first request.
func (w *connectedExchange) openIPCP(t *testing.T, previousID uint8, passes int) uint8 {
	t.Helper()
	request := w.take(t, ppp.ProtoIPCP, 1)
	if request[3] != previousID+1 {
		t.Fatalf("new IPCP Identifier = %d, want %d", request[3], previousID+1)
	}
	if !bytes.Equal(request[6:], ipcpRequestFor([]byte{3, 6, 0, 0, 0, 0})) {
		t.Fatalf("initial IPCP address: %x", request)
	}
	w.send(ppp.ProtoIPCP, 3, request[3], []byte{3, 6, 198, 51, 100, 2})
	request = w.take(t, ppp.ProtoIPCP, 1)
	if request[3] != previousID+2 {
		t.Fatalf("Nak IPCP Identifier = %d", request[3])
	}
	if !bytes.Equal(request[6:], ipcpRequestFor([]byte{3, 6, 198, 51, 100, 2})) {
		t.Fatalf("Nak address not adopted: %x", request)
	}
	// An old epoch's Identifier cannot acknowledge even byte-identical options.
	w.send(ppp.ProtoIPCP, 2, previousID, request[6:])
	peer := []byte{3, 6, 198, 51, 100, 1}
	w.send(ppp.ProtoIPCP, 1, 94, peer)
	ack := w.take(t, ppp.ProtoIPCP, 2)
	if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, 2, 94, peer)) {
		t.Fatalf("peer IPCP Ack: %x", ack)
	}
	if got := w.passes(); got != passes {
		t.Fatalf("premature PASS: modes %v", w.modes)
	}
	w.send(ppp.ProtoIPCP, 2, request[3], request[6:])
	return request[3]
}

// applyPending plays the interface consumer for the one pending result: it
// reads it, reports it applied through the session's Applied (acknowledge),
// and lets the worker pass forwarding. It returns the result it applied.
func applyPending(t *testing.T, s *connectedSession) iface.PPPoEResult {
	t.Helper()
	synctest.Wait()
	if len(s.changes) != 1 {
		t.Fatalf("pending results = %d, want one to apply", len(s.changes))
	}
	got := <-s.changes
	s.acknowledge(got.Seq)
	synctest.Wait()
	return got
}

// TestConnectedExchangeRestarts repeatedly drives the real keepalive consumer
// through CHAP, PAP, none and CHAP. It checks atomic results, old ownership,
// stale CHAP correlation, echo counters/timing and reuse of both reader sources.
func TestConnectedExchangeRestarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		magic, identifier := initial.magic, initial.ipcpIdentifier
		for epoch, auth := range []uint16{ppp.ProtoCHAP, ppp.ProtoPAP, 0, ppp.ProtoCHAP} {
			// sleep(timer): synctest advances the production echo ticker deterministically.
			time.Sleep(2 * testKeepaliveInterval)
			synctest.Wait()
			for id := uint8(1); id <= 2; id++ {
				echo := w.take(t, ppp.ProtoLCP, 9)
				if echo[3] != uint8(epoch*2)+id {
					t.Fatalf("echo Identifier = %d, want %d", echo[3], uint8(epoch*2)+id)
				}
				if binary.BigEndian.Uint32(echo[6:]) != magic {
					t.Fatal("echo used stale magic")
				}
			}
			before := s.current
			firstEvent := len(w.events)
			magic = w.openLCP(t, auth, magic)
			if w.events[firstEvent] != "drop" {
				t.Fatalf("restart did not DROP before write: %v", w.events[firstEvent:])
			}
			w.authenticate(t, auth, true)
			if s.current != before {
				t.Fatal("partial result published before IPCP")
			}
			identifier = w.openIPCP(t, identifier, epoch)
			if w.passes() != epoch {
				t.Fatalf("PASS before the interface applied the result: %v", w.modes)
			}
			applyPending(t, s)
			if w.passes() != epoch+1 {
				t.Fatalf("missing PASS: %v", w.modes)
			}
			if s.current.magic != magic {
				t.Fatal("new magic not published")
			}
			if s.current.localIP != netip.MustParseAddr("198.51.100.2") {
				t.Fatalf("local address: %v", s.current.localIP)
			}
			if s.current.peerIP != netip.MustParseAddr("198.51.100.1") {
				t.Fatalf("peer address: %v", s.current.peerIP)
			}
			if s.current.negMTU != 1400 {
				t.Fatalf("MTU = %d", s.current.negMTU)
			}
			if s.current.peerMRU != 1400 {
				t.Fatalf("peer MRU = %d", s.current.peerMRU)
			}
			if auth == ppp.ProtoCHAP {
				if s.current.chap == nil {
					t.Fatal("CHAP not enabled")
				}
				if s.current.chap.pending {
					t.Fatal("old CHAP response correlation survived")
				}
			} else if s.current.chap != nil {
				t.Fatal("CHAP retained after authentication change")
			}
			if auth != ppp.ProtoCHAP {
				w.send(ppp.ProtoCHAP, 1, 63, []byte{4, 9, 8, 7, 6})
				w.quiet(t)
			}
			w.send(ppp.ProtoCHAP, 4, 77, nil)
			w.send(ppp.ProtoLCP, 9, 99, []byte{0, 0, 0, 1})
			echo := w.take(t, ppp.ProtoLCP, 10)
			if binary.BigEndian.Uint32(echo[6:]) != magic {
				t.Fatal("echo reply used stale magic")
			}
			w.quiet(t)
			select {
			case <-done:
				t.Fatal("successful restart ended worker")
			default:
			}
		}
		if initial.magic != 0x12345678 {
			t.Fatal("initial ownership record mutated")
		}
		if !initial.chap.pending {
			t.Fatal("initial CHAP alias mutated")
		}
		if w.closes != 0 {
			t.Fatal("restart closed borrowed transport")
		}
		if w.reads != 0 {
			t.Fatal("restart created another reader")
		}
	})
}
