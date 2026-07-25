// Design: plan/learned/706-cpe-2-dhcp-server.md -- integration coverage for SO_BINDTODEVICE
//
// These tests pin socket_linux.go (listenDHCP): they exercise the real
// SO_BINDTODEVICE syscall and a real DISCOVER/OFFER exchange over UDP, asserting
// the PXE boot options (RFC 4578 / RFC 2132) the installer relies on, plus the
// device-filtering behavior the option provides. They require root (ports 67/68
// are privileged) and, for the negative case, CAP_NET_ADMIN. Both are present in
// the QEMU Alpine VM (see ai/rules/qemu-testing.md); otherwise the tests t.Skip.
//
// See socket_integration_linux_test.go in ../tftpserver for why the positive
// round-trip binds to "lo": locally-routed traffic ingresses on the loopback
// device, so only an lo-bound socket can both bind and receive it within one
// namespace. The OFFER is delivered by unicast to ciaddr:68 (responseAddr), so
// the DISCOVER carries ciaddr = the client's loopback address.

//go:build integration && linux

package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/slogutil"
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

// testHandler builds a PXE-enabled handler for the loopback subnet. serverIP and
// the TFTP server are deliberately distinct so the siaddr assertion proves the
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
	return newDHCPHandler(sub, netip.MustParseAddr("127.0.0.1"), pxe)
}

// createDummyForTest creates a dummy interface and registers cleanup, skipping
// the test if CAP_NET_ADMIN is unavailable.
func createDummyForTest(t *testing.T, name string) {
	t.Helper()
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("requires CAP_NET_ADMIN: create dummy %q: %v", name, err)
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
	conn, err := listenDHCP("lo")
	if err != nil {
		t.Skipf("cannot bind lo:67 (needs root): %v", err)
	}
	defer func() { _ = conn.Close() }()

	go serveMulti(conn, []*dhcpHandler{testHandler()}, slogutil.DiscardLogger())

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: clientAddr.AsSlice(), Port: 68})
	if err != nil {
		t.Skipf("cannot bind client 127.0.0.2:68 (needs root): %v", err)
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
	createDummyForTest(t, "zedhcpd0")

	conn, err := listenDHCP("zedhcpd0")
	if err != nil {
		t.Skipf("cannot bind zedhcpd0:67 (needs root): %v", err)
	}
	defer func() { _ = conn.Close() }()

	go serveMulti(conn, []*dhcpHandler{testHandler()}, slogutil.DiscardLogger())

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: clientAddr.AsSlice(), Port: 68})
	if err != nil {
		t.Skipf("cannot bind client 127.0.0.2:68 (needs root): %v", err)
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
