// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA policies
// Related: child.go -- childPolicyParams and selectorPort, the producers
// RFC: rfc/short/rfc4301.md -- tunnel mode SAs that ignore port fields (Section 7.1)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// rfc4301ChildWith builds a tunnel-mode Child SA over two prefixes carrying the one
// negotiated selector pair given, or no selector set at all when pair is nil.
func rfc4301ChildWith(pair *tsPair) *ChildSA {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	child := &ChildSA{
		InboundSPI:          1,
		OutboundSPI:         2,
		LocalAddr:           net.ParseIP("192.0.2.1"),
		RemoteAddr:          net.ParseIP("192.0.2.2"),
		TSLocal:             local,
		TSRemote:            remote,
		Owner:               testPolicyOwner,
		Mode:                modeTunnel,
		ReqID:               defaultReqID,
		SelectorsLocalIsTSi: true,
	}
	if pair != nil {
		child.Selectors = []tsPair{*pair}
	}
	return child
}

// rfc4301PortsAny reports whether both port selectors of a policy are unconstrained.
func rfc4301PortsAny(p dataplane.SPParams) bool {
	return p.SrcPort.IsAny() && p.DstPort.IsAny()
}

// VALIDATES: RFC4301-7.1-1. A tunnel mode Child SA with no port constraint installs, in
// both directions, a PROTECT policy in tunnel mode whose port selectors are ANY, so every
// packet inside the prefixes is carried whatever its port fields hold.
// PREVENTS: a default Child SA that only carries one port, which drops every non-initial
// fragment and every protocol without ports.
// RFC requirement: RFC4301-7.1-1 positive -- a tunnel mode Child SA without a port constraint installs any-port tunnel policies in both directions.
func TestRFC4301TunnelSAPassesTrafficWithoutRegardToPorts(t *testing.T) {
	for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
		p := childPolicyParams(rfc4301ChildWith(nil), dir)
		if p.Mode != dataplane.ModeTunnel {
			t.Errorf("dir %d: mode = %d, want tunnel (%d)", dir, p.Mode, dataplane.ModeTunnel)
		}
		if p.Action != dataplane.SPActionProtect {
			t.Errorf("dir %d: action = %d, want PROTECT", dir, p.Action)
		}
		if !rfc4301PortsAny(p) {
			t.Errorf("dir %d: ports = src %+v dst %+v, want both any", dir, p.SrcPort, p.DstPort)
		}
		if p.UpperProto != 0 {
			t.Errorf("dir %d: protocol = %d, want ANY (0)", dir, p.UpperProto)
		}
	}
}

// VALIDATES: RFC4301-7.1-1. The any-port form is decided by the negotiated selector and
// is not a constant: a Child SA whose selector names one port installs that port, in
// the orientation of each direction, and never the any-port policy.
// PREVENTS: a producer that answers ANY for every Child SA, which would pass the test
// above while programming more traffic than the peer negotiated.
// RFC requirement: RFC4301-7.1-1 negative -- a Child SA whose selector names a port never installs an any-port policy.
func TestRFC4301PortScopedSAIsNeverInstalledAsAnyPort(t *testing.T) {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	pair := &tsPair{
		I: tsSelector{Net: local, Proto: 6, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443}},
		R: tsSelector{Net: remote, Proto: 6, Port: ipsec.AnyPort()},
	}
	out := childPolicyParams(rfc4301ChildWith(pair), dataplane.SADirOut)
	if rfc4301PortsAny(out) {
		t.Fatal("a port-scoped selector installed an any-port outbound policy")
	}
	if out.SrcPort != dataplane.ExactPortMatch(443) {
		t.Fatalf("outbound source port = %+v, want exact 443", out.SrcPort)
	}
	in := childPolicyParams(rfc4301ChildWith(pair), dataplane.SADirIn)
	if rfc4301PortsAny(in) {
		t.Fatal("a port-scoped selector installed an any-port inbound policy")
	}
	if in.DstPort != dataplane.ExactPortMatch(443) {
		t.Fatalf("inbound destination port = %+v, want exact 443", in.DstPort)
	}
}

// VALIDATES: RFC4301-7.1-2. A Child SA that carries one protocol without regard to ports
// installs a policy naming that protocol with both port fields ANY.
// PREVENTS: a protocol-scoped SA that silently acquires a port constraint from a zero
// value, which would drop every packet of the protocol on any other port.
// RFC requirement: RFC4301-7.1-2 positive -- a selector naming a protocol with ANY ports installs that protocol with both ports unconstrained.
func TestRFC4301ProtocolScopedSASpecifiesPortsAsAny(t *testing.T) {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	pair := &tsPair{
		I: tsSelector{Net: local, Proto: 17, Port: ipsec.AnyPort()},
		R: tsSelector{Net: remote, Proto: 17, Port: ipsec.AnyPort()},
	}
	for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
		p := childPolicyParams(rfc4301ChildWith(pair), dir)
		if p.UpperProto != 17 {
			t.Errorf("dir %d: protocol = %d, want 17", dir, p.UpperProto)
		}
		if !rfc4301PortsAny(p) {
			t.Errorf("dir %d: ports = src %+v dst %+v, want both any", dir, p.SrcPort, p.DstPort)
		}
	}
}

// VALIDATES: RFC4301-7.1-2. Under the ANY port form no port number reaches the policy:
// a stray value in the Port field of an ANY selector is ignored, so the installed
// selector is unconstrained and never an exact match on that number.
// PREVENTS: the ANY form being read as "port 0" or as whatever number the field held,
// either of which narrows a protocol-scoped SA to one port.
// RFC requirement: RFC4301-7.1-2 negative -- a stray port number under the ANY form never reaches the installed policy.
func TestRFC4301AnyPortFormNeverLeaksAPortNumber(t *testing.T) {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	pair := &tsPair{
		I: tsSelector{Net: local, Proto: 6, Port: ipsec.PortSelector{Form: ipsec.PortAny, Port: 8080}},
		R: tsSelector{Net: remote, Proto: 6, Port: ipsec.PortSelector{Form: ipsec.PortAny, Port: 9090}},
	}
	for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
		p := childPolicyParams(rfc4301ChildWith(pair), dir)
		for _, port := range []dataplane.PortMatch{p.SrcPort, p.DstPort} {
			if port == dataplane.ExactPortMatch(8080) || port == dataplane.ExactPortMatch(9090) || port == dataplane.ExactPortMatch(0) {
				t.Errorf("dir %d: the ANY form installed an exact port %+v", dir, port)
			}
		}
	}
}
