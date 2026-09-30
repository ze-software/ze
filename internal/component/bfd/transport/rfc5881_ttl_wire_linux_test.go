//go:build linux

// VALIDATES: RFC 5881 Section 5 on the wire: the TTL or Hop Limit a Control
// packet leaves with, read from the IP header by the receiving socket
// (IP_RECVTTL, IPV6_RECVHOPLIMIT), in IPv4 and in IPv6, for a Control packet
// without an authentication section and for one carrying a Simple Password
// section.
// PREVENTS: an IPv6 socket that never sets IPV6_UNICAST_HOPS, and a TTL check
// that reads a socket option back rather than the packet that was sent.
package transport

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// ttlWirePort is the port the self-addressed sockets below bind. It is none of
// the BFD ports, so no other test's socket holds it.
const ttlWirePort = 47841

// rfc5881ControlBytes encodes a peer-less Control packet. With auth set it
// carries the A bit and a Simple Password section (type 1, length 7, key 1,
// password "abcd").
func rfc5881ControlBytes(auth bool) []byte {
	c := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       1,
		DesiredMinTxInterval:  1_000_000,
		RequiredMinRxInterval: 1_000_000,
	}
	section := []byte{1, 7, 1, 'a', 'b', 'c', 'd'}
	if auth {
		c.Auth = true
		c.Length = packet.MandatoryLen + uint8(len(section))
	}
	buf := make([]byte, int(c.Length))
	c.WriteTo(buf, 0)
	if auth {
		copy(buf[packet.MandatoryLen:], section)
	}
	return buf
}

// rfc5881ReceivedTTL starts a single-hop transport on addr, sends payload to
// addr, and returns the TTL or Hop Limit its own socket read off the packet.
func rfc5881ReceivedTTL(t *testing.T, addr netip.Addr, payload []byte) uint8 {
	t.Helper()
	u := &UDP{Bind: netip.AddrPortFrom(addr, ttlWirePort), Mode: api.SingleHop}
	if err := u.Start(); err != nil {
		t.Fatalf("Start on %s: %v", u.Bind, err)
	}
	defer u.Stop() //nolint:errcheck // Test cleanup; the transport is discarded.
	if err := u.Send(Outbound{To: addr, Mode: api.SingleHop, Bytes: payload}); err != nil {
		t.Fatalf("Send to %s: %v", addr, err)
	}
	select {
	case in := <-u.RX():
		return in.TTL
	case <-time.After(2 * time.Second):
		t.Fatalf("no packet came back to %s", u.Bind)
	}
	return 0
}

// RFC requirement: RFC5881-5-1 positive -- a Control packet without an
// authentication section, sent through the single-hop transport, is read off
// the wire with TTL 255 in IPv4 (127.88.82.1) and Hop Limit 255 in IPv6 (::1).
// RFC requirement: RFC5881-5-3 positive -- a Control packet with the A bit set
// and a Simple Password section, sent through the same transport, is read off
// the wire with TTL 255 in IPv4 and Hop Limit 255 in IPv6.
func TestRFC5881ControlLeavesWithTTL255OnTheWire(t *testing.T) {
	for _, addr := range []netip.Addr{netip.MustParseAddr("127.88.82.1"), netip.IPv6Loopback()} {
		for _, auth := range []bool{false, true} {
			if got := rfc5881ReceivedTTL(t, addr, rfc5881ControlBytes(auth)); got != 255 {
				t.Fatalf("%s, authentication section %v: packet left with TTL/Hop Limit %d, want 255", addr, auth, got)
			}
		}
	}
}
