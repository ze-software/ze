//go:build linux

package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// readIntOption reads one integer socket option back from conn.
func readIntOption(t *testing.T, conn net.PacketConn, level, opt int) int {
	t.Helper()
	sc, ok := conn.(syscall.Conn)
	if !ok {
		t.Fatalf("conn %T carries no SyscallConn", conn)
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var value int
	var getErr error
	if err := raw.Control(func(fd uintptr) {
		value, getErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if getErr != nil {
		t.Fatalf("getsockopt level=%d opt=%d: %v", level, opt, getErr)
	}
	return value
}

// openOrSkip opens the probe socket, and skips the test when the process
// holds no CAP_NET_RAW: the option can only be read back off a socket that
// opened. The QEMU run holds the capability and is where this test counts.
func openOrSkip(t *testing.T, family Family, df DFMode) net.PacketConn {
	t.Helper()
	conn, err := OpenICMP(context.Background(), family, netip.Addr{}, df)
	if errors.Is(err, unix.EPERM) {
		t.Skipf("raw ICMP socket needs CAP_NET_RAW: %v", err)
	}
	if err != nil {
		t.Fatalf("OpenICMP(%v, %v): %v", family, df, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn.PacketConn()
}

// TestOpenICMPInstallsDFMode proves each DF mode reaches the kernel as the
// option Ze owes: DFHonorCache is IP_PMTUDISC_DO, DFBypassCache is
// IP_PMTUDISC_PROBE, both with IP_RECVERR on, and DFOff is IP_PMTUDISC_DONT with IP_RECVERR left
// at their kernel default so a probe with DF off is the probe of before.
func TestOpenICMPInstallsDFMode(t *testing.T) {
	cases := []struct {
		name     string
		family   Family
		df       DFMode
		discover int
		recvErr  int
	}{
		{"v4-honor-cache", FamilyIPv4, DFHonorCache, unix.IP_PMTUDISC_DO, 1},
		{"v4-bypass-cache", FamilyIPv4, DFBypassCache, unix.IP_PMTUDISC_PROBE, 1},
		{"v6-honor-cache", FamilyIPv6, DFHonorCache, unix.IPV6_PMTUDISC_DO, 1},
		{"v6-bypass-cache", FamilyIPv6, DFBypassCache, unix.IPV6_PMTUDISC_PROBE, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := openOrSkip(t, tc.family, tc.df)
			opts := dfOptionsOf(tc.family)
			if got := readIntOption(t, conn, opts.level, opts.mtuDiscover); got != tc.discover {
				t.Errorf("IP_MTU_DISCOVER = %d, want %d", got, tc.discover)
			}
			if got := readIntOption(t, conn, opts.level, opts.recvErr); got != tc.recvErr {
				t.Errorf("IP_RECVERR = %d, want %d", got, tc.recvErr)
			}
		})
	}

	t.Run("v4-off-clears-the-bit", func(t *testing.T) {
		conn := openOrSkip(t, FamilyIPv4, DFOff)
		opts := dfOptionsOf(FamilyIPv4)
		if got := readIntOption(t, conn, opts.level, opts.mtuDiscover); got != unix.IP_PMTUDISC_DONT {
			t.Errorf("DFOff left IP_MTU_DISCOVER = %d, want IP_PMTUDISC_DONT: the kernel's WANT default sets the DF bit too", got)
		}
		if got := readIntOption(t, conn, opts.level, opts.recvErr); got != 0 {
			t.Errorf("DFOff left IP_RECVERR = %d, want 0", got)
		}
	})
}
