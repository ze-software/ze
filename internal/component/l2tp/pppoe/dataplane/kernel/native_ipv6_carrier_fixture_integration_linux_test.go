//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original native IPv6 carrier authority
// Related: prepare.go -- the real prepared access owns every admitted child
// Source-only TDD input. Numeric PPP setup remains production's CURRENT API;
// this fixture does not implement or stand in for an original-resource ABI.
package kernel

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
)

const nativeV6Wait = 2 * time.Second

// All resources belong to userns.Enter's private child process. No worker is
// started here. The retained netlink handle also owns foreign-namespace cleanup.
type nativeV6Fixture struct {
	access   dataplane.Access
	route    *netlink.Handle
	endpoint *nativeV6Endpoint
}

type nativeV6Endpoint struct {
	fd        int
	peerIndex int
	sid       uint16
	localMAC  net.HardwareAddr
	peerMAC   net.HardwareAddr
}

func nativeV6Defaults(t *testing.T) {
	t.Helper()
	// These are the test's own network namespace defaults, set before any
	// owned device is created. No timer or fabricated DAD completion is used.
	for key, value := range map[string]string{"accept_dad": "0", "addr_gen_mode": "1", "router_solicitations": "0", "accept_ra": "0"} {
		path := "/proc/sys/net/ipv6/conf/default/" + key
		if err := os.WriteFile(path, []byte(value), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func newNativeV6Fixture(t *testing.T) *nativeV6Fixture {
	t.Helper()
	nativeV6Defaults(t)
	// This test deliberately has no SKIP route: the selected Ze native image
	// must provide PPP, PPPoE, IPv6, packet sockets and veth.
	probe, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("native IPv6 prerequisite /dev/ppp: %v", err)
	}
	if err := unix.Close(probe); err != nil {
		t.Fatal(err)
	}
	route, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(route.Close)
	local, endpoint := nativeV6Veth(t, route, "v6access", "v6peer", 0, 1)
	access, err := prepareWithResolver(t.Context(), "ipv6-access", func() (iface.Backend, error) {
		return iface.OpenBackend(Name)
	}, func(logical string) (iface.Binding, error) {
		if logical != "ipv6-access" {
			return iface.Binding{}, fmt.Errorf("unexpected logical access %q", logical)
		}
		actual, err := route.LinkByIndex(local.Attrs().Index)
		if err != nil {
			return iface.Binding{}, err
		}
		attrs := actual.Attrs()
		return iface.Binding{OsName: attrs.Name, Ifindex: attrs.Index, OperMAC: attrs.HardwareAddr.String(), MTU: attrs.MTU}, nil
	})
	if err != nil {
		t.Fatalf("prepare real IPv6 access: %v", err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	return &nativeV6Fixture{access: access, route: route, endpoint: endpoint}
}

func nativeV6Veth(t *testing.T, route *netlink.Handle, name, peerName string, index, slot int) (netlink.Link, *nativeV6Endpoint) {
	t.Helper()
	localMAC := net.HardwareAddr{2, 0, 6, 0, byte(slot), 1}
	peerMAC := net.HardwareAddr{2, 0, 6, 0, byte(slot), 2}
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name, Index: index, HardwareAddr: localMAC}, PeerName: peerName, PeerHardwareAddr: peerMAC}
	if err := route.LinkAdd(link); err != nil {
		t.Fatalf("create owned IPv6 wire %s: %v", name, err)
	}
	// Capture the actual object in its original namespace before cleanup is
	// published. No test renames or deletes these veth objects afterward.
	actual, err := route.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := route.LinkDel(actual); err != nil {
			t.Error(err)
		}
	})
	if index != 0 {
		if actual.Attrs().Index != index {
			t.Fatalf("native replacement index=%d, want exact retired index %d", actual.Attrs().Index, index)
		}
	}
	peer, err := route.LinkByName(peerName)
	if err != nil {
		t.Fatal(err)
	}
	nativeV6NoTXOffload(t, name)
	nativeV6NoTXOffload(t, peerName)
	if err := route.LinkSetUp(actual); err != nil {
		t.Fatal(err)
	}
	if err := route.LinkSetUp(peer); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, int(nativeV6EtherProtocol(0x0003)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: nativeV6EtherProtocol(0x0003), Ifindex: peer.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatal(err)
	}
	return actual, &nativeV6Endpoint{fd: fd, peerIndex: peer.Attrs().Index, localMAC: localMAC, peerMAC: peerMAC}
}

func nativeV6EtherProtocol(protocol uint16) uint16 {
	return binary.NativeEndian.Uint16([]byte{byte(protocol >> 8), byte(protocol)})
}

// Veth normally advertises HW_CSUM. Disable and read back only these owned
// endpoints' TX checksum features so AF_PACKET observes completed wire bytes,
// rather than mistaking a CHECKSUM_PARTIAL pseudoheader seed for a bad reply.
// v7.3 net/ethtool/ioctl.c ethtool_set_one_feature handles the legacy command;
// x/sys/unix ifreqData supplies the pointer-preserving layout mirrored here.
func nativeV6NoTXOffload(t *testing.T, name string) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	value := struct {
		command uint32
		data    uint32
	}{command: unix.ETHTOOL_STXCSUM}
	request := struct {
		name    [unix.IFNAMSIZ]byte
		data    unsafe.Pointer
		padding [unsafe.Sizeof(unix.Ifreq{}) - unix.IFNAMSIZ - unix.SizeofPtr]byte
	}{data: unsafe.Pointer(&value)}
	copy(request.name[:], name)
	for _, command := range []uint32{unix.ETHTOOL_STXCSUM, unix.ETHTOOL_GTXCSUM} {
		value.command = command
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SIOCETHTOOL, uintptr(unsafe.Pointer(&request))) //nolint:gosec // pointer-preserving native ifreq/ethtool_value layout.
		runtime.KeepAlive(&value)
		if errno != 0 {
			t.Fatalf("owned veth checksum feature command %#x: %v", command, errno)
		}
	}
	if value.data != 0 {
		t.Fatalf("owned veth %s still has TX checksum offload", name)
	}
}

func (f *nativeV6Fixture) open(t *testing.T, sid uint16) (dataplane.OpenedSession, *nativeV6Endpoint) {
	t.Helper()
	request := dataplane.Request{AccessName: "ipv6-access", Device: f.access.Device(), ControlDevice: f.access.ControlDevice(), Namespace: f.access.Namespace(), SessionID: sid, CaptureProxyLCP: true}
	copy(request.PeerMAC[:], f.endpoint.peerMAC)
	opened, err := f.access.Open(t.Context(), request)
	if err != nil {
		t.Fatalf("real prepared Access.Open IPv6 child: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
		if err := opened.CloseTransport(); err != nil {
			t.Error(err)
		}
	})
	handle := opened.Dataplane()
	identity := handle.Identity()
	link, err := f.route.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("prepared child kind=%q, want actual PPP", link.Type())
	}
	if link.Attrs().Name != identity.InterfaceName {
		t.Fatal("prepared child native identity does not match its actual original unit")
	}
	// Match production afterLCPOpen: admin-up precedes IPv6 installation.
	// The native multicast route add refuses a down PPP device, while the
	// later PPP NETDEV_UP path does not recreate it. Address readiness alone
	// therefore did not establish useful IPv6 multicast admission.
	if err := handle.SetAdminUp(); err != nil {
		t.Fatal(err)
	}
	if err := handle.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatalf("install actual session IPv6 pair: %v", err)
	}
	if err := handle.Connect(); err != nil {
		t.Fatalf("enable actual connected IPv6 family: %v", err)
	}
	addresses, err := f.route.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	for _, address := range addresses {
		if !address.IP.Equal(net.ParseIP("fe80::1")) {
			continue
		}
		if address.Flags&unix.IFA_F_TENTATIVE != 0 {
			t.Fatal("actual configured IPv6 source is still tentative")
		}
		ready = true
	}
	if !ready {
		t.Fatal("original session IPv6 source was not installed")
	}
	f.requireAdmission(t, link)
	endpoint := *f.endpoint
	endpoint.sid = sid
	return opened, &endpoint
}

// requireAdmission checks native state produced by the same public setup.
// It never supplies a route, changes a mode or accepts an error as readiness.
func (f *nativeV6Fixture) requireAdmission(t *testing.T, link netlink.Link) {
	t.Helper()
	actual, err := f.route.LinkByIndex(link.Attrs().Index)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("original PPP is not administratively up before IPv6 acquisition")
	}
	if actual.Attrs().Flags&net.FlagMulticast == 0 {
		t.Fatal("original PPP has no native multicast capability")
	}
	disabled, err := os.ReadFile("/proc/sys/net/ipv6/conf/" + actual.Attrs().Name + "/disable_ipv6")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(disabled)) != "0" {
		t.Fatalf("IPv6 is disabled on original PPP: %q", disabled)
	}
	filter := &netlink.Route{LinkIndex: actual.Attrs().Index, Table: unix.RT_TABLE_LOCAL}
	routes, err := f.route.RouteListFiltered(netlink.FAMILY_V6, filter, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		if route.Dst == nil {
			continue
		}
		if route.Dst.String() != "ff00::/8" {
			continue
		}
		if route.Type != unix.RTN_MULTICAST {
			continue
		}
		return
	}
	t.Fatalf("original PPP has no native ff00::/8 multicast admission route after admin-up/address installation: %+v", routes)
}

func nativeV6Acquire(t *testing.T, opened dataplane.OpenedSession) ppp.IPv6Carrier {
	t.Helper()
	carrier, err := opened.Dataplane().OpenIPv6Carrier(t.Context())
	if carrier != nil {
		t.Cleanup(func() {
			if err := carrier.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatalf("acquire IPv6 carrier from original admitted owner: %v", err)
	}
	if carrier == nil {
		t.Fatal("original admitted owner returned no IPv6 carrier")
	}
	return carrier
}

// The checksum vector is independent of the builder and has a literal ICMPv6
// checksum 0x7d35. Neither injected input is encoded by the production carrier.
func nativeV6CheckVector(t *testing.T) {
	t.Helper()
	want, err := hex.DecodeString("6000000000083afffe800000000000000000000000000002ff02000000000000000000000000000285007d3500000000")
	if err != nil {
		t.Fatal(err)
	}
	if got := nativeV6RS(); !bytes.Equal(got, want) {
		t.Fatalf("test IPv6/ICMPv6 builder differs from literal vector: %x", got)
	}
	wantUDP, err := hex.DecodeString("60000000000c1101fe800000000000000000000000000002ff02000000000000000000000001000202220223000cfd0701000001")
	if err != nil {
		t.Fatal(err)
	}
	source, destination := netip.MustParseAddr("fe80::2"), netip.MustParseAddr("ff02::1:2")
	body := []byte{2, 0x22, 2, 0x23, 0, 12, 0, 0, 1, 0, 0, 1}
	binary.BigEndian.PutUint16(body[6:8], nativeV6Checksum(source, destination, 17, body))
	if got := nativeV6Packet(source, destination, 17, 1, body); !bytes.Equal(got, wantUDP) {
		t.Fatalf("test UDPv6 builder differs from literal vector: %x", got)
	}
}

func nativeV6Packet(source, destination netip.Addr, next, hop byte, body []byte) []byte {
	packet := make([]byte, 40+len(body))
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	packet[6], packet[7] = next, hop
	src, dst := source.As16(), destination.As16()
	copy(packet[8:24], src[:])
	copy(packet[24:40], dst[:])
	copy(packet[40:], body)
	return packet
}

func nativeV6Checksum(source, destination netip.Addr, next byte, body []byte) uint16 {
	src, dst := source.As16(), destination.As16()
	sum := uint32(len(body)) + uint32(next)
	for offset := 0; offset < 16; offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(src[offset:]))
		sum += uint32(binary.BigEndian.Uint16(dst[offset:]))
	}
	for offset := 0; offset+1 < len(body); offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(body[offset:]))
	}
	if len(body)%2 != 0 {
		sum += uint32(body[len(body)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func nativeV6RS() []byte {
	source, destination := netip.MustParseAddr("fe80::2"), netip.MustParseAddr("ff02::2")
	body := []byte{133, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(body[2:4], nativeV6Checksum(source, destination, 58, body))
	return nativeV6Packet(source, destination, 58, 255, body)
}

func nativeV6DHCP(message, marker byte) []byte {
	// A client-id option with a DUID-LL Ethernet address follows the XID.
	payload := []byte{message, 0, 0, marker, 0, 1, 0, 10, 0, 3, 0, 1, 2, 0, 6, 0, 0, 2}
	if message == 1 {
		return append(payload, 0, 8, 0, 2, 0, 0)
	} // Elapsed Time.
	// The reply has its own Server Identifier. This transport test does not
	// assert DHCP allocation, option policy or the separate server state machine.
	return append(payload, 0, 2, 0, 10, 0, 3, 0, 1, 2, 0, 6, 0, 0, 1)
}

func nativeV6Request(marker byte) []byte {
	source, destination := netip.MustParseAddr("fe80::2"), netip.MustParseAddr("ff02::1:2")
	payload := nativeV6DHCP(1, marker)
	body := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint16(body[0:2], 546)
	binary.BigEndian.PutUint16(body[2:4], 547)
	binary.BigEndian.PutUint16(body[4:6], uint16(len(body)))
	copy(body[8:], payload)
	checksum := nativeV6Checksum(source, destination, 17, body)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(body[6:8], checksum)
	return nativeV6Packet(source, destination, 17, 1, body)
}

func nativeV6RA(marker byte) []byte {
	return []byte{134, 0, 0, 0, 64, 0, 0, marker, 0, 0, 0, 0, 0, 0, 0, 0}
}

func (e *nativeV6Endpoint) inject(t *testing.T, packet []byte) {
	t.Helper()
	header := 14
	etherType := uint16(0x86dd)
	if e.sid != 0 {
		header = 22
		etherType = 0x8864
	}
	frame := make([]byte, header+len(packet))
	copy(frame[:6], e.localMAC)
	copy(frame[6:12], e.peerMAC)
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	if e.sid != 0 {
		frame[14] = 0x11
		binary.BigEndian.PutUint16(frame[16:18], e.sid)
		binary.BigEndian.PutUint16(frame[18:20], uint16(2+len(packet)))
		frame[20], frame[21] = 0, 0x57
	} else if packet[24] == 0xff {
		copy(frame[:6], []byte{0x33, 0x33, packet[36], packet[37], packet[38], packet[39]})
	}
	copy(frame[header:], packet)
	if err := unix.Sendto(e.fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: e.peerIndex, Protocol: nativeV6EtherProtocol(etherType), Halen: 6}); err != nil {
		t.Fatal(err)
	}
}

type nativeV6Observed struct {
	source      netip.Addr
	destination netip.Addr
	next        byte
	hop         byte
	body        []byte
}

func (e *nativeV6Endpoint) receive(t *testing.T, deadline time.Time) nativeV6Observed {
	t.Helper()
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("useful native IPv6 output never reached the actual peer wire")
		}
		fds := []unix.PollFd{{Fd: int32(e.fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, int((remaining+time.Millisecond-1)/time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if fds[0].Revents&unix.POLLIN == 0 {
			t.Fatalf("actual IPv6 peer wire not readable: events %#x", fds[0].Revents)
		}
		var buffer [2048]byte
		n, _, err := unix.Recvfrom(e.fd, buffer[:], 0)
		if err != nil {
			t.Fatal(err)
		}
		frame := buffer[:n]
		if len(frame) < 14 {
			continue
		}
		offset := 14
		if e.sid != 0 {
			if binary.BigEndian.Uint16(frame[12:14]) != 0x8864 {
				continue
			}
			if len(frame) < 22 {
				t.Fatal("truncated native PPPoE output")
			}
			if binary.BigEndian.Uint16(frame[16:18]) != e.sid {
				continue
			}
			if !bytes.Equal(frame[:6], e.peerMAC) {
				t.Fatal("original PPP output used another subscriber MAC")
			}
			if !bytes.Equal(frame[6:12], e.localMAC) {
				t.Fatal("original PPP output used another access MAC")
			}
			if !bytes.Equal(frame[14:16], []byte{0x11, 0}) {
				t.Fatal("invalid native PPPoE output header")
			}
			length := int(binary.BigEndian.Uint16(frame[18:20]))
			if length < 2 {
				t.Fatal("invalid native PPPoE output length")
			}
			if len(frame) < 20+length {
				t.Fatal("truncated native PPPoE output payload")
			}
			frame = frame[:20+length]
			if !bytes.Equal(frame[20:22], []byte{0, 0x57}) {
				continue
			}
			offset = 22
		} else {
			if binary.BigEndian.Uint16(frame[12:14]) != 0x86dd {
				continue
			}
			if !bytes.Equal(frame[6:12], e.localMAC) {
				t.Fatal("replacement control used another original device")
			}
		}
		packet := frame[offset:]
		if len(packet) < 40 {
			t.Fatal("truncated actual IPv6 output")
		}
		if packet[0]>>4 != 6 {
			t.Fatal("native wire did not carry IPv6")
		}
		length := int(binary.BigEndian.Uint16(packet[4:6]))
		if len(packet) < 40+length {
			t.Fatal("truncated actual IPv6 payload")
		}
		var source, destination [16]byte
		copy(source[:], packet[8:24])
		copy(destination[:], packet[24:40])
		return nativeV6Observed{source: netip.AddrFrom16(source), destination: netip.AddrFrom16(destination), next: packet[6], hop: packet[7], body: bytes.Clone(packet[40 : 40+length])}
	}
}

func nativeV6MatchRA(packet nativeV6Observed, marker byte) bool {
	if packet.next != 58 {
		return false
	}
	if len(packet.body) != 16 {
		return false
	}
	if packet.body[0] != 134 {
		return false
	}
	return binary.BigEndian.Uint16(packet.body[6:8]) == uint16(marker)
}

func nativeV6MatchDHCP(packet nativeV6Observed, marker byte) bool {
	if packet.next != 17 {
		return false
	}
	if len(packet.body) < 12 {
		return false
	}
	if binary.BigEndian.Uint16(packet.body[:2]) != 547 {
		return false
	}
	if binary.BigEndian.Uint16(packet.body[2:4]) != 546 {
		return false
	}
	return packet.body[11] == marker
}

func (e *nativeV6Endpoint) requireOutput(t *testing.T, next byte, marker byte, forbidden byte) {
	t.Helper()
	deadline := time.Now().Add(nativeV6Wait)
	for {
		packet := e.receive(t, deadline)
		match := nativeV6MatchRA
		if next == 17 {
			match = nativeV6MatchDHCP
		}
		if forbidden != 0 {
			if match(packet, forbidden) {
				t.Fatalf("retired original carrier emitted marker %d onto a replacement's actual wire", forbidden)
			}
		}
		if !match(packet, marker) {
			continue
		}
		if packet.source != netip.MustParseAddr("fe80::1") {
			t.Fatalf("native IPv6 source=%s, want configured original source", packet.source)
		}
		if nativeV6Checksum(packet.source, packet.destination, packet.next, packet.body) != 0 {
			t.Fatal("native reply checksum is invalid on actual wire")
		}
		if next == 58 {
			if packet.hop != 255 {
				t.Fatalf("native RA hop limit=%d, want 255", packet.hop)
			}
			if packet.destination != netip.MustParseAddr("ff02::1") {
				t.Fatalf("native RA destination=%s", packet.destination)
			}
			body := bytes.Clone(packet.body)
			body[2], body[3] = 0, 0
			if !bytes.Equal(body, nativeV6RA(marker)) {
				t.Fatalf("native RA payload changed: %x", packet.body)
			}
		} else {
			if packet.destination != netip.MustParseAddr("fe80::2") {
				t.Fatalf("native DHCPv6 destination=%s", packet.destination)
			}
			if binary.BigEndian.Uint16(packet.body[4:6]) != uint16(len(packet.body)) {
				t.Fatal("native DHCPv6 UDP length changed")
			}
			if !bytes.Equal(packet.body[8:], nativeV6DHCP(7, marker)) {
				t.Fatalf("native DHCPv6 payload changed: %x", packet.body[8:])
			}
		}
		return
	}
}

func nativeV6ReadRS(t *testing.T, carrier ppp.IPv6Carrier, endpoint *nativeV6Endpoint) {
	t.Helper()
	endpoint.inject(t, nativeV6RS())
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	if err := carrier.ReadRS(ctx); err != nil {
		t.Fatalf("useful original RS was not admitted from its actual PPP peer: %v", err)
	}
}

func nativeV6ReadDHCP(t *testing.T, carrier ppp.IPv6Carrier, endpoint *nativeV6Endpoint, marker byte) netip.AddrPort {
	t.Helper()
	endpoint.inject(t, nativeV6Request(marker))
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	var buffer [128]byte
	n, peer, err := carrier.ReadDHCPv6(ctx, buffer[:])
	if err != nil {
		t.Fatalf("useful original DHCPv6 request was not admitted from its actual PPP peer: %v", err)
	}
	if !bytes.Equal(buffer[:n], nativeV6DHCP(1, marker)) {
		t.Fatalf("original DHCPv6 request changed: %x", buffer[:n])
	}
	if peer.Addr().WithZone("") != netip.MustParseAddr("fe80::2") {
		t.Fatalf("actual DHCPv6 peer=%s", peer)
	}
	if peer.Port() != 546 {
		t.Fatalf("actual DHCPv6 peer port=%d", peer.Port())
	}
	return peer
}

func nativeV6Useful(t *testing.T, carrier ppp.IPv6Carrier, endpoint *nativeV6Endpoint, marker byte) {
	t.Helper()
	nativeV6CheckVector(t)
	nativeV6ReadRS(t, carrier, endpoint)
	peer := nativeV6ReadDHCP(t, carrier, endpoint, marker)
	if err := carrier.WriteRA(nativeV6RA(marker), 255); err != nil {
		t.Fatalf("useful original RA transmission: %v", err)
	}
	endpoint.requireOutput(t, 58, marker, 0)
	payload := nativeV6DHCP(7, marker)
	n, err := carrier.WriteDHCPv6(payload, peer)
	if err != nil {
		t.Fatalf("useful original DHCPv6 reply: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("native DHCPv6 short reply=%d", n)
	}
	endpoint.requireOutput(t, 17, marker, 0)
}

// Both one-shot workers report entry and their actual terminal read results.
// Entry is not represented as proof that a kernel syscall was already blocked.
// The watchdog is only a failure bound; elapsed time never supplies a join.
func nativeV6CancelReads(t *testing.T, carrier ppp.IPv6Carrier) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered := make(chan struct{}, 2)
	finished := make(chan error, 2)
	go func() {
		entered <- struct{}{}
		finished <- carrier.ReadRS(ctx)
	}()
	go func() {
		var buffer [128]byte
		entered <- struct{}{}
		_, _, err := carrier.ReadDHCPv6(ctx, buffer[:])
		finished <- err
	}()
	watchdog := time.NewTimer(nativeV6Wait)
	defer watchdog.Stop()
	for range 2 {
		select {
		case <-entered:
		case <-watchdog.C:
			t.Fatal("original native read worker did not enter")
		}
	}
	cancel()
	for range 2 {
		select {
		case err := <-finished:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("original native read cancellation=%v", err)
			}
		case <-watchdog.C:
			t.Fatal("original native read worker did not return its terminal result")
		}
	}
}

// A replacement listener is independently constructed from native sockets on
// its own real veth. It never supplies a result to the production owner.
type nativeV6Replacement struct {
	link     netlink.Link
	endpoint *nativeV6Endpoint
	ra       *ipv6.PacketConn
	dhcp     net.PacketConn
}

func nativeV6ReplacementNew(t *testing.T, route *netlink.Handle, name string, index int) *nativeV6Replacement {
	t.Helper()
	link, endpoint := nativeV6Veth(t, route, name, "v6replacement", index, 2)
	address, err := netlink.ParseAddr("fe80::1/64")
	if err != nil {
		t.Fatal(err)
	}
	address.Flags = unix.IFA_F_NODAD
	if err := route.AddrAdd(link, address); err != nil {
		t.Fatal(err)
	}
	// The independent Ethernet control uses a real permanent neighbor to send
	// unicast replies. Original PPP traffic uses no Ethernet/NDP shortcut.
	if err := route.NeighSet(&netlink.Neigh{LinkIndex: link.Attrs().Index, IP: net.ParseIP("fe80::2"), HardwareAddr: endpoint.peerMAC, State: netlink.NUD_PERMANENT}); err != nil {
		t.Fatal(err)
	}
	config := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var optionErr error
		err := raw.Control(func(fd uintptr) {
			optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			if optionErr != nil {
				return
			}
			optionErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, name)
		})
		if err != nil {
			return err
		}
		return optionErr
	}}
	raConn, err := config.ListenPacket(t.Context(), "ip6:ipv6-icmp", "::")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := raConn.Close(); err != nil {
			t.Error(err)
		}
	})
	ra := ipv6.NewPacketConn(raConn)
	device := &net.Interface{Index: link.Attrs().Index, Name: name}
	if err := ra.JoinGroup(device, &net.IPAddr{IP: net.ParseIP("ff02::2")}); err != nil {
		t.Fatal(err)
	}
	if err := ra.SetControlMessage(ipv6.FlagHopLimit|ipv6.FlagInterface, true); err != nil {
		t.Fatal(err)
	}
	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	if err := ra.SetICMPFilter(&filter); err != nil {
		t.Fatal(err)
	}
	dhcp, err := config.ListenPacket(t.Context(), "udp6", "[::]:547")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dhcp.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := ipv6.NewPacketConn(dhcp).JoinGroup(device, &net.IPAddr{IP: net.ParseIP("ff02::1:2")}); err != nil {
		t.Fatal(err)
	}
	return &nativeV6Replacement{link: link, endpoint: endpoint, ra: ra, dhcp: dhcp}
}

func (r *nativeV6Replacement) receiveRS(t *testing.T) {
	t.Helper()
	if err := r.ra.SetReadDeadline(time.Now().Add(nativeV6Wait)); err != nil {
		t.Fatal(err)
	}
	var buffer [128]byte
	n, control, _, err := r.ra.ReadFrom(buffer[:])
	if err != nil {
		t.Fatalf("independent replacement listener lost useful RS: %v", err)
	}
	if n != 8 {
		t.Fatalf("replacement RS length=%d", n)
	}
	if buffer[0] != 133 {
		t.Fatal("replacement listener received another ICMP message")
	}
	if control == nil {
		t.Fatal("replacement listener lost actual ingress metadata")
	}
	if control.IfIndex != r.link.Attrs().Index {
		t.Fatal("replacement positive control arrived on another native interface")
	}
	if control.HopLimit != 255 {
		t.Fatal("replacement positive control lost hop limit")
	}
}

func (r *nativeV6Replacement) receiveDHCP(t *testing.T, marker byte) {
	t.Helper()
	if err := r.dhcp.SetReadDeadline(time.Now().Add(nativeV6Wait)); err != nil {
		t.Fatal(err)
	}
	var buffer [128]byte
	n, address, err := r.dhcp.ReadFrom(buffer[:])
	if err != nil {
		t.Fatalf("independent replacement listener lost useful DHCPv6: %v", err)
	}
	if !bytes.Equal(buffer[:n], nativeV6DHCP(1, marker)) {
		t.Fatalf("replacement listener received another request: %x", buffer[:n])
	}
	peer, ok := address.(*net.UDPAddr)
	if !ok {
		t.Fatalf("replacement UDP source=%T", address)
	}
	if !peer.IP.Equal(net.ParseIP("fe80::2")) {
		t.Fatalf("replacement source=%s", peer)
	}
	if peer.Port != 546 {
		t.Fatalf("replacement source port=%d", peer.Port)
	}
}

func (r *nativeV6Replacement) useful(t *testing.T, marker byte) {
	t.Helper()
	r.endpoint.inject(t, nativeV6RS())
	r.receiveRS(t)
	r.endpoint.inject(t, nativeV6Request(marker))
	r.receiveDHCP(t, marker)
	r.writeRA(t, marker)
	r.endpoint.requireOutput(t, 58, marker, 0)
	r.writeDHCP(t, marker)
	r.endpoint.requireOutput(t, 17, marker, 0)
}

func (r *nativeV6Replacement) writeRA(t *testing.T, marker byte) {
	t.Helper()
	payload := nativeV6RA(marker)
	n, err := r.ra.WriteTo(payload, &ipv6.ControlMessage{IfIndex: r.link.Attrs().Index, HopLimit: 255}, &net.IPAddr{IP: net.ParseIP("ff02::1"), Zone: strconv.Itoa(r.link.Attrs().Index)})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatal("replacement control RA was short")
	}
}

func (r *nativeV6Replacement) writeDHCP(t *testing.T, marker byte) {
	t.Helper()
	payload := nativeV6DHCP(7, marker)
	n, err := r.dhcp.WriteTo(payload, &net.UDPAddr{IP: net.ParseIP("fe80::2"), Port: 546, Zone: strconv.Itoa(r.link.Attrs().Index)})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatal("replacement control DHCPv6 reply was short")
	}
}

func (f *nativeV6Fixture) rename(t *testing.T, opened dataplane.OpenedSession) *nativeV6Replacement {
	t.Helper()
	identity := opened.Dataplane().Identity()
	link, err := f.route.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.route.LinkSetName(link, "v6original"); err != nil {
		t.Fatal(err)
	}
	return nativeV6ReplacementNew(t, f.route, identity.InterfaceName, 0)
}

func (f *nativeV6Fixture) retire(t *testing.T, opened dataplane.OpenedSession) *nativeV6Replacement {
	t.Helper()
	identity := opened.Dataplane().Identity()
	link, err := f.route.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.route.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	return nativeV6ReplacementNew(t, f.route, identity.InterfaceName, int(identity.NativeIndex))
}

func nativeV6GroupUsers(t *testing.T, index int) map[string]int {
	t.Helper()
	data, err := os.ReadFile("/proc/net/igmp6")
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		actual, err := strconv.Atoi(fields[0])
		if err != nil {
			t.Fatal(err)
		}
		if actual != index {
			continue
		}
		users, err := strconv.Atoi(fields[3])
		if err != nil {
			t.Fatal(err)
		}
		groups[fields[2]] = users
	}
	for _, group := range []string{"ff020000000000000000000000000002", "ff020000000000000000000000010002"} {
		if groups[group] <= 0 {
			t.Fatalf("replacement has no independently held native membership for %s: %v", group, groups)
		}
	}
	return groups
}

func nativeV6Foreign(t *testing.T, work func(*netlink.Handle)) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	}()
	other, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	}()
	defer func() {
		if err := netns.Set(original); err != nil {
			t.Fatalf("restore original IPv6 fixture namespace: %v", err)
		}
		runtime.UnlockOSThread()
	}()
	nativeV6Defaults(t)
	route, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(route.Close)
	work(route)
}
