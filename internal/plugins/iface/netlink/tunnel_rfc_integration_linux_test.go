// Design: docs/features/interfaces.md -- tunnel kinds
// Related: tunnel_linux.go -- buildSittun, buildIp6tnl, parseTunnelRemote
// Related: tunnel_linux_test.go -- withTunnelNetNS, the netns harness
//
// Whole-stack proofs for two tunnel obligations Ze meets through the kernel
// datapath it configures (ai/rules/rfc-compliance.md). Each test installs the
// tunnel through the netlink backend Ze runs, reads back what Ze installed,
// and drives a packet through the kernel to observe the RFC outcome.
//
// VALIDATES: RFC 2473 Section 4.1.1 (c): a packet entering an ip6tnl tunnel
// with a non-zero Tunnel Encapsulation Limit leaves with an encapsulating
// Tunnel Encapsulation Limit option set to one less. RFC 4213 Section 3.6:
// the sit decapsulator Ze installs processes a packet only from the configured
// remote, and Ze refuses a remote that would disable that check.
// PREVENTS: an ip6tnl link installed with IP6_TNL_F_IGN_ENCAP_LIMIT, a sit
// link installed without its remote, and a wildcard (0.0.0.0) remote that
// makes the sit tunnel decapsulate from any source.

//go:build integration && linux

package ifacenetlink

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// ip6TnlFlagIgnEncapLimit is IP6_TNL_F_IGN_ENCAP_LIMIT (include/uapi/linux/ip6_tunnel.h).
const ip6TnlFlagIgnEncapLimit = 0x1

func hostToNet16(v uint16) uint16 { return v<<8 | v>>8 }

// underlayDummy creates an up dummy link carrying cidr, without DAD, as the
// tunnel underlay.
func underlayDummy(t *testing.T, name, cidr string) netlink.Link {
	t.Helper()
	if err := netlink.LinkAdd(&netlink.Dummy{Name: name}); err != nil {
		t.Skipf("dummy link unavailable: %v", err)
	}
	link := upLink(t, name)
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse %s: %v", cidr, err)
	}
	addr.Flags = unix.IFA_F_NODAD
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("address %s on %s: %v", cidr, name, err)
	}
	return link
}

// upLink sets the named link up and returns it.
func upLink(t *testing.T, name string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %s up: %v", name, err)
	}
	return link
}

// sendHeaderIncluded sends pkt, whose IP header the test wrote, on an
// IPPROTO_RAW socket (the header-included mode for both families).
func sendHeaderIncluded(t *testing.T, family int, pkt []byte, to unix.Sockaddr) {
	t.Helper()
	fd, err := unix.Socket(family, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("raw socket: %v", err)
	}
	defer unix.Close(fd) //nolint:errcheck // test socket
	if err := unix.Sendto(fd, pkt, 0, to); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// ---- RFC 2473 Section 4.1.1 (c) ----

// innerIPv6WithEncapLimit is an IPv6 packet to 2001:db8:ff::1 whose only
// extension header is a Destination Options header holding a Tunnel
// Encapsulation Limit option (RFC 2473 Section 5.1, type 4, length 1).
func innerIPv6WithEncapLimit(limit byte) []byte {
	pkt := make([]byte, 48)
	pkt[0] = 0x60
	binary.BigEndian.PutUint16(pkt[4:], 8) // Payload Length: the options header.
	pkt[6] = 60                            // Next Header: Destination Options.
	pkt[7] = 64
	copy(pkt[8:24], net.ParseIP("2001:db8:aa::1").To16())
	copy(pkt[24:40], net.ParseIP("2001:db8:ff::1").To16())
	// Next Header 59 (none), Hdr Ext Len 0, TEL option, PadN of one octet.
	copy(pkt[40:], []byte{59, 0, 4, 1, limit, 1, 1, 0})
	return pkt
}

// outerEncapLimit sends inner into the ip6tnl tunnel tel0 Ze installed for
// spec and returns the Tunnel Encapsulation Limit the outer packet carries on
// the underlay, and whether it carries one.
func outerEncapLimit(t *testing.T, b iface.Backend, spec iface.TunnelSpec, inner []byte) (byte, bool) {
	t.Helper()
	under := underlayDummy(t, "und6", "2001:db8::1/64")
	if err := b.CreateTunnel(spec); err != nil {
		t.Fatalf("create ip6tnl: %v", err)
	}
	tunnel := upLink(t, spec.Name)
	_, inside, err := net.ParseCIDR("2001:db8:ff::/64")
	if err != nil {
		t.Fatalf("parse route: %v", err)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: tunnel.Attrs().Index, Dst: inside}); err != nil {
		t.Fatalf("route into the tunnel: %v", err)
	}

	// Only an ETH_P_ALL tap sees a device's outgoing packets.
	capture, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, int(hostToNet16(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("packet socket: %v", err)
	}
	defer unix.Close(capture) //nolint:errcheck // test socket
	if err := unix.Bind(capture, &unix.SockaddrLinklayer{Protocol: hostToNet16(unix.ETH_P_ALL), Ifindex: under.Attrs().Index}); err != nil {
		t.Fatalf("bind packet socket: %v", err)
	}
	timeout := unix.NsecToTimeval(int64(2 * time.Second))
	if err := unix.SetsockoptTimeval(capture, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		t.Fatalf("receive timeout: %v", err)
	}

	var to unix.SockaddrInet6
	copy(to.Addr[:], inner[24:40])
	sendHeaderIncluded(t, unix.AF_INET6, inner, &to)

	remote := net.ParseIP(spec.RemoteAddress).To16()
	buf := make([]byte, 2048)
	for {
		n, _, err := unix.Recvfrom(capture, buf, 0)
		if err != nil {
			t.Fatalf("no encapsulated packet left on the underlay: %v", err)
		}
		outer := buf[:n]
		if n < 40 || outer[0]>>4 != 6 || !net.IP(outer[24:40]).Equal(remote) {
			continue // MLD, router solicitation, anything not for the tunnel.
		}
		return encapLimitOption(t, outer)
	}
}

// encapLimitOption walks the outer packet's Destination Options header, when
// the IPv6 header names one, for a Tunnel Encapsulation Limit option.
func encapLimitOption(t *testing.T, outer []byte) (byte, bool) {
	t.Helper()
	if outer[6] != 60 {
		return 0, false
	}
	if len(outer) < 48 {
		t.Fatalf("outer Destination Options header truncated: %x", outer)
	}
	end := 40 + (int(outer[41])+1)*8
	if end > len(outer) {
		t.Fatalf("outer Destination Options length %d past the packet", outer[41])
	}
	for off := 42; off < end; {
		optionType := outer[off]
		if optionType == 0 { // Pad1.
			off++
			continue
		}
		if off+2 > end {
			break
		}
		optionLength := int(outer[off+1])
		if optionType == 4 && optionLength == 1 {
			return outer[off+2], true
		}
		off += 2 + optionLength
	}
	return 0, false
}

// RFC requirement: RFC2473-4.1.1-2 positive -- the ip6tnl link Ze installs
// carries no IP6_TNL_F_IGN_ENCAP_LIMIT flag, and an IPv6 packet entering it
// with a Tunnel Encapsulation Limit of 3 leaves on the underlay with an
// encapsulating Tunnel Encapsulation Limit option of 2.
func TestRFC2473EncapLimitDecrementedIntoTheOuterHeader(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind: iface.TunnelKindIP6Tnl, Name: "tel0",
			LocalAddress: "2001:db8::1", RemoteAddress: "2001:db8::2",
		}
		limit, present := outerEncapLimit(t, b, spec, innerIPv6WithEncapLimit(3))

		link, err := netlink.LinkByName("tel0")
		if err != nil {
			t.Fatalf("lookup tel0: %v", err)
		}
		installed, ok := link.(*netlink.Ip6tnl)
		if !ok {
			t.Fatalf("tel0 is %T, want *netlink.Ip6tnl", link)
		}
		if installed.Flags&ip6TnlFlagIgnEncapLimit != 0 {
			t.Fatalf("Ze installed tel0 with IP6_TNL_F_IGN_ENCAP_LIMIT (flags %#x)", installed.Flags)
		}
		if !present {
			t.Fatal("the encapsulating headers carry no Tunnel Encapsulation Limit option")
		}
		if limit != 2 {
			t.Fatalf("outer Tunnel Encapsulation Limit = %d, want 2 (one less than the inner 3)", limit)
		}
	})
}

// RFC requirement: RFC2473-4.1.1-2 negative -- a configured encapsulation
// limit of 4 pushes toward the wrong outer value: a packet entering with a
// limit of 3 still leaves with 2, one less than the limit found in the
// packet, never the configured 4, and the installed link carries no
// IP6_TNL_F_IGN_ENCAP_LIMIT flag.
func TestRFC2473EncapLimitFromThePacketNotTheConfiguredLimit(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind: iface.TunnelKindIP6Tnl, Name: "tel0",
			LocalAddress: "2001:db8::1", RemoteAddress: "2001:db8::2",
			EncapLimit: 4, EncapLimitSet: true,
		}
		limit, present := outerEncapLimit(t, b, spec, innerIPv6WithEncapLimit(3))

		link, err := netlink.LinkByName("tel0")
		if err != nil {
			t.Fatalf("lookup tel0: %v", err)
		}
		installed, ok := link.(*netlink.Ip6tnl)
		if !ok {
			t.Fatalf("tel0 is %T, want *netlink.Ip6tnl", link)
		}
		if installed.Flags&ip6TnlFlagIgnEncapLimit != 0 {
			t.Fatalf("Ze installed tel0 with IP6_TNL_F_IGN_ENCAP_LIMIT (flags %#x)", installed.Flags)
		}
		if installed.EncapLimit != 4 {
			t.Fatalf("installed encapsulation limit = %d, want the configured 4", installed.EncapLimit)
		}
		if !present {
			t.Fatal("the encapsulating headers carry no Tunnel Encapsulation Limit option")
		}
		if limit != 2 {
			t.Fatalf("outer Tunnel Encapsulation Limit = %d, want 2 (inner 3 less one), not the configured 4", limit)
		}
	})
}

// ---- RFC 4213 Section 3.6 ----

// sixInFour is an IPv4 packet from source to 192.0.2.1 with Protocol 41,
// carrying a bare IPv6 header (Next Header 59). The kernel fills the IPv4
// Total Length and checksum on a header-included send.
func sixInFour(source string) []byte {
	pkt := make([]byte, 60)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:], 60)
	pkt[8] = 64
	pkt[9] = 41
	copy(pkt[12:16], net.ParseIP(source).To4())
	copy(pkt[16:20], net.ParseIP("192.0.2.1").To4())
	pkt[20] = 0x60
	pkt[26] = 59
	pkt[27] = 64
	copy(pkt[28:44], net.ParseIP("2001:db8:1::1").To16())
	copy(pkt[44:60], net.ParseIP("2001:db8:2::1").To16())
	return pkt
}

// sitRxPackets reads the named tunnel's received packet counter.
func sitRxPackets(t *testing.T, name string) uint64 {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}
	stats := link.Attrs().Statistics
	if stats == nil {
		t.Fatalf("%s reports no statistics", name)
	}
	return stats.RxPackets
}

// sitRxAfter sends one 6in4 packet from source to the local end and returns
// the tunnel's received counter once it moved, or after wait.
func sitRxAfter(t *testing.T, name, source string, before uint64, wait time.Duration) uint64 {
	t.Helper()
	to := unix.SockaddrInet4{Addr: [4]byte{192, 0, 2, 1}}
	sendHeaderIncluded(t, unix.AF_INET, sixInFour(source), &to)
	deadline := time.Now().Add(wait)
	for {
		now := sitRxPackets(t, name)
		if now != before || time.Now().After(deadline) {
			return now
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// sitWithRemote installs sit tunnel tsit1, 192.0.2.1 to remote, over a dummy
// underlay with loopback up, and sets it up.
func sitWithRemote(t *testing.T, b iface.Backend, remote string) {
	t.Helper()
	upLink(t, "lo")
	underlayDummy(t, "und4", "192.0.2.1/24")
	spec := iface.TunnelSpec{
		Kind: iface.TunnelKindSIT, Name: "tsit1",
		LocalAddress: "192.0.2.1", RemoteAddress: remote,
	}
	if err := b.CreateTunnel(spec); err != nil {
		t.Fatalf("create sit: %v", err)
	}
	upLink(t, "tsit1")
}

// RFC requirement: RFC4213-3.6-1 positive -- the sit link Ze installs carries
// the configured remote 198.51.100.1 and local 192.0.2.1, and a 6in4 packet
// whose outer source is that remote is decapsulated by the tunnel (its
// received counter moves).
func TestRFC4213SitDecapsulatesFromTheConfiguredRemote(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		sitWithRemote(t, b, "198.51.100.1")

		link, err := netlink.LinkByName("tsit1")
		if err != nil {
			t.Fatalf("lookup tsit1: %v", err)
		}
		installed, ok := link.(*netlink.Sittun)
		if !ok {
			t.Fatalf("tsit1 is %T, want *netlink.Sittun", link)
		}
		if !installed.Remote.Equal(net.ParseIP("198.51.100.1")) {
			t.Fatalf("installed remote = %v, want 198.51.100.1", installed.Remote)
		}
		if !installed.Local.Equal(net.ParseIP("192.0.2.1")) {
			t.Fatalf("installed local = %v, want 192.0.2.1", installed.Local)
		}

		before := sitRxPackets(t, "tsit1")
		if after := sitRxAfter(t, "tsit1", "198.51.100.1", before, 2*time.Second); after != before+1 {
			t.Fatalf("a packet from the configured remote moved the counter %d -> %d, want +1", before, after)
		}
	})
}

// RFC requirement: RFC4213-3.6-1 negative -- a 6in4 packet whose outer source
// is not the configured remote is not decapsulated by the tunnel (its received
// counter stays, while a packet from the remote sent next moves it), and Ze
// refuses a sit tunnel whose remote is missing or the wildcard 0.0.0.0, which
// would decapsulate from any source.
func TestRFC4213SitRefusesAnUnverifiedSource(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		sitWithRemote(t, b, "198.51.100.1")

		before := sitRxPackets(t, "tsit1")
		if after := sitRxAfter(t, "tsit1", "203.0.113.9", before, 300*time.Millisecond); after != before {
			t.Fatalf("a packet from 203.0.113.9, not the remote, moved the counter %d -> %d", before, after)
		}
		if after := sitRxAfter(t, "tsit1", "198.51.100.1", before, 2*time.Second); after != before+1 {
			t.Fatalf("positive control: a packet from the remote moved the counter %d -> %d, want +1", before, after)
		}

		for _, remote := range []string{"", "0.0.0.0"} {
			spec := iface.TunnelSpec{
				Kind: iface.TunnelKindSIT, Name: "tsit2",
				LocalAddress: "192.0.2.1", RemoteAddress: remote,
			}
			if err := b.CreateTunnel(spec); err == nil {
				t.Errorf("sit tunnel with remote %q accepted; it would decapsulate from any source", remote)
			}
			var missing netlink.LinkNotFoundError
			if _, err := netlink.LinkByName("tsit2"); !errors.As(err, &missing) {
				t.Errorf("remote %q: tsit2 exists in the kernel (lookup error %v)", remote, err)
				if link, lookupErr := netlink.LinkByName("tsit2"); lookupErr == nil {
					_ = netlink.LinkDel(link)
				}
			}
		}
	})
}
