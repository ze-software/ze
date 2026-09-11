//go:build linux

package transport

import (
	"net/netip"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// VALIDATES: RFC 5880 Section 6.8.6 -- the receiver can name the local address
// a packet was sent to and the interface it arrived on, which is what the
// first-packet session selection is keyed on. parseReceivedPktinfo reads both
// out of the IP_PKTINFO and IPV6_PKTINFO control messages the sockets enable in
// applySocketOptions.
// PREVENTS: the round-7 finding returning. readLoop used to stamp
// u.Bind.Addr() onto Inbound.Local, which is the wildcard the socket binds, so
// no session carrying a real local address could ever be selected for a packet
// whose Your Discriminator is zero.

// controlMessage lays one cmsg out the way the kernel does: a Cmsghdr whose
// Len covers the header plus the payload, then the payload, padded to the
// alignment ParseSocketControlMessage walks by.
func controlMessage(t *testing.T, level, typ int32, payload []byte) []byte {
	t.Helper()
	buf := make([]byte, unix.CmsgSpace(len(payload)))
	h := (*unix.Cmsghdr)(unsafe.Pointer(&buf[0]))
	h.Level = level
	h.Type = typ
	h.SetLen(unix.CmsgLen(len(payload)))
	copy(buf[unix.CmsgLen(0):], payload)
	return buf
}

func TestParseReceivedPktinfoReadsTheDestinationAndIngress(t *testing.T) {
	// struct in_pktinfo: ifindex (int32), ipi_spec_dst (4), ipi_addr (4).
	// ipi_spec_dst deliberately differs from ipi_addr: the session key holds
	// the address the peer sent TO, which is ipi_addr.
	v4 := make([]byte, unix.SizeofInet4Pktinfo)
	v4[0] = 7 // ifindex 7, host byte order, little-endian on this arch
	copy(v4[4:8], []byte{10, 0, 0, 1})
	copy(v4[8:12], []byte{192, 0, 2, 5})
	addr, ifindex := parseReceivedPktinfo(controlMessage(t, unix.IPPROTO_IP, unix.IP_PKTINFO, v4))
	if want := netip.MustParseAddr("192.0.2.5"); addr != want {
		t.Errorf("v4 destination = %v, want %v (ipi_addr, not ipi_spec_dst)", addr, want)
	}
	if ifindex != 7 {
		t.Errorf("v4 ifindex = %d, want 7", ifindex)
	}

	// struct in6_pktinfo: ipi6_addr (16), ipi6_ifindex (uint32).
	v6 := make([]byte, unix.SizeofInet6Pktinfo)
	copy(v6[0:16], netip.MustParseAddr("2001:db8::5").AsSlice())
	v6[16] = 9
	addr, ifindex = parseReceivedPktinfo(controlMessage(t, unix.IPPROTO_IPV6, unix.IPV6_PKTINFO, v6))
	if want := netip.MustParseAddr("2001:db8::5"); addr != want {
		t.Errorf("v6 destination = %v, want %v", addr, want)
	}
	if ifindex != 9 {
		t.Errorf("v6 ifindex = %d, want 9", ifindex)
	}
}

func TestParseReceivedPktinfoRefusesWhatItCannotRead(t *testing.T) {
	if addr, ifindex := parseReceivedPktinfo(nil); addr.IsValid() || ifindex != 0 {
		t.Errorf("empty oob = (%v, %d), want (invalid, 0): an absent control message must not read as an address", addr, ifindex)
	}
	short := controlMessage(t, unix.IPPROTO_IP, unix.IP_PKTINFO, make([]byte, 4))
	if addr, ifindex := parseReceivedPktinfo(short); addr.IsValid() || ifindex != 0 {
		t.Errorf("truncated in_pktinfo = (%v, %d), want (invalid, 0)", addr, ifindex)
	}
	other := controlMessage(t, unix.SOL_SOCKET, unix.SO_TIMESTAMP, make([]byte, 16))
	if addr, ifindex := parseReceivedPktinfo(other); addr.IsValid() || ifindex != 0 {
		t.Errorf("an unrelated control message = (%v, %d), want (invalid, 0)", addr, ifindex)
	}
}
