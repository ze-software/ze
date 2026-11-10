// VALIDATES: on a real Linux kernel, an ICMP error that arrives through a tunnel-mode
// ESP SA with outer headers no inbound policy of the SA accepts is delivered when the
// header it embeds, reversed, matches the SA's inbound policy, and dropped when it does
// not, exactly when the state carries the flag Ze projects for icmp-error-check.
// PREVENTS: trusting the projection test alone. That test reads the netlink struct Ze
// builds; only the kernel can say that XFRM_STATE_ICMP on that state makes icmp_rcv
// read the payload, and that the SPD Ze installs (a selector-pair protect entry above
// a discard catch-all) is what the reversed lookup meets.
//
// METHOD: one network namespace. A raw socket sends a forged ICMP Destination
// Unreachable from 10.9.9.9 to a local address. A test-only outbound policy hands it to
// an ESP state whose tunnel endpoints are both 127.0.0.1, so the same state encrypts it
// and decrypts it on loopback, and the decapsulated message meets the inbound SPD Ze's
// projections built. The verdict is read from the namespace's own counters: Icmp
// InMsgs moves only once icmp_rcv has accepted the message, and XfrmInPolBlock moves
// once per inbound policy check the discard catch-all answered.
//
// WHAT IT DOES NOT COVER: a FORWARDED ICMP error (the transit case Section 6.2 is
// about). ip_forward checks XFRM_POLICY_FWD, the kernel's xfrm_in_fwd_icmp re-reads the
// payload only when no forward policy matches the outer headers, and Ze's discard
// catch-all on fwd always matches (docs/architecture/linux-limitations.md,
// LL-XFRM-ICMP-TRANSIT).

//go:build linux

package dataplane

import (
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const (
	icmpErrLoop        = "127.0.0.1"
	icmpErrForgedFrom  = "10.9.9.9"
	icmpErrRemoteHost  = "10.2.0.5" // inside the SA's remote selector 10.2.0.0/24
	icmpErrStrangeHost = "10.7.0.5" // inside no selector of either SA
	icmpErrCheckedHost = "10.1.1.1" // local selector 10.1.1.0/24, checked SA
	icmpErrPlainHost   = "10.1.2.1" // local selector 10.1.2.0/24, unchecked SA
	icmpErrStatBlock   = "XfrmInPolBlock"
	icmpErrSPIChecked  = 0x00C0FFE1
	icmpErrSPIPlain    = 0x00C0FFE2
	icmpErrReqChecked  = 6201
	icmpErrReqPlain    = 6202
	icmpErrWaitMax     = 2 * time.Second
	icmpErrWaitStep    = 10 * time.Millisecond
	icmpErrTriggerPort = 40000
	icmpErrRemotePort  = 40001
)

// icmpErrSNMP reads one Icmp counter from the namespace's /proc/thread-self/net/snmp,
// for the thread reason encapStat gives.
func icmpErrSNMP(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/snmp")
	if err != nil {
		t.Fatalf("read snmp: %v", err)
	}
	var header []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "Icmp:" {
			continue
		}
		if header == nil {
			header = fields
			continue
		}
		for i, h := range header {
			if h == name && i < len(fields) {
				n, convErr := strconv.Atoi(fields[i])
				if convErr != nil {
					t.Fatalf("parse Icmp %s = %q: %v", name, fields[i], convErr)
				}
				return n
			}
		}
	}
	t.Fatalf("Icmp %s absent from /proc/thread-self/net/snmp", name)
	return 0
}

// icmpErrInstallSA installs one tunnel-mode Child SA on loopback: its state and inbound
// protect policy through Ze's projections, and a test-only outbound policy that sends
// every packet for localHost through that state.
func icmpErrInstallSA(t *testing.T, spi, reqID uint32, localHost string, check bool) {
	t.Helper()
	loop := net.ParseIP(icmpErrLoop)
	state, err := xfrmStateFromParams(SAParams{
		SPI: spi, Src: loop, Dst: loop, Proto: ProtoESP, Mode: ModeTunnel, ReqID: reqID,
		Dir: SADirIn, EncAlgo: "aes128gcm", EncKey: make([]byte, 20), IsAEAD: true,
		ICMPErrorCheck: check,
	})
	if err != nil {
		t.Fatalf("project state spi=%#x: %v", spi, err)
	}
	if err := netlink.XfrmStateAdd(state); err != nil {
		t.Fatalf("add state spi=%#x: %v", spi, err)
	}

	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	_, local, _ := net.ParseCIDR(localHost + "/24")
	in, err := xfrmPolicyFromParams(SPParams{
		Src: remote, Dst: local, Dir: SADirIn, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: reqID, Action: SPActionProtect, Priority: PriorityChildSA,
		TunnelSrc: loop, TunnelDst: loop,
	})
	if err != nil {
		t.Fatalf("project inbound policy reqid=%d: %v", reqID, err)
	}
	if err := netlink.XfrmPolicyAdd(in); err != nil {
		t.Fatalf("add inbound policy reqid=%d: %v", reqID, err)
	}

	_, anyNet, _ := net.ParseCIDR("0.0.0.0/0")
	_, hostNet, _ := net.ParseCIDR(localHost + "/32")
	injector := &netlink.XfrmPolicy{
		Src: anyNet, Dst: hostNet, Dir: netlink.XFRM_DIR_OUT, Priority: 10,
		Tmpls: []netlink.XfrmPolicyTmpl{{
			Src: loop, Dst: loop, Proto: netlink.XFRM_PROTO_ESP,
			Mode: netlink.XFRM_MODE_TUNNEL, Reqid: int(reqID),
		}},
	}
	if err := netlink.XfrmPolicyAdd(injector); err != nil {
		t.Fatalf("add injector policy for %s: %v", localHost, err)
	}
}

// icmpErrChecksum is the Internet checksum of b.
func icmpErrChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// icmpErrPacket builds a whole IPv4 datagram carrying an ICMP Destination Unreachable
// (type 3, code 1) from icmpErrForgedFrom to target, about a UDP datagram target sent
// to embeddedDst.
//
//	offset 0   IPv4 header, 20 octets: 10.9.9.9 -> target, protocol 1
//	offset 20  ICMP header, 8 octets: type 3, code 1, checksum, 4 unused
//	offset 28  embedded IPv4 header, 20 octets: target -> embeddedDst, protocol 17
//	offset 48  embedded UDP header, 8 octets: 40000 -> 40001
func icmpErrPacket(target, embeddedDst string) []byte {
	p := make([]byte, 56)
	p[0], p[8], p[9] = 0x45, 64, unix.IPPROTO_ICMP
	binary.BigEndian.PutUint16(p[2:], uint16(len(p)))
	copy(p[12:16], net.ParseIP(icmpErrForgedFrom).To4())
	copy(p[16:20], net.ParseIP(target).To4())

	icmp := p[20:]
	icmp[0], icmp[1] = 3, 1
	inner := icmp[8:]
	inner[0], inner[8], inner[9] = 0x45, 64, unix.IPPROTO_UDP
	binary.BigEndian.PutUint16(inner[2:], 28)
	copy(inner[12:16], net.ParseIP(target).To4())
	copy(inner[16:20], net.ParseIP(embeddedDst).To4())
	binary.BigEndian.PutUint16(inner[10:], icmpErrChecksum(inner[:20]))
	binary.BigEndian.PutUint16(inner[20:], icmpErrTriggerPort)
	binary.BigEndian.PutUint16(inner[22:], icmpErrRemotePort)
	binary.BigEndian.PutUint16(inner[24:], 8)
	binary.BigEndian.PutUint16(icmp[2:], icmpErrChecksum(icmp))
	return p
}

// icmpErrSend sends one forged ICMP error and reports whether icmp_rcv accepted it. It
// waits for the discard catch-all to answer wantBlocks inbound policy checks, so a
// refusal is read once the kernel has reached its verdict rather than after a guess at
// how long that takes.
func icmpErrSend(t *testing.T, fd int, target, embeddedDst string, wantBlocks int) bool {
	t.Helper()
	blocks := encapStat(t, icmpErrStatBlock)
	msgs := icmpErrSNMP(t, "InMsgs")

	to := &unix.SockaddrInet4{}
	copy(to.Addr[:], net.ParseIP(target).To4())
	if err := unix.Sendto(fd, icmpErrPacket(target, embeddedDst), 0, to); err != nil {
		t.Fatalf("send the forged ICMP error to %s: %v", target, err)
	}

	deadline := time.Now().Add(icmpErrWaitMax)
	for encapStat(t, icmpErrStatBlock)-blocks < wantBlocks {
		if time.Now().After(deadline) {
			t.Fatalf("ICMP error to %s about %s: %s moved by %d in %s, want %d; the message never reached the inbound policy check",
				target, embeddedDst, icmpErrStatBlock, encapStat(t, icmpErrStatBlock)-blocks, icmpErrWaitMax, wantBlocks)
		}
		time.Sleep(icmpErrWaitStep)
	}
	if got := encapStat(t, icmpErrStatBlock) - blocks; got != wantBlocks {
		t.Fatalf("ICMP error to %s about %s: %s moved by %d, want %d", target, embeddedDst, icmpErrStatBlock, got, wantBlocks)
	}
	// A refused message stops at the policy check, so InMsgs already holds its final
	// value. An accepted one raises InMsgs just after the check that let it through,
	// on the softirq's CPU, so a reader on another CPU can see the policy counter first:
	// it waits for InMsgs, bounded by the same deadline.
	for icmpErrSNMP(t, "InMsgs") == msgs {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(icmpErrWaitStep)
	}
	return true
}

// RFC requirement: RFC4301-6.2-4 positive -- an ICMP error that arrives on a
// tunnel-mode SA with outer headers outside the SA's inbound policy is delivered when
// its embedded header, reversed, matches that policy and the state carries the check.
// RFC requirement: RFC4301-6.2-4 negative -- an ICMP error that arrives on a tunnel-mode
// SA with outer headers outside the SA's inbound policy, and whose embedded header,
// reversed, matches no inbound policy of the SA, is dropped and never delivered.
// RFC requirement: RFC4301-6.2-5 negative -- an ICMP error whose embedded header,
// reversed, matches no inbound policy of the SA is dropped and never delivered.
// RFC requirement: RFC4301-6.2-5 positive -- an ICMP error whose embedded header,
// reversed, matches the SA's inbound policy passes the check and is delivered on a
// state carrying it.
func TestICMPErrorPayloadHeaderCheckedOnLocalDelivery(t *testing.T) {
	// One namespace probe per process: encapNetns gives the measurement.
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	// Linux registers loopback with disable_xfrm and disable_policy set
	// (net/ipv4/devinet.c, inetdev_event: IN_DEV_CONF_SET NOXFRM and NOPOLICY for
	// IFF_LOOPBACK). With disable_xfrm, the output route to a local address carries
	// DST_NOXFRM and xfrm_lookup skips the injector policy, so the forged error is never
	// encrypted. With disable_policy, ip_route_input_slow (and ip_rcv_finish_core for a
	// looped skb) set IPSKB_NOPOLICY, __xfrm_check_dev_nopolicy answers true, and no
	// inbound policy check runs at all. Both are cleared in this thread's namespace, so
	// the decapsulated message meets the inbound SPD the projections built.
	writeNetSysctl(t, "ipv4/conf/lo/disable_xfrm", "0")
	writeNetSysctl(t, "ipv4/conf/lo/disable_policy", "0")

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	for _, host := range []string{icmpErrCheckedHost, icmpErrPlainHost} {
		addr, parseErr := netlink.ParseAddr(host + "/32")
		if parseErr != nil {
			t.Fatalf("parse %s: %v", host, parseErr)
		}
		if err := netlink.AddrAdd(lo, addr); err != nil {
			t.Fatalf("add %s to lo: %v", host, err)
		}
	}

	_, anyNet, _ := net.ParseCIDR("0.0.0.0/0")
	catchAll, err := xfrmPolicyFromParams(SPParams{
		Src: anyNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard,
		Priority: PriorityUnmatched,
	})
	if err != nil {
		t.Fatalf("project the inbound discard catch-all: %v", err)
	}
	if err := netlink.XfrmPolicyAdd(catchAll); err != nil {
		t.Fatalf("add the inbound discard catch-all: %v", err)
	}
	icmpErrInstallSA(t, icmpErrSPIChecked, icmpErrReqChecked, icmpErrCheckedHost, true)
	icmpErrInstallSA(t, icmpErrSPIPlain, icmpErrReqPlain, icmpErrPlainHost, false)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("open a raw IPv4 socket in the test namespace: %v", err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			t.Errorf("close raw socket: %v", cerr)
		}
	}()

	// One goroutine, no subtests: the namespace is bound to this thread (encapNetns).
	// Consistent payload, checked SA: the outer check is refused once, the reversed
	// check passes, icmp_rcv accepts it.
	if !icmpErrSend(t, fd, icmpErrCheckedHost, icmpErrRemoteHost, 1) {
		t.Errorf("an ICMP error whose reversed payload header matches the SA was not delivered on a state carrying the check")
	}
	// Inconsistent payload, checked SA: refused twice, never delivered.
	if icmpErrSend(t, fd, icmpErrCheckedHost, icmpErrStrangeHost, 2) {
		t.Errorf("an ICMP error whose reversed payload header matches no policy of the SA was delivered")
	}
	// The discrimination control: the same consistent message on a state without the
	// flag is refused at the outer check and never reaches the payload, so the delivery
	// above is the flag's doing and not a policy that happened to match.
	if icmpErrSend(t, fd, icmpErrPlainHost, icmpErrRemoteHost, 1) {
		t.Errorf("an ICMP error was delivered on a state without XFRM_STATE_ICMP; the delivery above proves nothing about the flag")
	}
}
