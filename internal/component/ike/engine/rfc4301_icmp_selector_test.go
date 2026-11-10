// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: spd_policy.go -- spdICMPMatches, the operator ICMP selector producer
// Related: child.go -- icmpSelectorFromIKE, the negotiated ICMP selector producer
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Sections 4.4.1.1, 6.1)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// kernelICMPMatch applies the kernel's selector test to one ICMP message: the type
// is read into the source port and the code into the destination port, and each
// matches when (flow ^ sel) & mask is zero (net/xfrm/xfrm_policy.c,
// __xfrm4_selector_match; include/net/xfrm.h, xfrm_flowi_sport and xfrm_flowi_dport).
func kernelICMPMatch(sp dataplane.SPParams, typ, code uint16) bool {
	return (typ^sp.SrcPort.Port)&sp.SrcPort.Mask == 0 && (code^sp.DstPort.Port)&sp.DstPort.Mask == 0
}

// icmpEntry is a one-direction ICMP discard entry with the given selector.
func icmpEntry(t *testing.T, dir ipsec.SPDDirection, sel ipsec.ICMPSelector) ipsec.SPDPolicy {
	t.Helper()
	p := spdEntry(t, "icmp", dataplane.SPActionDiscard, dir)
	p.Protocol = dataplane.UpperProtoICMP
	p.ICMP = sel
	return p
}

// VALIDATES: RFC4301-4.4.1.1-4. For a single type and every code range, the policies
// an entry installs match an ICMP message exactly when RFC 4301's test
// (T-start*256) + C-start <= (t*256) + c <= (T-end*256) + C-end holds.
// PREVENTS: a code range rounded outward to a covering mask, which would discard or
// bypass codes the operator did not write, and a block lost in the decomposition.
// RFC requirement: RFC4301-4.4.1.1-4 positive -- the installed selectors of an ICMP code range match exactly the messages the RFC's range test admits.
// RFC requirement: RFC4301-4.4.1.1-4 negative -- an ICMP message the RFC's range test rejects, a code outside the range or a type one below or one above the entry's, matches none of the installed selectors.
func TestSPDICMPSelectorMatchesTheRFCRangeTest(t *testing.T) {
	const T = 3
	for cStart := range 256 {
		for cEnd := cStart; cEnd <= 255; cEnd++ {
			params := spdPolicyParams(icmpEntry(t, ipsec.SPDDirOut, ipsec.ICMPSelector{
				HasType: true, Type: T, HasCode: true, CodeStart: uint8(cStart), CodeEnd: uint8(cEnd),
			}))
			for _, typ := range []int{T - 1, T, T + 1} {
				for c := range 256 {
					rfc := T*256+cStart <= typ*256+c && typ*256+c <= T*256+cEnd
					hits := 0
					for i := range params {
						if kernelICMPMatch(params[i], uint16(typ), uint16(c)) {
							hits++
						}
					}
					if (hits > 0) != rfc {
						t.Fatalf("type %d codes %d-%d: message %d/%d matched %d policies, RFC test says %v",
							T, cStart, cEnd, typ, c, hits, rfc)
					}
					if hits > 1 {
						t.Fatalf("type %d codes %d-%d: message %d/%d matched %d overlapping policies",
							T, cStart, cEnd, typ, c, hits)
					}
				}
			}
		}
	}
}

// VALIDATES: a single type with ANY code is one exact type match, ANY type is no
// constraint, and type 0 stays exact rather than reading as ANY.
// PREVENTS: an echo-reply (type 0) entry installed as an any-type policy.
func TestSPDICMPSelectorTypeForms(t *testing.T) {
	typeZero := spdPolicyParams(icmpEntry(t, ipsec.SPDDirIn, ipsec.ICMPSelector{HasType: true, Type: 0}))
	if len(typeZero) != 1 {
		t.Fatalf("type 0 entry installed %d policies, want 1", len(typeZero))
	}
	if !kernelICMPMatch(typeZero[0], 0, 5) || kernelICMPMatch(typeZero[0], 8, 0) {
		t.Fatalf("type 0 selector %+v / %+v is not exactly type 0", typeZero[0].SrcPort, typeZero[0].DstPort)
	}
	anyType := spdPolicyParams(icmpEntry(t, ipsec.SPDDirIn, ipsec.ICMPSelector{}))
	if len(anyType) != 1 || !anyType[0].SrcPort.IsAny() || !anyType[0].DstPort.IsAny() {
		t.Fatalf("an any-type entry installed %+v, want one unconstrained policy", anyType)
	}
}

// VALIDATES: a both-direction ICMP entry installs the same type and code in each
// direction, while its prefixes swap.
// PREVENTS: the port swap of a both entry moving the type into the code field on the
// inbound half, which would match type 1 code 3 instead of type 3 code 1.
func TestSPDICMPSelectorDoesNotSwapWithDirection(t *testing.T) {
	params := spdPolicyParams(icmpEntry(t, ipsec.SPDDirBoth, ipsec.ICMPSelector{
		HasType: true, Type: 3, HasCode: true, CodeStart: 1, CodeEnd: 1,
	}))
	if len(params) != 2 {
		t.Fatalf("both entry installed %d policies, want 2", len(params))
	}
	for _, sp := range params {
		if sp.SrcPort != dataplane.ExactPortMatch(3) || sp.DstPort != dataplane.ExactPortMatch(1) {
			t.Errorf("dir %d: type %+v code %+v, want exactly type 3 code 1", sp.Dir, sp.SrcPort, sp.DstPort)
		}
	}
}

// VALIDATES: RFC4301-6.1.1-1 and RFC4301-6.1.1-2. An inbound discard entry naming
// one ICMP type becomes an inbound DISCARD policy for ICMP that matches that type
// and no other, and the same entry with action bypass becomes an inbound BYPASS.
// PREVENTS: the per-type accept/reject control of Section 6.1.1 matching every ICMP
// type (rejecting more than asked) or reaching the dataplane with no action.
// RFC requirement: RFC4301-6.1.1-1 positive -- an operator's inbound entry naming one ICMP type, with action discard or bypass, is built into one inbound SPD policy for ICMP carrying that action.
// RFC requirement: RFC4301-6.1.1-2 positive -- the inbound policy built from an entry naming ICMP type 3 matches ICMP type 3 under the kernel's port-mask match, with action discard and with action bypass.
// RFC requirement: RFC4301-6.1.1-1 negative -- an operator's inbound entry naming one ICMP type with action discard is built into an inbound DISCARD policy, never a BYPASS one.
// RFC requirement: RFC4301-6.1.1-2 negative -- the inbound policy built from an entry naming ICMP type 3 matches none of the other ICMP types 0 to 255, with action discard and with action bypass.
func TestSPDICMPTypeControlRejectsOrAcceptsOneType(t *testing.T) {
	for _, action := range []dataplane.SPAction{dataplane.SPActionDiscard, dataplane.SPActionBypass} {
		p := icmpEntry(t, ipsec.SPDDirIn, ipsec.ICMPSelector{HasType: true, Type: 3})
		p.Action = action
		params := spdPolicyParams(p)
		if len(params) != 1 {
			t.Fatalf("action %d: %d policies, want 1", action, len(params))
		}
		sp := params[0]
		if sp.Dir != dataplane.SADirIn || sp.Action != action || sp.UpperProto != dataplane.UpperProtoICMP {
			t.Fatalf("action %d: policy dir %d action %d proto %d, want inbound, the entry's action, ICMP",
				action, sp.Dir, sp.Action, sp.UpperProto)
		}
		for typ := range uint16(256) {
			if got := kernelICMPMatch(sp, typ, 0); got != (typ == 3) {
				t.Fatalf("action %d: type %d matched = %v, want only type 3", action, typ, got)
			}
		}
	}
}

// VALIDATES: RFC4301-6.1.2-1. An inbound discard entry whose remote prefix is the
// protected-side subnet and whose ICMP type is an error type becomes an inbound
// DISCARD policy whose source is that subnet, so the error is refused only from the
// protected side.
// PREVENTS: the protected-side control applying to the unprotected side, or the
// inbound half losing the swap that puts the remote prefix in the source field.
// RFC requirement: RFC4301-6.1.2-1 positive -- an inbound discard entry whose remote prefix is the protected-side subnet is built into one inbound DISCARD policy with that subnet as source and the local prefix as destination, matching ICMP type 3 and not type 0.
// RFC requirement: RFC4301-6.1.2-1 negative -- the inbound DISCARD policy built from an entry whose remote prefix is the protected-side subnet does not cover a source on the unprotected side (192.0.2.1, inside the local prefix), so the control does not reach ICMP from outside that subnet.
func TestSPDICMPErrorControlFromTheProtectedSide(t *testing.T) {
	p := icmpEntry(t, ipsec.SPDDirIn, ipsec.ICMPSelector{HasType: true, Type: 3})
	_, inside, err := net.ParseCIDR("10.1.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	p.RemotePrefix = inside
	params := spdPolicyParams(p)
	if len(params) != 1 {
		t.Fatalf("%d policies, want 1", len(params))
	}
	sp := params[0]
	// The negative side: a source outside the protected-side subnet is not under the
	// control. 192.0.2.1 lies in the entry's local prefix, the unprotected side here.
	// Errorf, so the exact check below still reports.
	if sp.Src != nil && sp.Src.Contains(net.ParseIP("192.0.2.1")) {
		t.Errorf("inbound policy source %s covers 192.0.2.1, a source outside the protected-side subnet", sp.Src)
	}
	if sp.Src.String() != "10.1.0.0/16" || sp.Dst.String() != p.LocalPrefix.String() {
		t.Fatalf("inbound policy %s -> %s, want 10.1.0.0/16 -> %s", sp.Src, sp.Dst, p.LocalPrefix)
	}
	if sp.Action != dataplane.SPActionDiscard || !kernelICMPMatch(sp, 3, 1) || kernelICMPMatch(sp, 0, 0) {
		t.Fatalf("policy %+v does not discard exactly ICMP type 3", sp)
	}
}

// VALIDATES: a negotiated ICMP selector (RFC 4301 Section 4.4.1.1, "For IKE, the
// message type is placed in the most significant 8 bits of the 16-bit selector") is
// programmed as a type and a code, from the sending side's value in each direction.
// PREVENTS: the 16-bit IKE value written into the kernel's source port as if it were
// a port, which matches type 2048 for echo request (8/0) and so no ICMP message.
func TestNegotiatedICMPSelectorProgramsTypeAndCode(t *testing.T) {
	_, local, _ := net.ParseCIDR("192.0.2.0/24")
	_, remote, _ := net.ParseCIDR("198.51.100.0/24")
	pair := policyPair{
		local: local, remote: remote, upperProto: dataplane.UpperProtoICMP,
		localPort:  dataplane.ExactPortMatch(0x0800), // type 8, code 0
		remotePort: dataplane.ExactPortMatch(0x0003), // type 0, code 3
	}
	out := childPairPolicy(&ChildSA{}, dataplane.SADirOut, pair)
	if out.SrcPort != dataplane.ExactPortMatch(8) || out.DstPort != dataplane.ExactPortMatch(0) {
		t.Errorf("outbound: type %+v code %+v, want exactly type 8 code 0 (the local side's echo request)", out.SrcPort, out.DstPort)
	}
	in := childPairPolicy(&ChildSA{}, dataplane.SADirIn, pair)
	if in.SrcPort != dataplane.ExactPortMatch(0) || in.DstPort != dataplane.ExactPortMatch(3) {
		t.Errorf("inbound: type %+v code %+v, want exactly type 0 code 3 (the remote side's value)", in.SrcPort, in.DstPort)
	}
	pair.remotePort = dataplane.AnyPortMatch()
	in = childPairPolicy(&ChildSA{}, dataplane.SADirIn, pair)
	if !in.SrcPort.IsAny() || !in.DstPort.IsAny() {
		t.Errorf("inbound with an any remote side: type %+v code %+v, want any", in.SrcPort, in.DstPort)
	}
}
