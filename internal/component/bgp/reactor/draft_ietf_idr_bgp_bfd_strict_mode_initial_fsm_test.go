// Design: docs/architecture/behavior/fsm-open-sent.md — strict-mode establishment gate.
// Related: docs/architecture/behavior/fsm-open-confirm.md — hold-down release.
package reactor

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestDraftBFDNonzeroHoldDownInitialFSM starts an Idle session with a nonzero
// hold-down and exchanges OPENs over bounded TCP sockets before advancing time.
// Down and newly Up BFD both permit the initial FSM and connection; only the
// transition to Established waits. A zero-interval control establishes on Up.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 10: "To avoid deadlock when
// utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled
// for a peer, the BGP FSM MUST be enabled." "That is, BFD hold-down procedures
// MUST NOT prevent BGP from establishing a connection with the remote BGP
// speaker."
//
// MUTATION: make Session.Start return without EventManualStart when strict
// mode has a nonzero HoldDown; the initial Active assertion must fail. Moving
// the hold-down gate to connectionEstablished must instead fail the TCP OPEN
// assertion. Removing bfdHoldDownPending's interval gate must fail the exact
// pending-state assertions before 400 ms, while the zero control still releases.
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 positive -- with strict mode and a 400 ms hold-down, both Down and newly Up BFD permit Idle to Active, accepted TCP to OpenSent, and the exact OPEN exchange before any clock advance.
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 negative -- initial FSM enablement does not bypass strict establishment: Down BFD withholds KEEPALIVE, newly Up BFD permits OpenConfirm but not Established before 400 ms; the zero-hold-down control establishes on Up without waiting.
func TestDraftBFDNonzeroHoldDownInitialFSM(t *testing.T) {
	for _, tc := range []struct {
		name     string
		initial  api.State
		holdDown uint32
	}{
		{name: "down-nonzero", initial: api.StateDown, holdDown: 400},
		{name: "up-nonzero", initial: api.StateUp, holdDown: 400},
		{name: "down-zero-control", initial: api.StateDown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			clock := sim.NewFakeClock(start)
			settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020301)
			settings.Connection = ConnectionPassive
			settings.ReceiveHoldTime = 90 * time.Second
			settings.DisableASN4 = true
			settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5, HoldDown: tc.holdDown}
			settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
			session := NewSession(settings)
			session.SetClock(clock)
			state := tc.initial
			session.setBFDStateReader(func() (api.State, time.Time, bool) { return state, start, true })
			t.Cleanup(func() {
				session.timers.StopAll()
				session.stopSendHoldTimer()
				session.closeConn()
			})

			if session.State() != fsm.StateIdle {
				t.Fatalf("initial state = %s, want Idle", session.State())
			}
			// Draft Section 10: start the real FSM, not a pre-seeded OpenSent fixture.
			if err := session.Start(); err != nil {
				t.Fatal(err)
			}
			if session.State() != fsm.StateActive {
				t.Fatalf("strict hold-down suppressed initial FSM: got %s, want Active", session.State())
			}
			if session.timers.IsBfdHoldDownTimerRunning() {
				t.Fatal("hold-down was armed before a connection existed")
			}

			far, near := collisionProofTCPPair(t)
			transport := &bfdInitialTransport{Conn: near}
			// Draft Section 10: installation and OPEN output precede any hold-down.
			if err := session.Accept(transport); err != nil {
				t.Fatal(err)
			}
			wantOpen := []byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0x00, 0x21, 0x01, // Length 33, OPEN.
				0x04, 0xfd, 0xe9, 0x00, 0x5a, // Version 4, AS 65001, hold 90.
				0x01, 0x02, 0x03, 0x01, // Identifier 1.2.3.1.
				0x04, 0x02, 0x02, 0x4a, 0x00, // Capability 74, length zero.
			}
			gotOpen, err := core4271ReadMessage(far)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotOpen, wantOpen) {
				t.Fatalf("initial OPEN = %x, want %x", gotOpen, wantOpen)
			}
			if session.Conn() != transport {
				t.Fatal("accepted connection was not retained")
			}
			bfdInitialCheckpoint(t, session, transport, fsm.StateOpenSent, fsm.SubStateNone, false, wantOpen)

			peerOpen := bytes.Clone(wantOpen)
			peerOpen[21] = 0xea // Remote AS 65002.
			peerOpen[27] = 0x02 // Remote identifier 1.2.3.2.
			// Draft Section 8.5.5: process the remote OPEN through the socket reader.
			bfdInitialReceive(t, session, far, peerOpen)
			if !session.Negotiated().BFDStrictMode {
				t.Fatal("the OPEN exchange did not negotiate strict mode")
			}
			if session.timers.BfdHoldDown() != time.Duration(tc.holdDown)*time.Millisecond {
				t.Fatalf("negotiated session lost configured hold-down: %s", session.timers.BfdHoldDown())
			}
			keepalive := append(bytes.Repeat([]byte{0xff}, 16), 0, 19, 4)
			wantWire := bytes.Clone(wantOpen)
			wantState, wantSubstate := fsm.StateOpenSent, fsm.SubStateOpenSentBfdUpPending
			if tc.initial == api.StateUp {
				wantState, wantSubstate = fsm.StateOpenConfirm, fsm.SubStateNone
				wantWire = append(wantWire, keepalive...)
			}
			bfdInitialCheckpoint(t, session, transport, wantState, wantSubstate, false, wantWire)

			// Draft Sections 8.5.6 and 10: confirmation cannot bypass the wait.
			bfdInitialReceive(t, session, far, keepalive)
			if tc.initial == api.StateDown {
				wantSubstate = fsm.SubStateOpenSentConfirmedBfdUpPending
			}
			bfdInitialCheckpoint(t, session, transport, wantState, wantSubstate, tc.initial == api.StateUp, wantWire)
			if !clock.Now().Equal(start) {
				t.Fatal("initial FSM or OPEN exchange consumed simulated hold-down time")
			}

			if tc.initial == api.StateDown {
				state = api.StateUp
				// Draft Section 8.5.1: the production subscriber delivers this event.
				if err := session.handleBFDEvent(fsm.EventBfdUp); err != nil {
					t.Fatal(err)
				}
			}
			if tc.holdDown != 0 {
				bfdInitialCheckpoint(t, session, transport, wantState, wantSubstate, true, wantWire)
				clock.Add(399 * time.Millisecond)
				bfdInitialCheckpoint(t, session, transport, wantState, wantSubstate, true, wantWire)
				clock.Add(time.Millisecond)
			}
			wantWire = append(bytes.Clone(wantOpen), keepalive...)
			bfdInitialCheckpoint(t, session, transport, fsm.StateEstablished, fsm.SubStateNone, false, wantWire)
			if !session.timers.IsKeepaliveTimerRunning() {
				t.Fatal("released session did not start its KEEPALIVE timer")
			}
			if tc.holdDown == 0 && !clock.Now().Equal(start) {
				t.Fatal("zero-hold-down control needed a clock advance")
			}

			// Read the actual far-end stream through EOF: exactly one KEEPALIVE
			// follows the OPEN, with no extra OPEN, NOTIFICATION or premature send.
			if err := session.Stop(); err != nil {
				t.Fatal(err)
			}
			session.stopSendHoldTimer()
			session.closeConn()
			rest, err := io.ReadAll(far)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(rest, keepalive) {
				t.Fatalf("post-OPEN TCP stream = %x, want one KEEPALIVE %x", rest, keepalive)
			}
		})
	}
}

// bfdInitialTransport records only bytes successfully written to a real socket.
// All fixture calls, including fake-clock callbacks, run synchronously; the
// snapshot makes absence of premature output assertable without a timed read.
// A zero value is not usable: Conn must be the fixture's connected TCP socket.
type bfdInitialTransport struct {
	net.Conn
	wire []byte
}

func (c *bfdInitialTransport) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.wire = append(c.wire, p[:n]...)
	return n, err
}

// bfdInitialReceive uses the same framed receive entry as the session tests.
// The bounded TCP pair buffers one small frame, so no sender goroutine is needed.
func bfdInitialReceive(t *testing.T, session *Session, far net.Conn, wire []byte) {
	t.Helper()
	n, err := far.Write(wire)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(wire) {
		t.Fatalf("peer write = %d octets, want %d", n, len(wire))
	}
	// Draft Sections 8.5.5 and 8.5.6: parse and dispatch the actual received frame.
	if err := session.ReadAndProcess(); err != nil {
		t.Fatal(err)
	}
}

// bfdInitialCheckpoint pins the exact FSM state and all output at a completed
// synchronous operation, including a fake-clock deadline callback.
func bfdInitialCheckpoint(t *testing.T, session *Session, transport *bfdInitialTransport,
	state fsm.State, substate fsm.BfdSubState, holdDown bool, wire []byte,
) {
	t.Helper()
	if session.State() != state {
		t.Fatalf("FSM = %s, want %s", session.State(), state)
	}
	if session.fsm.BfdSubState() != substate {
		t.Fatalf("BFD substate = %v, want %v", session.fsm.BfdSubState(), substate)
	}
	if session.timers.IsBfdHoldDownTimerRunning() != holdDown {
		t.Fatalf("hold-down timer running = %v, want %v", session.timers.IsBfdHoldDownTimerRunning(), holdDown)
	}
	if !bytes.Equal(transport.wire, wire) {
		t.Fatalf("TCP output = %x, want exactly %x", transport.wire, wire)
	}
	if state != fsm.StateEstablished && session.timers.IsKeepaliveTimerRunning() {
		t.Fatal("KEEPALIVE timer started before establishment")
	}
}
