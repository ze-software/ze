// VALIDATES: on Ze's kernel (K1, XFRM_ZE_CAP_ICMP_REVERSE), an ICMP message that
// arrives on a tunnel-mode ESP SA with outer headers outside the SA's own selector is
// given the Section 6.2 payload check only when it is an error: a forged echo request
// whose bytes after the ICMP header hold a header the SA's selector accepts, reversed,
// is dropped at the state's selector check and never reaches the payload decode, while
// a Destination Unreachable carrying the same embedded header is delivered under Ze's
// catch-all, and dropped when an operator discard entry matches it as received.
// PREVENTS: the reverse check reading the payload of every ICMP type, which delivered a
// non-error message from outside the selectors (D-5), and an operator discard that the
// reverse check overrode for ICMP arriving on the SA (D-6).
//
// METHOD: the harness of rfc4301_icmp_error_integration_linux_test.go, one network
// namespace with loopback ESP. The state carries an explicit selector, source
// 10.0.0.0/13 to local 10.1.1.0/24, and XFRM_STATE_ICMP; its inbound protect policy is
// remote 10.2.0.0/24 to the same local prefix. The verdict is read from the
// namespace's counters: XfrmInStateMismatch moves once the state's selector check
// refuses a message, XfrmInHdrError once a payload decode refuses one, XfrmInPolBlock
// once per inbound policy check a discard entry answered, and Icmp InMsgs only once
// icmp_rcv has accepted the message.
//
// WHAT IT DOES NOT COVER: a forwarded ICMP message (LL-XFRM-ICMP-TRANSIT,
// docs/architecture/linux-limitations.md), ICMPv6, and the engine path that builds
// these SPParams from config (the catch-all's role is set by engine/unmatched.go).

//go:build linux

package dataplane

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const (
	icmpNonErrSPI          = 0x00C0FFE3
	icmpNonErrReqID        = 6203
	icmpNonErrStatMismatch = "XfrmInStateMismatch"
	icmpNonErrStatHeader   = "XfrmInHdrError"
	icmpNonErrEchoRequest  = 8
	icmpNonErrDiscardOrder = 1000
)

// icmpNonErrInstallSA installs the loopback Child SA of icmpErrInstallSA for
// icmpErrCheckedHost, with XFRM_STATE_ICMP and an explicit state selector, source
// 10.0.0.0/13 to local 10.1.1.0/24, so a message whose own headers miss it is refused
// at the state's selector check (XfrmInStateMismatch) before any policy is looked up.
//
// The one loopback state also encrypts, and xfrm_state_look_at matches an output flow
// against x->sel, so the selector's source must hold the output flow's source as well
// as the reversed embedded source 10.2.0.5, while excluding the forged 10.9.9.9. A
// raw IP_HDRINCL send builds its flow from the socket, not from the header it carries
// (raw_sendmsg, net/ipv4/raw.c), so the output route picks the local source 10.1.1.1.
// 10.0.0.0/13 (10.0.0.0 to 10.7.255.255) holds both and not 10.9.9.9. A
// selector that missed the output flow would leave the message unsent, and the wait
// on XfrmInStateMismatch below would fail rather than pass.
func icmpNonErrInstallSA(t *testing.T) {
	t.Helper()
	loop := net.ParseIP(icmpErrLoop)
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	_, source, _ := net.ParseCIDR("10.0.0.0/13")
	_, local, _ := net.ParseCIDR(icmpErrCheckedHost + "/24")
	state, err := xfrmStateFromParams(SAParams{
		SPI: icmpNonErrSPI, Src: loop, Dst: loop, Proto: ProtoESP, Mode: ModeTunnel, ReqID: icmpNonErrReqID,
		EncAlgo: "aes128gcm", EncKey: make([]byte, 20), IsAEAD: true,
		ICMPErrorCheck: true,
		Sel:            &SASelector{Src: source, Dst: local},
	})
	if err != nil {
		t.Fatalf("project the selector-bound state: %v", err)
	}
	if err := netlink.XfrmStateAdd(state); err != nil {
		t.Fatalf("add the selector-bound state: %v", err)
	}

	in, err := xfrmPolicyFromParams(SPParams{
		Src: remote, Dst: local, Dir: SADirIn, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: icmpNonErrReqID, Action: SPActionProtect, Priority: PriorityChildSA,
		TunnelSrc: loop, TunnelDst: loop, Role: PolicyRoleChildSA,
	})
	if err != nil {
		t.Fatalf("project the inbound protect policy: %v", err)
	}
	if err := netlink.XfrmPolicyAdd(in); err != nil {
		t.Fatalf("add the inbound protect policy: %v", err)
	}

	_, anyNet, _ := net.ParseCIDR("0.0.0.0/0")
	_, hostNet, _ := net.ParseCIDR(icmpErrCheckedHost + "/32")
	injector := &netlink.XfrmPolicy{
		Src: anyNet, Dst: hostNet, Dir: netlink.XFRM_DIR_OUT, Priority: 10,
		Tmpls: []netlink.XfrmPolicyTmpl{{
			Src: loop, Dst: loop, Proto: netlink.XFRM_PROTO_ESP,
			Mode: netlink.XFRM_MODE_TUNNEL, Reqid: icmpNonErrReqID,
		}},
	}
	if err := netlink.XfrmPolicyAdd(injector); err != nil {
		t.Fatalf("add the injector policy: %v", err)
	}
}

// icmpNonErrEchoPacket is icmpErrPacket turned into an ICMP Echo Request (type 8, code
// 0) whose data is the forged embedded header: the bytes after the ICMP header are the
// IPv4 and UDP headers of a datagram target sent to embeddedDst, the shape the payload
// check reads in an error.
func icmpNonErrEchoPacket(target, embeddedDst string) []byte {
	p := icmpErrPacket(target, embeddedDst)
	icmp := p[20:]
	icmp[0], icmp[1] = icmpNonErrEchoRequest, 0
	binary.BigEndian.PutUint16(icmp[2:], 0)
	binary.BigEndian.PutUint16(icmp[2:], icmpErrChecksum(icmp))
	return p
}

// RFC requirement: RFC4301-6.2-4 positive -- an ICMP Destination Unreachable that
// arrives on a tunnel-mode SA whose state selector its outer headers miss, and whose
// embedded header, reversed, matches the SA, is delivered under an inbound discard
// catch-all installed with the catch-all role.
// RFC requirement: RFC4301-6.2-4 negative -- an ICMP Echo Request that arrives on the
// same SA with outer headers outside its state selector, and whose bytes after the ICMP
// header hold that same embedded header, is not delivered, is counted as a state
// selector mismatch, and raises no header-decode error.
// RFC requirement: RFC4301-6.1.2-1 positive -- the ICMP error that is delivered under
// the catch-all alone is not delivered once an operator-role inbound discard entry for
// its source address 10.9.9.9 (protocol any) is installed, and XfrmInPolBlock then
// moves by two for it, the outer and the reverse policy check.
// RFC requirement: RFC4301-6.1.2-1 negative -- before that operator entry is installed,
// the same ICMP error is delivered, with XfrmInPolBlock moving by one, the outer check.
// RFC requirement: RFC4301-5.2-11 positive -- an ICMP Echo Request received on a
// tunnel-mode SA whose own IP headers are not consistent with the SA's state selector
// is discarded: XfrmInStateMismatch moves by one for it and Icmp InMsgs does
// not count it.
func TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded(t *testing.T) {
	// One namespace probe per process: encapNetns gives the measurement.
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	// Loopback carries disable_xfrm and disable_policy, and
	// TestICMPErrorPayloadHeaderCheckedOnLocalDelivery says why both are cleared.
	writeNetSysctl(t, "ipv4/conf/lo/disable_xfrm", "0")
	writeNetSysctl(t, "ipv4/conf/lo/disable_policy", "0")

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	addr, err := netlink.ParseAddr(icmpErrCheckedHost + "/32")
	if err != nil {
		t.Fatalf("parse %s: %v", icmpErrCheckedHost, err)
	}
	if err := netlink.AddrAdd(lo, addr); err != nil {
		t.Fatalf("add %s to lo: %v", icmpErrCheckedHost, err)
	}

	// Ze's node-wide catch-all, as engine/unmatched.go builds it: the role is what
	// marks it XFRM_POLICY_ICMP, the one discard the reverse check may pass.
	_, anyNet, _ := net.ParseCIDR("0.0.0.0/0")
	catchAll, err := xfrmPolicyFromParams(SPParams{
		Src: anyNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard,
		Priority: PriorityUnmatched, Role: PolicyRoleCatchAll,
	})
	if err != nil {
		t.Fatalf("project the inbound discard catch-all: %v", err)
	}
	if err := netlink.XfrmPolicyAdd(catchAll); err != nil {
		t.Fatalf("add the inbound discard catch-all: %v", err)
	}
	icmpNonErrInstallSA(t)

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
	mismatches := encapStat(t, icmpNonErrStatMismatch)
	headers := encapStat(t, icmpNonErrStatHeader)
	blocks := encapStat(t, icmpErrStatBlock)
	msgs := icmpErrSNMP(t, "InMsgs")

	// (a) The forged echo request. Its outer headers miss the state's selector, and
	// xfrm_selector_inner_icmp_match reads no payload of a non-error type, so the outer
	// check stops at the state (one mismatch, no policy lookup). icmp_rcv must then
	// drop it without the reverse check: a kernel that still runs that check for every
	// type refuses the payload decode instead (XfrmInHdrError) or, on a stock kernel,
	// delivers the message.
	to := &unix.SockaddrInet4{}
	copy(to.Addr[:], net.ParseIP(icmpErrCheckedHost).To4())
	if err := unix.Sendto(fd, icmpNonErrEchoPacket(icmpErrCheckedHost, icmpErrRemoteHost), 0, to); err != nil {
		t.Fatalf("send the forged echo request: %v", err)
	}
	deadline := time.Now().Add(icmpErrWaitMax)
	for encapStat(t, icmpNonErrStatMismatch)-mismatches < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("forged echo request: %s did not move in %s; the message never reached the state's selector check",
				icmpNonErrStatMismatch, icmpErrWaitMax)
		}
		time.Sleep(icmpErrWaitStep)
	}

	// (b) The consistent error, the positive control. Sent only after (a) reached the
	// state check, and awaited to delivery, so it is also the barrier after which (a)'s
	// icmp_rcv, which runs in the same softirq call as that state check, has finished.
	if !icmpErrSend(t, fd, icmpErrCheckedHost, icmpErrRemoteHost, 1) {
		t.Errorf("an ICMP error whose reversed payload header matches the SA was not delivered under the catch-all")
	}
	if got := encapStat(t, icmpNonErrStatMismatch) - mismatches; got != 1 {
		t.Errorf("%s moved by %d across the echo request and the consistent error, want 1 (the echo request alone)",
			icmpNonErrStatMismatch, got)
	}
	if got := encapStat(t, icmpNonErrStatHeader) - headers; got != 0 {
		t.Errorf("%s moved by %d: the reverse check ran on the echo request, a non-error type", icmpNonErrStatHeader, got)
	}
	if got := icmpErrSNMP(t, "InMsgs") - msgs; got != 1 {
		t.Errorf("Icmp InMsgs moved by %d across the echo request and the consistent error, want 1 (the error alone): "+
			"the forged echo request was delivered", got)
	}
	if got := encapStat(t, icmpErrStatBlock) - blocks; got != 1 {
		t.Errorf("%s moved by %d, want 1 (the consistent error's outer check)", icmpErrStatBlock, got)
	}

	// (c) An operator discard for the forged source, protocol any, ranked above the
	// catch-all. The same consistent error is refused at the outer check and again at
	// the reverse check, which meets the entry as the message was received.
	_, forged, _ := net.ParseCIDR(icmpErrForgedFrom + "/32")
	discard, err := xfrmPolicyFromParams(SPParams{
		Src: forged, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard,
		Priority: icmpNonErrDiscardOrder, Role: PolicyRoleOperator,
	})
	if err != nil {
		t.Fatalf("project the operator discard entry: %v", err)
	}
	if err := netlink.XfrmPolicyAdd(discard); err != nil {
		t.Fatalf("add the operator discard entry: %v", err)
	}
	if icmpErrSend(t, fd, icmpErrCheckedHost, icmpErrRemoteHost, 2) {
		t.Errorf("an ICMP error matching an operator discard entry as received was delivered: the reverse check overrode the discard")
	}
}
