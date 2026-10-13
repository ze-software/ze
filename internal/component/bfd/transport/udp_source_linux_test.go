//go:build linux

package transport

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// VALIDATES: a Control packet whose session key carries a local address leaves
// the transport with that address as its IP source, on the multi-hop socket and
// on the single-hop socket that also pins the packet to its link. Method: a
// receiver socket on 127.0.0.1 reads the source of what the transport sends.
// The sending transport binds 127.0.0.3, so the kernel's own choice is 127.0.0.3
// and only a source Ze sets can make the packet arrive from 127.0.0.2. Linux lo
// holds 127.0.0.0/8, so every one of these addresses is local without any
// privilege.
// PREVENTS: the L3 defect of spec-bgp-live-receive-binding-and-bfd-keep-session
// (journal unwired-feature.md, 2026-10-12): a session pinned to 127.0.0.2 sent
// from the kernel's choice, its first packets demuxed onto the peer's session
// for the other local address, and that session went Down.

// sourceProbe receives one datagram on 127.0.0.1 and answers its source.
type sourceProbe struct {
	conn *net.UDPConn
}

func newSourceProbe(t *testing.T) (*sourceProbe, uint16) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen 127.0.0.1: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // Test cleanup; the socket is discarded.
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("probe LocalAddr is %T, want *net.UDPAddr", conn.LocalAddr())
	}
	return &sourceProbe{conn: conn}, uint16(local.Port) //nolint:gosec // A bound UDP port fits in 16 bits.
}

func (p *sourceProbe) source(t *testing.T) netip.Addr {
	t.Helper()
	if err := p.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 64)
	_, from, err := p.conn.ReadFromUDPAddrPort(buf)
	if err != nil {
		t.Fatalf("probe read: %v", err)
	}
	return from.Addr().Unmap()
}

// sourceSender starts a transport bound to 127.0.0.3 on the probe's port, so
// destination addresses every packet to the probe.
func sourceSender(t *testing.T, port uint16, mode api.HopMode) *UDP {
	t.Helper()
	u := &UDP{Bind: netip.AddrPortFrom(netip.MustParseAddr("127.0.0.3"), port), Mode: mode}
	if err := u.Start(); err != nil {
		t.Fatalf("start transport: %v", err)
	}
	t.Cleanup(func() { u.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	return u
}

func TestUDPSendLeavesFromThePinnedLocal(t *testing.T) {
	pinned := netip.MustParseAddr("127.0.0.2")
	peer := netip.MustParseAddr("127.0.0.1")
	cases := []struct {
		name      string
		mode      api.HopMode
		iface     string
		local     netip.Addr
		wantFrom  netip.Addr
		wantNoPin bool
	}{
		{name: "multi-hop pinned", mode: api.MultiHop, local: pinned, wantFrom: pinned},
		{name: "single-hop pinned, link pin", mode: api.SingleHop, iface: "lo", local: pinned, wantFrom: pinned},
		{name: "single-hop pinned, no interface", mode: api.SingleHop, local: pinned, wantFrom: pinned},
		// An unpinned session keeps the kernel's choice: the bound address.
		{name: "multi-hop unpinned", mode: api.MultiHop, wantFrom: netip.MustParseAddr("127.0.0.3")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe, port := newSourceProbe(t)
			u := sourceSender(t, port, tc.mode)
			out := Outbound{To: peer, Local: tc.local, Interface: tc.iface, Mode: tc.mode, Bytes: make([]byte, 24)}
			if err := u.Send(out); err != nil {
				t.Fatalf("send: %v", err)
			}
			if got := probe.source(t); got != tc.wantFrom {
				t.Errorf("packet left from %s, want %s", got, tc.wantFrom)
			}
		})
	}
}

// VALIDATES: the control message Ze builds carries the pinned source in the
// field the kernel reads on send: ipi_spec_dst for IPv4, ipi6_addr for IPv6,
// beside the interface index. Method: parse the built message back with the
// kernel's own layout. IPv6 has no second loopback address an unprivileged test
// can send from, so this is the IPv6 proof of the field.
// PREVENTS: the source landing in ipi_addr, which the kernel ignores on send.
func TestPktinfoPinCarriesTheSource(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.7")
	msgs, err := unix.ParseSocketControlMessage(pktinfoPin(3, v4, false))
	if err != nil || len(msgs) != 1 {
		t.Fatalf("parse v4 message: %v, %d messages", err, len(msgs))
	}
	if msgs[0].Header.Level != unix.IPPROTO_IP || msgs[0].Header.Type != unix.IP_PKTINFO {
		t.Fatalf("v4 message is level %d type %d", msgs[0].Header.Level, msgs[0].Header.Type)
	}
	// struct in_pktinfo: ipi_ifindex (4), ipi_spec_dst (4), ipi_addr (4).
	if got := netip.AddrFrom4([4]byte(msgs[0].Data[4:8])); got != v4 {
		t.Errorf("ipi_spec_dst = %s, want %s", got, v4)
	}
	if got := netip.AddrFrom4([4]byte(msgs[0].Data[8:12])); got.IsValid() && !got.IsUnspecified() {
		t.Errorf("ipi_addr = %s, want 0.0.0.0", got)
	}

	v6 := netip.MustParseAddr("2001:db8::7")
	msgs, err = unix.ParseSocketControlMessage(pktinfoPin(3, v6, true))
	if err != nil || len(msgs) != 1 {
		t.Fatalf("parse v6 message: %v, %d messages", err, len(msgs))
	}
	if msgs[0].Header.Level != unix.IPPROTO_IPV6 || msgs[0].Header.Type != unix.IPV6_PKTINFO {
		t.Fatalf("v6 message is level %d type %d", msgs[0].Header.Level, msgs[0].Header.Type)
	}
	// struct in6_pktinfo: ipi6_addr (16), ipi6_ifindex (4).
	if got := netip.AddrFrom16([16]byte(msgs[0].Data[0:16])); got != v6 {
		t.Errorf("ipi6_addr = %s, want %s", got, v6)
	}
}

// VALIDATES: a pinned source of the other address family than the socket is
// refused with errUDPSourceFamily and nothing is written.
// PREVENTS: an IPv6 source handed to an IPv4 socket's control message, which
// netip.Addr.As4 answers with a panic.
func TestUDPSendRefusesASourceOfTheOtherFamily(t *testing.T) {
	_, port := newSourceProbe(t)
	u := sourceSender(t, port, api.MultiHop)
	out := Outbound{To: netip.MustParseAddr("127.0.0.1"), Local: netip.MustParseAddr("::1"), Mode: api.MultiHop, Bytes: make([]byte, 24)}
	if err := u.Send(out); !errors.Is(err, errUDPSourceFamily) {
		t.Fatalf("send = %v, want errUDPSourceFamily", err)
	}
}
