// VALIDATES: that an ICMP type and code selector ze builds reaches the netlink policy
// with its masks, and that the real kernel then drops exactly the ICMP messages the
// selector names and passes the others.
// PREVENTS: an ICMP selector of type 0 or a code block installed with a mask derived
// from the value, which the kernel reads as "any" and so drops every ICMP message.

//go:build linux

package dataplane

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// VALIDATES: xfrmPolicyFromParams carries the type and code values and their masks.
// PREVENTS: the explicit masks dropped on the way to netlink, which installs type 0
// and a code block as any.
func TestICMPSelectorReachesTheBackend(t *testing.T) {
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	np, err := xfrmPolicyFromParams(SPParams{
		Src: loopback, Dst: loopback, Dir: SADirIn, Action: SPActionDiscard,
		Priority: 1000, UpperProto: UpperProtoICMP,
		SrcPort: ExactPortMatch(0), DstPort: PortMatch{Port: 4, Mask: 0xfffc},
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if np.SrcPort != 0 || np.SrcPortMask != 0xffff || np.DstPort != 4 || np.DstPortMask != 0xfffc {
		t.Fatalf("netlink selector sport %d/%#04x dport %d/%#04x, want 0/0xffff and 4/0xfffc",
			np.SrcPort, np.SrcPortMask, np.DstPort, np.DstPortMask)
	}
	if _, err := xfrmPolicyFromParams(SPParams{
		Src: loopback, Dst: loopback, Dir: SADirIn, Action: SPActionDiscard,
		Priority: 1000, UpperProto: UpperProtoICMP,
		SrcPort: PortMatch{Port: 5, Mask: 0xfffc}, DstPort: AnyPortMatch(),
	}); err == nil {
		t.Fatal("a port with bits outside its mask was accepted")
	}
}

// VALIDATES: xfrmStateFromParams carries an ICMP type 0 selector of a transport-mode
// state (SAParams.Sel, RFC 4301 Section 4.4.2) with its masks.
// PREVENTS: the state selector's masks dropped on the way to netlink. The kernel then
// reads type 0 as any and admits every ICMP message decrypted on that SA, where the
// negotiated selector names echo reply alone (XfrmInStateMismatch is never raised).
func TestICMPStateSelectorReachesTheBackend(t *testing.T) {
	_, local, _ := net.ParseCIDR("192.0.2.1/32")
	_, remote, _ := net.ParseCIDR("192.0.2.2/32")
	state, err := xfrmStateFromParams(SAParams{
		SPI: 0x0000E002, Src: net.ParseIP("192.0.2.2"), Dst: net.ParseIP("192.0.2.1"),
		Proto: ProtoESP, Mode: ModeTransport, ReqID: 7, Dir: SADirIn,
		EncAlgo: "aes128gcm", EncKey: make([]byte, 20), IsAEAD: true,
		Sel: &SASelector{
			Src: remote, Dst: local, UpperProto: UpperProtoICMP,
			SrcPort: ExactPortMatch(0), DstPort: PortMatch{Port: 4, Mask: 0xfffc},
		},
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	sel := state.Selector
	if sel == nil {
		t.Fatal("a transport-mode state with a negotiated selector carries no netlink selector")
	}
	if uint8(sel.Proto) != UpperProtoICMP || sel.SrcPort != 0 || sel.SrcPortMask != 0xffff || sel.DstPort != 4 || sel.DstPortMask != 0xfffc {
		t.Fatalf("netlink state selector proto %d sport %d/%#04x dport %d/%#04x, want %d 0/0xffff and 4/0xfffc",
			sel.Proto, sel.SrcPort, sel.SrcPortMask, sel.DstPort, sel.DstPortMask, UpperProtoICMP)
	}
}

// icmpEcho builds an ICMP echo request of the given code, identifier and sequence.
func icmpEcho(code uint8, id, seq uint16) []byte {
	b := []byte{8, code, 0, 0, 0, 0, 0, 0, 'z', 'e'}
	binary.BigEndian.PutUint16(b[4:], id)
	binary.BigEndian.PutUint16(b[6:], seq)
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	binary.BigEndian.PutUint16(b[2:], ^uint16(sum))
	return b
}

// icmpSeen is one ICMP message a raw socket received: type, code and sequence.
type icmpSeen struct {
	typ, code uint8
	seq       uint16
}

// VALIDATES: RFC4301-4.4.1.1-4 and RFC4301-6.1.1-2 on the kernel. With an inbound
// discard for type 0 (echo reply) and one for type 8 codes 4..7, the kernel drops an
// echo request of code 5 and the reply to an echo request, and delivers echo requests
// of code 8 and code 0.
// PREVENTS: the selector being exact only on paper. The kernel reads the type and code
// into the port pair and matches under the mask, so a mask lost or derived from the
// value would drop every ICMP message (type 0 as any), and a block wider than 4..7
// would drop code 8.
// RFC requirement: RFC4301-4.4.1.1-4 positive -- the kernel drops exactly the ICMP codes of the installed block and passes the code just outside it.
// RFC requirement: RFC4301-6.1.1-2 positive -- an inbound discard for one ICMP type drops that type arriving unprotected and passes another type.
func TestKernelICMPSelectorDropsOnlyTheNamedMessages(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	// Linux registers loopback with disable_policy set (net/ipv4/devinet.c,
	// inetdev_event: IN_DEV_CONF_SET NOPOLICY for IFF_LOOPBACK). A looped skb keeps its
	// output dst, so ip_rcv_finish_core sets IPSKB_NOPOLICY, __xfrm_check_dev_nopolicy
	// answers true, and raw_rcv and icmp_rcv run no inbound policy check: every discard
	// below would be skipped. It is cleared in the test's own namespace.
	writeNetSysctl(t, "ipv4/conf/lo/disable_policy", "0")
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	b := &xfrmBackend{}
	for _, sp := range []SPParams{
		{Src: loopback, Dst: loopback, Dir: SADirIn, Action: SPActionDiscard, Priority: 1000,
			UpperProto: UpperProtoICMP, SrcPort: ExactPortMatch(0), DstPort: AnyPortMatch()},
		{Src: loopback, Dst: loopback, Dir: SADirIn, Action: SPActionDiscard, Priority: 1000,
			UpperProto: UpperProtoICMP, SrcPort: ExactPortMatch(8), DstPort: PortMatch{Port: 4, Mask: 0xfffc}},
	} {
		if err := b.InstallPolicy(sp); err != nil {
			t.Fatalf("install %+v: %v", sp, err)
		}
		t.Cleanup(func() { _ = b.RemovePolicyParams(sp) })
	}

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_ICMP)
	if err != nil {
		t.Fatalf("raw ICMP socket: %v", err)
	}
	defer func() { _ = unix.Close(fd) }()
	tv := unix.NsecToTimeval((5 * time.Second).Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("receive timeout: %v", err)
	}
	const id = 0x5a45
	to := &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}
	// Sent in order on loopback, so each is processed before the next one is queued:
	// code 5 (dropped), code 8 (delivered, its reply dropped), then the sentinel.
	for _, m := range []struct {
		code uint8
		seq  uint16
	}{{5, 1}, {8, 2}, {0, 3}} {
		if err := unix.Sendto(fd, icmpEcho(m.code, id, m.seq), 0, to); err != nil {
			t.Fatalf("send echo code %d: %v", m.code, err)
		}
	}

	// Reading stops once both delivered requests are in. Loopback processes a CPU's
	// backlog in order, so on one CPU everything sent earlier has been judged by then.
	var seen []icmpSeen
	sawCode8, sawSentinel := false, false
	buf := make([]byte, 1500)
	for !sawCode8 || !sawSentinel {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("echo request code 8 or the code 0 sentinel never arrived; the discard dropped more than its selector names. Seen so far: %+v (%v)", seen, err)
		}
		ihl := int(buf[0]&0x0f) * 4
		if n < ihl+8 || binary.BigEndian.Uint16(buf[ihl+4:]) != id {
			continue
		}
		m := icmpSeen{typ: buf[ihl], code: buf[ihl+1], seq: binary.BigEndian.Uint16(buf[ihl+6:])}
		seen = append(seen, m)
		sawCode8 = sawCode8 || (m.typ == 8 && m.code == 8)
		sawSentinel = sawSentinel || (m.typ == 8 && m.seq == 3)
	}

	for _, m := range seen {
		switch {
		case m.typ == 8 && m.code == 5:
			t.Errorf("echo request code 5 was delivered; the discard for type 8 codes 4..7 did not drop it")
		case m.typ == 0:
			t.Errorf("an echo reply (seq %d) was delivered; the discard for type 0 did not drop it", m.seq)
		}
	}
}
