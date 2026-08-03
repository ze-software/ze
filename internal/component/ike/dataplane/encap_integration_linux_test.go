//go:build integration && linux

package dataplane

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The states below never decrypt anything, and they do not need to. Every assertion
// reads WHICH counter the kernel raised, and the counters separate "no state
// matched", "a state matched but its encapsulation form disagreed", and "a state
// matched and the payload was rejected". The third is a match.
var encapTestAEADKey = []byte("0123456789abcdef0123456789abcdef\x00\x00\x00\x01")

const (
	encapStatNoStates     = "XfrmInNoStates"
	encapStatMismatch     = "XfrmInStateMismatch"
	encapStatProtoError   = "XfrmInStateProtoError"
	encapLoopbackAddr     = "127.0.0.1"
	encapSPIBare          = 0x00ABCDE1
	encapSPIEncapsulated  = 0x00ABCDE2
	encapSPIUnknown       = 0x00ABCDEF
	encapNATTPortForTests = 4500
)

// encapNetns moves this goroutine into a fresh network namespace and brings loopback
// up, so the XFRM states below cannot touch the host.
func encapNetns(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		t.Skipf("no CLONE_NEWNET (needs root): %v", err)
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
}

// encapStat reads one counter out of the namespace's own /proc/net/xfrm_stat.
func encapStat(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("read xfrm_stat: %v", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parse %s = %q: %v", name, fields[1], convErr)
			}
			return n
		}
	}
	t.Fatalf("%s absent from /proc/net/xfrm_stat", name)
	return 0
}

// encapAddState installs one inbound ESP state on loopback, with or without the
// ESP-in-UDP encapsulation template.
func encapAddState(t *testing.T, spi int, withEncap bool) {
	t.Helper()
	state := &netlink.XfrmState{
		Src:   net.ParseIP(encapLoopbackAddr),
		Dst:   net.ParseIP(encapLoopbackAddr),
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Spi:   spi,
		Aead: &netlink.XfrmStateAlgo{
			Name:   "rfc4106(gcm(aes))",
			Key:    append([]byte(nil), encapTestAEADKey...),
			ICVLen: 128,
		},
	}
	if withEncap {
		state.Encap = &netlink.XfrmStateEncap{
			Type:    netlink.XFRM_ENCAP_ESPINUDP,
			SrcPort: encapNATTPortForTests,
			DstPort: encapNATTPortForTests,
		}
	}
	if err := netlink.XfrmStateAdd(state); err != nil {
		t.Fatalf("xfrm state add spi=%#x encap=%v: %v", spi, withEncap, err)
	}
}

// encapESPBytes builds one ESP header with a sequence number and some filler.
func encapESPBytes(spi uint32) []byte {
	p := make([]byte, 48)
	p[0] = byte(spi >> 24)
	p[1] = byte(spi >> 16)
	p[2] = byte(spi >> 8)
	p[3] = byte(spi)
	p[7] = 1
	return p
}

// encapKernelVerdict sends one ESP datagram, bare or UDP-encapsulated, and returns
// the name of the counter the kernel raised.
func encapKernelVerdict(t *testing.T, spi uint32, encapsulated bool) string {
	t.Helper()
	before := map[string]int{
		encapStatNoStates:   encapStat(t, encapStatNoStates),
		encapStatMismatch:   encapStat(t, encapStatMismatch),
		encapStatProtoError: encapStat(t, encapStatProtoError),
	}

	if encapsulated {
		uc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
		if err != nil {
			t.Fatalf("bind udp %d: %v", encapNATTPortForTests, err)
		}
		defer func() {
			if cerr := uc.Close(); cerr != nil {
				t.Errorf("close udp: %v", cerr)
			}
		}()
		rc, err := uc.SyscallConn()
		if err != nil {
			t.Fatalf("syscall conn: %v", err)
		}
		var setErr error
		if ctlErr := rc.Control(func(fd uintptr) {
			setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
		}); ctlErr != nil {
			t.Fatalf("control: %v", ctlErr)
		}
		if setErr != nil {
			t.Fatalf("UDP_ENCAP: %v", setErr)
		}
		dst := &net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests}
		if _, err := uc.WriteToUDP(encapESPBytes(spi), dst); err != nil {
			t.Fatalf("send encapsulated ESP: %v", err)
		}
	} else {
		c, err := net.ListenPacket("ip4:esp", encapLoopbackAddr)
		if err != nil {
			t.Skipf("no raw ESP socket (needs CAP_NET_RAW): %v", err)
		}
		defer func() {
			if cerr := c.Close(); cerr != nil {
				t.Errorf("close raw: %v", cerr)
			}
		}()
		if _, err := c.WriteTo(encapESPBytes(spi), &net.IPAddr{IP: net.ParseIP(encapLoopbackAddr)}); err != nil {
			t.Fatalf("send bare ESP: %v", err)
		}
	}

	var raised []string
	for _, name := range []string{encapStatNoStates, encapStatMismatch, encapStatProtoError} {
		if encapStat(t, name) > before[name] {
			raised = append(raised, name)
		}
	}
	if len(raised) != 1 {
		t.Fatalf("expected exactly one counter to move, got %v", raised)
	}
	return raised[0]
}

// VALIDATES: on Linux XFRM one inbound state accepts exactly ONE of the two ESP
// forms. A state with an ESP-in-UDP template rejects bare ESP, and a state without
// one rejects UDP-encapsulated ESP. Both rejections are XfrmInStateMismatch, which is
// a different counter from the one an unknown SPI raises.
// PREVENTS: installChildSA being changed to set the inbound encapsulation template
// unconditionally, on the belief that a state with a template still matches bare ESP.
// It does not, and that change would silently break every no-NAT tunnel's receive
// path.
//
// This is the measurement behind the MEASURED KERNEL CONSTRAINT comment in
// engine/child.go. It is also the evidence behind the open owner question OR-WP8-4
// in plan/learned/1313-rfcgate-1b-rfc7296-pilot.md.
//
// RFC 7296 Section 2.23 asks an implementation to receive BOTH forms at any time.
// One XFRM state per SPI cannot. Two states on one SPI do not help either. The state
// lookup is keyed on destination, SPI, protocol and family, so it returns the first
// match and the encapsulation check then drops the packet.
//
// It carries no RFC requirement tag on purpose. It records what the kernel does. It
// does not assert that Ze meets an obligation.
func TestEncapKernelBindsOneESPFormPerState(t *testing.T) {
	// One namespace probe per PROCESS. See encapOwnProcess for the measurement: a second
	// probe in the same binary reads a namespace where its datagrams never reach XFRM, so
	// every counter stays still. Nothing below changes; the body runs in the child.
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)

	encapAddState(t, encapSPIBare, false)
	encapAddState(t, encapSPIEncapsulated, true)

	// One goroutine, no subtests. runtime.LockOSThread binds the NAMESPACE to this
	// goroutine's thread. A t.Run subtest gets a different goroutine on a different
	// thread. That thread reads the HOST namespace and moves no counter at all.
	cases := []struct {
		name         string
		spi          uint32
		encapsulated bool
		want         string
	}{
		{"state without a template accepts bare ESP", encapSPIBare, false, encapStatProtoError},
		{"state without a template refuses encapsulated ESP", encapSPIBare, true, encapStatMismatch},
		{"state with a template refuses bare ESP", encapSPIEncapsulated, false, encapStatMismatch},
		{"state with a template accepts encapsulated ESP", encapSPIEncapsulated, true, encapStatProtoError},
	}
	for _, tc := range cases {
		if got := encapKernelVerdict(t, tc.spi, tc.encapsulated); got != tc.want {
			t.Errorf("%s: kernel raised %s, want %s", tc.name, got, tc.want)
		}
	}

	// The discrimination control lives here, on the same goroutine and the same
	// namespace, for the reason above. An SPI with no state raises a THIRD counter,
	// so the two verdicts above are real readings rather than a counter that never
	// moves.
	if got := encapKernelVerdict(t, encapSPIUnknown, false); got != encapStatNoStates {
		t.Errorf("an SPI with no state raised %s, want %s; the counters cannot discriminate and the table above proves nothing",
			got, encapStatNoStates)
	}
}
