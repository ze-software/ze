package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// RFC 5880 Section 6.8.6: "If bfd.RemoteDemandMode is 0, or bfd.SessionState
// is not Up, or bfd.RemoteSessionState is not Up, Demand mode is not active on
// the remote system and the local system MUST send periodic BFD Control
// packets (see section 6.8.7)."
//
// Each test below drives the engine into a state where exactly one of the
// three disjuncts holds and the other two do not, then asserts that a tick
// transmits a periodic Control packet. A mutant that stops periodic
// transmission in that one state therefore reddens that one test, which the
// tests covering the other disjuncts cannot catch.

// demandDisjunctPeer drives the session with the peer packets in order, then
// checks the local and remote session states the disjunct needs, and asserts
// that the next tick transmits a periodic Control packet carrying the
// session's My Discriminator. Each peer packet's State and D bit are given;
// every packet after the first carries the local discriminator as Your
// Discriminator, as a peer that has learned it does.
func demandDisjunctPeer(t *testing.T, peer []demandPeerPacket, localWant, remoteWant packet.State) {
	t.Helper()
	l, ct, key := newSingleHopLoop(t)
	m := machineFor(t, l, key)

	for _, p := range peer {
		var yourDiscr uint32
		if p.knowsLocal {
			yourDiscr = m.LocalDiscriminator()
		}
		l.handleInbound(rfc5880Inbound(key.Peer, key.Local, key.Interface, yourDiscr, func(c *packet.Control) {
			c.State = p.state
			c.Demand = p.demand
		}))
	}

	l.mu.Lock()
	local, remote := m.State(), m.RemoteState()
	l.mu.Unlock()
	if local != localWant {
		t.Fatalf("precondition: bfd.SessionState = %v, want %v", local, localWant)
	}
	if remote != remoteWant {
		t.Fatalf("precondition: bfd.RemoteSessionState = %v, want %v", remote, remoteWant)
	}

	ct.sent = false
	l.tick()
	if !ct.sent {
		t.Fatalf("no periodic Control packet: bfd.SessionState %v, bfd.RemoteSessionState %v", local, remote)
	}
	out, _, err := packet.ParseControl(ct.last.Bytes)
	if err != nil {
		t.Fatalf("ParseControl of the periodic packet: %v", err)
	}
	if out.MyDiscriminator != m.LocalDiscriminator() {
		t.Fatalf("periodic packet carries My Discriminator %d, want %d", out.MyDiscriminator, m.LocalDiscriminator())
	}
}

// demandPeerPacket is one Control packet the peer sends in a disjunct test.
type demandPeerPacket struct {
	state      packet.State
	demand     bool
	knowsLocal bool
}

// RFC requirement: RFC5880-6.8.6-15 positive -- the first disjunct alone,
// bfd.RemoteDemandMode 0 with both ends Up: the peer's Down packet moves the
// session to Init, its Up packet with D=0 moves it to Up with
// bfd.RemoteSessionState Up, and a tick transmits a periodic Control packet
// carrying the session's My Discriminator.
//
// VALIDATES: a clear D bit keeps periodic transmission running on a session
// that is Up at both ends.
// PREVENTS: code that stopped periodic transmission once both ends were Up,
// whatever the peer's D bit said.
func TestRFC5880PeriodicTransmitDemandClearBothUp(t *testing.T) {
	demandDisjunctPeer(t, []demandPeerPacket{
		{state: packet.StateDown},
		{state: packet.StateUp, knowsLocal: true},
	}, packet.StateUp, packet.StateUp)
}

// RFC requirement: RFC5880-6.8.6-15 positive -- the second disjunct alone,
// bfd.SessionState not Up with bfd.RemoteDemandMode 1 and
// bfd.RemoteSessionState Up: a local Down session that receives the peer's Up
// packet with D=1 stays Down (RFC 5880 Section 6.8.6 ignores Up while Down)
// and records the peer as Up, and a tick transmits a periodic Control packet
// carrying the session's My Discriminator.
//
// VALIDATES: the peer's D bit and its Up state do not stop periodic
// transmission while the local session is not Up.
// PREVENTS: code that stopped periodic transmission whenever the peer was Up
// and set D, without checking the local state.
func TestRFC5880PeriodicTransmitDemandSetLocalNotUpRemoteUp(t *testing.T) {
	demandDisjunctPeer(t, []demandPeerPacket{
		{state: packet.StateUp, demand: true, knowsLocal: true},
	}, packet.StateDown, packet.StateUp)
}

// RFC requirement: RFC5880-6.8.6-15 positive -- the third disjunct alone,
// bfd.RemoteSessionState not Up with bfd.RemoteDemandMode 1 and
// bfd.SessionState Up: the peer's Down packet moves the session to Init, its
// Init packet with D=1 moves it to Up while the peer is still in Init, and a
// tick transmits a periodic Control packet carrying the session's My
// Discriminator.
//
// VALIDATES: the peer's D bit does not stop periodic transmission while the
// peer is not Up, even with the local session Up.
// PREVENTS: code that stopped periodic transmission whenever the local session
// was Up and the peer set D, without checking the peer's state.
func TestRFC5880PeriodicTransmitDemandSetLocalUpRemoteNotUp(t *testing.T) {
	demandDisjunctPeer(t, []demandPeerPacket{
		{state: packet.StateDown},
		{state: packet.StateInit, demand: true, knowsLocal: true},
	}, packet.StateUp, packet.StateInit)
}
