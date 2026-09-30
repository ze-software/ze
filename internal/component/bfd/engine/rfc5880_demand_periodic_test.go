package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// RFC requirement: RFC5880-6.8.6-15 positive -- the two disjuncts that hold
// while the peer sets the D bit (bfd.RemoteDemandMode 1): with bfd.SessionState
// not Up (the peer's Down packet with D=1 moves the session to Init), and with
// bfd.SessionState Up but bfd.RemoteSessionState not Up (the peer's Init packet
// with D=1 moves the session to Up while the peer is still in Init), a tick
// transmits a periodic Control packet carrying the session's My
// Discriminator. The third disjunct, bfd.RemoteDemandMode 0, is
// TestRFC5880PeriodicTransmitWhenRemoteDemandInactive.
//
// VALIDATES: the peer's D bit alone does not stop periodic transmission while
// either end is not Up.
// PREVENTS: code that stopped periodic transmission whenever D=1, which the
// D=0 case cannot tell apart.
func TestRFC5880PeriodicTransmitWhileDemandBitSetAndNotBothUp(t *testing.T) {
	cases := []struct {
		name        string
		peerState   packet.State
		localState  packet.State
		useLocalDsc bool
	}{
		{name: "local Init, peer Down", peerState: packet.StateDown, localState: packet.StateInit},
		{name: "local Up, peer Init", peerState: packet.StateInit, localState: packet.StateUp, useLocalDsc: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, ct, key := newSingleHopLoop(t)
			m := machineFor(t, l, key)

			var yourDiscr uint32
			if tc.useLocalDsc {
				yourDiscr = m.LocalDiscriminator()
			}
			l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, yourDiscr, func(c *packet.Control) {
				c.State = tc.peerState
				c.Demand = true
			}))
			// The FSM copies the received D bit into bfd.RemoteDemandMode
			// (session/fsm.go, Receive); the Machine exposes no reader for it.
			l.mu.Lock()
			state := m.State()
			l.mu.Unlock()
			if state != tc.localState {
				t.Fatalf("precondition: session state = %v, want %v", state, tc.localState)
			}

			ct.sent = false
			l.tick()
			if !ct.sent {
				t.Fatalf("no periodic Control packet with the peer's D bit set, local %v, peer %v", tc.localState, tc.peerState)
			}
			out, _, err := packet.ParseControl(ct.last.Bytes)
			if err != nil {
				t.Fatalf("ParseControl of the periodic packet: %v", err)
			}
			if out.MyDiscriminator != m.LocalDiscriminator() {
				t.Fatalf("periodic packet carries My Discriminator %d, want %d", out.MyDiscriminator, m.LocalDiscriminator())
			}
		})
	}
}
