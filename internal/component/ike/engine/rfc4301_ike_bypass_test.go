// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: bypass.go -- ikeBypassPolicies, the producer
// RFC: rfc/short/rfc4301.md -- IKE traffic needs an explicit BYPASS entry (Section 5.2)
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// ikePorts are the two UDP ports IKE runs on, written as IANA numbers rather than as
// the transport constants, so a mutation of a constant moves one side only.
var ikePorts = [...]uint16{500, 4500}

// VALIDATES: RFC4301-5.2-4. For each address family and each IKE port there is an
// explicit BYPASS entry in SPD-I, keyed on ze's own port as the destination, and its
// SPD-O twin keyed on that port as the source.
// PREVENTS: an inbound IKE datagram meeting a wide PROTECT entry first and being
// discarded as unprotected traffic, which stops every rekey and Delete.
// RFC requirement: RFC4301-5.2-4 positive -- an explicit inbound and outbound BYPASS entry exists for UDP 500 and UDP 4500.
func TestRFC4301IKETrafficHasAnExplicitBypassEntry(t *testing.T) {
	for _, family := range ikeBypassFamilies {
		got := ikeBypassPolicies(family)
		for _, port := range ikePorts {
			var in, out bool
			for _, p := range got {
				if p.Action != dataplane.SPActionBypass || p.UpperProto != 17 {
					continue
				}
				switch {
				case p.Dir == dataplane.SADirIn && p.DstPort == dataplane.ExactPortMatch(port):
					in = true
				case p.Dir == dataplane.SADirOut && p.SrcPort == dataplane.ExactPortMatch(port):
					out = true
				}
			}
			if !in {
				t.Errorf("family %v: no SPD-I BYPASS entry for UDP destination port %d", family, port)
			}
			if !out {
				t.Errorf("family %v: no SPD-O BYPASS entry for UDP source port %d", family, port)
			}
		}
	}
}

// VALIDATES: RFC4301-5.2-4. The IKE exemption is never anything but a BYPASS: no entry
// in the set carries PROTECT or DISCARD, none carries a transform template, and none
// leaves ze's own port unconstrained, so the entry is an explicit IKE exemption rather
// than a wildcard hole.
// PREVENTS: a PROTECT entry on the IKE ports handing ze's own IKE to ESP, and an
// any-port bypass exempting traffic the operator asked ze to protect.
// RFC requirement: RFC4301-5.2-4 negative -- no IKE entry is PROTECT or DISCARD, carries a template, or leaves ze's own port unconstrained.
func TestRFC4301IKEBypassIsNeverAProtectOrAWildcard(t *testing.T) {
	for _, family := range ikeBypassFamilies {
		for i, p := range ikeBypassPolicies(family) {
			if p.Action == dataplane.SPActionProtect || p.Action == dataplane.SPActionDiscard {
				t.Errorf("family %v policy[%d]: action %d is not BYPASS", family, i, p.Action)
			}
			if p.Mode != 0 || p.TunnelSrc != nil || p.TunnelDst != nil || p.ReqID != 0 {
				t.Errorf("family %v policy[%d]: an IKE bypass carries a transform template", family, i)
			}
			own := p.DstPort
			if p.Dir == dataplane.SADirOut {
				own = p.SrcPort
			}
			if own.IsAny() {
				t.Errorf("family %v policy[%d]: ze's own IKE port is unconstrained", family, i)
			}
			if own.Port != 500 && own.Port != 4500 {
				t.Errorf("family %v policy[%d]: own port %d is not an IKE port", family, i, own.Port)
			}
		}
	}
}
