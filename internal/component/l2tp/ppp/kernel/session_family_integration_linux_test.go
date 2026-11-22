//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- native PPP family modes.

package kernel

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// A fresh, task-owned real PPP unit proves that each protocol's installed mode
// changes independently. This is ioctl state evidence, not channel packet proof.
// It runs in a user, network and mount namespace of its own (userns.Enter),
// whose root holds the CAP_NET_ADMIN Linux ppp_open requires of the opener.
func TestKernelNCPRealUnitModes(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fd, err := openDevPPP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if _, err := ioctlGetSetInt(fd, pppiocNewUnit, -1); err != nil {
		t.Fatal(err)
	}
	readMode := func(protocol int32) int32 {
		t.Helper()
		// Linux include/uapi/linux/ppp-ioctl.h: PPPIOCGNPMODE is
		// _IOWR('t',76,struct npioctl), native protocol + enum NPmode.
		// https://kernel.googlesource.com/pub/scm/linux/kernel/git/kishon/linux-phy/+/f74fd13f4585e418a3e630a82468be58bf1d98c1/include/uapi/linux/ppp-ioctl.h
		request := [2]int32{protocol, -1}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc008744c, uintptr(unsafe.Pointer(&request))) //nolint:gosec // ioctl requires a native npioctl pointer.
		if errno != 0 {
			t.Fatalf("read mode for protocol %#x: %v", protocol, errno)
		}
		return request[1]
	}
	assertModes := func(v4, v6 int32) {
		t.Helper()
		if got := readMode(0x21); got != v4 {
			t.Errorf("installed IPv4 mode %d, want %d", got, v4)
		}
		if got := readMode(0x57); got != v6 {
			t.Errorf("installed IPv6 mode %d, want %d", got, v6)
		}
	}
	if err := SetIPv4Mode(fd, NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := SetIPv6Mode(fd, NPModeDrop); err != nil {
		t.Fatal(err)
	}
	assertModes(1, 1)
	if err := SetIPv4Mode(fd, NPModePass); err != nil {
		t.Fatal(err)
	}
	assertModes(0, 1)
	if err := SetIPv6Mode(fd, NPModePass); err != nil {
		t.Fatal(err)
	}
	assertModes(0, 0)
	if err := SetIPv4Mode(fd, NPModeDrop); err != nil {
		t.Fatal(err)
	}
	assertModes(1, 0)
	if err := SetIPv6Mode(fd, NPModeDrop); err != nil {
		t.Fatal(err)
	}
	assertModes(1, 1)
}
