//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual original PPP channel operations
// Related: native_channel_owner_fixture_integration_linux_test.go -- real PPPoE/veth/channel fixture
// UAPI: Linux v7.3 plus frozen original-file patches 0007 through 0010.
// Numeric channel attachment is test-controlled fixture setup. These tests do
// not establish 0011 atomic transport capture or production SessionHandle wiring.

package kernel_test

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestNativeChannelOwnerProbe requires the channel capability receipt before
// guarded operations, checks the frozen compat-safe layout, and proves the
// query leaves the real unconnected channel's packet route unchanged.
func TestNativeChannelOwnerProbe(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	if unsafe.Sizeof(nativeChannelPeer{}) != 16 {
		t.Fatal("peer request size is not the frozen 16 bytes")
	}
	if unsafe.Alignof(nativeChannelPeer{}) != 4 {
		t.Fatal("peer request alignment is not four bytes")
	}
	if unsafe.Sizeof(nativeChannelReply{}) != 16 {
		t.Fatal("channel query size is not the frozen 16 bytes")
	}
	if unsafe.Alignof(nativeChannelReply{}) != 4 {
		t.Fatal("channel query alignment is not four bytes")
	}
	reply := unit.expected()
	if err := nativeOwnerProbe(unit.fd, &reply); err != nil {
		t.Fatal(err)
	}
	want := uint32(nativeChannelConnectBit | nativeChannelBridgeBit)
	if reply.features&want != want {
		t.Fatalf("unit capability receipt lacks 0010 operations: got %#x need %#x", reply.features, want)
	}
	for _, change := range []struct {
		name  string
		apply func(*nativeChannelReply)
	}{
		{"abi", func(r *nativeChannelReply) { r.abi++ }},
		{"features", func(r *nativeChannelReply) { r.features = 1 }},
		{"channel", func(r *nativeChannelReply) { r.channel = 1 }},
		{"reserved", func(r *nativeChannelReply) { r.reserved = 1 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			r := nativeChannelReply{abi: nativeOwnerABI}
			change.apply(&r)
			nativeOwnerErrno(t, nativeChannelReadOwner(channel.fd, &r), unix.EINVAL)
		})
	}
	wrong := nativeChannelReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativeChannelReadOwner(unit.fd, &wrong), unix.EBADF)
	nativeChannelReady(t, channel)
	nativeChannelQueueFlow(t, channel, channel.fd, 1)
	unit.probe(t, unit.fd)
}

// TestNativeChannelOwnerConnectLifecycle proves creator duplicates authorize
// the actual connect/disconnect producers and that legacy numeric commands
// still route useful packets into, then out of, the same held unit.
func TestNativeChannelOwnerConnectLifecycle(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	creator := nativeChannelDuplicate(t, unit.fd)
	channelDuplicate := nativeChannelDuplicate(t, channel.fd)
	nativeChannelRequireOperation(t, channelDuplicate, nativeChannelConnect, creator)
	nativeChannelQueueFlow(t, channel, unit.fd, 2)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, unit.fd)
	nativeChannelRequireOperation(t, channelDuplicate, nativeChannelDisconnect, creator)
	nativeChannelQueueFlow(t, channel, channel.fd, 3)
	if err := unix.IoctlSetPointerInt(channel.fd, nativeChannelLegacyConnect, unit.number); err != nil {
		t.Fatal(err)
	}
	nativeChannelQueueFlow(t, channel, unit.fd, 4)
	if err := unix.IoctlSetInt(channel.fd, nativeChannelLegacyDisconnect, 0); err != nil {
		t.Fatal(err)
	}
	nativeChannelQueueFlow(t, channel, channel.fd, 5)
	nativeChannelReady(t, channel)
	unit.probe(t, creator)
}

// TestNativeChannelOwnerWrongExpectedUnit preserves an existing live unit
// relationship when another genuine creator is supplied as the expected peer.
func TestNativeChannelOwnerWrongExpectedUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := nativeOwnerFixtureNew(t)
	victim := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, original.fd)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelDisconnect, victim.fd), unix.ESTALE)
	nativeChannelQueueFlow(t, channel, original.fd, 6)
	victim.probe(t, victim.fd)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, original.fd)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, victim.fd)
	nativeChannelQueueFlow(t, channel, victim.fd, 7)
}

// TestNativeChannelOwnerAttachedUnitRefused proves a separately opened
// PPPIOCATTACH file has no original-creator authority, live or retired.
func TestNativeChannelOwnerAttachedUnitRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	attached, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, attached) })
	if err := unix.IoctlSetPointerInt(attached, 0x4004743d, original.number); err != nil {
		t.Fatal(err)
	}
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelConnect, attached), unix.ESTALE)
	nativeChannelQueueFlow(t, channel, channel.fd, 8)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, original.fd)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelDisconnect, attached), unix.ESTALE)
	nativeChannelQueueFlow(t, channel, original.fd, 9)
	if err := netlink.LinkDel(original.link); err != nil {
		t.Fatal(err)
	}
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelDisconnect, attached), unix.ESTALE)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, original.fd)
	nativeChannelQueueFlow(t, channel, channel.fd, 10)
}

// TestNativeChannelOwnerRetiredOriginalReuse settles the retained original
// relationship after its unit number is reused, while a live replacement's
// creator cannot authorize that cleanup and its useful native state survives.
func TestNativeChannelOwnerRetiredOriginalReuse(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	creator := nativeChannelDuplicate(t, original.fd)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, original.fd)
	nativeChannelQueueFlow(t, channel, original.fd, 11)
	if err := netlink.LinkDel(original.link); err != nil {
		t.Fatal(err)
	}
	probe := original.expected()
	nativeOwnerErrno(t, nativeOwnerProbe(original.fd, &probe), unix.ESTALE)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelConnect, original.fd), unix.ESTALE)
	victim := nativeChannelUnitAt(t, original.number)
	victim.mtu(t, victim.fd, victim.link.Attrs().Index, 1771)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelDisconnect, victim.fd), unix.ESTALE)
	nativeOwnerRequireMTU(t, victim.link, 1771)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, creator)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, original.fd)
	nativeChannelQueueFlow(t, channel, channel.fd, 12)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, victim.fd)
	nativeChannelQueueFlow(t, channel, victim.fd, 13)
	victim.probe(t, victim.fd)
	nativeOwnerRequireMTU(t, victim.link, 1771)
}

// TestNativeChannelOwnerBridgeLifecycle proves guarded and legacy bridges
// carry the exact frame in both directions, then unbridge restores the real
// channel queues. Duplicate channel files retain the same native authority.
func TestNativeChannelOwnerBridgeLifecycle(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeChannelFixtureNew(t, 0)
	second := nativeChannelFixtureNew(t, 1)
	duplicate := nativeChannelDuplicate(t, second.fd)
	nativeChannelRequireOperation(t, first.fd, nativeChannelBridge, duplicate)
	nativeChannelBridgeFlow(t, first, second, 14)
	nativeChannelBridgeFlow(t, second, first, 15)
	nativeChannelRequireOperation(t, second.fd, nativeChannelUnbridge, first.fd)
	nativeChannelRequireOperation(t, first.fd, nativeChannelUnbridge, duplicate)
	nativeChannelQueueFlow(t, first, first.fd, 16)
	nativeChannelQueueFlow(t, second, second.fd, 17)
	if err := unix.IoctlSetPointerInt(first.fd, nativeChannelLegacyBridge, second.index); err != nil {
		t.Fatal(err)
	}
	nativeChannelBridgeFlow(t, first, second, 18)
	nativeChannelBridgeFlow(t, second, first, 19)
	if err := unix.IoctlSetInt(first.fd, nativeChannelLegacyUnbridge, 0); err != nil {
		t.Fatal(err)
	}
	nativeChannelQueueFlow(t, first, first.fd, 20)
	nativeChannelQueueFlow(t, second, second.fd, 21)
}

// TestNativeChannelOwnerWrongExpectedBridge preserves two useful bridges
// when cleanup names a different peer, and rejects occupied-peer admission.
func TestNativeChannelOwnerWrongExpectedBridge(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	a := nativeChannelFixtureNew(t, 0)
	b := nativeChannelFixtureNew(t, 1)
	c := nativeChannelFixtureNew(t, 2)
	d := nativeChannelFixtureNew(t, 3)
	nativeChannelRequireOperation(t, a.fd, nativeChannelBridge, b.fd)
	nativeChannelRequireOperation(t, c.fd, nativeChannelBridge, d.fd)
	nativeOwnerErrno(t, nativeChannelOperate(a.fd, nativeChannelUnbridge, c.fd), unix.ESTALE)
	nativeOwnerErrno(t, nativeChannelOperate(a.fd, nativeChannelBridge, c.fd), unix.EALREADY)
	nativeChannelBridgeFlow(t, a, b, 22)
	nativeChannelBridgeFlow(t, c, d, 23)
	nativeChannelRequireOperation(t, a.fd, nativeChannelUnbridge, b.fd)
	nativeOwnerErrno(t, nativeChannelOperate(a.fd, nativeChannelUnbridge, c.fd), unix.ESTALE)
	nativeOwnerErrno(t, nativeChannelOperate(a.fd, nativeChannelBridge, c.fd), unix.EALREADY)
	nativeChannelQueueFlow(t, a, a.fd, 24)
	nativeChannelBridgeFlow(t, c, d, 25)
}

// TestNativeChannelOwnerNamespacePairs rejects original unit/channel and
// channel/channel pairs from different actual namespaces, preserving both
// useful packet paths. The current caller's namespace is not authority.
func TestNativeChannelOwnerNamespacePairs(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	localUnit := nativeOwnerFixtureNew(t)
	local := nativeChannelFixtureNew(t, 0)
	var remoteUnit *nativeOwnerFixture
	var remote *nativeChannelFixture
	createdUnitForeignNamespace(t, func() {
		remoteUnit = nativeOwnerFixtureNew(t)
		remote = nativeChannelFixtureNew(t, 0)
	})
	nativeOwnerErrno(t, nativeChannelOperate(local.fd, nativeChannelConnect, remoteUnit.fd), unix.EXDEV)
	nativeOwnerErrno(t, nativeChannelOperate(local.fd, nativeChannelDisconnect, remoteUnit.fd), unix.EXDEV)
	nativeOwnerErrno(t, nativeChannelOperate(local.fd, nativeChannelBridge, remote.fd), unix.EXDEV)
	nativeOwnerErrno(t, nativeChannelOperate(local.fd, nativeChannelUnbridge, remote.fd), unix.EXDEV)
	nativeChannelQueueFlow(t, local, local.fd, 26)
	nativeChannelQueueFlow(t, remote, remote.fd, 27)
	nativeChannelRequireOperation(t, local.fd, nativeChannelConnect, localUnit.fd)
	nativeChannelQueueFlow(t, local, localUnit.fd, 28)
}

// TestNativeChannelOwnerDeadChannel closes the actual transport to complete
// native unregister, then requires closed admission and settled idempotent
// cleanup through the still-held original channel file.
func TestNativeChannelOwnerDeadChannel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	dead := nativeChannelFixtureNew(t, 0)
	live := nativeChannelFixtureNew(t, 1)
	nativeChannelRequireOperation(t, dead.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, dead, unit.fd, 29)
	nativeChannelRetire(t, dead)
	reply := nativeChannelReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativeChannelReadOwner(dead.fd, &reply), unix.ENOTCONN)
	nativeOwnerErrno(t, nativeChannelOperate(dead.fd, nativeChannelConnect, unit.fd), unix.ENOTCONN)
	nativeOwnerErrno(t, nativeChannelOperate(dead.fd, nativeChannelBridge, live.fd), unix.ENOTCONN)
	nativeOwnerErrno(t, nativeChannelOperate(live.fd, nativeChannelBridge, dead.fd), unix.ENOTCONN)
	nativeChannelRequireOperation(t, dead.fd, nativeChannelDisconnect, unit.fd)
	nativeChannelRequireOperation(t, dead.fd, nativeChannelUnbridge, live.fd)
	nativeChannelQueueFlow(t, live, live.fd, 30)
	unit.probe(t, unit.fd)
}

// TestNativeChannelOwnerMalformedPeer rejects every fixed request's bad ABI,
// flags and reserved fields, bad descriptors, wrong object kinds and self
// bridges before any native relationship can replace the unconnected control.
func TestNativeChannelOwnerMalformedPeer(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	other := nativeChannelFixtureNew(t, 1)
	unbound, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, unbound) })
	nonPPP, err := unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, nonPPP) })
	closed, err := unix.FcntlInt(uintptr(unit.fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	nativeOwnerClose(t, closed)
	reply := nativeChannelReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativeChannelReadOwner(closed, &reply), unix.EBADF)
	for _, command := range []uintptr{nativeChannelConnect, nativeChannelDisconnect, nativeChannelBridge, nativeChannelUnbridge} {
		peerFD := unit.fd
		switch command {
		case nativeChannelBridge, nativeChannelUnbridge:
			peerFD = other.fd
		}
		for _, bad := range []nativeChannelPeer{
			{abi: nativeOwnerABI + 1, fd: int32(peerFD)},
			{abi: nativeOwnerABI, fd: int32(peerFD), flags: 1},
			{abi: nativeOwnerABI, fd: int32(peerFD), reserved: 1},
		} {
			nativeOwnerErrno(t, nativeChannelRequest(channel.fd, command, &bad), unix.EINVAL)
		}
		for _, badFD := range []int{-1, closed, unbound, nonPPP} {
			nativeOwnerErrno(t, nativeChannelOperate(channel.fd, command, badFD), unix.EBADF)
		}
		nativeOwnerErrno(t, nativeChannelOperate(unit.fd, command, peerFD), unix.EBADF)
		nativeOwnerErrno(t, nativeChannelOperate(unbound, command, peerFD), unix.EBADF)
	}
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelConnect, other.fd), unix.EBADF)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelDisconnect, other.fd), unix.EBADF)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelBridge, unit.fd), unix.EBADF)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelUnbridge, unit.fd), unix.EBADF)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelBridge, channel.fd), unix.EINVAL)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelUnbridge, channel.fd), unix.EINVAL)
	nativeChannelQueueFlow(t, channel, channel.fd, 31)
	nativeChannelQueueFlow(t, other, other.fd, 32)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, channel, unit.fd, 33)
}

// TestNativeChannelOwnerDisconnectJoin starts two actual cleanup syscalls from
// one barrier. Both must return only after the relationship settles, after
// which a useful channel packet and a fresh connect must work on every round.
func TestNativeChannelOwnerDisconnectJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	for iteration := range 8 {
		nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, unit.fd)
		first, second := nativeChannelRunTwo(t,
			func() error { return nativeChannelOperate(channel.fd, nativeChannelDisconnect, unit.fd) },
			func() error { return nativeChannelOperate(channel.fd, nativeChannelDisconnect, unit.fd) })
		if first != nil {
			t.Fatalf("first joined disconnect: %v", first)
		}
		if second != nil {
			t.Fatalf("second joined disconnect: %v", second)
		}
		nativeChannelQueueFlow(t, channel, channel.fd, byte(40+iteration))
	}
	nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, channel, unit.fd, 48)
}

// TestNativeChannelOwnerUnbridgeJoin reverses the pair order of concurrent
// cleanup calls, joins both native operations, and requires useful unbridged
// queues and re-bridge traffic before the next round.
func TestNativeChannelOwnerUnbridgeJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeChannelFixtureNew(t, 0)
	second := nativeChannelFixtureNew(t, 1)
	for iteration := range 8 {
		nativeChannelRequireOperation(t, first.fd, nativeChannelBridge, second.fd)
		nativeChannelBridgeFlow(t, first, second, byte(50+iteration))
		left, right := nativeChannelRunTwo(t,
			func() error { return nativeChannelOperate(first.fd, nativeChannelUnbridge, second.fd) },
			func() error { return nativeChannelOperate(second.fd, nativeChannelUnbridge, first.fd) })
		if left != nil {
			t.Fatalf("first joined unbridge: %v", left)
		}
		if right != nil {
			t.Fatalf("reverse joined unbridge: %v", right)
		}
		nativeChannelQueueFlow(t, first, first.fd, byte(60+iteration))
		nativeChannelQueueFlow(t, second, second.fd, byte(70+iteration))
	}
}

// TestNativeChannelOwnerLegacyDisconnectJoin exercises the legacy ppp_mutex
// order concurrently with the guarded RTNL/trylock boundary. EBUSY is an
// explicit guarded refusal; both calls join before settled useful cleanup.
func TestNativeChannelOwnerLegacyDisconnectJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	for iteration := range 8 {
		nativeChannelRequireOperation(t, channel.fd, nativeChannelConnect, unit.fd)
		guarded, legacy := nativeChannelRunTwo(t,
			func() error { return nativeChannelOperate(channel.fd, nativeChannelDisconnect, unit.fd) },
			func() error { return unix.IoctlSetInt(channel.fd, nativeChannelLegacyDisconnect, 0) })
		if guarded != nil {
			nativeOwnerErrno(t, guarded, unix.EBUSY)
		}
		if legacy != nil {
			nativeOwnerErrno(t, legacy, unix.EINVAL)
		}
		nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, unit.fd)
		nativeChannelQueueFlow(t, channel, channel.fd, byte(90+iteration))
	}
}

// TestNativeChannelOwnerLegacyUnbridgeJoin applies the same lock-order proof
// to pair cleanup, requiring both calls to join before exact queue readback.
func TestNativeChannelOwnerLegacyUnbridgeJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeChannelFixtureNew(t, 0)
	second := nativeChannelFixtureNew(t, 1)
	for iteration := range 8 {
		nativeChannelRequireOperation(t, first.fd, nativeChannelBridge, second.fd)
		guarded, legacy := nativeChannelRunTwo(t,
			func() error { return nativeChannelOperate(second.fd, nativeChannelUnbridge, first.fd) },
			func() error { return unix.IoctlSetInt(first.fd, nativeChannelLegacyUnbridge, 0) })
		if guarded != nil {
			nativeOwnerErrno(t, guarded, unix.EBUSY)
		}
		if legacy != nil {
			nativeOwnerErrno(t, legacy, unix.EINVAL)
		}
		nativeChannelRequireOperation(t, first.fd, nativeChannelUnbridge, second.fd)
		nativeChannelQueueFlow(t, first, first.fd, byte(100+iteration))
		nativeChannelQueueFlow(t, second, second.fd, byte(110+iteration))
	}
}

// TestNativeChannelOwnerConnectUnregisterJoin races actual channel retirement
// against admission. After both native syscalls join, admission is closed and
// cleanup is settled, while an independent live channel/unit still carries data.
func TestNativeChannelOwnerConnectUnregisterJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit := nativeOwnerFixtureNew(t)
	channel := nativeChannelFixtureNew(t, 0)
	live := nativeChannelFixtureNew(t, 1)
	transport := channel.takeTransport()
	connectErr, closeErr := nativeChannelRunTwo(t,
		func() error { return nativeChannelOperate(channel.fd, nativeChannelConnect, unit.fd) },
		func() error { return unix.Close(transport) })
	if closeErr != nil {
		t.Fatalf("actual transport close did not join: %v", closeErr)
	}
	if connectErr != nil {
		if !errors.Is(connectErr, unix.ENOTCONN) {
			t.Fatalf("racing connect returned %v, want success before retirement or ENOTCONN", connectErr)
		}
	}
	reply := nativeChannelReply{abi: nativeOwnerABI}
	nativeOwnerErrno(t, nativeChannelReadOwner(channel.fd, &reply), unix.ENOTCONN)
	nativeChannelRequireOperation(t, channel.fd, nativeChannelDisconnect, unit.fd)
	nativeOwnerErrno(t, nativeChannelOperate(channel.fd, nativeChannelConnect, unit.fd), unix.ENOTCONN)
	nativeChannelRequireOperation(t, live.fd, nativeChannelConnect, unit.fd)
	nativeChannelQueueFlow(t, live, unit.fd, 80)
}

// TestNativeChannelOwnerUnbridgeUnregisterJoin joins real unregister with
// expected-pair unbridge; the survivor must be useful and may form a fresh
// bridge after the retired original's idempotent cleanup returns.
func TestNativeChannelOwnerUnbridgeUnregisterJoin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeChannelFixtureNew(t, 0)
	second := nativeChannelFixtureNew(t, 1)
	third := nativeChannelFixtureNew(t, 2)
	nativeChannelRequireOperation(t, first.fd, nativeChannelBridge, second.fd)
	nativeChannelBridgeFlow(t, first, second, 81)
	transport := first.takeTransport()
	unbridgeErr, closeErr := nativeChannelRunTwo(t,
		func() error { return nativeChannelOperate(second.fd, nativeChannelUnbridge, first.fd) },
		func() error { return unix.Close(transport) })
	if closeErr != nil {
		t.Fatalf("actual transport close did not join: %v", closeErr)
	}
	if unbridgeErr != nil {
		t.Fatalf("joined expected unbridge failed: %v", unbridgeErr)
	}
	nativeChannelRequireOperation(t, first.fd, nativeChannelUnbridge, second.fd)
	nativeChannelQueueFlow(t, second, second.fd, 82)
	nativeChannelRequireOperation(t, second.fd, nativeChannelBridge, third.fd)
	nativeChannelBridgeFlow(t, second, third, 83)
}
