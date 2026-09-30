// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4303_inbound_key_test.go -- the inbound SA's key as installChildSA builds it
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, ESP sender and listeners reused here
// RFC: rfc/short/rfc4303.md -- the inbound SA lookup key an IKE Child SA sets (Section 2.1)
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains.

//go:build linux

package engine

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

// inKeyOtherLocal is a local address of the namespace that the Child SA did not negotiate.
const inKeyOtherLocal = "10.250.0.3"

// inKeySendTo is sadSelSender.send aimed at target rather than the negotiated local
// address: the same SPI, sequence numbering, key and sealing, so the destination is the
// only field that differs from a packet the SA accepts.
func inKeySendTo(t *testing.T, s *sadSelSender, target string, nextHeader byte, payload []byte) {
	t.Helper()
	s.seq++
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], s.spi)
	binary.BigEndian.PutUint32(header[4:8], s.seq)
	iv := make([]byte, 8)
	binary.BigEndian.PutUint32(iv[4:8], s.seq)

	padOctets := (4 - (len(payload)+2)%4) % 4
	plain := make([]byte, 0, len(payload)+padOctets+2)
	plain = append(plain, payload...)
	for i := range padOctets {
		plain = append(plain, byte(i+1))
	}
	plain = append(plain, byte(padOctets), nextHeader)

	nonce := append(append([]byte{}, s.salt...), iv...)
	packet := append(append(header, iv...), s.aead.Seal(nil, nonce, plain, header)...)
	if _, err := s.conn.WriteToIP(packet, &net.IPAddr{IP: net.ParseIP(target)}); err != nil {
		t.Fatalf("send ESP to %s: %v", target, err)
	}
}

// TestRFC4303InboundESPToAnotherLocalAddressMapsToNoSA proves, on real XFRM, that the
// inbound SA of an IKE Child SA is found by its negotiated destination as well as its SPI.
// Method: a tunnel-mode Child SA installed by createFirstChildSA with local address
// 10.250.0.1; one ESP packet under its inbound SPI to 10.250.0.1 is the control (found:
// XfrmInNoStates unchanged, delivered); the same packet to 10.250.0.3, another local
// address of the namespace, must match no SA: XfrmInNoStates rises by one and nothing is
// delivered.
func TestRFC4303InboundESPToAnotherLocalAddressMapsToNoSA(t *testing.T) {
	// RFC requirement: RFC4303-2.1-2 negative -- ESP under the Child SA's inbound SPI to a local address other than the negotiated one maps to no SA (XfrmInNoStates rises by one, nothing delivered), while the same packet to the negotiated address is found.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	addr, err := netlink.ParseAddr(inKeyOtherLocal + "/32")
	if err != nil {
		t.Fatalf("parse %s: %v", inKeyOtherLocal, err)
	}
	if err := netlink.AddrAdd(lo, addr); err != nil {
		t.Fatalf("add %s to lo: %v", inKeyOtherLocal, err)
	}
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	payload := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))

	noStates := sadSelStat(t, inStepStatNoStates)
	sender.send(t, sadSelProtoIPv4, payload)
	if !sadSelDelivered(t, inside) {
		t.Fatal("control: an ESP packet to the negotiated local address was not delivered")
	}
	if got := sadSelStat(t, inStepStatNoStates) - noStates; got != 0 {
		t.Fatalf("control: %s rose by %d for the negotiated local address, want 0", inStepStatNoStates, got)
	}

	noStates = sadSelStat(t, inStepStatNoStates)
	inKeySendTo(t, sender, inKeyOtherLocal, sadSelProtoIPv4, payload)
	if sadSelDelivered(t, inside) {
		t.Fatal("an ESP packet to a local address the Child SA did not negotiate was delivered")
	}
	if got := sadSelStat(t, inStepStatNoStates) - noStates; got != 1 {
		t.Fatalf("%s rose by %d for another local address, want 1", inStepStatNoStates, got)
	}
}
