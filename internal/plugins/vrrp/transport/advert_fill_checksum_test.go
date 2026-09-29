// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- advertisement transmit path
// Related: transport.go -- encodeLocked, the encoder under test
package transport

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// VALIDATES: the frame the transport sends carries the fields of the
// advertisement it was given and, for IPv4, the checksum RFC 5798 defines.
//
// TestSendAdvertFillsFieldsAndChecksum checks the two transmit steps of the
// RFC Section 7.2 list through the production encoder (encodeLocked), not
// through a test that calls WriteTo and FillChecksum itself: a transmit path
// that skipped the checksum, or wrote a stale field, reddens it.
//
// Method: an IPv4 instance whose parent primary is 192.0.2.251 sends a VRRPv3
// advertisement for VRID 10, priority 100, 1000 ms and 192.0.2.1. The sent
// frame is the 20-octet IPv4 header followed by the VRRP message. Every VRRP
// field is compared with literals, and the checksum with an RFC 1071 sum this
// test computes over the RFC 5798 Section 5.2.8 pseudo-header (source,
// 224.0.0.18, zero, protocol 112, VRRP length) and the message with its
// checksum field zeroed. The IPv6 case compares the fields only: on IPv6 the
// kernel writes the checksum (IPV6_CHECKSUM, backend_linux.go), so the frame
// the transport hands over carries none.
//
// RFC requirement: RFC5798-7.2-1 positive -- the IPv4 VRRPv3 frame the transport sends (encodeLocked transport.go) carries Version 3, Type 1, VRID 10, Priority 100, Count 1, Max Adver Int 100 centiseconds and 192.0.2.1, and a checksum equal to an RFC 1071 sum computed by the test over the RFC 5798 Section 5.2.8 pseudo-header and the message.
// RFC requirement: RFC9568-7.2-1 positive -- the IPv4 and IPv6 VRRPv3 frames the transport sends (encodeLocked transport.go) carry the Version, Type, VRID, Priority, Count, Reserve, Max Advertise Interval and address fields of the advertisement state handed to it, compared with literals.
func TestSendAdvertFillsFieldsAndChecksum(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.251", Family: "ipv4"}})
		fb := &fakeBackend{}
		tr := New(fb)
		key, err := tr.OpenInstance(v4Spec())
		if err != nil {
			t.Fatalf("OpenInstance: %v", err)
		}
		if err := tr.UpdateAdvert(key, v4Params()); err != nil {
			t.Fatalf("UpdateAdvert: %v", err)
		}
		if err := tr.SendAdvert(key); err != nil {
			t.Fatalf("SendAdvert: %v", err)
		}
		frame := fb.last().lastAdvert()
		if len(frame) != ipv4HeaderLen+packet.HeaderLen+4 {
			t.Fatalf("sent frame is %d octets, want %d", len(frame), ipv4HeaderLen+packet.HeaderLen+4)
		}
		message := frame[ipv4HeaderLen:]
		assertAdvertFields(t, message, []byte{192, 0, 2, 1})

		source := netip.MustParseAddr("192.0.2.251").As4()
		target := netip.MustParseAddr("224.0.0.18").As4()
		pseudo := make([]byte, 0, 12+len(message))
		pseudo = append(pseudo, source[:]...)
		pseudo = append(pseudo, target[:]...)
		pseudo = append(pseudo, 0, 112)
		pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(message)))
		pseudo = append(pseudo, message...)
		pseudo[12+6], pseudo[12+7] = 0, 0
		want := rfc1071Checksum(pseudo)
		if got := binary.BigEndian.Uint16(message[6:8]); got != want {
			t.Fatalf("checksum = %#04x, want %#04x (RFC 5798 pseudo-header form)", got, want)
		}
	})
	t.Run("ipv6", func(t *testing.T) {
		fb := &fakeBackend{}
		tr := New(fb)
		key, err := tr.OpenInstance(v6Spec())
		if err != nil {
			t.Fatalf("OpenInstance: %v", err)
		}
		params := v4Params()
		params.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1")}
		if err := tr.UpdateAdvert(key, params); err != nil {
			t.Fatalf("UpdateAdvert: %v", err)
		}
		if err := tr.SendAdvert(key); err != nil {
			t.Fatalf("SendAdvert: %v", err)
		}
		frame := fb.last().lastAdvert()
		if len(frame) != packet.HeaderLen+16 {
			t.Fatalf("sent frame is %d octets, want %d", len(frame), packet.HeaderLen+16)
		}
		linkLocal := netip.MustParseAddr("fe80::1").As16()
		assertAdvertFields(t, frame, linkLocal[:])
	})
}

// assertAdvertFields compares the fixed VRRPv3 fields of a sent message, and
// its address list, with the literals the test's advertisement state implies.
func assertAdvertFields(t *testing.T, message, addresses []byte) {
	t.Helper()
	// Version 3 and Type 1, VRID 10, Priority 100, Count 1, Reserve 0 with
	// Max Advertise Interval 100 centiseconds.
	want := []byte{0x31, 10, 100, 1, 0x00, 0x64}
	if !slices.Equal(message[:6], want) {
		t.Fatalf("fixed fields = % x, want % x", message[:6], want)
	}
	if !slices.Equal(message[packet.HeaderLen:], addresses) {
		t.Fatalf("addresses = % x, want % x", message[packet.HeaderLen:], addresses)
	}
}

// rfc1071Checksum is the one's complement of the one's complement sum of b,
// computed here so the expectation does not come from the code under test.
func rfc1071Checksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}
