//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- original transport/channel capture
// Related: native_channel_owner_fixture_integration_linux_test.go -- packet and wire observations
// Frozen C ABI: 0011 plus REQUIRED0011a, after0007 through0010.
// This fixture deliberately never invokes PPPIOCGCHAN or PPPIOCATTCHAN.

package kernel_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	nativePPPoXCaptureCommand = 0x4010742d
	nativePPPoXCaptureQuery   = 0xc014742c
	nativePPPoXCaptureBit     = 1 << 8
)

// Exact compat-safe struct ppp_transport_owner:20 bytes, alignment four.
type nativePPPoXCaptureReply struct {
	abi      uint32
	features uint32
	channel  int32
	unit     int32
	reserved uint32
}

type nativePPPoXCaptureFixture struct {
	channel *nativeChannelFixture
	link    netlink.Link
	name    string
}

// nativePPPoXCaptureNew creates only an actual PPPoE/veth transport. It MUST
// remain uncaptured until the test calls the new native ioctl. Its raw fds and
// link are owned by this test; no production Go owner type is assumed.
func nativePPPoXCaptureNew(t *testing.T, slot int) *nativePPPoXCaptureFixture {
	t.Helper()
	local := net.HardwareAddr{2, 0, 1, 0, byte(slot), 1}
	peer := net.HardwareAddr{2, 0, 1, 0, byte(slot), 2}
	name := fmt.Sprintf("pc%da", slot)
	peerName := fmt.Sprintf("pc%db", slot)
	link := &netlink.Veth{Name: name, HardwareAddr: local, PeerName: peerName, PeerHardwareAddr: peer}
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup.Close() })
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
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
	channel := &nativeChannelFixture{fd: -1, transport: -1, wire: -1, index: -1, peerIndex: peerLink.Attrs().Index, sid: uint16(800 + slot), localMAC: local, peerMAC: peer}
	fixture := &nativePPPoXCaptureFixture{channel: channel, link: localLink, name: name}
	t.Cleanup(func() {
		if channel.transport >= 0 {
			nativeOwnerClose(t, channel.transport)
		}
		if channel.fd >= 0 {
			nativeOwnerClose(t, channel.fd)
		}
		if channel.wire >= 0 {
			nativeOwnerClose(t, channel.wire)
		}
	})
	channel.wire, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0x6488)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Bind(channel.wire, &unix.SockaddrLinklayer{Protocol: 0x6488, Ifindex: channel.peerIndex}); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptInt(channel.wire, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatal(err)
	}
	channel.transport, err = unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Connect(channel.transport, &unix.SockaddrPPPoE{SID: channel.sid, Remote: peer, Dev: name}); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func nativePPPoXCaptureRead(fd int, reply *nativePPPoXCaptureReply) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nativePPPoXCaptureQuery, uintptr(unsafe.Pointer(reply))) //nolint:gosec // frozen native UAPI.
	if errno != 0 {
		return errno
	}
	return nil
}

func nativePPPoXCaptureRequest(transport int, peer *nativeChannelPeer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(transport), nativePPPoXCaptureCommand, uintptr(unsafe.Pointer(peer))) //nolint:gosec // frozen native UAPI.
	if errno != 0 {
		return errno
	}
	return nil
}

func nativePPPoXCaptureInto(transport, destination int) error {
	request := nativeChannelPeer{abi: nativeOwnerABI, fd: int32(destination)}
	return nativePPPoXCaptureRequest(transport, &request)
}

func nativePPPoXCaptureProbe(t *testing.T, transport, expectedUnit int) nativePPPoXCaptureReply {
	t.Helper()
	reply := nativePPPoXCaptureReply{abi: nativeOwnerABI}
	if err := nativePPPoXCaptureRead(transport, &reply); err != nil {
		t.Fatalf("0011 positive original-transport receipt before capture: %v (ENOTTY/missing CAPTURE is semantic RED, never numeric fallback)", err)
	}
	want := uint32(nativePPPoXCaptureBit | nativeChannelConnectBit | nativeChannelBridgeBit)
	if reply.abi != nativeOwnerABI || reply.features != want || reply.reserved != 0 {
		t.Fatalf("original transport receipt=%+v, want ABI1/features%#x/reserved0", reply, want)
	}
	if reply.channel <= 0 || reply.unit != int32(expectedUnit) {
		t.Fatalf("original native relationship labels=%+v, expected attached unit %d", reply, expectedUnit)
	}
	return reply
}

func nativePPPoXCaptureOpen(t *testing.T) int {
	t.Helper()
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, fd) })
	return fd
}

// nativePPPoXCaptureBind transfers one test-owned destination into the existing
// fixture cleanup. The diagnostic label is checked only after native binding.
func nativePPPoXCaptureBind(t *testing.T, fixture *nativePPPoXCaptureFixture) {
	t.Helper()
	if fixture.channel.fd >= 0 {
		t.Fatal("fixture already has a destination owner")
	}
	reply := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	fixture.channel.fd = fd
	if err := nativePPPoXCaptureInto(fixture.channel.transport, fd); err != nil {
		t.Fatalf("atomic original transport capture: %v", err)
	}
	fixture.channel.index = int(reply.channel)
	nativeChannelReady(t, fixture.channel)
}

func nativePPPoXCapturePeek(t *testing.T, transport int, payload []byte) {
	t.Helper()
	nativeChannelReadable(t, transport)
	var buffer [128]byte
	count, _, err := unix.Recvfrom(transport, buffer[:], unix.MSG_PEEK|unix.MSG_DONTWAIT)
	if err != nil {
		t.Fatalf("uncaptured transport is no longer readable: %v", err)
	}
	if !bytes.Equal(buffer[:count], payload) {
		t.Fatalf("transport queued=%x, want exact PPP payload %x", buffer[:count], payload)
	}
}

func nativePPPoXCaptureReadExact(t *testing.T, fd int, payload []byte) {
	t.Helper()
	nativeChannelReadable(t, fd)
	var buffer [128]byte
	count, err := unix.Read(fd, buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:count], payload) {
		t.Fatalf("captured queue=%x, want exact PPP payload %x", buffer[:count], payload)
	}
}

func nativePPPoXCaptureQueueEmpty(t *testing.T, fd int) {
	t.Helper()
	var buffer [128]byte
	_, err := unix.Read(fd, buffer[:])
	nativeOwnerErrno(t, err, unix.EAGAIN)
}

func nativePPPoXCaptureWire(t *testing.T, channel *nativeChannelFixture, marker byte) {
	t.Helper()
	payload := nativeChannelPayload(marker)
	count, err := unix.Write(channel.fd, payload)
	if err != nil || count != len(payload) {
		t.Fatalf("actual captured-channel write=%d,error=%v", count, err)
	}
	nativeChannelReadable(t, channel.wire)
	var buffer [2048]byte
	count, _, err = unix.Recvfrom(channel.wire, buffer[:], 0)
	if err != nil {
		t.Fatal(err)
	}
	if count < 20+len(payload) {
		t.Fatalf("actual captured-channel wire frame too short: %x", buffer[:count])
	}
	if !bytes.Equal(buffer[:6], channel.peerMAC) || !bytes.Equal(buffer[6:12], channel.localMAC) ||
		binary.BigEndian.Uint16(buffer[12:14]) != 0x8864 ||
		!bytes.Equal(buffer[14:16], []byte{0x11, 0}) ||
		binary.BigEndian.Uint16(buffer[16:18]) != channel.sid ||
		int(binary.BigEndian.Uint16(buffer[18:20])) != len(payload) ||
		!bytes.Equal(buffer[20:20+len(payload)], payload) {
		t.Fatalf("captured channel changed native Ethernet/PPPoE/PPP frame: %x", buffer[:count])
	}
}

func nativePPPoXCaptureReconnect(t *testing.T, fixture *nativePPPoXCaptureFixture) {
	t.Helper()
	fixture.channel.sid++
	if err := unix.Connect(fixture.channel.transport, &unix.SockaddrPPPoE{SID: fixture.channel.sid, Remote: fixture.channel.peerMAC, Dev: fixture.name}); err != nil {
		t.Fatalf("real same-socket PPPoE reconnect: %v", err)
	}
}

// Drop only this locked thread's effective CAP_NET_ADMIN while retaining the
// permitted set for restoration. MUST restore before returning or failing the
// test, and MUST unlock the OS thread only after restoration has succeeded.
func nativePPPoXCaptureWithoutAdmin(t *testing.T, operation func() error) error {
	t.Helper()
	runtime.LockOSThread()
	unlock := true
	defer func() {
		if unlock {
			runtime.UnlockOSThread()
		}
	}()
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var original [2]unix.CapUserData
	if err := unix.Capget(&header, &original[0]); err != nil {
		t.Fatal(err)
	}
	const mask = uint32(1 << unix.CAP_NET_ADMIN)
	if original[0].Effective&mask == 0 {
		t.Fatal("capability negative lacks its privileged positive prerequisite")
	}
	dropped := original
	dropped[0].Effective &^= mask
	if err := unix.Capset(&header, &dropped[0]); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Capset(&header, &original[0]); err != nil {
			// Goexit on a still-locked goroutine retires its OS thread.
			// Never return altered credentials to the runtime's thread pool.
			unlock = false
			t.Fatalf("restore test thread capability before further native operations: %v", err)
		}
	}()
	return operation()
}

func nativePPPoXCaptureRaceError(t *testing.T, err error, allowed ...unix.Errno) {
	t.Helper()
	if err == nil {
		return
	}
	for _, candidate := range allowed {
		if errors.Is(err, candidate) {
			return
		}
	}
	t.Fatalf("unexpected native capture race result: %v", err)
}
