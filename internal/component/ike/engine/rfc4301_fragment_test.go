// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: bypass.go -- ikeBypassPolicies, one of the two port-scoped BYPASS producers
// Related: spd_policy.go -- spdPolicyParams, the other one
// RFC: rfc/short/rfc4301.md -- stateful fragment checking (Section 7.4)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// portScopedBypasses collects every BYPASS policy under test that carries a
// non-trivial port selector, which is the population RFC 4301 Section 7.4 binds:
// "BYPASS traffic for which a non-trivial port range is specified".
//
// A policy with both ports ANY is outside the requirement, so it is not collected.
// Reading the PORT rather than the producer is what keeps this honest: a future
// bypass that gains a port joins the population without anybody remembering to add
// it here.
func portScopedBypasses(policies []dataplane.SPParams) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(policies))
	for _, p := range policies {
		if p.Action != dataplane.SPActionBypass {
			continue
		}
		if p.SrcPort.IsAny() && p.DstPort.IsAny() {
			continue
		}
		out = append(out, p)
	}
	return out
}

// VALIDATES: RFC4301-7.4-2. Every port-scoped BYPASS entry Ze can install is confined
// to the two directions in which the Linux stack completes fragment reassembly before
// the XFRM policy check reads the ports. Ze installs no forward-direction bypass, and
// the forward path is the one place the check classifies a non-initial fragment.
//
// THE KERNEL HALF, read in Linux v6.8 and not inferred:
//
//   - Inbound IPv4. ip_local_deliver (net/ipv4/ip_input.c:242) calls ip_defrag for
//     every fragment BEFORE the NF_INET_LOCAL_IN hook, and only then reaches
//     ip_local_deliver_finish -> ip_protocol_deliver_rcu, which is where
//     xfrm4_policy_check(NULL, XFRM_POLICY_IN, skb) runs. The policy check therefore
//     never sees a non-initial fragment; it sees the reassembled datagram.
//
//   - Inbound IPv6. There is no unconditional defrag. The Fragment header is a
//     protocol handler, and frag_protocol (net/ipv6/reassembly.c:413) carries
//     INET6_PROTO_NOPOLICY, so ip6_protocol_deliver_rcu SKIPS xfrm6_policy_check for
//     it and runs ipv6_frag_rcv. Its resubmit loop then reaches the transport
//     protocol, which carries no NOPOLICY flag and does take the check, on the
//     reassembled datagram.
//
//   - Outbound. ip_fragment is called from __ip_finish_output
//     (net/ipv4/ip_output.c), at the end of the output path, strictly after the XFRM
//     policy lookup that routing performed. The outbound decision is made on the
//     whole datagram.
//
//   - Forward. ip_forward calls xfrm4_policy_check(NULL, XFRM_POLICY_FWD, skb) with
//     NO defrag, so that path DOES classify a non-initial fragment on ports the
//     kernel could not read.
//
// So the obligation is met through the stack for exactly the directions Ze installs,
// and this test is the Ze half of that: it asserts the thing Ze itself produces, the
// direction of every port-scoped BYPASS selector, rather than the kernel behavior.
//
// PREVENTS: a port-scoped BYPASS reaching the forward direction. Such an entry would
// be classified against a non-initial fragment whose ports read as zero, which is the
// forged-fragment bypass RFC 4301 Section 7.4 and Appendix D.4 describe, and no test
// over the priority numbers or the selector prefixes would see it.
// RFC requirement: RFC4301-7.4-2 positive -- no port-scoped bypass reaches the
// forward path, where the kernel would classify a non-initial fragment.
func TestPortScopedBypassNeverReachesTheForwardPath(t *testing.T) {
	var all []dataplane.SPParams

	// Producer one: ze's own IKE control-plane exemption, on UDP 500 and 4500.
	for _, family := range ikeBypassFamilies {
		all = append(all, ikeBypassPolicies(family)...)
	}

	// Producer two: an operator entry that asks for every direction the grammar
	// offers, with a port on each side, which is the widest port-scoped bypass a
	// configuration can express.
	_, local, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("parse local prefix: %v", err)
	}
	_, remote, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse remote prefix: %v", err)
	}
	all = append(all, spdPolicyParams(ipsec.SPDPolicy{
		Name:         "pass-mgmt",
		Action:       dataplane.SPActionBypass,
		Order:        1000,
		Direction:    ipsec.SPDDirBoth,
		Protocol:     protoUDP,
		LocalPrefix:  local,
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: 161},
		RemotePrefix: remote,
		RemotePort:   ipsec.AnyPort(),
	})...)

	scoped := portScopedBypasses(all)
	if len(scoped) == 0 {
		t.Fatal("no port-scoped bypass was collected, so this test would pass against a producer that installs none; " +
			"ikeBypassPolicies alone owes four per family")
	}

	for _, p := range scoped {
		if p.Dir == dataplane.SADirFwd {
			t.Errorf("a port-scoped bypass is installed in the forward direction (src=%v dst=%v sport=%v dport=%v): "+
				"ip_forward runs xfrm4_policy_check with no reassembly, so a forged non-initial fragment "+
				"matching its addresses and protocol would be passed in the clear",
				p.Src, p.Dst, p.SrcPort, p.DstPort)
		}
		if p.Dir != dataplane.SADirIn && p.Dir != dataplane.SADirOut {
			t.Errorf("a port-scoped bypass carries direction %d, which is neither in nor out; "+
				"the reassembly-before-check finding covers those two directions only", p.Dir)
		}
	}
}

// VALIDATES: RFC4301-7.4-2. The IKE control-plane bypass really is port-scoped, so
// the requirement binds it and the test above is not vacuous.
// PREVENTS: the population check passing because the population is empty. If
// ikeBypassPolicies ever widened its ports to ANY, the entries would leave the
// Section 7.4 population and the direction assertion would hold over nothing, while
// the exemption itself became far wider than ze's own sockets.
// RFC requirement: RFC4301-7.4-2 negative -- the IKE bypass is not any-port, so it is
// inside the population this requirement binds.
func TestIKEBypassIsPortScopedSoSection74Binds(t *testing.T) {
	for _, family := range ikeBypassFamilies {
		policies := ikeBypassPolicies(family)
		if len(policies) != 4 {
			t.Fatalf("family %v: %d bypass policies, want 4 (two ports, two directions)", family, len(policies))
		}
		for _, p := range policies {
			if p.SrcPort.IsAny() && p.DstPort.IsAny() {
				t.Errorf("family %v dir %d: the IKE bypass constrains no port, so it exempts every UDP flow "+
					"between any two addresses rather than ze's own IKE sockets", family, p.Dir)
			}
			if p.UpperProto != protoUDP {
				t.Errorf("family %v dir %d: upper protocol %d, want UDP (%d); a port selector under another "+
					"protocol names a field that protocol may not carry", family, p.Dir, p.UpperProto, protoUDP)
			}
		}
	}
}
