// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, sender and listeners reused here
//
// VALIDATES: against a real Linux XFRM stack, that the non-initial fragments of an inner
// packet arriving on a tunnel-mode Child SA are held to the policy applied to the whole
// packet (RFC 4301 Section 7.3): the fragments of a packet the inbound policy Ze installs
// admits are reassembled and delivered, and the fragments of a packet it forbids are
// discarded with it.
// PREVENTS: a non-initial fragment, which carries no ports, slipping past the selector
// check its initial fragment met, so a peer delivers a packet the SA does not cover by
// splitting it.
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains.

//go:build linux

package engine

import (
	"encoding/binary"
	"testing"
)

// fragFirstOctets is the payload length of the initial fragment: a multiple of eight,
// as every fragment but the last must carry, and longer than the UDP header, so the
// non-initial fragment carries no port.
const fragFirstOctets = 16

// fragSplit splits a whole inner IPv4 packet (20-octet header) into an initial and one
// non-initial fragment of the same datagram.
func fragSplit(packet []byte) (initial, nonInitial []byte) {
	payload := packet[20:]
	build := func(part []byte, offsetUnits uint16, more bool) []byte {
		frag := make([]byte, 20, 20+len(part))
		copy(frag, packet[:20])
		binary.BigEndian.PutUint16(frag[2:4], uint16(20+len(part)))
		binary.BigEndian.PutUint16(frag[4:6], 0x5a17)
		flags := offsetUnits
		if more {
			flags |= 0x2000
		}
		binary.BigEndian.PutUint16(frag[6:8], flags)
		frag[10], frag[11] = 0, 0
		binary.BigEndian.PutUint16(frag[10:12], sadSelChecksum(frag))
		return append(frag, part...)
	}
	return build(payload[:fragFirstOctets], 0, true), build(payload[fragFirstOctets:], fragFirstOctets/8, false)
}

// TestRFC4301FragmentsOfACompliantPacketAreDelivered proves the admitting side. Method: a
// tunnel-mode Child SA negotiated for UDP from 10.1.0.0/16 to local port 5000 of
// 10.2.0.0/16; an inner packet inside the selectors is sent as two fragments, each in its
// own ESP packet on the SA, non-initial first; the reassembled packet is delivered and
// no policy drop is counted.
func TestRFC4301FragmentsOfACompliantPacketAreDelivered(t *testing.T) {
	// RFC requirement: RFC4301-7.3-5 positive -- the non-initial fragment of an inner
	// packet that complies with the inbound policy Ze installs for the SA is accepted:
	// the packet is reassembled and delivered, XfrmInNoPols unchanged.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	initial, nonInitial := fragSplit(sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	before := sadSelStat(t, sadSelStatNoPols)
	sender.send(t, sadSelProtoIPv4, nonInitial)
	sender.send(t, sadSelProtoIPv4, initial)
	if !sadSelDelivered(t, inside) {
		t.Fatal("the fragments of a packet inside the negotiated selectors were not delivered")
	}
	if got := sadSelStat(t, sadSelStatNoPols) - before; got != 0 {
		t.Errorf("%s moved by %d for a compliant fragmented packet, want 0", sadSelStatNoPols, got)
	}
}

// TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded proves the refusing side. Method:
// the same Child SA; an inner packet whose source lies outside the selector is sent as
// two fragments on the SA, non-initial first. Its non-initial fragment carries nothing
// a selector names but the addresses; the packet is discarded (XfrmInNoPols moves by
// one) and nothing is delivered.
func TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded(t *testing.T) {
	// RFC requirement: RFC4301-7.3-5 negative -- the non-initial fragment of an inner
	// packet that does not comply with the inbound policy for the SA (source outside
	// the selector) is discarded with the packet: XfrmInNoPols moves by one and nothing
	// is delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	initial, nonInitial := fragSplit(sadSelIPv4(sadSelInnerStray, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	before := sadSelStat(t, sadSelStatNoPols)
	sender.send(t, sadSelProtoIPv4, nonInitial)
	sender.send(t, sadSelProtoIPv4, initial)
	if sadSelDelivered(t, inside) {
		t.Error("the fragments of a packet outside the negotiated selectors were delivered")
	}
	if got := sadSelStat(t, sadSelStatNoPols) - before; got != 1 {
		t.Errorf("%s moved by %d, want 1 (the non-compliant packet's fragments discarded)", sadSelStatNoPols, got)
	}
}
