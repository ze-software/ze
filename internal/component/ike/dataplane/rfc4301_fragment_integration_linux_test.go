// VALIDATES: that the port-scoped BYPASS entries ze installs are stored in the real
// kernel SPD in the two directions where Linux reassembles a fragment before the XFRM
// policy check reads its ports, and in no other direction.
// PREVENTS: shipping the Section 7.4 verdict on a reading of the kernel alone. The
// unit test asserts the direction of the SPParams ze BUILDS. Only the kernel can say
// which direction the policy was actually stored in, and a netlink direction
// conversion is one off-by-one away from putting a port-scoped exemption on the
// forward path, where a non-initial fragment IS classified with unreadable ports.

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// fragTestPort is the local IKE-like port these policies are scoped to. It is not 500
// or 4500, so a running ze on the same host cannot be mistaken for this test's work.
const fragTestPort = 45500

// fragBypassPair builds the outbound and inbound halves of one port-scoped bypass, in
// exactly the shape ikeBypassPolicies builds them (engine/bypass.go): the local port
// is pinned and the remote port is left free, in both directions.
func fragBypassPair() []SPParams {
	anyNet := &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	return []SPParams{
		{
			Src: anyNet, Dst: anyNet,
			Dir: SADirOut, Action: SPActionBypass, Priority: PriorityIKEBypass,
			UpperProto: unix.IPPROTO_UDP,
			SrcPort:    ExactPortMatch(fragTestPort), DstPort: AnyPortMatch(),
		},
		{
			Src: anyNet, Dst: anyNet,
			Dir: SADirIn, Action: SPActionBypass, Priority: PriorityIKEBypass,
			UpperProto: unix.IPPROTO_UDP,
			SrcPort:    AnyPortMatch(), DstPort: ExactPortMatch(fragTestPort),
		},
	}
}

// VALIDATES: RFC4301-7.4-2. Every port-scoped BYPASS ze installs is stored by the
// kernel in XFRM_DIR_IN or XFRM_DIR_OUT, never XFRM_DIR_FWD.
//
// WHY THE DIRECTION IS THE WHOLE QUESTION. Linux reassembles before the policy check
// on the two directions ze uses, and does not on the third:
//
//   - IN: ip_local_deliver (net/ipv4/ip_input.c:242) calls ip_defrag before the
//     LOCAL_IN hook, so xfrm4_policy_check in ip_protocol_deliver_rcu reads a
//     reassembled datagram. For IPv6, frag_protocol (net/ipv6/reassembly.c:413)
//     carries INET6_PROTO_NOPOLICY, so the check is skipped for the Fragment header
//     and applied to the transport protocol after ipv6_frag_rcv reassembles.
//   - OUT: ip_fragment runs in __ip_finish_output (net/ipv4/ip_output.c), after the
//     routing-time XFRM lookup, so the decision is made on the whole datagram.
//   - FWD: ip_forward calls xfrm4_policy_check(XFRM_POLICY_FWD) with NO defrag, so a
//     non-initial fragment is classified on ports the kernel could not read.
//
// So a port-scoped bypass on FWD is the forged-fragment hole of Section 7.4 and
// Appendix D.4, and this test reads the kernel's own answer about where ze put it.
// PREVENTS: the netlink direction conversion (netlink.Dir(p.Dir - 1)) drifting. Ze
// counts SADir from 1 and the kernel counts from 0, and an off-by-one there moves an
// exemption onto the forward path with every unit test still green.
// RFC requirement: RFC4301-7.4-2 positive -- the kernel stores no port-scoped bypass
// on the forward path.
func TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath(t *testing.T) {
	b := &xfrmBackend{}

	installed := 0
	for _, p := range fragBypassPair() {
		if err := b.InstallPolicy(p); err != nil {
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				t.Skipf("XFRM policy install needs CAP_NET_ADMIN: %v", err)
			}
			t.Fatalf("install the port-scoped bypass (dir %d): %v", p.Dir, err)
		}
		t.Cleanup(func() { _ = b.RemovePolicyParams(p) })
		installed++
	}

	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}

	// Read back only the policies THIS test installed, by the port it pinned. A host
	// running ze carries its own bypass on 500 and 4500, and judging those would make
	// the verdict depend on what else is running.
	seen := 0
	for i := range policies {
		p := &policies[i]
		if p.Proto != unix.IPPROTO_UDP {
			continue
		}
		if p.SrcPort != fragTestPort && p.DstPort != fragTestPort {
			continue
		}
		seen++
		if p.Dir == netlink.XFRM_DIR_FWD {
			t.Errorf("the kernel stored a port-scoped bypass on the FORWARD path (sport=%d dport=%d): "+
				"ip_forward classifies a non-initial fragment with no reassembly, so a forged fragment "+
				"matching these addresses and protocol would be passed in the clear",
				p.SrcPort, p.DstPort)
		}
		if p.Dir != netlink.XFRM_DIR_IN && p.Dir != netlink.XFRM_DIR_OUT {
			t.Errorf("the kernel stored a port-scoped bypass in direction %s, which is neither in nor out", p.Dir)
		}
		if p.Action != netlink.XFRM_POLICY_ALLOW {
			t.Errorf("dir %s: action = %s, want allow", p.Dir, p.Action)
		}
	}

	// The absence assertion needs its positive control. A read that found nothing
	// would satisfy every check above while proving that no policy was installed at
	// all (ai/rules/interop-and-goal-validation.md).
	if seen != installed {
		t.Fatalf("read back %d of the %d port-scoped bypasses this test installed; "+
			"a direction assertion over a population the kernel does not hold proves nothing", seen, installed)
	}
}
