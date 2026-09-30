// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 manual-key ESP SAs installed into XFRM.
// Related: ipsec_install.go -- buildIPsecInterfaceSAs, the states this probe installs.
// Related: rfc4302_ah_unknown_spi_linux_test.go -- the namespace, counter and socket helpers.
// Related: rfc4303_esp_test.go -- the unit tests over the ESP SA parameters Ze builds.
//
// VALIDATES: against a real Linux XFRM stack, what the ESP SAs the OSPF installer puts in
// the SAD produce. RFC 4303 Section 2: the header in front of every ESP packet Ze's SAs emit
// carries 50. RFC 4303 Section 2.1: the manual configuration decides which inbound packets
// map to an SA, by the destination it names and not by the sender's address.
// PREVENTS: an ESP SA installed so that the kernel writes another protocol number (51 for an
// integrity-only SA that reads like AH), and an inbound ESP packet mapping to an SA the
// configuration did not key on its destination.
//
// No privilege is needed: ahProbeOwnNamespace re-runs the unit in a user and network
// namespace of its own, where the unit is root and every XFRM change stays private.

//go:build linux

package ospf

import (
	"encoding/binary"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

const (
	espProbeLink  = "ospfesp0"
	espProbeSPI   = 256
	espProbeProto = 50
	// espProbeEthAll is ETH_P_ALL: only a tap on every protocol sees what a link transmits.
	espProbeEthAll = 0x0003
)

var (
	espProbeLocal = netip.MustParseAddr("fe80::1")
	// espProbeOther is a second address of the router that no ESP state is keyed on.
	espProbeOther = netip.MustParseAddr("fe80::2")
)

// espProbeSetup brings up a dummy link carrying fe80::1 and fe80::2 and installs the OSPF
// ESP SAs for block through the installer's own path over the real XFRM backend. It
// returns the ifindex.
func espProbeSetup(t *testing.T, block ipsecInterfaceConfig) int {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	link := &netlink.Dummy{}
	link.Name = espProbeLink
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("dummy link unavailable: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", espProbeLink, err)
	}
	for _, local := range []netip.Addr{espProbeLocal, espProbeOther} {
		addr, err := netlink.ParseAddr(local.String() + "/64")
		if err != nil {
			t.Fatalf("parse %s: %v", local, err)
		}
		addr.Flags = syscall.IFA_F_NODAD
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("add %s: %v", local, err)
		}
	}
	ifindex := link.Attrs().Index

	inst := newIPsecInstaller(nil, nil)
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Logf("close xfrm backend: %v", err)
		}
	})
	inst.setTransportSource(func(string) (netip.Addr, int, bool) { return espProbeLocal, ifindex, true })
	inst.setConfig([]interfaceConfig{{Name: "eth1", IPsec: &block}})
	inst.onInterfaceUp(ifindex, "eth1")
	if _, ok := inst.status("eth1"); !ok {
		t.Fatal("the installer reports no IPsec status for the interface")
	}
	return ifindex
}

// espProbeSend sends one ESP packet from source to target under espProbeSPI: an 8-octet
// header, 8 octets of payload and a zero ICV sized for HMAC-SHA-256-128.
func espProbeSend(t *testing.T, ifindex int, source, target netip.Addr) {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, espProbeProto)
	if err != nil {
		t.Fatalf("raw ESP socket: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck // Test socket; nothing to recover on close.

	from := &syscall.SockaddrInet6{Addr: source.As16(), ZoneId: uint32(ifindex)}
	if err := syscall.Bind(fd, from); err != nil {
		t.Fatalf("bind ESP socket to %s: %v", source, err)
	}
	packet := make([]byte, 8+8+16)
	binary.BigEndian.PutUint32(packet[0:4], espProbeSPI)
	binary.BigEndian.PutUint32(packet[4:8], 1) // Sequence Number
	to := &syscall.SockaddrInet6{Addr: target.As16(), ZoneId: uint32(ifindex)}
	if err := syscall.Sendto(fd, packet, 0, to); err != nil {
		t.Fatalf("send ESP to %s: %v", target, err)
	}
}

// espProbeNextHeader sends one OSPF (89) packet to ff02::5 out of the link and returns the
// Next Header field of the IPv6 packet the kernel put on the wire, read off a packet socket.
func espProbeNextHeader(t *testing.T, ifindex int) byte {
	t.Helper()
	proto := int(espProbeHostToNet(espProbeEthAll))
	capture, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, proto)
	if err != nil {
		t.Fatalf("packet socket: %v", err)
	}
	defer syscall.Close(capture) //nolint:errcheck // Test socket; nothing to recover on close.
	if err := syscall.Bind(capture, &syscall.SockaddrLinklayer{Protocol: uint16(proto), Ifindex: ifindex}); err != nil {
		t.Fatalf("bind packet socket: %v", err)
	}
	tv := syscall.Timeval{Usec: 200_000}
	if err := syscall.SetsockoptTimeval(capture, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("set receive timeout: %v", err)
	}

	ospf, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, int(ospfv3transport.Protocol))
	if err != nil {
		t.Fatalf("raw OSPF socket: %v", err)
	}
	defer syscall.Close(ospf) //nolint:errcheck // Test socket; nothing to recover on close.
	group := &syscall.SockaddrInet6{Addr: ospfv3transport.AllSPFRouters.As16(), ZoneId: uint32(ifindex)}
	if err := syscall.Sendto(ospf, make([]byte, 16), 0, group); err != nil {
		t.Fatalf("send OSPF to %s: %v", ospfv3transport.AllSPFRouters, err)
	}

	want := ospfv3transport.AllSPFRouters.As16()
	buf := make([]byte, 2048)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, _, err := syscall.Recvfrom(capture, buf, 0)
		if err != nil {
			continue
		}
		// The IPv6 header is 40 octets: version in the top nibble, Next Header at offset 6,
		// destination at 24..40.
		if n >= 40 && buf[0]>>4 == 6 && [16]byte(buf[24:40]) == want {
			return buf[6]
		}
	}
	t.Fatalf("no packet to %s left %s", ospfv3transport.AllSPFRouters, espProbeLink)
	return 0
}

// espProbeHostToNet puts a 16-bit value in network order for a socket protocol argument.
func espProbeHostToNet(v uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return binary.NativeEndian.Uint16(b[:])
}

func TestRFC4303ESPPacketOnTheWireCarriesProtocol50(t *testing.T) {
	// Goal: RFC 4303 Section 2 on the packets Ze's ESP SAs produce. Method: install an esp
	// interface with aes128 and sha256, send OSPF to ff02::5, and read the packet the kernel
	// put on the link.
	// RFC requirement: RFC4303-2-1 positive -- RFC 4303 Section 2: "The (outer) protocol
	// header (IPv4, IPv6, or Extension) that immediately precedes the ESP header SHALL contain
	// the value 50 in its Protocol (IPv4) or Next Header (IPv6, Extension) field". The IPv6
	// header of the OSPF packet sent under Ze's ESP SA carries Next Header 50.
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := espProbeSetup(t, ipsecInterfaceConfig{
		SPI: espProbeSPI, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
		EncAlgo: "aes128", EncKey: hexKey(16),
	})
	if got := espProbeNextHeader(t, ifindex); got != espProbeProto {
		t.Fatalf("Next Header before ESP = %d, want 50", got)
	}
}

func TestRFC4303IntegrityOnlyESPOnTheWireIsNotAH(t *testing.T) {
	// Goal: the value is 50 for every ESP SA, including the one that looks most like AH.
	// Method: install an integrity-only esp interface (null cipher, sha256), send OSPF to
	// ff02::5, and read the packet the kernel put on the link.
	// RFC requirement: RFC4303-2-1 negative -- the input is pushed toward the AH shape (an
	// integrity transform and no cipher), and the header in front of the ESP header still
	// carries 50, not AH's 51, so an installer that chose the protocol from the algorithms
	// rather than from ESP fails this test.
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := espProbeSetup(t, ipsecInterfaceConfig{
		SPI: espProbeSPI, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32), EncAlgo: "null",
	})
	if got := espProbeNextHeader(t, ifindex); got != espProbeProto {
		t.Fatalf("Next Header before an integrity-only ESP header = %d, want 50", got)
	}
}

func TestRFC4303InboundESPMapsByConfiguredDestination(t *testing.T) {
	// Goal: RFC 4303 Section 2.1 on the SAD the OSPF installer fills. Method: install the
	// interface's ESP SAs (SPI 256, keyed on fe80::1), send an ESP packet to fe80::1 from the
	// router's other address fe80::2, and read the kernel's counters.
	// RFC requirement: RFC4303-2.1-2 positive -- the indication the manual configuration set
	// is destination matching without source matching: a packet to the configured destination
	// maps to its SA whatever its source (XfrmInNoStates does not move; the SA's own ICV check,
	// XfrmInStateProtoError, rejects the forged packet), and nothing reaches OSPF.
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := espProbeSetup(t, ipsecInterfaceConfig{
		SPI: espProbeSPI, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	ospfSocket := ahProbeListen(t)

	noStates := ahProbeCounter(t, ahProbeStat)
	icvFailures := ahProbeCounter(t, ahProbeICVStat)
	espProbeSend(t, ifindex, espProbeOther, espProbeLocal)
	if ahProbeReceived(ospfSocket) {
		t.Fatal("an ESP packet with a forged ICV reached the OSPF socket")
	}
	if got := ahProbeCounter(t, ahProbeStat) - noStates; got != 0 {
		t.Fatalf("%s rose by %d for the configured destination, want 0", ahProbeStat, got)
	}
	if got := ahProbeCounter(t, ahProbeICVStat) - icvFailures; got != 1 {
		t.Fatalf("%s rose by %d for the configured destination, want 1", ahProbeICVStat, got)
	}
}

func TestRFC4303InboundESPToUnconfiguredDestinationMapsToNoSA(t *testing.T) {
	// Goal: the destination is part of the mapping the configuration set. Method: the same
	// packet under the same SPI, sent to fe80::2, an address of this router no ESP state is
	// keyed on.
	// RFC requirement: RFC4303-2.1-2 negative -- a packet whose destination the manual
	// configuration did not name maps to no SA, even with the configured SPI: it is counted
	// as SA-less (XfrmInNoStates rises by one) and nothing reaches OSPF.
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := espProbeSetup(t, ipsecInterfaceConfig{
		SPI: espProbeSPI, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	ospfSocket := ahProbeListen(t)

	noStates := ahProbeCounter(t, ahProbeStat)
	espProbeSend(t, ifindex, espProbeLocal, espProbeOther)
	if ahProbeReceived(ospfSocket) {
		t.Fatal("an ESP packet to an unconfigured destination reached the OSPF socket")
	}
	if got := ahProbeCounter(t, ahProbeStat) - noStates; got != 1 {
		t.Fatalf("%s rose by %d for an unconfigured destination, want 1", ahProbeStat, got)
	}
}
