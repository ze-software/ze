//go:build integration && linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- production raw RSVP carriage.
//
// Two isolated namespaces share two veth links. The normal endpoint route uses
// the second link; the explicit hop uses the first. AF_PACKET captures inspect
// the frames before the peer's production transport diverts Router Alert traffic.
// These tests require CAP_SYS_ADMIN, CAP_NET_ADMIN and CAP_NET_RAW. The bypass
// test also requires Linux MPLS IP encapsulation. No fake transport is involved.
package rsvpte

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const rsvpWireWait = 3 * time.Second

var (
	rsvpWireSource   = netip.MustParseAddr("198.18.0.77")
	rsvpWireEndpoint = netip.MustParseAddr("203.0.113.9")
	rsvpWireMerge    = netip.MustParseAddr("203.0.113.7")
)

type rsvpWireLink struct {
	router     netlink.Link
	peer       netlink.Link
	routerAddr netip.Addr
	peerAddr   netip.Addr
	capture    int
}

type rsvpWireLab struct {
	routes   *netlink.Handle
	sender   Transport
	receiver Transport
	links    [2]rsvpWireLink
}

// newRSVPWireLab returns to the original namespace before any Send call. The
// transport's route-query socket must retain the namespace where it was opened.
func newRSVPWireLab(t *testing.T) *rsvpWireLab {
	t.Helper()
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("restore original namespace: %v", err)
			// A thread in the wrong namespace MUST NOT reenter the Go pool.
			return
		}
		if err := origin.Close(); err != nil {
			t.Errorf("close original namespace handle: %v", err)
		}
		runtime.UnlockOSThread()
	})
	routerNS := rsvpWireNamespace(t)
	peerNS := rsvpWireNamespace(t)
	router := rsvpWireNetlink(t, routerNS)
	peer := rsvpWireNetlink(t, peerNS)
	lab := &rsvpWireLab{routes: router}

	for i := range lab.links {
		name, peerName := fmt.Sprintf("rsvp%d", i), fmt.Sprintf("peer%d", i)
		veth := &netlink.Veth{
			Name: name, MTU: 9000,
			PeerName:      peerName,
			PeerNamespace: netlink.NsFd(peerNS),
			PeerMTU:       9000,
		}
		if err := router.LinkAdd(veth); err != nil {
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EOPNOTSUPP) {
				t.Skipf("veth creation unavailable: %v", err)
			}
			t.Fatal(err)
		}
		lab.links[i].router = rsvpWireFindLink(t, router, name)
		lab.links[i].peer = rsvpWireFindLink(t, peer, peerName)
		lab.links[i].routerAddr = netip.AddrFrom4([4]byte{192, 0, byte(2 + i), 1})
		lab.links[i].peerAddr = netip.AddrFrom4([4]byte{192, 0, byte(2 + i), 2})
		rsvpWireAddress(t, router, lab.links[i].router, lab.links[i].routerAddr)
		rsvpWireAddress(t, peer, lab.links[i].peer, lab.links[i].peerAddr)
		rsvpWireNeighbor(t, router, lab.links[i].router, lab.links[i].peerAddr, lab.links[i].peer.Attrs().HardwareAddr)
		rsvpWireNeighbor(t, peer, lab.links[i].peer, lab.links[i].routerAddr, lab.links[i].router.Attrs().HardwareAddr)
	}
	// Both namespaces route the endpoint via link 1. A PATH sent via link 0
	// reaches a transit router with a usable forwarding route, not a local IP.
	for _, destination := range []netip.Addr{rsvpWireEndpoint, rsvpWireMerge} {
		rsvpWireRoute(t, router, &netlink.Route{
			Dst: rsvpWirePrefix(destination), LinkIndex: lab.links[1].router.Attrs().Index,
			Gw: lab.links[1].peerAddr.AsSlice(),
		})
		rsvpWireRoute(t, peer, &netlink.Route{
			Dst: rsvpWirePrefix(destination), LinkIndex: lab.links[1].peer.Attrs().Index,
			Gw: lab.links[1].routerAddr.AsSlice(),
		})
	}
	rsvpWireRoute(t, peer, &netlink.Route{
		Dst: rsvpWirePrefix(rsvpWireSource), LinkIndex: lab.links[0].peer.Attrs().Index,
		Gw: lab.links[0].routerAddr.AsSlice(),
	})

	rsvpWireEnter(t, routerNS)
	lab.sender = rsvpWireTransport(t, lab.links[0].routerAddr)
	rsvpWireEnter(t, peerNS)
	// Linux calls ip_call_ra_chain from ip_forward. Router Alert interception
	// requires forwarding, a route, and an ingress accepted by source validation.
	for _, setting := range []struct{ path, value string }{
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/peer0/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/peer1/rp_filter", "0"},
	} {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			t.Fatalf("set %s: %v", setting.path, err)
		}
	}
	lab.receiver = rsvpWireTransport(t, lab.links[0].peerAddr)
	for i := range lab.links {
		lab.links[i].capture = rsvpWireCapture(t, lab.links[i].peer.Attrs().Index)
	}
	rsvpWireEnter(t, origin)
	return lab
}

func rsvpWireNamespace(t *testing.T) netns.NsHandle {
	t.Helper()
	ns, err := netns.New()
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("network namespaces require CAP_SYS_ADMIN: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ns.Close(); err != nil {
			t.Errorf("close namespace handle: %v", err)
		}
	})
	return ns
}

func rsvpWireNetlink(t *testing.T, ns netns.NsHandle) *netlink.Handle {
	t.Helper()
	h, err := netlink.NewHandleAt(ns, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	lo := rsvpWireFindLink(t, h, "lo")
	if err := h.LinkSetUp(lo); err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skipf("link configuration requires CAP_NET_ADMIN: %v", err)
		}
		t.Fatal(err)
	}
	return h
}

func rsvpWireFindLink(t *testing.T, h *netlink.Handle, name string) netlink.Link {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func rsvpWireAddress(t *testing.T, h *netlink.Handle, link netlink.Link, addr netip.Addr) {
	t.Helper()
	if err := h.AddrAdd(link, &netlink.Addr{IPNet: &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(24, 32)}}); err != nil {
		t.Fatal(err)
	}
	if err := h.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
}

func rsvpWireNeighbor(t *testing.T, h *netlink.Handle, link netlink.Link, addr netip.Addr, mac net.HardwareAddr) {
	t.Helper()
	if err := h.NeighSet(&netlink.Neigh{
		LinkIndex: link.Attrs().Index, Family: unix.AF_INET, State: unix.NUD_PERMANENT,
		IP: addr.AsSlice(), HardwareAddr: mac,
	}); err != nil {
		t.Fatal(err)
	}
}

func rsvpWirePrefix(addr netip.Addr) *net.IPNet {
	return &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(32, 32)}
}

func rsvpWireRoute(t *testing.T, h *netlink.Handle, route *netlink.Route) {
	t.Helper()
	if err := h.RouteAdd(route); err != nil {
		t.Fatalf("add route %v: %v", route, err)
	}
}

func rsvpWireEnter(t *testing.T, ns netns.NsHandle) {
	t.Helper()
	if err := netns.Set(ns); err != nil {
		t.Fatal(err)
	}
}

func rsvpWireTransport(t *testing.T, local netip.Addr) Transport {
	t.Helper()
	tr, err := openRawTransport(local)
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("raw RSVP transport requires CAP_NET_RAW: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tr.Close(); err != nil {
			t.Errorf("close transport: %v", err)
		}
	})
	return tr
}

func rsvpWireCapture(t *testing.T, ifindex int) int {
	t.Helper()
	protocol := int(uint16(unix.ETH_P_ALL)<<8 | uint16(unix.ETH_P_ALL)>>8)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close capture: %v", err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: ifindex, Protocol: uint16(protocol)}); err != nil {
		t.Fatal(err)
	}
	return fd
}

// rsvpWireFrame waits for one inbound RSVP frame, including MPLS-carried RSVP.
// The deadline bounds unrelated traffic as well as a socket that receives none.
func rsvpWireFrame(t *testing.T, fd int, wait time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(wait)
	var buf [9216]byte
	for time.Now().Before(deadline) {
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		remaining := time.Until(deadline)
		n, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return nil
		}
		n, from, err := unix.Recvfrom(fd, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		fromLink, ok := from.(*unix.SockaddrLinklayer)
		if !ok || fromLink.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		_, ip := rsvpWireIP(buf[:n])
		if len(ip) >= 20 && ip[9] == rsvpProtocol {
			return bytes.Clone(buf[:n])
		}
	}
	return nil
}

func rsvpWireIP(frame []byte) ([]uint32, []byte) {
	if len(frame) < 14 {
		return nil, nil
	}
	kind := binary.BigEndian.Uint16(frame[12:14])
	offset := 14
	var labels []uint32
	if kind == unix.ETH_P_MPLS_UC {
		// Each iteration consumes a complete four-byte label from the frame.
		for offset+4 <= len(frame) {
			label := binary.BigEndian.Uint32(frame[offset : offset+4])
			labels = append(labels, label>>12)
			offset += 4
			if label&0x100 != 0 {
				break
			}
		}
	} else if kind != unix.ETH_P_IP {
		return nil, nil
	}
	return labels, frame[offset:]
}

func rsvpWireAssertIP(t *testing.T, frame []byte, source, destination netip.Addr, payload []byte, alert bool) {
	t.Helper()
	_, ip := rsvpWireIP(frame)
	if len(ip) < 20 {
		t.Fatalf("no IPv4 frame received: %x", frame)
	}
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || ihl > len(ip) {
		t.Fatalf("invalid IPv4 header length: %x", ip)
	}
	if got := netip.AddrFrom4([4]byte(ip[12:16])); got != source {
		t.Fatalf("IP source %s, want %s", got, source)
	}
	if got := netip.AddrFrom4([4]byte(ip[16:20])); got != destination {
		t.Fatalf("IP destination %s, want %s", got, destination)
	}
	if alert {
		if !bytes.Equal(ip[20:ihl], []byte{0x94, 4, 0, 0}) {
			t.Fatalf("IPv4 options %x, want Router Alert 94040000", ip[20:ihl])
		}
		if ip[8] != payload[4] {
			t.Fatalf("IP TTL %d, want RSVP Send_TTL %d", ip[8], payload[4])
		}
	} else if ihl != 20 {
		t.Fatalf("ordinary reply inherited IP options: %x", ip[20:ihl])
	}
	if ip[0]>>4 != 4 || ip[9] != rsvpProtocol || internetChecksum(ip[:ihl]) != 0 {
		t.Fatalf("invalid RSVP IPv4 header: %x", ip[:ihl])
	}
	total := int(binary.BigEndian.Uint16(ip[2:4]))
	if total != ihl+len(payload) || total > len(ip) {
		t.Fatalf("IP datagram length %d, want %d (captured %d)", total, ihl+len(payload), len(ip))
	}
	if !bytes.Equal(ip[ihl:total], payload) {
		t.Fatalf("RSVP payload changed: %x, want %x", ip[ihl:total], payload)
	}
}

func rsvpWireReceive(t *testing.T, tr Transport, payload []byte) Packet {
	t.Helper()
	timer := time.NewTimer(rsvpWireWait)
	defer timer.Stop()
	select {
	case packet, ok := <-tr.Recv():
		if !ok {
			t.Fatal("receive channel closed before packet")
		}
		if !bytes.Equal(packet.Payload, payload) {
			t.Fatalf("received payload %x, want %x", packet.Payload, payload)
		}
		return packet
	case <-timer.C:
		t.Fatal("RSVP packet did not reach the production receiver")
		return Packet{}
	}
}

func rsvpWirePSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: rsvpWireEndpoint, TunnelID: 7, ExtTunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: rsvpWireSource, LSPID: 9},
		SenderTSpec:    FlowSpec{TokenRate: 1000, TokenBucket: 1000, PeakRate: 1000},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
}

// TestIntegrationRSVPPathCarriage checks PATH, PathTear and ResvConf on the wire
// and at a transit receiver whose local address differs from the destination.
// RFC requirement: RFC2205-x-1 positive -- production PATH carriage includes the IPv4 Router Alert option on the captured frame.
// RFC requirement: RFC3209-x-1 positive -- an explicitly routed PATH retains Router Alert while the kernel sends it to the selected next hop.
// RFC requirement: RFC2205-3-37 positive -- native route queries send PATH and PathTear toward the explicit hop rather than the endpoint's ordinary route.
// RFC requirement: RFC2205-3-40 positive -- the transit receiver observes the actual incoming interface, original IP source and IP TTL.
// RFC requirement: RFC2205-3-41 positive -- native carriage selects the explicit hop's link while preserving an IP destination routed through the other link.
// RFC requirement: RFC2205-3-42 positive -- the captured PATH carries the requested sender source and Send_TTL in its IPv4 header.
func TestIntegrationRSVPPathCarriage(t *testing.T) {
	lab := newRSVPWireLab(t)
	local, err := lab.sender.LocalAddresses()
	if err != nil {
		t.Fatal(err)
	}
	remote, err := lab.receiver.LocalAddresses()
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range lab.links {
		has := func(addresses []InterfaceAddress, address netip.Addr) bool {
			return slices.ContainsFunc(addresses, func(a InterfaceAddress) bool { return a.Address == address })
		}
		if !has(local, link.routerAddr) || has(local, link.peerAddr) ||
			!has(remote, link.peerAddr) || has(remote, link.routerAddr) {
			t.Fatalf("address inventory crossed transport namespaces: sender=%v receiver=%v", local, remote)
		}
	}
	psb := rsvpWirePSB()
	// RFC 2205 Sections 3.1.9 and A.14: ResvConf type 7 carries SESSION,
	// ERROR_SPEC, RESV_CONFIRM, STYLE and a flow descriptor list.
	confirmation := encodeMessage(7, 41, []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeErrorSpec(b, errorSpec{ErrorNode: lab.links[0].routerAddr}) },
		func(b []byte) int {
			encodeObjectHeader(b, objectHeader{Length: 8, ClassNum: ClassResvConfirm, CType: CTypeIPv4})
			addr := rsvpWireEndpoint.As4()
			copy(b[4:8], addr[:])
			return 8
		},
		func(b []byte) int { return encodeStyle(b, StyleFixedFilter) },
		func(b []byte) int { return encodeFlowSpec(b, ClassFlowSpec, psb.SenderTSpec) },
		func(b []byte) int { return encodeFilterSpec(b, psb.SenderTemplate) },
	})
	for _, msg := range []struct {
		name    string
		payload []byte
	}{
		{"PATH", buildPath(psb, lab.links[0].routerAddr, 37)},
		{"PathTear", buildPathTear(psb, lab.links[0].routerAddr)},
		{"ResvConf", confirmation},
	} {
		t.Run(msg.name, func(t *testing.T) {
			route := PathRoute{Source: rsvpWireSource, Destination: rsvpWireEndpoint, NextHop: lab.links[0].peerAddr}
			if err := lab.sender.SendPath(route, msg.payload); err != nil {
				t.Fatal(err)
			}
			frame := rsvpWireFrame(t, lab.links[0].capture, rsvpWireWait)
			rsvpWireAssertIP(t, frame, route.Source, route.Destination, msg.payload, true)
			if !bytes.Equal(frame[:6], lab.links[0].peer.Attrs().HardwareAddr) {
				t.Fatalf("L2 destination %x, want ERO next hop %x", frame[:6], lab.links[0].peer.Attrs().HardwareAddr)
			}
			packet := rsvpWireReceive(t, lab.receiver, msg.payload)
			parsed, err := DecodeMessage(packet.Payload)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.HasHop && parsed.Hop.NextHop != lab.links[0].routerAddr {
				t.Fatalf("RSVP_HOP %s, want outgoing interface %s", parsed.Hop.NextHop, lab.links[0].routerAddr)
			}
			if packet.Src != route.Source || packet.Dst != route.Destination || packet.TTL != msg.payload[4] || packet.IfIndex != lab.links[0].peer.Attrs().Index {
				t.Fatalf("transit metadata: %+v", packet)
			}
		})
	}
	// Positive control for the ordinary endpoint route on the other interface.
	// Send remains point-to-point and cannot inherit PATH's source or options.
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	reply := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, lab.links[0].routerAddr)
	wantReply := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, lab.links[1].routerAddr)
	if err := lab.sender.Send(lab.links[1].peerAddr, reply); err != nil {
		t.Fatal(err)
	}
	rsvpWireAssertIP(t, rsvpWireFrame(t, lab.links[1].capture, rsvpWireWait), lab.links[1].routerAddr, lab.links[1].peerAddr, wantReply, false)
	packet := rsvpWireReceive(t, lab.receiver, wantReply)
	if packet.Src != lab.links[1].routerAddr || packet.Dst != lab.links[1].peerAddr || packet.IfIndex != lab.links[1].peer.Attrs().Index {
		t.Fatalf("receiver bound too narrowly for another local address: %+v", packet)
	}
}

// TestIntegrationRSVPPathAvoidsDataFEC checks that an installed endpoint push
// route cannot carry ordinary PATH past its selected adjacent control hop.
// RFC requirement: RFC2205-3-37 positive -- an ordinary PATH reaches the selected RSVP hop even after a data FEC is installed for its IP destination.
// RFC requirement: RFC2205-3-39 positive -- the adjacent production receiver receives a nonlocal IPv4 PATH despite an installed endpoint data FEC.
func TestIntegrationRSVPPathAvoidsDataFEC(t *testing.T) {
	lab := newRSVPWireLab(t)
	link := lab.links[1]
	const dataLabel = 16011
	fec := &netlink.Route{
		Dst: rsvpWirePrefix(rsvpWireEndpoint), LinkIndex: link.router.Attrs().Index,
		Gw: link.peerAddr.AsSlice(), Encap: &netlink.MPLSEncap{Labels: []int{dataLabel}},
	}
	if err := lab.routes.RouteReplace(fec); err != nil {
		if errors.Is(err, unix.EOPNOTSUPP) {
			t.Skipf("MPLS IP encapsulation requires CONFIG_MPLS_IPTUNNEL: %v", err)
		}
		if errors.Is(err, unix.ENOENT) {
			t.Skipf("MPLS IP encapsulation requires CONFIG_MPLS_IPTUNNEL: %v", err)
		}
		t.Fatal(err)
	}

	// A destination-routed datagram proves that the endpoint FEC is active
	// on this same link. The subsequent PATH must reach the peer as IPv4.
	psb := rsvpWirePSB()
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	reply := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, link.routerAddr)
	if err := lab.sender.Send(rsvpWireEndpoint, reply); err != nil {
		t.Fatal(err)
	}
	frame := rsvpWireFrame(t, link.capture, rsvpWireWait)
	labels, _ := rsvpWireIP(frame)
	if !slices.Equal(labels, []uint32{dataLabel}) {
		t.Fatalf("endpoint-routed control carried labels %v, want data FEC label %d", labels, dataLabel)
	}
	rsvpWireAssertIP(t, frame, link.routerAddr, rsvpWireEndpoint, reply, false)

	// Native loose-hop resolution retains the remote lookup destination
	// separately from the adjacent next hop. Keep both, as the producer does.
	selected, err := lab.sender.ResolveRoute(netip.PrefixFrom(rsvpWireEndpoint, 32), rsvpWireEndpoint, 0)
	if err != nil {
		t.Fatal(err)
	}
	route := PathRoute{
		Source: rsvpWireSource, Destination: rsvpWireEndpoint,
		NextHop: selected.NextHop, Lookup: selected.Lookup, IfIndex: selected.IfIndex,
	}
	psb.ERO = []eroHop{
		{Address: netip.PrefixFrom(selected.NextHop, 32)},
		{Address: netip.PrefixFrom(rsvpWireEndpoint, 32), Loose: true},
	}
	payload := buildPath(psb, link.routerAddr, 37)
	if err := lab.sender.SendPath(route, payload); err != nil {
		t.Fatal(err)
	}
	frame = rsvpWireFrame(t, link.capture, rsvpWireWait)
	labels, _ = rsvpWireIP(frame)
	if len(labels) != 0 {
		t.Fatalf("ordinary PATH entered endpoint data FEC with labels %v", labels)
	}
	rsvpWireAssertIP(t, frame, route.Source, route.Destination, payload, true)
	if !bytes.Equal(frame[:6], link.peer.Attrs().HardwareAddr) {
		t.Fatalf("L2 destination %x, want selected control hop %x", frame[:6], link.peer.Attrs().HardwareAddr)
	}
	packet := rsvpWireReceive(t, lab.receiver, payload)
	if packet.Src != route.Source {
		t.Fatalf("control source %s, want %s", packet.Src, route.Source)
	}
	if packet.Dst != route.Destination {
		t.Fatalf("control destination %s, want %s", packet.Dst, route.Destination)
	}
	if packet.TTL != payload[4] {
		t.Fatalf("control TTL %d, want %d", packet.TTL, payload[4])
	}
	if packet.IfIndex != link.peer.Attrs().Index {
		t.Fatalf("control interface %d, want %d", packet.IfIndex, link.peer.Attrs().Index)
	}
}

// TestIntegrationRSVPSelectedBypass proves that two bypasses to the same merge
// point select distinct labeled paths, while ordinary sends remain unmarked.
func TestIntegrationRSVPSelectedBypass(t *testing.T) {
	lab := newRSVPWireLab(t)
	var routes [2]netlink.Route
	var rules [2]*netlink.Rule
	for i := range routes {
		table := uint32(0x5a000001 + i)
		routes[i] = netlink.Route{
			Table: int(table), Dst: rsvpWirePrefix(rsvpWireMerge),
			LinkIndex: lab.links[i].router.Attrs().Index, Gw: lab.links[i].peerAddr.AsSlice(),
			Encap: &netlink.MPLSEncap{Labels: []int{16001 + i}},
		}
		if err := lab.routes.RouteAdd(&routes[i]); err != nil {
			if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOENT) {
				t.Skipf("MPLS IP encapsulation requires CONFIG_MPLS_IPTUNNEL: %v", err)
			}
			t.Fatal(err)
		}
		rules[i] = netlink.NewRule()
		rules[i].Family, rules[i].Table, rules[i].Priority = unix.AF_INET, int(table), 1
		rules[i].Mark = table
		mask := ^uint32(0)
		rules[i].Mask = &mask
		if err := lab.routes.RuleAdd(rules[i]); err != nil {
			t.Fatal(err)
		}
	}
	guard := netlink.NewRule()
	guard.Family, guard.Priority, guard.Type = unix.AF_INET, 2, unix.FR_ACT_UNREACHABLE
	guard.Mark = 0x5a000000
	mask := uint32(0xffff0000)
	guard.Mask = &mask
	if err := lab.routes.RuleAdd(guard); err != nil {
		t.Fatal(err)
	}
	payload := buildPath(rsvpWirePSB(), lab.links[0].routerAddr, 53)
	// Concurrent marked senders and an ordinary reply share the transport.
	// Captures assert the route label and physical interface for every send.
	const sends = 8
	var workers sync.WaitGroup
	errorsCh := make(chan error, 3)
	for i := range routes {
		workers.Go(func() {
			for range sends {
				if err := lab.sender.SendPath(PathRoute{
					Source: rsvpWireSource, Destination: rsvpWireEndpoint,
					NextHop: rsvpWireMerge, TableID: uint32(routes[i].Table),
				}, payload); err != nil {
					errorsCh <- err
					return
				}
			}
		})
	}
	reply := encodeMessage(MsgTypeResv, defaultIPTTL, nil)
	workers.Go(func() {
		for range sends {
			if err := lab.sender.Send(lab.links[0].peerAddr, reply); err != nil {
				errorsCh <- err
				return
			}
		}
	})
	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	for i := range lab.links {
		wantPayload := buildPath(rsvpWirePSB(), lab.links[i].routerAddr, 53)
		labeled, ordinary := 0, 0
		wantOrdinary := 0
		if i == 0 {
			wantOrdinary = sends
		}
		for range sends + wantOrdinary {
			frame := rsvpWireFrame(t, lab.links[i].capture, rsvpWireWait)
			labels, _ := rsvpWireIP(frame)
			if len(labels) == 0 {
				rsvpWireAssertIP(t, frame, lab.links[0].routerAddr, lab.links[0].peerAddr, reply, false)
				ordinary++
				continue
			}
			if len(labels) != 1 || labels[0] != uint32(16001+i) {
				t.Fatalf("link %d carried labels %v", i, labels)
			}
			rsvpWireAssertIP(t, frame, rsvpWireSource, rsvpWireEndpoint, wantPayload, true)
			labeled++
		}
		if labeled != sends || ordinary != wantOrdinary {
			t.Fatalf("link %d: labeled=%d ordinary=%d", i, labeled, ordinary)
		}
	}

	selected := PathRoute{Source: rsvpWireSource, Destination: rsvpWireEndpoint, NextHop: rsvpWireMerge, TableID: uint32(routes[0].Table)}
	if err := lab.routes.RouteDel(&routes[0]); err != nil {
		t.Fatal(err)
	}
	if err := lab.sender.SendPath(selected, payload); err == nil {
		t.Fatal("missing bypass route fell back to the ordinary merge-point route")
	}
	if err := lab.routes.RuleDel(rules[0]); err != nil {
		t.Fatal(err)
	}
	if err := lab.sender.SendPath(selected, payload); err == nil {
		t.Fatal("missing bypass rule fell back to the ordinary merge-point route")
	}
	// The transport must also reject the wrong table when no guard exists.
	if err := lab.routes.RuleDel(guard); err != nil {
		t.Fatal(err)
	}
	if err := lab.sender.SendPath(selected, payload); err == nil {
		t.Fatal("route query accepted main instead of the selected private table")
	}
	// A private IP route without its MPLS stack is not an established bypass.
	routes[0].Encap = nil
	rsvpWireRoute(t, lab.routes, &routes[0])
	if err := lab.routes.RuleAdd(rules[0]); err != nil {
		t.Fatal(err)
	}
	if err := lab.sender.SendPath(selected, payload); err == nil {
		t.Fatal("unlabeled private route accepted as a bypass")
	}
	for i := range lab.links {
		if frame := rsvpWireFrame(t, lab.links[i].capture, 150*time.Millisecond); frame != nil {
			t.Fatalf("failed bypass send emitted a frame on link %d: %x", i, frame)
		}
	}
	// The previous socket mark must not affect the next ordinary PATH.
	selected.TableID = 0
	selected.NextHop = lab.links[0].peerAddr
	if err := lab.sender.SendPath(selected, payload); err != nil {
		t.Fatal(err)
	}
	frame := rsvpWireFrame(t, lab.links[0].capture, rsvpWireWait)
	labels, _ := rsvpWireIP(frame)
	if len(labels) != 0 {
		t.Fatalf("ordinary PATH inherited bypass labels %v", labels)
	}
	rsvpWireAssertIP(t, frame, rsvpWireSource, rsvpWireEndpoint, payload, true)
}

// TestIntegrationRSVPReceiveOversize sends a datagram larger than the common
// IPv4 Router Alert carrier permits, followed by a valid one.
func TestIntegrationRSVPReceiveOversize(t *testing.T) {
	lab := newRSVPWireLab(t)
	oversized := make([]byte, maxRSVPPacket+4)
	copy(oversized, encodeMessage(MsgTypeResv, defaultIPTTL, nil))
	binary.BigEndian.PutUint16(oversized[6:8], uint16(len(oversized)))
	// Inject a peer datagram below IPv4's absolute limit without using the
	// production sender, which correctly refuses this carrier-sized message.
	raw, ok := lab.sender.(*rawTransport)
	if !ok {
		t.Fatalf("lab sender is %T", lab.sender)
	}
	if err := unix.SetsockoptInt(raw.sendFD, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DONT); err != nil {
		t.Fatal(err)
	}
	if err := unix.Sendto(raw.sendFD, oversized, 0, &unix.SockaddrInet4{Addr: lab.links[0].peerAddr.As4()}); err != nil {
		t.Fatal(err)
	}
	valid := buildPath(rsvpWirePSB(), lab.links[0].routerAddr, 39)
	if err := lab.sender.SendPath(PathRoute{Source: rsvpWireSource, Destination: rsvpWireEndpoint, NextHop: lab.links[0].peerAddr}, valid); err != nil {
		t.Fatal(err)
	}
	rsvpWireReceive(t, lab.receiver, valid)
}

// TestIntegrationRSVPCloseIdle verifies that Close wakes a receiver blocked in
// the poller and makes both send paths fail rather than use a recycled fd.
func TestIntegrationRSVPCloseIdle(t *testing.T) {
	lab := newRSVPWireLab(t)
	if err := lab.sender.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-lab.sender.Recv(); ok {
		t.Fatal("receive channel remained open after Close")
	}
	payload := buildPath(rsvpWirePSB(), lab.links[0].routerAddr, 37)
	if err := lab.sender.Send(lab.links[0].peerAddr, payload); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Send after Close: %v", err)
	}
	if err := lab.sender.SendPath(PathRoute{Source: rsvpWireSource, Destination: rsvpWireEndpoint, NextHop: lab.links[0].peerAddr}, payload); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("SendPath after Close: %v", err)
	}
}
