// Design: docs/architecture/appliance/kernel-profiles.md -- owned UDP6 receive keeps the PPP RX receipt
// Related: created_unit_control_socket_fixture_integration_linux_test.go -- actual original-unit control socket fixtures
// Related: native_channel_owner_fixture_integration_linux_test.go -- real PPPoE channel and peer wire injection

//go:build integration && linux

package kernel_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	controlSocketReceiveWait       = 5 * time.Second
	controlSocketReceiveClientPort = 546
	// The carried kernel stamps an RX receipt only on UDP to this port
	// (ppp_control_rx_candidate), so the datagram is DHCPv6-addressed.
	controlSocketReceiveServerPort = 547
)

// TestCreatedUnitControlSocketNativeUDP6ReceivesPeerDatagram proves an owned
// UDP6 control socket reads a datagram its PPP peer sent over the unit's link.
//
// VALIDATES: a real PPPoE frame carrying IPv6/UDP to port 547 enters the
// original unit, gets its RX receipt at PPP input, and reaches recvmsg on the
// owned socket with its exact payload and link-local source.
// PREVENTS: UDP queueing erasing that receipt (udp_try_make_stateless reset
// every skb extension), after which the owned recvmsg dropped every datagram.
func TestCreatedUnitControlSocketNativeUDP6ReceivesPeerDatagram(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, backend, link := controlSocketNativeUnitWithBackend(t, "zcudp6rx")
	operations := controlSocketReceiveOperations(t, unit, backend)
	channel := nativeChannelFixtureNew(t, 9)
	if err := unit.WithDescriptor(func(fd int) error {
		if err := nativeChannelOperate(channel.fd, nativeChannelConnect, fd); err != nil {
			return err
		}
		return pppkernel.SetIPv6Mode(fd, pppkernel.NPModePass)
	}); err != nil {
		t.Fatalf("connect the real PPPoE channel to the original unit: %v", err)
	}
	if err := operations.SetAdminUp(t.Context()); err != nil {
		t.Fatalf("set the original unit up: %v", err)
	}
	local := netip.AddrPortFrom(netip.MustParseAddr("fe80::1"), controlSocketReceiveServerPort)
	peer := netip.AddrPortFrom(netip.MustParseAddr("fe80::2"), controlSocketReceiveClientPort)
	if err := operations.AddAddressP2P(t.Context(), netip.PrefixFrom(local.Addr(), 64), netip.PrefixFrom(peer.Addr(), 64)); err != nil {
		t.Fatalf("install the original unit's IPv6 pair: %v", err)
	}
	controlSocketReceiveAddressReady(t, link, local.Addr())
	// Opening the owned socket gives the unit its control owner, which PPP
	// input needs before it stamps the receipt.
	conn := controlSocketReceiveConn(t, controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6))
	payload := []byte{1, 0, 0x5a, 0x7e, 'o', 'w', 'n', 'e', 'd'}
	channel.inject(t, append([]byte{0x00, 0x57}, controlSocketReceivePacket(peer, local, payload)...))
	if err := conn.SetReadDeadline(time.Now().Add(controlSocketReceiveWait)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 256)
	count, source, err := conn.ReadFromUDPAddrPort(buffer)
	if err != nil {
		t.Fatalf("owned UDP6 socket read no datagram from its PPP peer within %s: %v "+
			"(a datagram whose RX receipt UDP queueing erased is dropped at recvmsg)", controlSocketReceiveWait, err)
	}
	if !bytes.Equal(buffer[:count], payload) {
		t.Fatalf("owned UDP6 socket read %x, want the peer's exact payload %x", buffer[:count], payload)
	}
	if source.Addr().WithZone("") != peer.Addr() {
		t.Fatalf("owned UDP6 datagram source address %s, want %s", source.Addr(), peer.Addr())
	}
	if source.Port() != peer.Port() {
		t.Fatalf("owned UDP6 datagram source port %d, want %d", source.Port(), peer.Port())
	}
	zone := source.Addr().Zone()
	if zone != link.Attrs().Name && zone != strconv.Itoa(link.Attrs().Index) {
		t.Fatalf("owned UDP6 datagram arrived on zone %q, want the original unit %s", zone, link.Attrs().Name)
	}
}

// controlSocketReceiveOperations binds the unit's guarded link writer from its
// own creation backend. Its cleanup closes the unit file first, because
// CloseNativeOperations MUST follow that closure; the fixture's later Close
// then returns the recorded result.
func controlSocketReceiveOperations(t *testing.T, unit *pppkernel.CreatedUnit, backend iface.Backend) pppkernel.NativeUnitOperations {
	t.Helper()
	provider, ok := backend.(iface.NativePPPBackend)
	if !ok {
		t.Fatal("actual original backend has no native PPP writer")
	}
	operations, err := unit.BindNativeOperations(t.Context(), backend, func(owner *pppkernel.CreatedUnit, index int, number int32) (pppkernel.NativeUnitOperations, error) {
		return provider.BindNativePPPUnit(owner, index, number)
	})
	t.Cleanup(func() {
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), controlSocketReceiveWait)
		defer cancel()
		if err := unit.CloseNativeOperations(ctx); err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Fatalf("bind the original unit's native writer: %v", err)
	}
	return operations
}

// controlSocketReceiveAddressReady waits until the exact local address leaves
// TENTATIVE. Linux v7.3 addrconf_dad_start queues DAD work even for a NOARP
// PPP device, and the worker clears the flag; a tentative address accepts no
// unicast, so the install ACK alone does not make the address usable.
func controlSocketReceiveAddressReady(t *testing.T, link netlink.Link, local netip.Addr) {
	t.Helper()
	const interval = 10 * time.Millisecond
	const attempts = int(controlSocketReceiveWait/interval) + 1
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var addresses []netlink.Addr
	for range attempts {
		var err error
		addresses, err = netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			t.Fatalf("read the original unit's IPv6 addresses: %v", err)
		}
		for _, address := range addresses {
			actual, valid := netip.AddrFromSlice(address.IP)
			if !valid {
				continue
			}
			if actual != local {
				continue
			}
			if address.Flags&unix.IFA_F_DADFAILED != 0 {
				t.Fatalf("original unit address %s failed DAD: %+v", local, address)
			}
			if address.Flags&unix.IFA_F_TENTATIVE == 0 {
				return
			}
		}
		<-ticker.C
	}
	t.Fatalf("original unit address %s did not leave TENTATIVE within %s; addresses: %+v", local, controlSocketReceiveWait, addresses)
}

// controlSocketReceiveConn binds the owned socket to the DHCPv6 server port,
// as the production carrier does, and converts it to a UDP connection the
// test closes. The owned file keeps its own close in the fixture.
func controlSocketReceiveConn(t *testing.T, file *os.File) *net.UDPConn {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var bindErr error
	controlErr := raw.Control(func(fd uintptr) {
		bindErr = unix.Bind(int(fd), &unix.SockaddrInet6{Port: controlSocketReceiveServerPort})
	})
	if err := errors.Join(bindErr, controlErr); err != nil {
		t.Fatalf("bind the owned UDP6 socket to port %d: %v", controlSocketReceiveServerPort, err)
	}
	packet, err := net.FilePacketConn(file)
	if err != nil {
		t.Fatalf("convert the owned UDP6 file: %v", err)
	}
	t.Cleanup(func() {
		if err := packet.Close(); err != nil {
			t.Error(err)
		}
	})
	conn, ok := packet.(*net.UDPConn)
	if !ok {
		t.Fatalf("owned UDP6 file converted to %T, want *net.UDPConn", packet)
	}
	return conn
}

// controlSocketReceivePacket builds the peer's IPv6 header and UDP datagram,
// with the UDP checksum over the IPv6 pseudo-header.
func controlSocketReceivePacket(source, destination netip.AddrPort, payload []byte) []byte {
	packet := make([]byte, 48+len(payload))
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(8+len(payload)))
	packet[6], packet[7] = unix.IPPROTO_UDP, 64
	sourceAddress, destinationAddress := source.Addr().As16(), destination.Addr().As16()
	copy(packet[8:24], sourceAddress[:])
	copy(packet[24:40], destinationAddress[:])
	datagram := packet[40:]
	binary.BigEndian.PutUint16(datagram[0:2], source.Port())
	binary.BigEndian.PutUint16(datagram[2:4], destination.Port())
	binary.BigEndian.PutUint16(datagram[4:6], uint16(len(datagram)))
	copy(datagram[8:], payload)
	sum := uint32(len(datagram)) + unix.IPPROTO_UDP
	for offset := 8; offset < 40; offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(packet[offset:]))
	}
	for offset := 0; offset+1 < len(datagram); offset += 2 {
		sum += uint32(binary.BigEndian.Uint16(datagram[offset:]))
	}
	if len(datagram)%2 != 0 {
		sum += uint32(datagram[len(datagram)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	checksum := ^uint16(sum)
	// IPv6 forbids a zero UDP checksum; a computed zero is sent as all ones.
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(datagram[6:8], checksum)
	return packet
}
