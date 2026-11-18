//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- native delayed client pair.
// Related: session_created_integration_linux_test.go -- real producer fixtures.
// These roots require the corrected atomic ORIGINAL PPPoX channel supplier
// and original-file unit-attachment primitive. They are not native proof yet.

package kernel_test

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

func nativeClientDisconnect(channel *pppkernel.CreatedChannel) error {
	// Observation-only legacy control: unlike exact-peer cleanup, EINVAL
	// discriminates an absent relationship. Product cleanup never calls it.
	return channel.WithDescriptor(func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.PPPIOCDISCONN, 0)
		if errno != 0 {
			return errno
		}
		return nil
	})
}

func nativeClientOriginalOperation(channel *pppkernel.CreatedChannel, unit *pppkernel.CreatedUnit, command uintptr) error {
	return unit.WithDescriptor(func(unitFD int) error {
		return channel.WithDescriptor(func(channelFD int) error {
			request := struct {
				abi      uint32
				fd       int32
				flags    uint32
				reserved uint32
			}{abi: 1, fd: int32(unitFD)}
			_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(channelFD), command,
				uintptr(unsafe.Pointer(&request))) //nolint:gosec // Root0010 fixed 16-byte peer ABI.
			if errno != 0 {
				return errno
			}
			return nil
		})
	})
}

func nativeClientOpen(t *testing.T, unit *pppkernel.CreatedUnit, channel *pppkernel.CreatedChannel) *pppkernel.ClientSession {
	t.Helper()
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	client, err := pppkernel.NewClientSession(cfg)
	if client != nil {
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("native constructor returned no client")
	}
	return client
}

// Construction must leave the native channel unattached. Rename and a foreign
// caller namespace cannot replace either original file or its immutable label.
func TestClientPairNativeDelayedConnectSurvivesRenameAndCallerNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zclient-origin")
	channel := createdSessionChannel(t, "zclient-access", 61)
	client := nativeClientOpen(t, unit, channel)
	if err := nativeClientDisconnect(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("constructor attached channel: %v", err)
	}
	if err := netlink.LinkSetName(link, "zclient-renamed"); err != nil {
		t.Fatal(err)
	}
	createdUnitForeignNamespace(t, func() {
		if err := client.Connect(); err != nil {
			t.Fatalf("original files lost to caller namespace: %v", err)
		}
	})
	if client.Identity() != unit.Identity() {
		t.Fatal("immutable original receipt changed")
	}
	if err := nativeClientDisconnect(channel); err != nil {
		t.Fatalf("live original was not attached: %v", err)
	}
	if err := nativeClientOriginalOperation(channel, unit, 0x40107432); err != nil {
		t.Fatalf("original-pair reconnect refused: %v", err)
	}
	if err := nativeClientOriginalOperation(channel, unit, 0x40107431); err != nil {
		t.Fatalf("original-pair cleanup refused: %v", err)
	}
	if err := nativeClientDisconnect(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("original-pair cleanup left attachment: %v", err)
	}
}

// With both owning files held, delete/reuse must refuse and leave no hidden
// channel attachment. This BEFORE-call root is separate from the required
// deterministic BETWEEN-check-and-ioctl producer interleaving control.
func TestClientPairNativeDeletedUnitReuseRefusesAttachment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zclient-origin")
	channel := createdSessionChannel(t, "zclient-access", 62)
	client := nativeClientOpen(t, unit, channel)
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("live original control: %v", err)
	}
	if err := netlink.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	replacement, _, _ := createdUnitNative(t, "zclient-reused")
	if replacement.UnitNumber() != unit.UnitNumber() {
		t.Fatalf("native reuse prerequisite: original=%d replacement=%d", unit.UnitNumber(), replacement.UnitNumber())
	}
	if err := client.Connect(); !errors.Is(err, unix.ENXIO) {
		t.Fatalf("dead original connect did not refuse: %v", err)
	}
	if err := nativeClientDisconnect(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("refusal left hidden attachment: %v", err)
	}
	if _, err := replacement.Counters(); err != nil {
		t.Fatalf("refusal harmed replacement: %v", err)
	}
}

// Channel stop must leave the native unit live through the caller's joins;
// explicit final cleanup then ends that exact file, with repeated closes safe.
func TestClientPairNativeSplitCloseKeepsUnitUntilCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-origin")
	channel := createdSessionChannel(t, "zclient-access", 63)
	client := nativeClientOpen(t, unit, channel)
	for range 2 {
		if err := client.CloseChannel(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("channel stop released unit: %v", err)
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("channel was not closed: %v", err)
	}
	for range 2 {
		if err := client.CloseUnit(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("final cleanup retained unit: %v", err)
	}
}

// The downstream operation accepts an owning file rather than the same unit
// number in a different namespace. The native primitive root supplies actual
// originals directly so userspace namespace preflight cannot mask the guard.
func TestClientPairNativePrimitiveForeignUnitFileRefuses(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-origin")
	channel := createdSessionChannel(t, "zclient-access", 64)
	createdUnitForeignNamespace(t, func() {
		foreign, _, _ := createdUnitNative(t, "zclient-foreign")
		if foreign.UnitNumber() != unit.UnitNumber() {
			t.Fatal("foreign same-number prerequisite was not established")
		}
		err := nativeClientOriginalOperation(channel, foreign, 0x40107432)
		if !errors.Is(err, unix.EXDEV) {
			t.Fatalf("foreign original file was accepted: %v", err)
		}
	})
	if err := nativeClientDisconnect(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("foreign refusal left hidden attachment: %v", err)
	}
}

// Admission's positive original-unit query runs in its retained namespace,
// before original-file borrowing. The foreign caller remains in its own
// namespace; subsequent native peer operations use the exact retained files.
func TestOriginalPairNativeAdmissionFromForeignCallerNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-origin")
	channel := createdSessionChannel(t, "zclient-access", 65)
	createdUnitForeignNamespace(t, func() {
		before, err := os.Stat("/proc/thread-self/ns/net")
		if err != nil {
			t.Fatal(err)
		}
		client := nativeClientOpen(t, unit, channel)
		after, err := os.Stat("/proc/thread-self/ns/net")
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) {
			t.Fatal("admission moved the caller's namespace")
		}
		if err := client.Connect(); err != nil {
			t.Fatalf("foreign caller lost original pair: %v", err)
		}
		if err := nativeClientDisconnect(channel); err != nil {
			t.Fatalf("foreign admission never attached: %v", err)
		}
		if err := nativeClientOriginalOperation(channel, unit, 0x40107432); err != nil {
			t.Fatal(err)
		}
		if err := nativeClientOriginalOperation(channel, unit, 0x40107431); err != nil {
			t.Fatal(err)
		}
	})
}
