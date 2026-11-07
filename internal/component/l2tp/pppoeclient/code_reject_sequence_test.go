// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- session Code-Reject sequence.
// Related: session.go -- IPCP negotiation; network_phase.go -- opened IPCP.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientCodeRejectSequence checks exact packets across network handling,
// interrupted IPCP, successful IPCP, and a restart that sends no Code-Reject.
func TestClientCodeRejectSequence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		seed, initial := w.session(t)
		initial.ipcpCodeRejectID = 253
		s, err := newConnectedSession(w, initial, seed.cfg, w.setMode)
		if err != nil {
			t.Fatal(err)
		}
		done, stop := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		assertCodeRejectSequence(t, w, 254)
		magic := w.openLCP(t, 0, initial.magic)
		request := w.take(t, ppp.ProtoIPCP, 1)
		assertCodeRejectSequence(t, w, 255)
		magic = w.openLCP(t, 0, magic) // Interrupt the exchange after its rejection.
		if len(s.changes) != 0 {
			t.Fatal("interrupted IPCP published a result")
		}
		// Complete this epoch before its network rejection. Replacement of current
		// must not restore a stale sequence, and uint8 wrap must retain zero.
		identifier := w.openIPCP(t, request[3], 0)
		assertCodeRejectSequence(t, w, 0)
		magic = w.openLCP(t, 0, magic)
		w.openIPCP(t, identifier, 1)
		assertCodeRejectSequence(t, w, 1)
		if s.current.magic != magic {
			t.Fatal("last restart did not publish its result")
		}
	})
}

// TestClientCodeRejectInitialSequence drives the shared initial phase owner,
// interrupts initial IPCP, then hands the negotiated values to the connected
// owner at the descriptor boundary. Kernel connection is outside this unit test.
func TestClientCodeRejectInitialSequence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		unit := w.unit
		reader := testFrameReader(t, w.frames, unit)
		w.unit = w.frames // Initial IPCP uses the unconnected channel.
		cfg := sessionConfig{mtu: 1492, keepaliveInterval: testKeepaliveInterval, keepaliveFailures: testKeepaliveFailures}
		stop := make(chan struct{})
		defer close(stop)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		var ipcpID, lcpID, papID, rejectID uint8
		type outcome struct {
			state negotiatedSession
			err   error
		}
		result := make(chan outcome, 1)
		go func() {
			state, err := negotiatePhases(w, reader, cfg, 0, nil,
				&ipcpID, stop, logger, &lcpID, &papID, &rejectID)
			result <- outcome{state, err}
		}()
		request := w.take(t, ppp.ProtoLCP, 1)
		magic := binary.BigEndian.Uint32(request[12:])
		w.send(ppp.ProtoLCP, 1, 91, exchangePeerOptions(0))
		w.take(t, ppp.ProtoLCP, 2)
		w.send(ppp.ProtoLCP, 2, request[3], request[6:])
		request = w.take(t, ppp.ProtoIPCP, 1)
		assertCodeRejectSequence(t, w, 1)
		w.send(ppp.ProtoLCP, 1, 92, exchangePeerOptions(0))
		restart := w.take(t, ppp.ProtoLCP, 1)
		if binary.BigEndian.Uint32(restart[12:]) == magic {
			t.Fatal("initial reentry reused magic")
		}
		magic = binary.BigEndian.Uint32(restart[12:])
		w.take(t, ppp.ProtoLCP, 2)
		w.send(ppp.ProtoLCP, 2, restart[3], restart[6:])
		assertCodeRejectInitialIPCP(t, w, request[3])
		opened := <-result
		if opened.err != nil {
			t.Fatal(opened.err)
		}
		if rejectID != 2 {
			t.Fatalf("initial retained Code-Reject Identifier = %d", rejectID)
		}
		w.unit = unit
		initial := sessionResult{
			localIP: opened.state.localIP, peerIP: opened.state.peerIP,
			negMTU: opened.state.negMTU, magic: opened.state.magic,
			peerMRU: opened.state.peerMRU, chap: opened.state.chap,
			reader:         reader,
			ipcpIdentifier: ipcpID, lcpIdentifier: lcpID, papIdentifier: papID,
			ipcpCodeRejectID: rejectID,
		}
		s, err := newConnectedSession(w, initial, cfg, w.setMode)
		if err != nil {
			t.Fatal(err)
		}
		go keepaliveLoop(s, make(chan struct{}), stop, logger)
		assertCodeRejectSequence(t, w, 3)
		magic = w.openLCP(t, 0, magic)
		request = w.take(t, ppp.ProtoIPCP, 1)
		assertCodeRejectSequence(t, w, 4)
		magic = w.openLCP(t, 0, magic)
		if len(s.changes) != 0 {
			t.Fatal("interrupted restart published a result")
		}
		ipcpID = w.openIPCP(t, request[3], 0)
		assertCodeRejectSequence(t, w, 5)
		magic = w.openLCP(t, 0, magic)
		w.openIPCP(t, ipcpID, 1)
		assertCodeRejectSequence(t, w, 6)
		if s.current.magic != magic {
			t.Fatal("repeated restart did not publish its result")
		}
	})
}

// assertCodeRejectInitialIPCP injects a rejection while the initial request is
// still outstanding, then completes the same exchange through the peer script.
func assertCodeRejectInitialIPCP(t *testing.T, w *connectedExchange, previousID uint8) {
	t.Helper()
	request := w.take(t, ppp.ProtoIPCP, 1)
	assertCodeRejectSequence(t, w, 2)
	// Return the untouched request to the script's output queue; no new packet
	// is produced by this fixture operation.
	w.writes <- request
	w.openIPCP(t, previousID, 0)
}

// assertCodeRejectSequence checks the Identifier and complete rejected packet
// independently of the production encoder, including its Protocol field.
func assertCodeRejectSequence(t *testing.T, w *connectedExchange, identifier uint8) {
	t.Helper()
	w.send(ppp.ProtoIPCP, 12, 93, []byte{0xa5})
	reject := w.take(t, ppp.ProtoIPCP, 7)
	want := exchangeFrame(ppp.ProtoIPCP, 7, identifier, []byte{12, 93, 0, 5, 0xa5})
	if !bytes.Equal(reject, want) {
		t.Fatalf("Code-Reject = %x, want %x", reject, want)
	}
}
