// Design: docs/architecture/testing/interop.md -- independent peer egress observations.
// Related: frame.go -- portable IPv4/ESP observation without a packet socket.
package main

import (
	"encoding/binary"
	"errors"
	"testing"
)

// TestParsePacketReadsBothESPForms reads SPI and sequence from independent IP packets.
func TestParsePacketReadsBothESPForms(t *testing.T) {
	for _, form := range []espForm{formBare, formUDP} {
		got, err := parsePacket(capturedPacket{data: packetFixture(form), outgoing: true}, fixtureKey())
		if err != nil {
			t.Fatal(err)
		}
		if got.form != form {
			t.Fatalf("form = %d, want %d", got.form, form)
		}
		if got.sequence != 42 {
			t.Fatalf("sequence = %d, want 42 from ESP header", got.sequence)
		}
		if got.key != fixtureKey() {
			t.Fatalf("wire identity = %+v, want %+v", got.key, fixtureKey())
		}
	}
}

// TestParsePacketRefusesIngressAndPlaintext requires socket direction and ESP framing.
func TestParsePacketRefusesIngressAndPlaintext(t *testing.T) {
	ingress := capturedPacket{data: packetFixture(formBare)}
	if _, err := parsePacket(ingress, fixtureKey()); !errors.Is(err, errUnrelatedPacket) {
		t.Fatalf("ingress = %v, want unrelated packet", err)
	}
	plain := packetFixture(formBare)
	plain[9] = 1 // ICMP before XFRM must never count as encrypted egress.
	if _, err := parsePacket(capturedPacket{data: plain, outgoing: true}, fixtureKey()); !errors.Is(err, errUnrelatedPacket) {
		t.Fatalf("plaintext = %v, want unrelated packet", err)
	}
}

// TestParsePacketRefusesMalformedMatchingPackets never manufactures a sequence from short bytes.
func TestParsePacketRefusesMalformedMatchingPackets(t *testing.T) {
	for _, length := range []int{0, 1, 19, 20, 23, 27} {
		packet := packetFixture(formBare)
		if _, err := parsePacket(capturedPacket{data: packet[:length], outgoing: true}, fixtureKey()); err == nil {
			t.Fatalf("truncated packet of %d bytes was accepted", length)
		}
	}
	fragment := packetFixture(formBare)
	binary.BigEndian.PutUint16(fragment[6:8], 0x2000)
	if _, err := parsePacket(capturedPacket{data: fragment, outgoing: true}, fixtureKey()); err == nil {
		t.Fatal("a fragment supplied a whole-packet ESP observation")
	}
}

// TestParsePacketIgnoresOtherSAs distinguishes irrelevant traffic from a usable observation.
func TestParsePacketIgnoresOtherSAs(t *testing.T) {
	packet := packetFixture(formBare)
	packet[23]++
	_, err := parsePacket(capturedPacket{data: packet, outgoing: true}, fixtureKey())
	if !errors.Is(err, errUnrelatedPacket) {
		t.Fatalf("other SPI = %v, want unrelated-packet result", err)
	}
}

// TestParsePacketRejectsIKEAndZeroSequence keeps UDP/4500 control traffic outside ESP evidence.
func TestParsePacketRejectsIKEAndZeroSequence(t *testing.T) {
	ike := packetFixture(formUDP)
	clear(ike[28:32])
	if _, err := parsePacket(capturedPacket{data: ike, outgoing: true}, fixtureKey()); err == nil {
		t.Fatal("the non-ESP marker was read as ESP")
	}
	zero := packetFixture(formBare)
	clear(zero[24:28])
	if _, err := parsePacket(capturedPacket{data: zero, outgoing: true}, fixtureKey()); err == nil {
		t.Fatal("sequence zero was accepted")
	}
}

func packetFixture(form espForm) []byte {
	length, protocol, off := 28, byte(50), 20
	if form == formUDP {
		length, protocol, off = 36, 17, 28
	}
	packet := make([]byte, length)
	packet[0], packet[8], packet[9] = 0x45, 64, protocol
	binary.BigEndian.PutUint16(packet[2:4], uint16(length))
	source, target := fixtureKey().source.As4(), fixtureKey().target.As4()
	copy(packet[12:16], source[:])
	copy(packet[16:20], target[:])
	if form == formUDP {
		binary.BigEndian.PutUint16(packet[20:22], 4500)
		binary.BigEndian.PutUint16(packet[22:24], 4500)
		binary.BigEndian.PutUint16(packet[24:26], 16)
	}
	binary.BigEndian.PutUint32(packet[off:off+4], fixtureKey().spi)
	binary.BigEndian.PutUint32(packet[off+4:off+8], 42)
	return packet
}

// FuzzParsePacket exercises the untrusted frame boundary, including truncated IP headers.
func FuzzParsePacket(f *testing.F) {
	f.Add(packetFixture(formBare))
	f.Add(packetFixture(formUDP))
	f.Fuzz(func(t *testing.T, packet []byte) {
		_, err := parsePacket(capturedPacket{data: packet, outgoing: true}, fixtureKey())
		if err != nil {
			return
		}
	})
}
