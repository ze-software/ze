//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual original PPP channel operations
// Related: native_owner_guard_integration_linux_test.go -- original unit and errno prerequisites
// Source: Linux v7.3 pppoe_connect, pppox_ioctl, ppp_input and ppp_receive_nonmp_frame.
// Numeric channel handoff below is controlled fixture setup, not proof of the
// separate 0011 atomic transport-to-channel capture or product SessionHandle.

package kernel_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Test-local fixed 0010 UAPI: every field is 32 bits, size 16, alignment 4.
const (
	nativeChannelConnect          = 0x40107432
	nativeChannelDisconnect       = 0x40107431
	nativeChannelBridge           = 0x40107430
	nativeChannelUnbridge         = 0x4010742f
	nativeChannelQuery            = 0xc010742e
	nativeChannelConnectBit       = 1 << 6
	nativeChannelBridgeBit        = 1 << 7
	nativeChannelLegacyConnect    = 0x4004743a
	nativeChannelLegacyDisconnect = 0x7439
	nativeChannelLegacyBridge     = 0x40047435
	nativeChannelLegacyUnbridge   = 0x7434
)

type nativeChannelPeer struct {
	abi      uint32
	fd       int32
	flags    uint32
	reserved uint32
}

type nativeChannelReply struct {
	abi      uint32
	features uint32
	channel  int32
	reserved uint32
}

// nativeChannelFixture owns a real PPPoE transport, attached /dev/ppp file and
// a raw peer on its private veth. Only the test goroutines named at each call
// may run concurrently. Main-thread retirement transfers its transport fd once.
type nativeChannelFixture struct {
	fd        int
	transport int
	wire      int
	index     int
	peerIndex int
	sid       uint16
	localMAC  net.HardwareAddr
	peerMAC   net.HardwareAddr
}

func nativeChannelFixtureNew(t *testing.T, slot int) *nativeChannelFixture {
	t.Helper()
	local := net.HardwareAddr{2, 0, 0, 0, byte(slot), 1}
	peer := net.HardwareAddr{2, 0, 0, 0, byte(slot), 2}
	name := fmt.Sprintf("nc%da", slot)
	peerName := fmt.Sprintf("nc%db", slot)
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name, HardwareAddr: local}, PeerName: peerName, PeerHardwareAddr: peer}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer cleanup.Close()
		if err := cleanup.LinkDel(link); err != nil {
			t.Error(err)
		}
	})
	localLink, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	peerLink, err := netlink.LinkByName(peerName)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(localLink); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(peerLink); err != nil {
		t.Fatal(err)
	}
	fixture := &nativeChannelFixture{fd: -1, transport: -1, wire: -1, peerIndex: peerLink.Attrs().Index, sid: uint16(400 + slot), localMAC: local, peerMAC: peer}
	t.Cleanup(func() {
		if fixture.transport >= 0 {
			nativeOwnerClose(t, fixture.transport)
		}
		if fixture.fd >= 0 {
			nativeOwnerClose(t, fixture.fd)
		}
		if fixture.wire >= 0 {
			nativeOwnerClose(t, fixture.wire)
		}
	})
	fixture.wire, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0x6488)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Bind(fixture.wire, &unix.SockaddrLinklayer{Protocol: 0x6488, Ifindex: fixture.peerIndex}); err != nil {
		t.Fatal(err)
	}
	// Ignore our injected outgoing packets; only ingress from the kernel's
	// other veth end can satisfy a bridged-wire read.
	if err := unix.SetsockoptInt(fixture.wire, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatal(err)
	}
	fixture.transport, err = unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Connect(fixture.transport, &unix.SockaddrPPPoE{SID: fixture.sid, Remote: peer, Dev: name}); err != nil {
		t.Fatal(err)
	}
	fixture.index, err = unix.IoctlGetInt(fixture.transport, 0x80047437)
	if err != nil {
		t.Fatalf("fixture PPPIOCGCHAN genuine transport: %v", err)
	}
	if fixture.index <= 0 {
		t.Fatalf("genuine transport has invalid channel label %d", fixture.index)
	}
	fixture.fd, err = unix.Open("/dev/ppp", unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetPointerInt(fixture.fd, 0x40047438, fixture.index); err != nil {
		t.Fatalf("fixture attach actual held channel: %v", err)
	}
	nativeChannelReady(t, fixture)
	return fixture
}

func nativeChannelReady(t *testing.T, fixture *nativeChannelFixture) {
	t.Helper()
	reply := nativeChannelReply{abi: nativeOwnerABI}
	if err := nativeChannelReadOwner(fixture.fd, &reply); err != nil {
		t.Fatalf("0010 bound-channel ABI probe before guarded operations: %v", err)
	}
	want := uint32(nativeChannelConnectBit | nativeChannelBridgeBit)
	if reply.features != want {
		t.Fatalf("channel feature reply=%#x, want exactly %#x", reply.features, want)
	}
	if reply.abi != nativeOwnerABI {
		t.Fatalf("channel ABI changed to %d", reply.abi)
	}
	if reply.channel != int32(fixture.index) {
		t.Fatalf("channel output label=%d, actual fixture=%d", reply.channel, fixture.index)
	}
	if reply.reserved != 0 {
		t.Fatalf("channel query returned reserved bits %#x", reply.reserved)
	}
}

func nativeChannelReadOwner(fd int, reply *nativeChannelReply) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nativeChannelQuery, uintptr(unsafe.Pointer(reply))) //nolint:gosec // exact frozen UAPI layout.
	if errno != 0 {
		return errno
	}
	return nil
}

func nativeChannelRequest(fd int, command uintptr, peer *nativeChannelPeer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), command, uintptr(unsafe.Pointer(peer))) //nolint:gosec // exact frozen UAPI layout.
	if errno != 0 {
		return errno
	}
	return nil
}

func nativeChannelOperate(fd int, command uintptr, peerFD int) error {
	peer := nativeChannelPeer{abi: nativeOwnerABI, fd: int32(peerFD)}
	return nativeChannelRequest(fd, command, &peer)
}

func nativeChannelRequireOperation(t *testing.T, fd int, command uintptr, peerFD int) {
	t.Helper()
	if err := nativeChannelOperate(fd, command, peerFD); err != nil {
		t.Fatalf("native guarded channel command %#x: %v", command, err)
	}
}

func nativeChannelDuplicate(t *testing.T, fd int) int {
	t.Helper()
	duplicate, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, duplicate) })
	return duplicate
}

// The unknown, uncompressed PPP protocol 0x1235 reaches the attached unit's
// control queue; without a unit it reaches the channel queue. The actual v7.3
// ppp_input and ppp_receive_nonmp_frame branches decide this observable route.
func nativeChannelPayload(marker byte) []byte {
	return []byte{0x12, 0x35, 'o', 'w', 'n', 'e', 'r', marker}
}

func (fixture *nativeChannelFixture) inject(t *testing.T, payload []byte) {
	t.Helper()
	frame := make([]byte, 20+len(payload))
	copy(frame[:6], fixture.localMAC)
	copy(frame[6:12], fixture.peerMAC)
	binary.BigEndian.PutUint16(frame[12:14], 0x8864)
	frame[14] = 0x11
	binary.BigEndian.PutUint16(frame[16:18], fixture.sid)
	binary.BigEndian.PutUint16(frame[18:20], uint16(len(payload)))
	copy(frame[20:], payload)
	if err := unix.Sendto(fixture.wire, frame, 0, &unix.SockaddrLinklayer{Protocol: 0x6488, Ifindex: fixture.peerIndex, Halen: 6}); err != nil {
		t.Fatal(err)
	}
}

func nativeChannelReadable(t *testing.T, fd int) {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	if _, err := unix.Poll(fds, 3000); err != nil {
		t.Fatal(err)
	}
	if fds[0].Revents&unix.POLLIN == 0 {
		t.Fatalf("actual packet did not reach descriptor %d, events %#x", fd, fds[0].Revents)
	}
}

func nativeChannelQueueFlow(t *testing.T, fixture *nativeChannelFixture, target int, marker byte) {
	t.Helper()
	payload := nativeChannelPayload(marker)
	fixture.inject(t, payload)
	nativeChannelReadable(t, target)
	var buffer [128]byte
	count, err := unix.Read(target, buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:count], payload) {
		t.Fatalf("native queue received %x, want exact injected PPP frame %x", buffer[:count], payload)
	}
}

func nativeChannelBridgeFlow(t *testing.T, source, target *nativeChannelFixture, marker byte) {
	t.Helper()
	payload := nativeChannelPayload(marker)
	source.inject(t, payload)
	nativeChannelReadable(t, target.wire)
	var buffer [2048]byte
	count, _, err := unix.Recvfrom(target.wire, buffer[:], 0)
	if err != nil {
		t.Fatal(err)
	}
	if count < 20 {
		t.Fatalf("native bridged wire frame too short: %x", buffer[:count])
	}
	if !bytes.Equal(buffer[:6], target.peerMAC) {
		t.Fatalf("native bridge used wrong destination MAC: %x", buffer[:6])
	}
	if !bytes.Equal(buffer[6:12], target.localMAC) {
		t.Fatalf("native bridge used wrong source MAC: %x", buffer[6:12])
	}
	if binary.BigEndian.Uint16(buffer[12:14]) != 0x8864 {
		t.Fatalf("native bridge used wrong EtherType: %x", buffer[12:14])
	}
	if !bytes.Equal(buffer[14:16], []byte{0x11, 0}) {
		t.Fatalf("native bridge used wrong PPPoE header: %x", buffer[14:16])
	}
	if binary.BigEndian.Uint16(buffer[16:18]) != target.sid {
		t.Fatalf("native bridge used wrong session ID: %x", buffer[16:18])
	}
	length := int(binary.BigEndian.Uint16(buffer[18:20]))
	if length != len(payload) {
		t.Fatalf("native bridged PPP length=%d, want %d", length, len(payload))
	}
	if count < 20+length {
		t.Fatalf("native bridge truncated its PPP payload: %x", buffer[:count])
	}
	if !bytes.Equal(buffer[20:20+length], payload) {
		t.Fatalf("native bridge changed PPP payload: got %x, want %x", buffer[20:20+length], payload)
	}
}

// takeTransport transfers this test-owned raw descriptor to exactly one closer.
// MUST be called on the main test goroutine before starting that closer. That
// closer MUST be joined before test cleanup or any subsequent assertion.
func (fixture *nativeChannelFixture) takeTransport() int {
	fd := fixture.transport
	fixture.transport = -1
	return fd
}

func nativeChannelRetire(t *testing.T, fixture *nativeChannelFixture) {
	t.Helper()
	nativeOwnerClose(t, fixture.takeTransport())
}

// nativeChannelRunTwo owns two one-shot syscall workers, releases them from one
// barrier, and receives both terminal results. Its timer is only a deadlock
// watchdog; success requires physical receipt of both completed syscall results.
func nativeChannelRunTwo(t *testing.T, first, second func() error) (error, error) {
	t.Helper()
	start := make(chan struct{})
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { <-start; firstDone <- first() }()
	go func() { <-start; secondDone <- second() }()
	close(start)
	watchdog := time.NewTimer(10 * time.Second)
	defer watchdog.Stop()
	var firstErr, secondErr error
	select {
	case firstErr = <-firstDone:
	case <-watchdog.C:
		t.Fatal("first native syscall did not join")
	}
	select {
	case secondErr = <-secondDone:
	case <-watchdog.C:
		t.Fatal("second native syscall did not join")
	}
	return firstErr, secondErr
}

func nativeChannelUnitAt(t *testing.T, number int) *nativeOwnerFixture {
	t.Helper()
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, fd) })
	actual := int32(number)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc004743e, uintptr(unsafe.Pointer(&actual))) //nolint:gosec // explicit test-owned PPPIOCNEWUNIT reuse.
	if errno != 0 {
		t.Fatalf("create genuine replacement PPP unit %d: %v", number, errno)
	}
	if actual != int32(number) {
		t.Fatalf("replacement unit label=%d, want %d", actual, number)
	}
	link, err := netlink.LinkByName(fmt.Sprintf("ppp%d", actual))
	if err != nil {
		t.Fatal(err)
	}
	fixture := &nativeOwnerFixture{fd: fd, socket: nativeOwnerSocket(t), number: number, link: link}
	fixture.probe(t, fd)
	return fixture
}
