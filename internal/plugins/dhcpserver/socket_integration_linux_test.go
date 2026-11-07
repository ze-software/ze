// Design: docs/architecture/provisioning/dhcp-server.md -- integration coverage for SO_BINDTODEVICE
//
// These tests pin socket_linux.go (listenDHCP): they exercise the real
// SO_BINDTODEVICE syscall and a real DISCOVER/OFFER exchange over UDP, asserting
// the PXE boot options (RFC 4578 / RFC 2132) the installer relies on, plus the
// device-filtering behavior the option provides. They require root (ports 67/68
// are privileged) and, for the negative case, CAP_NET_ADMIN. Each test runs in a
// user namespace of its own (userns.Enter), which holds both, so a refused bind
// or dummy link fails it.
//
// See rfc1350_socket_integration_linux_test.go in ../tftpserver for why the positive
// round-trip binds to "lo": locally-routed traffic ingresses on the loopback
// device, so only an lo-bound socket can both bind and receive it within one
// namespace. The OFFER is delivered by unicast to ciaddr:68 (responseAddr), so
// the DISCOVER carries ciaddr = the client's loopback address.

//go:build linux

package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/userns"
)

const pxeVendorClass = "PXEClient:Arch:00000:UNDI:002001"

// buildLoopbackDiscover encodes a BOOTREQUEST/DHCPDISCOVER with the PXE markers
// the server keys on: option 60 "PXEClient:..." and option 93 (client arch).
// Unlike the package's buildPXEDiscover helper, it sets ciaddr so the server
// unicasts the OFFER back to the client's loopback address (responseAddr), which
// is what makes delivery over lo work in this integration test.
func buildLoopbackDiscover(mac net.HardwareAddr, ciaddr netip.Addr, arch uint16) []byte {
	pkt := make([]byte, dhcpHeaderLen)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], 0x0a0b0c0d) // xid
	// flags (10:12) = 0 -> unicast reply path; ciaddr drives delivery.
	ci := ciaddr.As4()
	copy(pkt[12:16], ci[:])
	copy(pkt[28:34], mac)

	pkt = binary.BigEndian.AppendUint32(pkt, magicCookie)
	pkt = append(pkt, optMessageType, 1, msgDiscover, optVendorClassID, byte(len(pxeVendorClass)))
	pkt = append(pkt, pxeVendorClass...)
	pkt = append(pkt, optClientArch, 2, byte(arch>>8), byte(arch),
		optParamReqList, 4, optSubnetMask, optRouter, optTFTPServerName, optBootfileName,
		optEnd)
	return pkt
}

// testHandler builds a PXE-enabled handler for the loopback subnet. The
// server's address (127.0.0.1, which lo holds) and the TFTP server are
// deliberately distinct so the siaddr assertion proves the
// PXE path overwrote siaddr with the TFTP server address.
func testHandler() *dhcpHandler {
	sub := subnetConfig{
		Prefix:        netip.MustParsePrefix("127.0.0.0/8"),
		Ranges:        []addressRange{{Name: "r", Start: netip.MustParseAddr("127.0.0.50"), Stop: netip.MustParseAddr("127.0.0.60")}},
		LeaseTimeSec:  600,
		DefaultRouter: netip.MustParseAddr("127.0.0.1"),
	}
	pxe := pxeConfig{
		Enabled:      true,
		TFTPServer:   netip.MustParseAddr("127.0.0.9"),
		BootfileBIOS: "ipxe.pxe",
		BootfileUEFI: "ipxe.efi",
	}
	return newDHCPHandler(sub, pxe)
}

// createDummyForTest creates a dummy interface and registers cleanup, failing
// the test when the kernel refuses it.
func createDummyForTest(t *testing.T, name string) {
	t.Helper()
	link := &netlink.Dummy{Name: name}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("create dummy %q: %v", name, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set dummy %q up: %v", name, err)
	}
}

// clientAddr is the loopback address the test client binds on port 68 and sets
// as ciaddr so the OFFER is unicast back to it.
var clientAddr = netip.MustParseAddr("127.0.0.2")

// TestListenDHCPLoopbackPXEOffer binds the DHCP listener to lo with
// SO_BINDTODEVICE and asserts a real PXE OFFER carries options 43/60/66/67 and
// the siaddr from the PXE config.
func TestListenDHCPLoopbackPXEOffer(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	conn, err := listenDHCP("lo")
	if err != nil {
		t.Fatalf("bind lo:67: %v", err)
	}
	defer func() { _ = conn.Close() }()

	go serveMulti(conn, "lo", []*dhcpHandler{testHandler()}, slogutil.DiscardLogger())

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: clientAddr.AsSlice(), Port: 68})
	if err != nil {
		t.Fatalf("bind client 127.0.0.2:68: %v", err)
	}
	defer func() { _ = cli.Close() }()

	mac := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	disc := buildLoopbackDiscover(mac, clientAddr, 0) // arch 0 -> BIOS bootfile
	if _, err := cli.WriteToUDP(disc, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 67}); err != nil {
		t.Fatalf("send DISCOVER: %v", err)
	}

	if err := cli.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 1500)
	n, _, err := cli.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read OFFER over lo-bound socket: %v", err)
	}
	resp := buf[:n]

	if got := parseMsgType(resp); got != msgOffer {
		t.Fatalf("message type: got %d, want OFFER(%d)", got, msgOffer)
	}
	if got := string(parseOptionBytes(resp, optVendorClassID)); got != "PXEClient" {
		t.Errorf("option 60 (vendor class): got %q, want %q", got, "PXEClient")
	}
	if got := string(parseOptionBytes(resp, optTFTPServerName)); got != "127.0.0.9" {
		t.Errorf("option 66 (TFTP server): got %q, want %q", got, "127.0.0.9")
	}
	if got := string(parseOptionBytes(resp, optBootfileName)); got != "ipxe.pxe" {
		t.Errorf("option 67 (bootfile): got %q, want %q", got, "ipxe.pxe")
	}
	if got := parseOptionBytes(resp, optVendorSpecific); len(got) == 0 {
		t.Errorf("option 43 (vendor-specific) missing")
	}
	// RFC 2131 Section 4.3.1: siaddr identifies the next-server (TFTP) address.
	if siaddr := netip.AddrFrom4([4]byte(resp[20:24])); siaddr != netip.MustParseAddr("127.0.0.9") {
		t.Errorf("siaddr: got %s, want 127.0.0.9", siaddr)
	}
}

// TestListenDHCPDeviceFilter proves SO_BINDTODEVICE restricts the socket to its
// device: a listener bound to a dummy interface must not answer a DISCOVER that
// arrives over loopback.
func TestListenDHCPDeviceFilter(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	createDummyForTest(t, "zedhcpd0")
	// The dummy holds no address, and a listener whose device holds none in a
	// configured subnet answers nothing anyway. The stand-in address keeps the
	// device filter the only reason this listener can stay silent.
	previous := dhcpDeviceAddrs
	dhcpDeviceAddrs = func(string) ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil }
	t.Cleanup(func() { dhcpDeviceAddrs = previous })

	conn, err := listenDHCP("zedhcpd0")
	if err != nil {
		t.Fatalf("bind zedhcpd0:67: %v", err)
	}
	defer func() { _ = conn.Close() }()

	go serveMulti(conn, "zedhcpd0", []*dhcpHandler{testHandler()}, slogutil.DiscardLogger())

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: clientAddr.AsSlice(), Port: 68})
	if err != nil {
		t.Fatalf("bind client 127.0.0.2:68: %v", err)
	}
	defer func() { _ = cli.Close() }()

	mac := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
	disc := buildLoopbackDiscover(mac, clientAddr, 0)
	if _, err := cli.WriteToUDP(disc, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 67}); err != nil {
		t.Fatalf("send DISCOVER: %v", err)
	}

	if err := cli.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 1500)
	if n, _, err := cli.ReadFromUDP(buf); err == nil {
		t.Fatalf("expected no OFFER from device-bound socket over lo, got %d bytes", n)
	}
}

// TestSharedSegmentReservationOverLoopback serves two subnets of one shared
// network through a real listener whose device holds an address in each, and
// reads the device's addresses from the kernel.
// VALIDATES: the wiring of AC-1 and AC-3: serveMulti reads lo's addresses
// over netlink (readDeviceAddrs), resolves the segment holding 127.0.0.0/8 and
// 10.99.0.0/24, offers a client reserved in the second subnet its reservation
// with that subnet's server identifier, and offers an unreserved client an
// address of the subnet of lo's primary address.
// PREVENTS: the first subnet's handler claiming a client reserved in the
// second, which is what the first-answer dispatch did.
// METHOD: lo gets 10.99.0.1/24 beside 127.0.0.1/8 in the test's own network
// namespace. ciaddr carries the client's loopback address only so the OFFER
// is unicast back to it; the selection reads no hint ahead of a reservation.
func TestSharedSegmentReservationOverLoopback(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	second, err := netlink.ParseAddr("10.99.0.1/24")
	if err != nil {
		t.Fatalf("parse 10.99.0.1/24: %v", err)
	}
	if err := netlink.AddrAdd(lo, second); err != nil {
		t.Fatalf("add 10.99.0.1/24 to lo: %v", err)
	}

	reserved := net.HardwareAddr{0x02, 0, 0, 0, 0, 0x02}
	handlers := buildHandlers(serverConfig{Enabled: true, SharedNetworks: []sharedNetwork{{Name: "lan", Subnets: []subnetConfig{
		{
			Prefix:        netip.MustParsePrefix("127.0.0.0/8"),
			Ranges:        []addressRange{{Name: "r", Start: netip.MustParseAddr("127.0.0.50"), Stop: netip.MustParseAddr("127.0.0.60")}},
			LeaseTimeSec:  600,
			DefaultRouter: netip.MustParseAddr("127.0.0.1"),
		},
		{
			Prefix:         netip.MustParsePrefix("10.99.0.0/24"),
			Ranges:         []addressRange{{Name: "r", Start: netip.MustParseAddr("10.99.0.50"), Stop: netip.MustParseAddr("10.99.0.60")}},
			LeaseTimeSec:   600,
			DefaultRouter:  netip.MustParseAddr("10.99.0.1"),
			StaticMappings: []staticMapping{{Name: "printer", MAC: reserved, IP: netip.MustParseAddr("10.99.0.20")}},
		},
	}}}}, nil)
	defer func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	}()

	conn, err := listenDHCP("lo")
	if err != nil {
		t.Fatalf("bind lo:67: %v", err)
	}
	defer func() { _ = conn.Close() }()
	go serveMulti(conn, "lo", handlers, slogutil.DiscardLogger())

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: clientAddr.AsSlice(), Port: 68})
	if err != nil {
		t.Fatalf("bind client 127.0.0.2:68: %v", err)
	}
	defer func() { _ = cli.Close() }()

	for _, tc := range []struct {
		mac      net.HardwareAddr
		yiaddr   func(netip.Addr) bool
		want     string
		serverID string
	}{
		{reserved, func(a netip.Addr) bool { return a == netip.MustParseAddr("10.99.0.20") }, "the reservation 10.99.0.20", "10.99.0.1"},
		{net.HardwareAddr{0x02, 0, 0, 0, 0, 0x03}, netip.MustParsePrefix("127.0.0.0/8").Contains, "an address of 127.0.0.0/8", "127.0.0.1"},
	} {
		if _, err := cli.WriteToUDP(buildLoopbackDiscover(tc.mac, clientAddr, 0), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 67}); err != nil {
			t.Fatalf("send DISCOVER: %v", err)
		}
		if err := cli.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		buf := make([]byte, 1500)
		n, _, err := cli.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("%s: read OFFER: %v", tc.mac, err)
		}
		offer := buf[:n]
		if got := parseMsgType(offer); got != msgOffer {
			t.Fatalf("%s: message type %d, want OFFER", tc.mac, got)
		}
		if got := netip.AddrFrom4([4]byte(offer[16:20])); !tc.yiaddr(got) {
			t.Errorf("%s: yiaddr %v, want %s", tc.mac, got, tc.want)
		}
		if got := parseOptionAddr(offer, optServerID); got != netip.MustParseAddr(tc.serverID) {
			t.Errorf("%s: server identifier %v, want %s", tc.mac, got, tc.serverID)
		}
	}
}
