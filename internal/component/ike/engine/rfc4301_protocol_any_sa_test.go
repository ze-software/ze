// VALIDATES: RFC 4301 Section 7.1, "If the SA will carry traffic without regard to a
// specific protocol value (i.e., ANY is specified as the (Next Layer) protocol selector
// value), then the port field values are undefined and MUST be set to ANY as well." on
// the path that builds a Child SA's selectors: the peer's proposal is narrowed by
// narrowSelectors (ts_narrow.go), and the narrowed pair becomes the policy the kernel
// holds through childPolicyParams (child.go).
// PREVENTS: a protocol-ANY Child SA installed with a port constraint, whether the port
// came from the peer's proposal as a single port or as OPAQUE, on either side of the pair.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// RFC requirement: RFC4301-7.1-3 positive -- a peer proposal with protocol ANY and ports ANY narrows to a protocol-ANY pair, and the Child SA policies built from it, inbound and outbound, carry protocol ANY with both ports ANY.
func TestRFC4301ProtocolAnyChildSACarriesAnyPorts(t *testing.T) {
	proposedI := []tsSelector{selPort(t, "10.0.0.0/24", ipsec.AnyPort(), 0)}
	proposedR := []tsSelector{selPort(t, "10.1.0.0/24", ipsec.AnyPort(), 0)}
	for _, policy := range [][]tsPair{nil, {{I: sel(t, "10.0.0.0/16"), R: sel(t, "10.1.0.0/16")}}} {
		pairs, ok := narrowSelectors(proposedI, proposedR, policy, nil)
		if !ok || len(pairs) != 1 {
			t.Fatalf("policy %v: narrowing a protocol-ANY proposal gave %v ok=%v, want one pair", policy, pairs, ok)
		}
		child := &ChildSA{
			TSLocal:   pairs[0].R.Net,
			TSRemote:  pairs[0].I.Net,
			Selectors: pairs,
		}
		for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
			p := childPolicyParams(child, dir)
			if p.UpperProto != 0 {
				t.Errorf("policy %v dir %d: protocol %d, want ANY (0)", policy, dir, p.UpperProto)
			}
			if !p.SrcPort.IsAny() || !p.DstPort.IsAny() {
				t.Errorf("policy %v dir %d: ports %+v/%+v under protocol ANY, want ANY/ANY", policy, dir, p.SrcPort, p.DstPort)
			}
		}
	}
}

// RFC requirement: RFC4301-7.1-3 negative -- a peer proposal that pairs protocol ANY with a single port or an OPAQUE port, on the initiator or the responder side, narrows to no pair, so no Child SA is built with a port under protocol ANY.
func TestRFC4301ProtocolAnyChildSARefusesAPort(t *testing.T) {
	anyPort := selPort(t, "10.1.0.0/24", ipsec.AnyPort(), 0)
	for _, port := range []ipsec.PortSelector{
		{Form: ipsec.PortSingle, Port: 443},
		{Form: ipsec.PortOpaque},
	} {
		withPort := selPort(t, "10.0.0.0/24", port, 0)
		cases := []struct {
			name      string
			proposedI []tsSelector
			proposedR []tsSelector
		}{
			{"initiator side", []tsSelector{withPort}, []tsSelector{anyPort}},
			{"responder side", []tsSelector{anyPort}, []tsSelector{withPort}},
		}
		for _, tc := range cases {
			for _, policy := range [][]tsPair{nil, {{I: sel(t, "10.0.0.0/8"), R: sel(t, "10.0.0.0/8")}}} {
				if pairs, ok := narrowSelectors(tc.proposedI, tc.proposedR, policy, nil); ok || len(pairs) != 0 {
					t.Errorf("port %v %s policy %v: protocol ANY with a port narrowed to %v, want no pair", port, tc.name, policy, pairs)
				}
			}
		}
	}
}
