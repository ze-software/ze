//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- atomic original PPPoX capture
// Related: native_pppox_capture_fixture_integration_linux_test.go
// Native TDD only: frozen0011+0011a, never a future production Go API.
// Without0011 every root must fail its initial positive transport receipt.
// Numeric channel labels are readback only and never select an operation.

package kernel_test

import (
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

func TestNativePPPoXCaptureProbePreservesQueuedFrame(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	if unsafe.Sizeof(nativePPPoXCaptureReply{}) != 20 || unsafe.Alignof(nativePPPoXCaptureReply{}) != 4 {
		t.Fatal("transport query is not fixed20/alignment4")
	}
	if unsafe.Sizeof(nativeChannelPeer{}) != 16 || unsafe.Alignof(nativeChannelPeer{}) != 4 {
		t.Fatal("capture peer is not fixed16/alignment4")
	}
	before := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	payload := nativeChannelPayload(101)
	fixture.channel.inject(t, payload)
	nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
	for _, bad := range []nativePPPoXCaptureReply{
		{abi: nativeOwnerABI + 1},
		{abi: nativeOwnerABI, features: 1},
		{abi: nativeOwnerABI, channel: 1},
		{abi: nativeOwnerABI, unit: 1},
		{abi: nativeOwnerABI, reserved: 1},
	} {
		nativeOwnerErrno(t, nativePPPoXCaptureRead(fixture.channel.transport, &bad), unix.EINVAL)
		nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
	}
	after := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	if before != after {
		t.Fatalf("non-mutating probe changed original relationship: before%+v after%+v", before, after)
	}
	nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
	nativePPPoXCaptureBind(t, fixture)
	nativePPPoXCaptureReadExact(t, fixture.channel.fd, payload)
	nativePPPoXCaptureQueueEmpty(t, fixture.channel.fd)
	var buffer [128]byte
	_, _, err := unix.Recvfrom(fixture.channel.transport, buffer[:], unix.MSG_DONTWAIT)
	nativeOwnerErrno(t, err, unix.EIO)
	nativeChannelQueueFlow(t, fixture.channel, fixture.channel.fd, 102)
	nativePPPoXCaptureWire(t, fixture.channel, 103)
}

func TestNativePPPoXCaptureDuplicatesAndNativeRelationship(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	reply := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	unit := nativeOwnerFixtureNew(t)
	transportDup := nativeChannelDuplicate(t, fixture.channel.transport)
	destination := nativePPPoXCaptureOpen(t)
	if err := nativePPPoXCaptureInto(transportDup, destination); err != nil {
		t.Fatal(err)
	}
	fixture.channel.index = int(reply.channel)
	channelDup := nativeChannelDuplicate(t, destination)
	channelReply := nativeChannelReply{abi: nativeOwnerABI}
	if err := nativeChannelReadOwner(channelDup, &channelReply); err != nil {
		t.Fatal(err)
	}
	if channelReply.channel != reply.channel {
		t.Fatalf("captured duplicate relationship changed: %+v", channelReply)
	}
	nativeChannelRequireOperation(t, channelDup, nativeChannelConnect, unit.fd)
	nativePPPoXCaptureProbe(t, transportDup, unit.number)
	nativeChannelQueueFlow(t, fixture.channel, unit.fd, 104)
	nativeChannelRequireOperation(t, destination, nativeChannelDisconnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	nativeChannelQueueFlow(t, fixture.channel, channelDup, 105)
}

func TestNativePPPoXCaptureFDZero(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	reply := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	destination := nativePPPoXCaptureOpen(t)
	saved, err := unix.FcntlInt(0, unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		t.Fatalf("save dedicated child stdin before fd0 proof: %v", err)
	}
	defer nativeOwnerClose(t, saved)
	originalFlags, err := unix.FcntlInt(0, unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	restoreFlags := 0
	if originalFlags&unix.FD_CLOEXEC != 0 {
		restoreFlags = unix.O_CLOEXEC
	}
	if err := unix.Dup3(destination, 0, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Dup3(saved, 0, restoreFlags); err != nil {
			t.Errorf("restore owned child stdin: %v", err)
		}
	}()
	if err := nativePPPoXCaptureInto(fixture.channel.transport, 0); err != nil {
		t.Fatalf("fd0 atomic capture refused: %v", err)
	}
	readback := nativeChannelReply{abi: nativeOwnerABI}
	if err := nativeChannelReadOwner(0, &readback); err != nil {
		t.Fatal(err)
	}
	if readback.channel != reply.channel {
		t.Fatalf("fd0 bound wrong original: %+v", readback)
	}
	nativeChannelQueueFlow(t, fixture.channel, 0, 106)
	nativeChannelQueueFlow(t, fixture.channel, destination, 107)
}

func TestNativePPPoXCaptureMalformedLeavesTransportUnbound(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	destination := nativePPPoXCaptureOpen(t)
	unit := nativeOwnerFixtureNew(t)
	busy := nativePPPoXCaptureNew(t, 1)
	nativePPPoXCaptureBind(t, busy)
	nonPPP, err := unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer nativeOwnerClose(t, nonPPP)
	closed, err := unix.FcntlInt(uintptr(destination), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	nativeOwnerClose(t, closed)
	payload := nativeChannelPayload(108)
	fixture.channel.inject(t, payload)
	cases := []struct {
		name  string
		peer  nativeChannelPeer
		errno unix.Errno
	}{
		{"abi", nativeChannelPeer{abi: nativeOwnerABI + 1, fd: int32(destination)}, unix.EINVAL},
		{"flags", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(destination), flags: 1}, unix.EINVAL},
		{"reserved", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(destination), reserved: 1}, unix.EINVAL},
		{"negative", nativeChannelPeer{abi: nativeOwnerABI, fd: -1}, unix.EINVAL},
		{"closed", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(closed)}, unix.EBADF},
		{"non-ppp", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(nonPPP)}, unix.EBADF},
		{"transport-kind", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(fixture.channel.transport)}, unix.EBADF},
		{"bound-channel", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(busy.channel.fd)}, unix.EBUSY},
		{"bound-unit", nativeChannelPeer{abi: nativeOwnerABI, fd: int32(unit.fd)}, unix.EBUSY},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nativeOwnerErrno(t, nativePPPoXCaptureRequest(fixture.channel.transport, &c.peer), c.errno)
			nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
			nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
		})
	}
	if err := nativePPPoXCaptureInto(fixture.channel.transport, destination); err != nil {
		t.Fatal(err)
	}
	nativePPPoXCaptureReadExact(t, destination, payload)
	nativePPPoXCaptureQueueEmpty(t, destination)
	nativeChannelQueueFlow(t, busy.channel, busy.channel.fd, 109)
	unit.probe(t, unit.fd)
}

func TestNativePPPoXCaptureBusyDestinationKeepsActualChannel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativePPPoXCaptureNew(t, 0)
	second := nativePPPoXCaptureNew(t, 1)
	nativePPPoXCaptureBind(t, first)
	nativePPPoXCaptureProbe(t, second.channel.transport, -1)
	payload := nativeChannelPayload(110)
	second.channel.inject(t, payload)
	nativeOwnerErrno(t, nativePPPoXCaptureInto(second.channel.transport, first.channel.fd), unix.EBUSY)
	nativePPPoXCapturePeek(t, second.channel.transport, payload)
	nativeChannelReady(t, first.channel)
	nativeChannelQueueFlow(t, first.channel, first.channel.fd, 111)
	nativePPPoXCaptureBind(t, second)
	nativePPPoXCaptureReadExact(t, second.channel.fd, payload)
	nativeChannelRequireOperation(t, first.channel.fd, nativeChannelBridge, second.channel.fd)
	nativeChannelBridgeFlow(t, first.channel, second.channel, 112)
	nativeChannelBridgeFlow(t, second.channel, first.channel, 113)
	nativeChannelRequireOperation(t, first.channel.fd, nativeChannelUnbridge, second.channel.fd)
}

func TestNativePPPoXCaptureTargetNamespacePermission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	destination := nativePPPoXCaptureOpen(t)
	payload := nativeChannelPayload(114)
	fixture.channel.inject(t, payload)
	nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
	err := nativePPPoXCaptureWithoutAdmin(t, func() error {
		return nativePPPoXCaptureInto(fixture.channel.transport, destination)
	})
	nativeOwnerErrno(t, err, unix.EPERM)
	nativePPPoXCapturePeek(t, fixture.channel.transport, payload)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	if err := nativePPPoXCaptureInto(fixture.channel.transport, destination); err != nil {
		t.Fatalf("restored target-net authority lost useful capture: %v", err)
	}
	nativePPPoXCaptureReadExact(t, destination, payload)
	nativeChannelQueueFlow(t, fixture.channel, destination, 115)
}

func TestNativePPPoXCaptureForeignCallerUsesSocketNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	destination := nativePPPoXCaptureOpen(t)
	var foreignUnit *nativeOwnerFixture
	createdUnitForeignNamespace(t, func() {
		foreignUnit = nativeOwnerFixtureNew(t)
		nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
		if err := nativePPPoXCaptureInto(fixture.channel.transport, destination); err != nil {
			t.Fatalf("foreign caller changed original socket namespace authority: %v", err)
		}
	})
	nativeOwnerErrno(t, nativeChannelOperate(destination, nativeChannelConnect, foreignUnit.fd), unix.EXDEV)
	nativeChannelQueueFlow(t, fixture.channel, destination, 116)
	localUnit := nativeOwnerFixtureNew(t)
	nativeChannelRequireOperation(t, destination, nativeChannelConnect, localUnit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, localUnit.number)
	nativeChannelQueueFlow(t, fixture.channel, localUnit.fd, 117)
}

func TestNativePPPoXCaptureDeviceDownSameSocketReconnect(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	nativePPPoXCaptureBind(t, fixture)
	unit := nativeOwnerFixtureNew(t)
	oldFD := fixture.channel.fd
	oldReply := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	nativeChannelRequireOperation(t, oldFD, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, fixture.channel, unit.fd, 118)
	if err := netlink.LinkSetDown(fixture.link); err != nil {
		t.Fatal(err)
	}
	dead := nativePPPoXCaptureReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativePPPoXCaptureRead(fixture.channel.transport, &dead), unix.ENOTCONN)
	freshDestination := nativePPPoXCaptureOpen(t)
	nativeOwnerErrno(t, nativePPPoXCaptureInto(fixture.channel.transport, freshDestination), unix.ENOTCONN)
	oldChannel := nativeChannelReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativeChannelReadOwner(oldFD, &oldChannel), unix.ENOTCONN)
	nativeChannelRequireOperation(t, oldFD, nativeChannelDisconnect, unit.fd)
	if err := netlink.LinkSetUp(fixture.link); err != nil {
		t.Fatal(err)
	}
	nativePPPoXCaptureReconnect(t, fixture)
	newReply := nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	if newReply.channel == oldReply.channel {
		t.Fatal("real reconnect did not create a distinguishable new channel label")
	}
	if err := nativePPPoXCaptureInto(fixture.channel.transport, freshDestination); err != nil {
		t.Fatal(err)
	}
	nativeOwnerErrno(t, nativeChannelOperate(oldFD, nativeChannelConnect, unit.fd), unix.ENOTCONN)
	nativeOwnerErrno(t, nativeChannelOperate(oldFD, nativeChannelBridge, freshDestination), unix.ENOTCONN)
	nativeChannelRequireOperation(t, freshDestination, nativeChannelConnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, unit.number)
	nativeChannelQueueFlow(t, fixture.channel, unit.fd, 119)
	nativeChannelRequireOperation(t, oldFD, nativeChannelDisconnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, unit.number)
	nativeChannelQueueFlow(t, fixture.channel, unit.fd, 120)
	nativeChannelRequireOperation(t, freshDestination, nativeChannelDisconnect, unit.fd)
	nativeChannelQueueFlow(t, fixture.channel, freshDestination, 121)
	unit.probe(t, unit.fd)
}

func TestNativePPPoXCaptureRetiredUnitCleanupReadback(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPoXCaptureNew(t, 0)
	nativePPPoXCaptureBind(t, fixture)
	unit := nativeOwnerFixtureNew(t)
	nativeChannelRequireOperation(t, fixture.channel.fd, nativeChannelConnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, unit.number)
	if err := netlink.LinkDel(unit.link); err != nil {
		t.Fatal(err)
	}
	// Unit unregister removes the IDR entry but does not detach its channels.
	nativePPPoXCaptureProbe(t, fixture.channel.transport, unit.number)
	replacement := nativeChannelUnitAt(t, unit.number)
	nativeOwnerErrno(t, nativeChannelOperate(fixture.channel.fd, nativeChannelDisconnect, replacement.fd), unix.ESTALE)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, unit.number)
	nativeChannelRequireOperation(t, fixture.channel.fd, nativeChannelDisconnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	nativeChannelRequireOperation(t, fixture.channel.fd, nativeChannelDisconnect, unit.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
	nativeChannelQueueFlow(t, fixture.channel, fixture.channel.fd, 122)
	nativeChannelRequireOperation(t, fixture.channel.fd, nativeChannelConnect, replacement.fd)
	nativePPPoXCaptureProbe(t, fixture.channel.transport, replacement.number)
	nativeChannelQueueFlow(t, fixture.channel, replacement.fd, 123)
}

func TestNativePPPoXCaptureUnregisterJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	control := nativePPPoXCaptureNew(t, 20)
	nativePPPoXCaptureBind(t, control)
	unit := nativeOwnerFixtureNew(t)
	for round := range 8 {
		fixture := nativePPPoXCaptureNew(t, round)
		nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
		destination := nativePPPoXCaptureOpen(t)
		captured, down := nativeChannelRunTwo(t,
			func() error { return nativePPPoXCaptureInto(fixture.channel.transport, destination) },
			func() error { return netlink.LinkSetDown(fixture.link) })
		if down != nil {
			t.Fatalf("actual device-down did not settle: %v", down)
		}
		nativePPPoXCaptureRaceError(t, captured, unix.ENOTCONN, unix.EBUSY)
		transport := nativePPPoXCaptureReply{abi: nativeOwnerABI}
		nativeOwnerErrno(t, nativePPPoXCaptureRead(fixture.channel.transport, &transport), unix.ENOTCONN)
		if captured == nil {
			channel := nativeChannelReply{abi: nativeOwnerABI}
			nativeOwnerErrno(t, nativeChannelReadOwner(destination, &channel), unix.ENOTCONN)
			nativeOwnerErrno(t, nativeChannelOperate(destination, nativeChannelConnect, unit.fd), unix.ENOTCONN)
			nativeChannelRequireOperation(t, destination, nativeChannelDisconnect, unit.fd)
		}
		nativeChannelQueueFlow(t, control.channel, control.channel.fd, byte(124+round))
	}
	nativeChannelRequireOperation(t, control.channel.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, control.channel, unit.fd, 132)
}

func TestNativePPPoXCaptureFinalTransportCloseJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	control := nativePPPoXCaptureNew(t, 20)
	nativePPPoXCaptureBind(t, control)
	unit := nativeOwnerFixtureNew(t)
	for round := range 8 {
		fixture := nativePPPoXCaptureNew(t, round)
		nativePPPoXCaptureProbe(t, fixture.channel.transport, -1)
		destination := nativePPPoXCaptureOpen(t)
		transport := fixture.channel.takeTransport()
		// No test-owned open or fd duplication occurs until BOTH actual
		// syscalls return. A failed fd acquisition is an explicit refusal.
		captured, closed := nativeChannelRunTwo(t,
			func() error { return nativePPPoXCaptureInto(transport, destination) },
			func() error { return unix.Close(transport) })
		if closed != nil {
			t.Fatalf("final original transport close did not settle: %v", closed)
		}
		nativePPPoXCaptureRaceError(t, captured, unix.EBADF, unix.ENOTCONN, unix.EBUSY)
		if captured == nil {
			channel := nativeChannelReply{abi: nativeOwnerABI}
			nativeOwnerErrno(t, nativeChannelReadOwner(destination, &channel), unix.ENOTCONN)
			nativeOwnerErrno(t, nativeChannelOperate(destination, nativeChannelConnect, unit.fd), unix.ENOTCONN)
			nativeChannelRequireOperation(t, destination, nativeChannelDisconnect, unit.fd)
		}
		nativeChannelQueueFlow(t, control.channel, control.channel.fd, byte(133+round))
	}
	nativeChannelRequireOperation(t, control.channel.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, control.channel, unit.fd, 141)
}
