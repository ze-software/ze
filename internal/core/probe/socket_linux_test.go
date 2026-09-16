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

// rawConnOf is the raw descriptor access of a loopback UDP socket, the kind
// of foreign socket the two exports serve.
func rawConnOf(t *testing.T, conn net.PacketConn) syscall.RawConn {
	t.Helper()
	sc, ok := conn.(syscall.Conn)
	if !ok {
		t.Fatalf("conn %T carries no SyscallConn", conn)
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	return raw
}

// TestWithDFModeInstallsThenRestores proves the export the IKE transport
// sends its probe through: during send the socket holds the mode's
// IP_MTU_DISCOVER value, after send it holds the value it had before, and
// a send that fails restores it too. Needs no privilege: a UDP socket on
// loopback carries the option.
func TestWithDFModeInstallsThenRestores(t *testing.T) {
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	raw := rawConnOf(t, conn)
	prior := readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER)

	cases := []struct {
		df   DFMode
		want int
	}{{DFHonorCache, unix.IP_PMTUDISC_DO}, {DFBypassCache, unix.IP_PMTUDISC_PROBE}, {DFOff, unix.IP_PMTUDISC_DONT}}
	for _, c := range cases {
		during := -1
		err := WithDFMode(raw, FamilyIPv4, c.df, func() error {
			during = readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER)
			return nil
		})
		if err != nil {
			t.Fatalf("%v: WithDFMode: %v", c.df, err)
		}
		if during != c.want {
			t.Errorf("%v: IP_MTU_DISCOVER during send = %d, want %d", c.df, during, c.want)
		}
		if after := readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER); after != prior {
			t.Errorf("%v: IP_MTU_DISCOVER after send = %d, want the prior %d", c.df, after, prior)
		}
	}

	sendErr := errors.New("send refused")
	if err := WithDFMode(raw, FamilyIPv4, DFHonorCache, func() error { return sendErr }); !errors.Is(err, sendErr) {
		t.Errorf("a failed send answered %v, want the send's own error", err)
	}
	if after := readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER); after != prior {
		t.Errorf("IP_MTU_DISCOVER after a failed send = %d, want the prior %d", after, prior)
	}
	if err := WithDFMode(raw, FamilyIPv4, DFUnspecified, func() error { return nil }); !errors.Is(err, ErrDFUnspecified) {
		t.Errorf("the zero mode answered %v, want ErrDFUnspecified", err)
	}
}

// TestEnableErrorQueueSetsRecvErr proves the export the IKE transport
// installs at creation reaches the kernel as IP_RECVERR=1 on a socket this
// package did not open.
func TestEnableErrorQueueSetsRecvErr(t *testing.T) {
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if before := readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_RECVERR); before != 0 {
		t.Fatalf("IP_RECVERR before = %d, want the kernel default 0", before)
	}
	if err := EnableErrorQueue(rawConnOf(t, conn), FamilyIPv4); err != nil {
		t.Fatalf("EnableErrorQueue: %v", err)
	}
	if after := readIntOption(t, conn, unix.IPPROTO_IP, unix.IP_RECVERR); after != 1 {
		t.Errorf("IP_RECVERR after = %d, want 1", after)
	}
}
