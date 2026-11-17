//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- captured raw socket authority.
// Related: rfc4301_ipv6_fragment_policy_integration_linux_test.go -- IPv6 output builders.
// Related: rfc4301_transport_output_fragment_integration_linux_test.go -- IPv4 output builders.
// These are ordinary preservation regressions, not RFC conformance claims.

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestRawSocketFragmentPolicyAuthority distinguishes a matching OUT socket
// exemption from a selector miss and explicit policy absence. Whole controls
// pin each disposition before the same inputs reach local fragment gathering.
//
//ze:guest-root
func TestRawSocketFragmentPolicyAuthority(t *testing.T) {
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		t.Run(rawSocketFragmentFamily(family), func(t *testing.T) {
			probe := newRawSocketFragmentProbe(t, family, netlink.XFRM_POLICY_BLOCK)
			matched := probe.sender(t, 0, "")
			missed := probe.sender(t, 0, "")
			absent := probe.sender(t, 0, "")
			probe.socketPolicy(t, matched, transportFragmentPort)
			probe.socketPolicy(t, missed, transportFragmentPort+1)
			probe.control(t, matched, "matching socket policy")
			probe.blocked(t, missed, "socket selector miss")
			probe.blocked(t, absent, "explicit socket policy absence")
			for _, tailFirst := range []bool{false, true} {
				// A completing socket's exemption cannot authorize fragments
				// captured when the other socket had no OUT policy.
				probe.mixed(t, absent, matched, tailFirst, "absence cannot borrow a socket exemption")
				probe.control(t, matched, "matching policy survives the mixed attempt")
			}

			// In the permitting namespace, absent receipts may combine across
			// sockets. Two independently allocated policies may not, even when
			// their selectors and actions have identical values.
			probe.fallback.Action = netlink.XFRM_POLICY_ALLOW
			if err := netlink.XfrmPolicyUpdate(probe.fallback); err != nil {
				t.Fatalf("permit the namespace's ordinary raw output: %v", err)
			}
			otherAbsent := probe.sender(t, 0, "")
			otherPolicy := probe.sender(t, 0, "")
			probe.socketPolicy(t, otherPolicy, transportFragmentPort)
			probe.control(t, absent, "first absent socket")
			probe.control(t, otherAbsent, "second absent socket")
			probe.control(t, otherPolicy, "independent matching socket policy")
			for _, tailFirst := range []bool{false, true} {
				body := probe.body("equal absent authority combines across sockets")
				parts := probe.fragments(body)
				probe.interleave(t, absent, otherAbsent, parts, tailFirst)
				probe.wantBody(t, body)
				probe.mixed(t, matched, otherPolicy, tailFirst, "independent policy receipts stay separate")
				probe.mixed(t, absent, matched, tailFirst, "absence and a present policy stay separate")
				probe.control(t, matched, "same policy remains deliverable")
			}
		})
	}
}

// TestRawSocketFragmentRouteAuthority preserves marked policy dispositions and
// keeps differing socket marks and device bindings out of one fragment queue.
// Each admitted authority has whole and both fragment-order delivery controls.
//
//ze:guest-root
func TestRawSocketFragmentRouteAuthority(t *testing.T) {
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		t.Run(rawSocketFragmentFamily(family), func(t *testing.T) {
			probe := newRawSocketFragmentProbe(t, family, netlink.XFRM_POLICY_BLOCK)
			const firstMark, secondMark, refusedMark = 0x7601, 0x7602, 0x7603
			probe.markPolicy(t, firstMark)
			probe.markPolicy(t, secondMark)
			first := probe.sender(t, firstMark, "")
			second := probe.sender(t, secondMark, "")
			refused := probe.sender(t, refusedMark, "")
			probe.control(t, first, "first admitted mark")
			probe.control(t, second, "second admitted mark")
			probe.blocked(t, refused, "unconfigured mark cannot use either exemption")
			for _, tailFirst := range []bool{false, true} {
				probe.mixed(t, first, second, tailFirst, "different captured marks stay separate")
				probe.control(t, first, "first mark after mixed attempt")
				probe.control(t, second, "second mark after mixed attempt")
			}
			probe.fallback.Action = netlink.XFRM_POLICY_ALLOW
			if err := netlink.XfrmPolicyUpdate(probe.fallback); err != nil {
				t.Fatalf("permit the unmarked route-authority control: %v", err)
			}
			plain := probe.sender(t, 0, "")
			probe.control(t, plain, "unmarked authority on the same ordinary route")
			for _, tailFirst := range []bool{false, true} {
				probe.mixed(t, plain, first, tailFirst, "marks stay separate with identical namespace disposition")
				probe.control(t, plain, "unmarked authority after mixed attempt")
			}

			bound := probe.sender(t, firstMark, espPolZeLink)
			probe.control(t, bound, "explicitly bound output device")
			for _, tailFirst := range []bool{false, true} {
				probe.mixed(t, first, bound, tailFirst, "bound and unbound authority stay separate")
				probe.control(t, bound, "bound device after mixed attempt")
				probe.control(t, first, "unbound device after mixed attempt")
			}
		})
	}
}

// TestRawSocketFragmentPolicyLifetime replaces and closes an OUT policy owner
// after its first fragment. Queued receipts cannot acquire a later generation;
// fresh generations and equal absent authority remain independently usable.
//
//ze:guest-root
func TestRawSocketFragmentPolicyLifetime(t *testing.T) {
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		t.Run(rawSocketFragmentFamily(family), func(t *testing.T) {
			probe := newRawSocketFragmentProbe(t, family, netlink.XFRM_POLICY_ALLOW)
			sender := probe.sender(t, 0, "")
			probe.socketPolicy(t, sender, transportFragmentPort)
			probe.control(t, sender, "original policy generation")
			for _, tailFirst := range []bool{false, true} {
				parts := probe.fragments(probe.body("replacing a policy cannot complete the old generation"))
				initial, final := rawSocketFragmentOrder(tailFirst)
				probe.send(t, sender, parts[initial], false)
				probe.socketPolicy(t, sender, transportFragmentPort)
				probe.send(t, sender, parts[final], false)
				probe.wantAbsent(t, "replacement policy generation")
				probe.control(t, sender, "fresh replacement generation")
			}

			queued := probe.sender(t, 0, "")
			probe.socketPolicy(t, queued, transportFragmentPort)
			parts := probe.fragments(probe.body("a closed policy owner cannot lend its queued receipt"))
			probe.send(t, queued, parts[0], false)
			queued.close(t)
			probe.send(t, sender, parts[1], false)
			probe.wantAbsent(t, "closed policy owner")
			probe.control(t, sender, "live policy after the other owner closed")

			for _, tailFirst := range []bool{false, true} {
				origin := probe.sender(t, 0, "")
				completing := probe.sender(t, 0, "")
				body := probe.body("absent authority survives closing its originating socket")
				parts := probe.fragments(body)
				initial, final := rawSocketFragmentOrder(tailFirst)
				probe.send(t, origin, parts[initial], false)
				origin.close(t)
				probe.send(t, completing, parts[final], false)
				probe.wantBody(t, body)
				probe.control(t, completing, "absent socket after the origin closed")
			}
		})
	}
}

// rawSocketFragmentProbe reuses the existing locked-thread veth lab. Every
// method MUST run on its creating test goroutine; no method is concurrent-safe.
type rawSocketFragmentProbe struct {
	lab      *espPolLab
	family   int
	source   net.IP
	target   net.IP
	receiver *net.UDPConn
	fallback *netlink.XfrmPolicy
	nextID   uint16
}

func newRawSocketFragmentProbe(t *testing.T, family int, action netlink.PolicyAction) *rawSocketFragmentProbe {
	t.Helper()
	probe := &rawSocketFragmentProbe{lab: newEspPolLab(t), family: family, nextID: 0x7600}
	if family == unix.AF_INET6 {
		ipv6FragmentPolicyAddresses(t, probe.lab)
		probe.source = net.ParseIP(transportFragmentRemote)
		probe.target = net.ParseIP(transportFragmentLocal)
		probe.lab.enter(t, probe.lab.peer)
		probe.receiver = transportFragmentListen(t, transportFragmentPort)
	} else {
		probe.source = net.ParseIP(espPolZeOuter)
		probe.target = net.ParseIP(espPolPeerOuter)
		probe.lab.enter(t, probe.lab.peer)
		probe.receiver = sadSelListen(t, espPolPeerOuter, transportFragmentPort)
	}
	probe.lab.enter(t, probe.lab.ze)
	probe.fallback = probe.policy(5000, action)
	probe.installPolicy(t, probe.fallback)
	// An unrelated constrained entry keeps the guard live independently of
	// the socket policies and the disposition under test.
	guard := probe.policy(100, netlink.XFRM_POLICY_BLOCK)
	guard.Proto = unix.IPPROTO_UDP
	guard.DstPort = transportFragmentPort + 9
	probe.installPolicy(t, guard)
	caps := rfcScopeCapabilities(t)
	if caps[3] != rfcScopeABIVersion {
		t.Fatalf("raw-output scope ABI=%#x, want %#x", caps[3], rfcScopeABIVersion)
	}
	if caps[2] == 0 {
		t.Fatal("the constrained namespace policy did not activate fragment gathering")
	}
	return probe
}

func rawSocketFragmentFamily(family int) string {
	if family == unix.AF_INET6 {
		return "ipv6"
	}
	return "ipv4"
}

func (p *rawSocketFragmentProbe) policy(priority int, action netlink.PolicyAction) *netlink.XfrmPolicy {
	bits := 128
	if p.family == unix.AF_INET {
		bits = 32
	}
	return &netlink.XfrmPolicy{
		Src: &net.IPNet{IP: p.source, Mask: net.CIDRMask(bits, bits)},
		Dst: &net.IPNet{IP: p.target, Mask: net.CIDRMask(bits, bits)},
		Dir: netlink.XFRM_DIR_OUT, Priority: priority, Action: action,
	}
}

func (p *rawSocketFragmentProbe) installPolicy(t *testing.T, policy *netlink.XfrmPolicy) {
	t.Helper()
	if err := netlink.XfrmPolicyAdd(policy); err != nil {
		t.Fatalf("install the raw-output namespace policy: %v", err)
	}
	t.Cleanup(func() {
		p.lab.enter(t, p.lab.ze)
		if err := netlink.XfrmPolicyDel(policy); err != nil {
			t.Errorf("remove the raw-output namespace policy: %v", err)
		}
	})
}

func (p *rawSocketFragmentProbe) markPolicy(t *testing.T, mark uint32) {
	t.Helper()
	policy := p.policy(1000, netlink.XFRM_POLICY_ALLOW)
	policy.Proto = unix.IPPROTO_UDP
	policy.DstPort = transportFragmentPort
	policy.Mark = &netlink.XfrmMark{Value: mark, Mask: ^uint32(0)}
	p.installPolicy(t, policy)
}

// rawSocketFragmentSender owns its fd so an intentional close cannot race a
// later cleanup closing a reused descriptor. It is confined to the lab thread.
type rawSocketFragmentSender struct {
	fd int
}

func (p *rawSocketFragmentProbe) sender(t *testing.T, mark int, device string) *rawSocketFragmentSender {
	t.Helper()
	fd, err := unix.Socket(p.family, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("open the raw socket authority probe: %v", err)
	}
	sender := &rawSocketFragmentSender{fd: fd}
	t.Cleanup(func() { sender.close(t) })
	if p.family == unix.AF_INET6 {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_HDRINCL, 1); err != nil {
			t.Fatal(err)
		}
		address := &unix.SockaddrInet6{}
		copy(address.Addr[:], p.source.To16())
		if err := unix.Bind(fd, address); err != nil {
			t.Fatalf("bind the IPv6 raw authority probe: %v", err)
		}
	} else {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
			t.Fatal(err)
		}
		address := &unix.SockaddrInet4{}
		copy(address.Addr[:], p.source.To4())
		if err := unix.Bind(fd, address); err != nil {
			t.Fatalf("bind the IPv4 raw authority probe: %v", err)
		}
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_MARK, mark); err != nil {
		t.Fatalf("set the raw socket's mark: %v", err)
	}
	if device != "" {
		if err := unix.BindToDevice(fd, device); err != nil {
			t.Fatalf("bind the raw socket to %s: %v", device, err)
		}
	}
	return sender
}

func (s *rawSocketFragmentSender) close(t *testing.T) {
	t.Helper()
	if s.fd < 0 {
		return
	}
	if err := unix.Close(s.fd); err != nil {
		t.Errorf("close the raw socket authority probe: %v", err)
	}
	s.fd = -1
}

func (p *rawSocketFragmentProbe) socketPolicy(t *testing.T, sender *rawSocketFragmentSender, port uint16) {
	t.Helper()
	policy := nl.XfrmUserpolicyInfo{
		Sel: nl.XfrmSelector{Family: uint16(p.family), Proto: unix.IPPROTO_UDP},
		Lft: nl.XfrmLifetimeCfg{
			SoftByteLimit: nl.XFRM_INF, HardByteLimit: nl.XFRM_INF,
			SoftPacketLimit: nl.XFRM_INF, HardPacketLimit: nl.XFRM_INF,
		},
		Dir: uint8(netlink.XFRM_DIR_OUT), Action: uint8(netlink.XFRM_POLICY_ALLOW),
	}
	policy.Sel.Saddr.FromIP(p.source)
	policy.Sel.Daddr.FromIP(p.target)
	policy.Sel.PrefixlenS, policy.Sel.PrefixlenD = 128, 128
	if p.family == unix.AF_INET {
		policy.Sel.PrefixlenS, policy.Sel.PrefixlenD = 32, 32
	}
	var networkPort [2]byte
	binary.BigEndian.PutUint16(networkPort[:], port)
	policy.Sel.Dport = binary.NativeEndian.Uint16(networkPort[:])
	policy.Sel.DportMask = ^uint16(0)
	level, option := unix.IPPROTO_IPV6, unix.IPV6_XFRM_POLICY
	if p.family == unix.AF_INET {
		level, option = unix.IPPROTO_IP, unix.IP_XFRM_POLICY
	}
	if err := unix.SetsockoptString(sender.fd, level, option, string(policy.Serialize())); err != nil {
		t.Fatalf("install the raw socket's exact OUT exemption: %v", err)
	}
}

func (p *rawSocketFragmentProbe) whole(body []byte) []byte {
	udp := transportOutputUDP(p.source, body)
	if p.family == unix.AF_INET6 {
		return transportFragmentIPv6(unix.IPPROTO_UDP, udp)
	}
	p.nextID++
	return transportOutputIPv4(p.source, p.target, udp, 0, p.nextID)
}

func (p *rawSocketFragmentProbe) fragments(body []byte) [2][]byte {
	udp := transportOutputUDP(p.source, body)
	p.nextID++
	if p.family == unix.AF_INET6 {
		return [2][]byte{
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[:24], 1, uint32(p.nextID))),
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[24:], 24, uint32(p.nextID))),
		}
	}
	return [2][]byte{
		transportOutputIPv4(p.source, p.target, udp[:24], 0x2000, p.nextID),
		transportOutputIPv4(p.source, p.target, udp[24:], 3, p.nextID),
	}
}

func (p *rawSocketFragmentProbe) send(t *testing.T, sender *rawSocketFragmentSender, packet []byte, rejected bool) {
	t.Helper()
	var target unix.Sockaddr
	if p.family == unix.AF_INET6 {
		address := &unix.SockaddrInet6{}
		copy(address.Addr[:], p.target.To16())
		target = address
	} else {
		address := &unix.SockaddrInet4{}
		copy(address.Addr[:], p.target.To4())
		target = address
	}
	err := unix.Sendto(sender.fd, packet, 0, target)
	if err == nil {
		return
	}
	if rejected {
		if errors.Is(err, unix.EPERM) {
			return
		}
		if errors.Is(err, unix.EACCES) {
			return
		}
	}
	t.Fatalf("send the raw socket authority probe: %v", err)
}

func rawSocketFragmentOrder(tailFirst bool) (initial, final int) {
	if tailFirst {
		return 1, 0
	}
	return 0, 1
}

// body gives every datagram its own exact receive expectation, so an earlier
// permitted control cannot satisfy a later row that lost its packet.
func (p *rawSocketFragmentProbe) body(label string) []byte {
	return []byte(label + " " + strconv.Itoa(int(p.nextID)))
}

func (p *rawSocketFragmentProbe) interleave(t *testing.T, first, second *rawSocketFragmentSender, parts [2][]byte, tailFirst bool) {
	t.Helper()
	initial, final := rawSocketFragmentOrder(tailFirst)
	p.send(t, first, parts[initial], false)
	p.send(t, second, parts[final], false)
}

func (p *rawSocketFragmentProbe) control(t *testing.T, sender *rawSocketFragmentSender, label string) {
	t.Helper()
	t.Log(label)
	body := p.body("permitted whole raw authority: " + label)
	p.send(t, sender, p.whole(body), false)
	p.wantBody(t, body)
	for _, tailFirst := range []bool{false, true} {
		body := p.body("permitted fragmented raw authority: " + label)
		parts := p.fragments(body)
		p.interleave(t, sender, sender, parts, tailFirst)
		p.wantBody(t, body)
	}
}

func (p *rawSocketFragmentProbe) blocked(t *testing.T, sender *rawSocketFragmentSender, label string) {
	t.Helper()
	t.Log(label)
	before := espPolStat(t, outStepStatPolBlock)
	p.send(t, sender, p.whole(p.body("forbidden whole raw authority: "+label)), true)
	p.wantBlock(t, label, before)
	for _, tailFirst := range []bool{false, true} {
		parts := p.fragments(p.body("forbidden fragmented raw authority: " + label))
		initial, final := rawSocketFragmentOrder(tailFirst)
		before := espPolStat(t, outStepStatPolBlock)
		p.send(t, sender, parts[initial], true)
		p.send(t, sender, parts[final], true)
		p.wantBlock(t, label, before)
	}
}

func (p *rawSocketFragmentProbe) wantBlock(t *testing.T, label string, before int) {
	t.Helper()
	awaitForwardKernelCounter(t, label, uint64(before+1),
		func(t *testing.T) uint64 { return uint64(espPolStat(t, outStepStatPolBlock)) })
	if got := espPolStat(t, outStepStatPolBlock); got != before+1 {
		t.Fatalf("%s: policy block count=%d, want %d", label, got, before+1)
	}
	p.wantAbsent(t, label)
}

func (p *rawSocketFragmentProbe) mixed(t *testing.T, first, second *rawSocketFragmentSender, tailFirst bool, label string) {
	t.Helper()
	t.Logf("%s, tail-first=%v", label, tailFirst)
	parts := p.fragments(p.body("different authority must not assemble: " + label))
	p.interleave(t, first, second, parts, tailFirst)
	p.wantAbsent(t, label)
}

func (p *rawSocketFragmentProbe) wantBody(t *testing.T, body []byte) {
	t.Helper()
	if p.family == unix.AF_INET6 {
		ipv6FragmentPolicyBody(t, p.receiver, body)
		return
	}
	transportOutputBody(t, p.receiver, p.source, body)
}

func (p *rawSocketFragmentProbe) wantAbsent(t *testing.T, label string) {
	t.Helper()
	if err := p.receiver.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 256)
	n, _, err := p.receiver.ReadFromUDP(buffer)
	if err == nil {
		t.Fatalf("%s delivered forbidden body %q", label, buffer[:n])
	}
	var timeout net.Error
	if !errors.As(err, &timeout) {
		t.Fatal(err)
	}
	if !timeout.Timeout() {
		t.Fatal(err)
	}
}
