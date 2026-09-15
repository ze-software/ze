//go:build integration && linux

// Design: docs/architecture/diagnostics/active-probes.md -- the unprivileged fallback proven against the kernel
//
// VALIDATES: A-4 (the unprivileged datagram ICMP socket accepts
// IP_MTU_DISCOVER and IP_RECVERR and reports the next-hop MTU on its error
// queue), AC-7, the Security Review row "Authorization failing open" against
// the real datagram opener, and whether the kernel hands a datagram socket
// another flow's error (it does not: net/ipv4/ping.c ping_err looks the
// socket up by the quoted identifier).
// PREVENTS: a fallback that opens but cannot measure, an identifier the
// probers match replies on that is not the one the kernel writes, and a
// datagram socket standing in for a raw refusal that was not privilege.
//
// The thread that drops CAP_NET_RAW ends with the test goroutine rather
// than returning to the pool, so the capability set of no other test
// thread changes. Capabilities are per thread on Linux, and unix.Capset
// is a plain syscall on the calling thread.

package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// pingGroupRangePath is the sender namespace's copy of the sysctl: the
// key is per network namespace, and the thread that writes it is inside
// the sender namespace.
const pingGroupRangePath = "/proc/sys/net/ipv4/ping_group_range"

// setPingGroupRange writes the range the datagram socket is checked
// against. "1 0" is the kernel's disabled default.
func setPingGroupRange(t *testing.T, value string) {
	t.Helper()
	if err := os.WriteFile(pingGroupRangePath, []byte(value), 0o644); err != nil {
		t.Fatalf("write %s: %v", pingGroupRangePath, err)
	}
}

// dropNetRaw removes CAP_NET_RAW from the calling thread's effective,
// permitted and inheritable sets, so a raw socket open answers EPERM.
func dropNetRaw() error {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return err
	}
	mask := uint32(1) << unix.CAP_NET_RAW
	data[0].Effective &^= mask
	data[0].Permitted &^= mask
	data[0].Inheritable &^= mask
	return unix.Capset(&hdr, &data[0])
}

// dropNetRawOnThisThread drops CAP_NET_RAW on the test goroutine's thread,
// which withClampedPath already locked and placed in the sender namespace,
// and locks it once more. withClampedPath's cleanup unlocks once, so the
// goroutine ends still locked and the runtime terminates the thread rather
// than returning a thread without CAP_NET_RAW to the pool.
func dropNetRawOnThisThread(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if err := dropNetRaw(); err != nil {
		t.Fatalf("drop CAP_NET_RAW: %v", err)
	}
}

// TestProbeUnprivilegedSocketReportsNextHopMTU is A-4 and AC-7: with the
// raw socket refused for privilege and the process group inside
// ping_group_range, OpenICMP answers the datagram kind, the oversized DF
// probe is refused by the router, and the error queue reports the clamp
// quoting the identifier the kernel assigned, which is the one the Socket
// reports.
func TestProbeUnprivilegedSocketReportsNextHopMTU(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		setPingGroupRange(t, "0 0")
		dropNetRawOnThisThread(t)
		for _, df := range []DFMode{DFHonorCache, DFBypassCache} {
			sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, df)
			if err != nil {
				t.Fatalf("%v: OpenICMP without CAP_NET_RAW inside ping_group_range: %v", df, err)
			}
			t.Cleanup(func() { sock.Close() }) //nolint:errcheck // test cleanup
			if sock.Kind() != SocketDatagram {
				t.Fatalf("%v: kind %v, want datagram", df, sock.Kind())
			}
			id := sock.Identifier()
			if id == 0 {
				t.Fatalf("%v: the datagram socket reports identifier 0", df)
			}
			const seq = 7
			if err := sendEcho(t, sock, FamilyIPv4, farAddr4, 0x1111, seq, pathFillPayload); err != nil {
				t.Fatalf("%v: send: %v", df, err)
			}
			got := awaitReadEvent(t, sock, FamilyIPv4, id, seq)
			if got.reply {
				t.Fatalf("%v: a %d-octet DF datagram crossed the %d clamp on the datagram socket", df, pathLinkMTU, pathClampMTU)
			}
			if !errors.Is(got.readErr, unix.EMSGSIZE) {
				t.Fatalf("%v: ordinary read returned %v, want EMSGSIZE: ping_err did not wake the reader", df, got.readErr)
			}
			entries := drainAll(t, sock, FamilyIPv4)
			if len(entries) != 1 {
				t.Fatalf("%v: %d queued entries, want 1: %+v", df, len(entries), entries)
			}
			e := entries[0]
			if e.Outcome != ErrQueueMTUReported || e.MTU != pathClampMTU {
				t.Errorf("%v: outcome %v mtu %d, want %v %d", df, e.Outcome, e.MTU, ErrQueueMTUReported, pathClampMTU)
			}
			if e.Offender != routerNear4 {
				t.Errorf("%v: offender %v, want the router %v", df, e.Offender, routerNear4)
			}
			// The header Ze wrote carried 0x1111; the kernel rewrote it to
			// the socket's identifier, and the quoted echo shows that.
			want := QuotedEcho{Present: true, ID: id, Seq: seq}
			if e.Echo != want {
				t.Errorf("%v: quoted echo %+v, want %+v (the kernel-assigned identifier)", df, e.Echo, want)
			}
		}
	})
}

// TestProbeUnprivilegedSocketNeedsThePingGroupRange proves the range is
// what admits the datagram socket: with the kernel's disabled default, no
// socket opens and the error names both fixes.
func TestProbeUnprivilegedSocketNeedsThePingGroupRange(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		setPingGroupRange(t, "1 0")
		dropNetRawOnThisThread(t)
		sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFHonorCache)
		if err == nil {
			sock.Close() //nolint:errcheck // the test failed already
			t.Fatalf("a %v socket opened without CAP_NET_RAW and outside ping_group_range", sock.Kind())
		}
		if !errors.Is(err, unix.EPERM) {
			t.Errorf("error %v does not carry the raw refusal EPERM", err)
		}
		if !errors.Is(err, unix.EACCES) {
			t.Errorf("error %v does not carry the datagram refusal EACCES", err)
		}
		for _, want := range []string{"CAP_NET_RAW", "ping_group_range"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %s", err, want)
			}
		}
	})
}

// TestProbeUnprivilegedFallbackIsNotTriedForOtherRefusals is the guard
// against the live datagram opener: a raw refusal that is not privilege,
// with ping_group_range open, answers the raw refusal and opens nothing.
func TestProbeUnprivilegedFallbackIsNotTriedForOtherRefusals(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		setPingGroupRange(t, "0 0")
		openRawICMP = func(context.Context, string, string, func(string, string, syscall.RawConn) error) (net.PacketConn, error) {
			return nil, syscall.EAFNOSUPPORT
		}
		t.Cleanup(func() { openRawICMP = listenRawICMP })
		dropNetRawOnThisThread(t)
		sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFHonorCache)
		if err == nil {
			sock.Close() //nolint:errcheck // the test failed already
			t.Fatalf("a %v socket opened over a raw refusal that was not privilege", sock.Kind())
		}
		if !errors.Is(err, syscall.EAFNOSUPPORT) {
			t.Errorf("error %v does not carry the raw refusal", err)
		}
		if strings.Contains(err.Error(), "ping_group_range") {
			t.Errorf("error %q reports the datagram opener, which the guard must not reach", err)
		}
	})
}

// TestProbeUnprivilegedSocketIgnoresAnotherFlow answers the question A-3
// left open for the datagram kind: two datagram sockets, one refused, and
// the other's queue stays empty, because the kernel looks the socket up by
// the quoted identifier before queueing.
func TestProbeUnprivilegedSocketIgnoresAnotherFlow(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		setPingGroupRange(t, "0 0")
		dropNetRawOnThisThread(t)
		first, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFBypassCache)
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		defer first.Close() //nolint:errcheck // test cleanup
		second, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFBypassCache)
		if err != nil {
			t.Fatalf("second: %v", err)
		}
		defer second.Close() //nolint:errcheck // test cleanup
		if first.Kind() != SocketDatagram || second.Kind() != SocketDatagram {
			t.Fatalf("kinds %v and %v, want datagram", first.Kind(), second.Kind())
		}
		if first.Identifier() == second.Identifier() {
			t.Fatalf("two datagram sockets share identifier %#x", first.Identifier())
		}
		if err := sendEcho(t, second, FamilyIPv4, farAddr4Other, 0, 1, pathFillPayload); err != nil {
			t.Fatalf("second send: %v", err)
		}
		got := awaitReadEvent(t, second, FamilyIPv4, second.Identifier(), 1)
		if !errors.Is(got.readErr, unix.EMSGSIZE) {
			t.Fatalf("second: read returned %v, want EMSGSIZE", got.readErr)
		}
		own := drainAll(t, second, FamilyIPv4)
		if len(own) != 1 || own[0].Echo.ID != second.Identifier() {
			t.Fatalf("second queued %+v, want its own refusal", own)
		}
		foreign := drainAll(t, first, FamilyIPv4)
		if len(foreign) != 0 {
			t.Errorf("the first datagram socket holds %d entries about the other flow: %+v", len(foreign), foreign)
		}
		t.Logf("datagram kind: the first socket held %d entries about the other flow", len(foreign))
	})
}
