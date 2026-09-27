//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the address owner's physical-MAC answers
package vrrp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft" // registers the nft firewall backend
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// VALIDATES: the address owner's LAN answers carry only the virtual MAC.
//
// TestVRRPOwnerAnswersWithVirtualMACOnly checks what a host on the LAN learns
// when it resolves the address owner's virtual address. The parent holds the
// address as a real address and the virtual-MAC macvlan holds it too, as the
// product installs it. Without the owner filter both answer, and the host
// captures the parent's physical MAC in an ARP reply and in a Neighbor
// Advertisement's Target Link-Layer Address option; that phase is the positive
// control, proving the capture sees a physical-MAC answer when one is sent.
// With the product's owner filter applied through the firewall backend, every
// answer carries the virtual MAC. Withdrawing the filter restores the parent's
// answer, so the filter's lifetime is what silences it.
//
// The IPv6 half also proves the precondition the design rests on: a /128 owner
// address on the macvlan (accept_dad=0, the product recipe) and the same
// address on the parent (DAD on) both end up usable, neither tentative nor
// failed.
func TestVRRPOwnerAnswersWithVirtualMACOnly(t *testing.T) {
	// RFC requirement: RFC3768-8.2-1 positive -- with the owner filter applied, every ARP reply for the address owner's virtual address carries the virtual router MAC as sender hardware address, and none the parent's physical MAC (ownerARPReplyTerm ownerfilter.go).
	// RFC requirement: RFC3768-8.2-1 negative -- control: without the filter the parent answers the same request with its physical MAC, and withdrawing the filter restores that answer (ownerFilterTables ownerfilter.go).
	// RFC requirement: RFC5798-8.1.2-1 positive -- with the owner filter applied, the address owner's ARP replies carry only the virtual router MAC (ownerARPReplyTerm ownerfilter.go).
	// RFC requirement: RFC5798-8.2.2-1 positive -- with the owner filter applied, every Neighbor Advertisement for the address owner's IPv6 virtual address carries the virtual router MAC in its Target Link-Layer Address option, and none the physical MAC (ownerNDAdvertTerm ownerfilter.go).
	// RFC requirement: RFC5798-8.2.2-1 negative -- control: without the filter the parent advertises its physical MAC for the owned address (ownerFilterTables ownerfilter.go).
	// RFC requirement: RFC9568-8.2.2-1 positive -- with the owner filter applied, every Neighbor Advertisement for the address owner's IPv6 virtual address carries the Virtual Router MAC in its Target Link-Layer Address option, and none the physical MAC (ownerNDAdvertTerm ownerfilter.go).
	// RFC requirement: RFC9568-8.2.2-1 negative -- control: without the filter the parent, which holds the owned address as a real address, advertises its physical MAC for it (ownerFilterTables ownerfilter.go).
	w := newGatewayWire(t)
	if err := firewall.LoadBackend("nft"); err != nil {
		t.Skipf("nft firewall backend unavailable: %v", err)
	}
	t.Cleanup(func() { _ = firewall.CloseBackend() })

	parent := w.parent.Attrs().Name
	v4 := netip.MustParseAddr("192.0.2.254") // the parent's own address: the owner case
	v6 := netip.MustParseAddr("2001:db8::254")
	if err := w.backend.AddAddress(parent, "2001:db8::254/64"); err != nil {
		t.Fatal(err)
	}
	ownerAwaitUsable(t, w.parent, v6)

	macs := map[uint8]net.HardwareAddr{}
	for _, group := range []struct {
		device string
		family string
		ver    uint8
		vip    netip.Addr
		real   netip.Prefix
	}{
		{"zo4", familyIPv4, packet.V4, v4, netip.MustParsePrefix("192.0.2.254/24")},
		{"zo6", familyIPv6, packet.V6, v6, netip.MustParsePrefix("2001:db8::254/64")},
	} {
		vmac := packet.VirtualMAC(group.ver, 10)
		macs[group.ver] = net.HardwareAddr(vmac[:])
		if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
			Name: group.device, Parent: parent, MAC: macString(vmac),
			Mode: iface.MacvlanModePrivate, Alias: "ze:owned:owner-answer-proof",
		}); err != nil {
			t.Fatal(err)
		}
		if err := applyDataplaneSysctls(parent, group.device, group.family); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { revertDataplaneSysctls(parent, group.device, group.family) })
		spec := GroupSpec{
			Family: group.family, VRID: 10, Version: 3, Priority: ownerPriority, IsOwner: true,
			VIPs: []netip.Addr{group.vip}, realAddresses: []netip.Addr{group.vip},
			realPrefixes: []netip.Prefix{group.real},
		}
		for _, cidr := range spec.vipCIDRs(spec.VIPs) {
			if err := w.backend.AddAddress(group.device, cidr); err != nil {
				t.Fatal(err)
			}
		}
	}
	ownerAwaitUsable(t, gatewayLink(t, "zo6"), v6)
	// The parent kept its address through the macvlan's install.
	ownerAwaitUsable(t, w.parent, v6)

	physical := w.parent.Attrs().HardwareAddr
	arpFD := ownerSocket(t, w.peer, unix.ETH_P_ARP)
	ndFD := ownerSocket(t, w.peer, unix.ETH_P_IPV6)

	// Control: no filter, so the parent answers with its physical MAC.
	ownerExpect(t, "control ARP", ownerResolveARP(t, w.peer, arpFD, v4), physical, macs[packet.V4], true)
	ownerExpect(t, "control ND", ownerResolveND(t, w.peer, ndFD, v6), physical, macs[packet.V6], true)

	if err := setOwnerFilter("vrrp:zo4", parent, []netip.Addr{v4, v6}); err != nil {
		t.Fatalf("apply owner filter: %v", err)
	}
	t.Cleanup(func() { _ = clearOwnerFilter("vrrp:zo4") })
	ownerExpect(t, "filtered ARP", ownerResolveARP(t, w.peer, arpFD, v4), physical, macs[packet.V4], false)
	ownerExpect(t, "filtered ND", ownerResolveND(t, w.peer, ndFD, v6), physical, macs[packet.V6], false)

	if err := clearOwnerFilter("vrrp:zo4"); err != nil {
		t.Fatalf("withdraw owner filter: %v", err)
	}
	ownerExpect(t, "withdrawn ARP", ownerResolveARP(t, w.peer, arpFD, v4), physical, macs[packet.V4], true)
}

// ownerExpect checks the link-layer addresses one resolution collected. The
// virtual MAC MUST be among them. The physical MAC MUST be among them when
// wantPhysical, and MUST NOT otherwise.
func ownerExpect(t *testing.T, phase string, got []net.HardwareAddr, physical, virtual net.HardwareAddr, wantPhysical bool) {
	t.Helper()
	sawPhysical, sawVirtual := false, false
	for _, mac := range got {
		switch {
		case bytes.Equal(mac, physical):
			sawPhysical = true
		case bytes.Equal(mac, virtual):
			sawVirtual = true
		default:
			t.Errorf("%s: answer carries unexpected MAC %s", phase, mac)
		}
	}
	if !sawVirtual {
		t.Errorf("%s: no answer carried the virtual MAC %s (got %v)", phase, virtual, got)
	}
	if sawPhysical != wantPhysical {
		t.Errorf("%s: physical MAC %s answered = %v, want %v (got %v)", phase, physical, sawPhysical, wantPhysical, got)
	}
}

// ownerAwaitUsable waits for addr on link to leave DAD, and fails if it is
// tentative after the bound or DAD declared it a duplicate.
func ownerAwaitUsable(t *testing.T, link netlink.Link, addr netip.Addr) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		flags, found := ownerAddrFlags(t, link, addr)
		if !found {
			t.Fatalf("%s holds no %s", link.Attrs().Name, addr)
		}
		if flags&unix.IFA_F_DADFAILED != 0 {
			t.Fatalf("%s: DAD failed for %s", link.Attrs().Name, addr)
		}
		if flags&unix.IFA_F_TENTATIVE == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: %s still tentative after 5s", link.Attrs().Name, addr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func ownerAddrFlags(t *testing.T, link netlink.Link, addr netip.Addr) (int, bool) {
	t.Helper()
	list, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if a.IP.Equal(net.IP(addr.AsSlice())) {
			return a.Flags, true
		}
	}
	return 0, false
}

func ownerSocket(t *testing.T, link netlink.Link, ethertype uint16) int {
	t.Helper()
	protocol := ethertype<<8 | ethertype>>8
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(protocol))
	if err != nil {
		t.Fatalf("packet socket: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Protocol: protocol}); err != nil {
		t.Fatal(err)
	}
	return fd
}

// ownerResolveARP broadcasts one who-has for vip from the peer and returns the
// sender hardware address of every ARP reply for vip seen in a bounded window.
func ownerResolveARP(t *testing.T, peer netlink.Link, fd int, vip netip.Addr) []net.HardwareAddr {
	t.Helper()
	source := peer.Attrs().HardwareAddr
	frame := make([]byte, 0, 42)
	frame = append(frame, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	frame = append(frame, source...)
	frame = append(frame, 0x08, 0x06, 0x00, 0x01, 0x08, 0x00, 6, 4, 0x00, 0x01)
	frame = append(frame, source...)
	frame = append(frame, 192, 0, 2, 20, 0, 0, 0, 0, 0, 0)
	frame = append(frame, vip.AsSlice()...)
	ownerSend(t, peer, fd, frame)

	var got []net.HardwareAddr
	for _, f := range ownerCapture(t, fd) {
		if len(f) < 42 || binary.BigEndian.Uint16(f[12:14]) != unix.ETH_P_ARP {
			continue
		}
		arp := f[14:]
		if binary.BigEndian.Uint16(arp[6:8]) != 2 || !bytes.Equal(arp[14:18], vip.AsSlice()) {
			continue
		}
		got = append(got, net.HardwareAddr(bytes.Clone(arp[8:14])))
	}
	return got
}

// ownerResolveND multicasts one Neighbor Solicitation for vip to its
// solicited-node group and returns the Target Link-Layer Address option of
// every Neighbor Advertisement for vip seen in a bounded window.
func ownerResolveND(t *testing.T, peer netlink.Link, fd int, vip netip.Addr) []net.HardwareAddr {
	t.Helper()
	target := vip.As16()
	source := netip.MustParseAddr("fe80::20").As16()
	group := [16]byte{0xff, 0x02, 10: 0, 11: 1, 12: 0xff, 13: target[13], 14: target[14], 15: target[15]}
	mac := peer.Attrs().HardwareAddr

	icmp := make([]byte, 0, 32)
	icmp = append(icmp, 135, 0, 0, 0, 0, 0, 0, 0)
	icmp = append(icmp, target[:]...)
	icmp = append(icmp, 1, 1)
	icmp = append(icmp, mac...)
	binary.BigEndian.PutUint16(icmp[2:4], ownerICMPv6Checksum(source, group, icmp))

	frame := make([]byte, 0, 14+40+len(icmp))
	frame = append(frame, 0x33, 0x33, group[12], group[13], group[14], group[15])
	frame = append(frame, mac...)
	frame = append(frame, 0x86, 0xdd, 0x60, 0, 0, 0, 0, byte(len(icmp)), 58, 255)
	frame = append(frame, source[:]...)
	frame = append(frame, group[:]...)
	frame = append(frame, icmp...)
	ownerSend(t, peer, fd, frame)

	var got []net.HardwareAddr
	for _, f := range ownerCapture(t, fd) {
		if len(f) < 14+40+24 || binary.BigEndian.Uint16(f[12:14]) != unix.ETH_P_IPV6 {
			continue
		}
		ip6 := f[14:]
		if ip6[6] != 58 {
			continue
		}
		na := ip6[40:]
		if na[0] != 136 || !bytes.Equal(na[8:24], target[:]) {
			continue
		}
		tlla, ok := ownerTargetLinkLayer(na[24:])
		if !ok {
			t.Fatalf("Neighbor Advertisement for %s carries no Target Link-Layer Address option: %x", vip, na)
		}
		got = append(got, tlla)
	}
	return got
}

// ownerTargetLinkLayer walks the ND options for option type 2 (RFC 4861
// Section 4.6.1). Each option's length is in units of 8 octets, and a zero
// length ends the walk, as RFC 4861 requires the receiver to discard it.
func ownerTargetLinkLayer(options []byte) (net.HardwareAddr, bool) {
	for len(options) >= 8 {
		size := int(options[1]) * 8
		if size == 0 || size > len(options) {
			return nil, false
		}
		if options[0] == 2 {
			return net.HardwareAddr(bytes.Clone(options[2:8])), true
		}
		options = options[size:]
	}
	return nil, false
}

func ownerICMPv6Checksum(source, target [16]byte, icmp []byte) uint16 {
	var sum uint32
	add := func(b []byte) {
		for i := 0; i+1 < len(b); i += 2 {
			sum += uint32(binary.BigEndian.Uint16(b[i:]))
		}
		if len(b)%2 == 1 {
			sum += uint32(b[len(b)-1]) << 8
		}
	}
	add(source[:])
	add(target[:])
	sum += uint32(len(icmp))
	sum += 58
	add(icmp)
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

func ownerSend(t *testing.T, link netlink.Link, fd int, frame []byte) {
	t.Helper()
	var dst [8]byte
	copy(dst[:], frame[:6])
	sa := &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Halen: 6, Addr: dst}
	if err := unix.Sendto(fd, frame, 0, sa); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// ownerCapture reads every frame the socket receives in a one-second window.
// The window is the whole bound: an absent answer is part of what is asserted,
// so the read cannot stop at the first frame.
func ownerCapture(t *testing.T, fd int) [][]byte {
	t.Helper()
	var frames [][]byte
	var buf [2048]byte
	poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := unix.Poll(poll, 50); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			t.Fatal(err)
		}
		for {
			n, from, err := unix.Recvfrom(fd, buf[:], 0)
			if errors.Is(err, unix.EAGAIN) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if sa, ok := from.(*unix.SockaddrLinklayer); ok && sa.Pkttype == unix.PACKET_OUTGOING {
				continue
			}
			frames = append(frames, bytes.Clone(buf[:n]))
		}
	}
	return frames
}
