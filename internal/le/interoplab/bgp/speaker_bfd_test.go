package bgp

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// TestSpeakerOpenCarriesBFDStrictCapability holds the speaker's half of
// draft-ietf-idr-bgp-bfd-strict-mode Section 5: "Capability code: 74" and
// "Capability length: 0 octets".
//
// VALIDATES: --bfd-strict puts the two-octet TLV 74 0 in the OPEN's capability
// parameter and leaves every other capability where it was.
//
// PREVENTS: A positive scenario in which ze's BfdStrictNegotiated is FALSE
// because the peer never advertised the capability. Such a scenario would take
// the unmodified RFC 4271 rail and pass while proving nothing about strict
// mode.
func TestSpeakerOpenCarriesBFDStrictCapability(t *testing.T) {
	open, err := speakerOpen(65003, 90, net.ParseIP("172.30.0.10"), nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(open, []byte{74, 0}) {
		t.Fatalf("OPEN omitted the BFD Strict-Mode capability: %x", open)
	}
	// The capability parameter's own length must still cover what follows it,
	// or ze reads the OPEN as malformed and answers a NOTIFICATION rather than
	// negotiating anything.
	parameterLength := int(open[bgpHeaderLength+9])
	if bgpHeaderLength+10+parameterLength != len(open) {
		t.Fatalf("optional parameter length %d does not cover the OPEN body: %x", parameterLength, open)
	}
}

// TestBFDResponderEncodesRFC5880ControlPacket holds the packet layout of RFC
// 5880 Section 4.1 field by field.
//
// VALIDATES: 24 octets, version 1 in the top three bits, the state in the top
// two bits of the second octet, a non-zero Detect Mult, the length octet, both
// discriminators, and a zero Required Min Echo RX Interval.
//
// PREVENTS: A silently malformed responder. Ze's parser refuses a packet whose
// version is not 1, whose Detect Mult is zero, or whose My Discriminator is
// zero, and a refused packet leaves the BFD session Down: the scenario would
// then report ze holding the BGP session correctly for entirely the wrong
// reason.
func TestBFDResponderEncodesRFC5880ControlPacket(t *testing.T) {
	responder := newBFDResponder(net.ParseIP("172.30.0.2"), 0)
	packet := responder.encodeControl(bfdStateUp, 0xDEADBEEF)

	if len(packet) != 24 {
		t.Fatalf("packet length = %d, want 24", len(packet))
	}
	if packet[0]>>5 != 1 {
		t.Fatalf("version = %d, want 1", packet[0]>>5)
	}
	if packet[0]&0x1f != 0 {
		t.Fatalf("diagnostic = %d, want 0 (No Diagnostic)", packet[0]&0x1f)
	}
	if packet[1]>>6 != bfdStateUp {
		t.Fatalf("state = %d, want Up", packet[1]>>6)
	}
	if packet[1]&0x3f != 0 {
		t.Fatalf("flags = %#x, want none set", packet[1]&0x3f)
	}
	if packet[2] == 0 {
		t.Fatal("Detect Mult is zero, which RFC 5880 Section 6.8.6 refuses")
	}
	if packet[3] != 24 {
		t.Fatalf("length octet = %d, want 24", packet[3])
	}
	if got := uint32(packet[4])<<24 | uint32(packet[5])<<16 | uint32(packet[6])<<8 | uint32(packet[7]); got == 0 {
		t.Fatal("My Discriminator is zero, which RFC 5880 Section 6.8.6 refuses")
	}
	if !bytes.Equal(packet[8:12], []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Fatalf("Your Discriminator = %x, want the remote discriminator", packet[8:12])
	}
	if !bytes.Equal(packet[20:24], []byte{0, 0, 0, 0}) {
		t.Fatalf("Required Min Echo RX = %x, want 0 (echo refused)", packet[20:24])
	}
}

// TestBFDResponderWalksTheRFC5880StateMachine holds the receive procedures of
// RFC 5880 Section 6.8.6 that a responder needs.
//
// VALIDATES: Down plus a received Down goes to Init; Init plus a received Init
// or Up goes to Up; a received Down on an Up session goes back to Down; and
// bfd.RemoteDiscr follows the received My Discriminator.
//
// PREVENTS: A responder that never leaves Down, which would make the positive
// scenario unable to distinguish "ze held the session correctly" from "the test
// peer was broken". The oracle fails closed on exactly that, and this is what
// keeps the fail-closed branch from being the only branch anybody ever sees.
func TestBFDResponderWalksTheRFC5880StateMachine(t *testing.T) {
	responder := newBFDResponder(net.ParseIP("172.30.0.2"), 0)
	far := newBFDResponder(net.ParseIP("172.30.0.10"), 0)

	// The far end opens in Down, as RFC 5880 Section 6.8.1 requires.
	state, ok := responder.receive(far.encodeControl(bfdStateDown, 0))
	if !ok || state != bfdStateInit {
		t.Fatalf("Down + received Down = %d (accepted %v), want Init", state, ok)
	}
	if responder.remote != far.discriminator {
		t.Fatalf("bfd.RemoteDiscr = %#x, want the received My Discriminator %#x", responder.remote, far.discriminator)
	}
	if up, _ := responder.isUp(); up {
		t.Fatal("Init is not Up")
	}

	state, _ = responder.receive(far.encodeControl(bfdStateInit, responder.discriminator))
	if state != bfdStateUp {
		t.Fatalf("Init + received Init = %d, want Up", state)
	}
	up, at := responder.isUp()
	if !up || at.IsZero() {
		t.Fatalf("the Up transition was not latched with a time: %v %v", up, at)
	}

	state, _ = responder.receive(far.encodeControl(bfdStateDown, responder.discriminator))
	if state != bfdStateDown {
		t.Fatalf("Up + received Down = %d, want Down", state)
	}
}

// TestBFDResponderRefusesMalformedPackets is the negative half of RFC 5880
// Section 6.8.6's own refusal list.
//
// VALIDATES: A short packet, a wrong version, a zero Detect Mult, a zero My
// Discriminator, a length octet below 24 and the Multipoint bit each leave the
// state untouched and are reported as not accepted.
//
// PREVENTS: A responder that comes Up on rubbish, which would make the positive
// scenario green against a ze that sent nothing at all.
func TestBFDResponderRefusesMalformedPackets(t *testing.T) {
	valid := newBFDResponder(net.ParseIP("172.30.0.10"), 0).encodeControl(bfdStateDown, 0)

	cases := map[string]func([]byte) []byte{
		"short":            func(p []byte) []byte { return p[:23] },
		"version 0":        func(p []byte) []byte { p[0] = 0; return p },
		"zero detect mult": func(p []byte) []byte { p[2] = 0; return p },
		"length below 24":  func(p []byte) []byte { p[3] = 23; return p },
		"multipoint set":   func(p []byte) []byte { p[1] |= 0x01; return p },
		"zero my discr":    func(p []byte) []byte { copy(p[4:8], []byte{0, 0, 0, 0}); return p },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			responder := newBFDResponder(net.ParseIP("172.30.0.2"), 0)
			packet := mutate(append([]byte(nil), valid...))
			state, ok := responder.receive(packet)
			if ok {
				t.Fatalf("a %s packet was accepted", name)
			}
			if state != bfdStateDown {
				t.Fatalf("state moved to %d on a %s packet", state, name)
			}
		})
	}
}

// TestBFDStrictOracleJudgesTheOrdering holds the oracle's own verdicts, which
// is what an interop scenario's PASS rests on.
//
// VALIDATES: A KEEPALIVE after BFD came up passes; a KEEPALIVE before it fails
// naming Section 8.5.5; no KEEPALIVE at all after BFD came up fails naming
// Section 8.5.1; and a BFD session that never came up fails rather than passing
// on a hold nobody tested.
//
// PREVENTS: The vacuous pass. An oracle that only checked "ze sent no
// KEEPALIVE" is satisfied by a ze that crashed, by a peer that never dialed,
// and by a BFD responder whose socket failed to open.
func TestBFDStrictOracleJudgesTheOrdering(t *testing.T) {
	silenceEnded := time.Now()
	// RFC 5880 Section 6.8.6 brings ze Up one packet flight before this end,
	// so a CORRECT ze sends its KEEPALIVE slightly before this end's own Up.
	// The oracle must pass that, and it is the case the first lab run failed
	// on before the boundary was corrected: measured 3.657525ms.
	localUp := silenceEnded.Add(600 * time.Millisecond)

	cases := []struct {
		name           string
		up             bool
		delay          time.Duration
		firstKeepalive time.Time
		wantFailures   int
	}{
		{"keepalive after the silence", true, 15 * time.Second, silenceEnded.Add(500 * time.Millisecond), 0},
		{"keepalive one flight before this end reached Up", true, 15 * time.Second, localUp.Add(-4 * time.Millisecond), 0},
		{"keepalive during the silence", true, 15 * time.Second, silenceEnded.Add(-3 * time.Second), 1},
		{"no keepalive at all", true, 15 * time.Second, time.Time{}, 1},
		{"bfd never came up", false, 15 * time.Second, time.Time{}, 1},
		{"no silent window configured", true, 0, silenceEnded.Add(-3 * time.Second), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responder := newBFDResponder(net.ParseIP("172.30.0.2"), tc.delay)
			responder.silenceEndedAt.Store(silenceEnded.UnixNano())
			if tc.up {
				responder.up.Store(true)
				responder.upAt.Store(localUp.UnixNano())
			}
			verdict := &speakerVerdict{plugin: speakerOracleBFDStrictHold}
			applyBFDStrictOracle(responder, tc.firstKeepalive, verdict)
			if len(verdict.failures) != tc.wantFailures {
				t.Fatalf("failures = %d, want %d: %v", len(verdict.failures), tc.wantFailures, verdict.failures)
			}
		})
	}
}
