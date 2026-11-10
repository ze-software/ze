// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the outer header DF bit
// Related: xfrm_linux.go -- xfrmNoPMTUDisc, the projection whose kernel effect this reads
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit, Section 5.1.2.1 tunnel header DSCP
//
// VALIDATES: the outer IPv4 header the kernel emits for a tunnel-mode SA installed
// through the XFRM backend carries the DF treatment the SA asked for, read off the
// wire rather than off the installed state.
// PREVENTS: a treatment that installs as the flag ze meant and does not change the
// packet: the unit tests prove the flag, only the kernel proves what the flag does.
// METHOD: two network namespaces joined by a veth carrying the outer pair. Side a has
// an XFRM interface bound to the SA's if_id, an inner address on it, one outbound
// tunnel-mode state and one outbound policy installed through xfrmBackend. Side b
// holds the outer address only and a raw IPPROTO_ESP socket, which the kernel hands a
// copy of every ESP packet before the protocol handler (raw_local_deliver), header
// included. Side a sends UDP datagrams with the inner DF bit forced on or off by
// IP_MTU_DISCOVER, and the test reads the outer header of each ESP packet b receives.
//
// Each subtest runs in a user and network namespace of the test's own
// (userns.Enter), so it needs no host privilege.

//go:build linux

package dataplane

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const (
	outerHdrSPI  = 0x43010081
	outerHdrPort = 43081
	// ipv4DF is the DF bit of the IPv4 flags and fragment offset field (RFC 791).
	ipv4DF = 0x4000
)

// innerSend is one inner datagram side a sends: whether its own DF bit is set, and the
// IP_TOS byte it carries.
type innerSend struct {
	df  bool
	tos int
}

// outerHeader is the part of a received ESP packet's outer IPv4 header the test
// reads.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|Version|  IHL  |  DS / ECN (1) |         Total Length          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|         Identification        |Flags (6)|   Fragment Offset   |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  ...  |  Protocol (9) = 50   |  ...  then the ESP SPI at IHL*4 |
type outerHeader struct {
	tos uint8
	df  bool
}

// emitOuterHeaders installs one outbound IPv4-in-IPv4 tunnel-mode SA built by sa, sends
// each datagram in sends through it, and returns the outer header of each ESP packet
// side b received for the SA's SPI, in order.
func emitOuterHeaders(t *testing.T, sa SAParams, sends []innerSend) []outerHeader {
	t.Helper()
	encapNetns(t)
	home, err := netns.Get()
	if err != nil {
		t.Fatalf("handle on side a's namespace: %v", err)
	}
	t.Cleanup(func() { _ = home.Close() })
	peer, err := netns.New()
	if err != nil {
		t.Fatalf("create side b's namespace: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	if err := netns.Set(home); err != nil {
		t.Fatalf("return to side a's namespace: %v", err)
	}

	veth := &netlink.Veth{Name: "ohdr0", PeerName: "ohdr1", PeerNamespace: netlink.NsFd(int(peer))}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("veth: %v", err)
	}
	innerA, innerB := net.ParseIP("10.81.0.1"), net.ParseIP("10.81.0.2")
	crossFamilySide(t, "ohdr0", sa.Src.String(), 24, innerA, innerB)

	b := &xfrmBackend{}
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("install SA spi=%#x: %v", sa.SPI, err)
	}
	anyInner := &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	policy := SPParams{
		Src: anyInner, Dst: anyInner, Dir: SADirOut, Proto: ProtoESP, Mode: ModeTunnel,
		IfID: crossFamilyIfID, ReqID: crossFamilyReqID, Owner: "outer-header", Priority: PriorityChildSA,
		TunnelSrc: sa.Src, TunnelDst: sa.Dst,
	}
	if err := b.InstallPolicy(policy); err != nil {
		t.Fatalf("install the outbound policy: %v", err)
	}

	var fd int
	crossFamilyIn(t, peer, home, func() {
		link, err := netlink.LinkByName("ohdr1")
		if err != nil {
			t.Fatalf("find ohdr1: %v", err)
		}
		crossFamilyAddr(t, link, sa.Dst, 24)
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("ohdr1 up: %v", err)
		}
		fd, err = unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_ESP)
		if err != nil {
			t.Fatalf("raw ESP socket on side b: %v", err)
		}
	})
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 3}); err != nil {
		t.Fatalf("receive timeout on the raw socket: %v", err)
	}

	got := make([]outerHeader, 0, len(sends))
	for i, send := range sends {
		sendInner(t, innerA, innerB, send)
		got = append(got, readOuterHeader(t, fd, sa.SPI, i))
	}
	return got
}

// sendInner sends one UDP datagram from innerA to innerB with the inner DF bit and TOS
// byte send asks for. IP_PMTUDISC_DO sets DF on every datagram, IP_PMTUDISC_DONT never.
func sendInner(t *testing.T, innerA, innerB net.IP, send innerSend) {
	t.Helper()
	conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: innerA}, &net.UDPAddr{IP: innerB, Port: outerHdrPort})
	if err != nil {
		t.Fatalf("dial %s from %s: %v", innerB, innerA, err)
	}
	defer func() { _ = conn.Close() }()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("raw conn: %v", err)
	}
	pmtu := unix.IP_PMTUDISC_DONT
	if send.df {
		pmtu = unix.IP_PMTUDISC_DO
	}
	var optErr error
	if err := raw.Control(func(s uintptr) {
		if optErr = unix.SetsockoptInt(int(s), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, pmtu); optErr != nil {
			return
		}
		optErr = unix.SetsockoptInt(int(s), unix.IPPROTO_IP, unix.IP_TOS, send.tos)
	}); err != nil {
		t.Fatalf("socket control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("set the inner DF and TOS: %v", optErr)
	}
	if _, err := conn.Write([]byte("ze outer header probe")); err != nil {
		t.Fatalf("send through the tunnel: %v", err)
	}
}

// readOuterHeader reads ESP packets on fd until one carries spi, and returns its outer
// header. A packet of another SPI is skipped; the receive timeout ends the wait.
func readOuterHeader(t *testing.T, fd int, spi uint32, index int) outerHeader {
	t.Helper()
	var buf [2048]byte
	for range 16 {
		n, _, err := unix.Recvfrom(fd, buf[:], 0)
		if err != nil {
			t.Fatalf("datagram %d: side b received no ESP packet: %v", index, err)
		}
		pkt := buf[:n]
		if len(pkt) < 20 {
			continue
		}
		ihl := int(pkt[0]&0x0f) * 4
		if len(pkt) < ihl+4 || binary.BigEndian.Uint32(pkt[ihl:ihl+4]) != spi {
			continue
		}
		return outerHeader{
			tos: pkt[1],
			df:  binary.BigEndian.Uint16(pkt[6:8])&ipv4DF != 0,
		}
	}
	t.Fatalf("datagram %d: no ESP packet carried spi %#x", index, spi)
	return outerHeader{}
}

// outerHdrSA is the outbound IPv4-in-IPv4 tunnel-mode SA the wire tests install.
func outerHdrSA() SAParams {
	return SAParams{
		SPI: outerHdrSPI, Src: net.ParseIP("192.0.2.81"), Dst: net.ParseIP("192.0.2.82"),
		Dir: SADirOut, Proto: ProtoESP, Mode: ModeTunnel,
		IfID: crossFamilyIfID, ReqID: crossFamilyReqID, ReplayWin: 32,
		EncAlgo: "aes256gcm", EncKey: encapTestAEADKey, IsAEAD: true,
	}
}

// RFC requirement: RFC4301-8.1-1 positive -- on the wire, a tunnel-mode SA with DF copy emits an outer header whose DF bit equals the inner datagram's, set and clear.
// RFC requirement: RFC4301-8.1-2 positive -- on the wire, a tunnel-mode SA with DF clear emits a clear outer DF bit for an inner datagram whose DF is set, and one with DF copy emits a set bit for it.
func TestXFRMOuterHeaderCarriesTheDFTreatment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	cases := []struct {
		name string
		df   DFTreatment
		sent []innerSend
		want []bool
	}{
		{"copy", DFCopy, []innerSend{{df: true}, {df: false}}, []bool{true, false}},
		{"clear", DFClear, []innerSend{{df: true}}, []bool{false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sa := outerHdrSA()
			sa.DF = tc.df
			got := emitOuterHeaders(t, sa, tc.sent)
			for i, h := range got {
				if h.df != tc.want[i] {
					t.Errorf("DF %s, inner DF %t: the outer header's DF bit is %t, want %t",
						tc.df, tc.sent[i].df, h.df, tc.want[i])
				}
			}
		})
	}
}

// RFC requirement: RFC4301-5.1.2.1-1 positive -- on the wire, a tunnel-mode SA copying the outer DSCP emits the inner DSCP, and one mapping it to 0 emits DSCP 0 for an inner datagram marked EF (46).
func TestXFRMOuterHeaderCarriesTheOuterDSCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	zero, err := FixedOuterDSCP(0)
	if err != nil {
		t.Fatal(err)
	}
	const ef = 46
	for _, tc := range []struct {
		name string
		dscp OuterDSCP
		want uint8
	}{{"copy", OuterDSCPCopy, ef}, {"mapped to 0", zero, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			sa := outerHdrSA()
			sa.OuterDSCP = tc.dscp
			got := emitOuterHeaders(t, sa, []innerSend{{df: true, tos: ef << 2}})
			if dscp := got[0].tos >> 2; dscp != tc.want {
				t.Errorf("outer DSCP %s, inner DSCP %d: the outer header carries DSCP %d, want %d",
					tc.dscp, ef, dscp, tc.want)
			}
		})
	}
}
