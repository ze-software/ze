package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2 positive -- with a configured 400 ms hold-down still unserved, the accepted TCP connection carries Ze's OPEN and processes the peer's OPEN without advancing time.
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2 negative -- neither BFD Down nor BFD Up with an unserved hold-down suppresses the connection or OPEN exchange; only establishment is withheld.
func TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN(t *testing.T) {
	for _, state := range []api.State{api.StateDown, api.StateUp} {
		start := time.Now()
		fc := sim.NewFakeClock(start)
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
		settings.Connection = ConnectionPassive
		settings.ReceiveHoldTime = 90 * time.Second
		settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5, HoldDown: 400}
		settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
		session := NewSession(settings)
		session.SetClock(fc)
		session.setBFDStateReader(func() (api.State, time.Time, bool) { return state, start, true })
		if err := session.Start(); err != nil {
			t.Fatal(err)
		}
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
		open := acceptWithReader(t, session, server, client)
		if len(open) < 19 || open[18] != byte(msgtype.TypeOPEN) || session.Conn() == nil {
			t.Fatalf("BFD %v: no OPEN on the accepted connection: %x", state, open)
		}
		drainMessages(t, client)
		if err := session.handleOpen(openBodyWithBFDStrict(90)); err != nil {
			t.Fatal(err)
		}
		if err := session.handleKeepalive(); err != nil {
			t.Fatal(err)
		}
		if !fc.Now().Equal(start) || session.State() == fsm.StateEstablished || !session.Negotiated().BFDStrictMode {
			t.Fatalf("BFD %v: OPEN was not exchanged before the hold-down elapsed", state)
		}
		if state == api.StateUp && !session.timers.IsBfdHoldDownTimerRunning() {
			t.Fatal("the nonzero configured hold-down did not withhold establishment")
		}
		if state == api.StateDown && session.fsm.BfdSubState() != fsm.SubStateOpenSentConfirmedBfdUpPending {
			t.Fatal("Down BFD did not leave the exchanged OPEN waiting for BFD")
		}
	}
}

// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1 negative -- disabling BFD in either pending sub-state substitutes the effective BfdAdminDown event: a KEEPALIVE is sent and the pending state is released rather than sending Cease or doing nothing.
func TestDraftBFDDisabledConfigChangeReleasesPendingSession(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
		messages := drainMessages(t, client)
		if err := session.handleOpen(openBodyWithBFDStrict(90)); err != nil {
			t.Fatal(err)
		}
		want := fsm.StateOpenConfirm
		if confirmed {
			if err := session.handleKeepalive(); err != nil {
				t.Fatal(err)
			}
			want = fsm.StateEstablished
		}
		session.mu.Lock()
		bfd := *session.settings.BFD
		bfd.Enabled = false
		session.settings.BFD = &bfd
		session.mu.Unlock()
		session.raiseBFDStrictConfigChanged()
		if session.State() != want || session.fsm.BfdSubState() != fsm.SubStateNone {
			t.Fatalf("confirmed=%v: disabled BFD did not release pending state: %s/%v", confirmed, session.State(), session.fsm.BfdSubState())
		}
		select {
		case msg := <-messages:
			if len(msg) < 19 || msg[18] != byte(msgtype.TypeKEEPALIVE) {
				t.Fatalf("disabled BFD sent %x, want KEEPALIVE", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("disabled BFD produced no effective AdminDown release")
		}
	}
}
