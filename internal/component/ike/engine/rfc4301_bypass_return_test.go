// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: spd_policy.go -- spdPolicyParams, the producer
// RFC: rfc/short/rfc4301.md -- a bypass entry's SPD-I and SPD-O halves (Section 4.4.1)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// rfc4301BypassEntry is one operator BYPASS entry with a distinct local and remote
// side on both the address and the port, so a mirrored entry and an unmirrored copy
// can be told apart.
func rfc4301BypassEntry(dir ipsec.SPDDirection) ipsec.SPDPolicy {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	return ipsec.SPDPolicy{
		Name:         "mgmt",
		Action:       dataplane.SPActionBypass,
		Order:        1000,
		Direction:    dir,
		Protocol:     6,
		LocalPrefix:  local,
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: 22},
		RemotePrefix: remote,
		RemotePort:   ipsec.AnyPort(),
	}
}

// rfc4301Half returns the entry of one direction, or nil when the set has none.
func rfc4301Half(set []dataplane.SPParams, dir dataplane.SADir) *dataplane.SPParams {
	for i := range set {
		if set[i].Dir == dir {
			return &set[i]
		}
	}
	return nil
}

// VALIDATES: RFC4301-4.4.1-11. A BYPASS entry whose return traffic also bypasses yields an
// SPD-I entry, and that entry permits the RETURN packet: its source is the remote
// prefix, its destination the local prefix, and the ports travel with them.
// PREVENTS: the return packet of a bypassed flow arriving with no SPD-I match and being
// discarded.
// RFC requirement: RFC4301-4.4.1-11 positive -- a both-direction BYPASS entry installs one SPD-I entry whose selector is the inbound traffic: source 10.2.0.0/24, destination 10.1.0.0/24, source port any, destination port 22.
func TestRFC4301BypassReturnTrafficHasAnSPDIEntry(t *testing.T) {
	in := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirBoth)), dataplane.SADirIn)
	if in == nil {
		t.Fatal("a both-direction bypass produced no SPD-I entry")
	}
	if in.Action != dataplane.SPActionBypass {
		t.Fatalf("SPD-I action = %d, want BYPASS", in.Action)
	}
	if in.Src.String() != "10.2.0.0/24" || in.Dst.String() != "10.1.0.0/24" {
		t.Fatalf("SPD-I selector = %v -> %v, want the return flow 10.2.0.0/24 -> 10.1.0.0/24", in.Src, in.Dst)
	}
	if in.DstPort != dataplane.ExactPortMatch(22) || !in.SrcPort.IsAny() {
		t.Fatalf("SPD-I ports = src %+v dst %+v, want src any, dst 22", in.SrcPort, in.DstPort)
	}
}

// VALIDATES: RFC4301-4.4.1-11. The SPD-I entry is never an unmirrored copy of the outbound
// one, and an entry the operator scoped to SPD-O alone never grows an SPD-I twin.
// PREVENTS: an inbound entry that matches the outbound flow's orientation, which permits
// nothing that arrives, and an inbound hole the operator never wrote.
// RFC requirement: RFC4301-4.4.1-11 negative -- the SPD-I entry never carries the outbound traffic's selector values, and an out-only BYPASS entry installs no SPD-I entry.
func TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy(t *testing.T) {
	in := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirBoth)), dataplane.SADirIn)
	if in == nil {
		t.Fatal("a both-direction bypass produced no SPD-I entry")
	}
	if in.Src.String() == "10.1.0.0/24" || in.SrcPort == dataplane.ExactPortMatch(22) {
		t.Fatal("the SPD-I entry carries the outbound orientation")
	}
	if got := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirOut)), dataplane.SADirIn); got != nil {
		t.Fatalf("an out-only bypass grew an SPD-I entry: %+v", *got)
	}
}

// VALIDATES: RFC4301-4.4.1-12. An inbound-bypassed entry whose return traffic also bypasses
// yields the SPD-O entry that return traffic is matched against: source local, destination
// remote, ports in the outbound orientation.
// PREVENTS: the reply to a bypassed inbound packet meeting a PROTECT entry, or no entry,
// on its way out.
// RFC requirement: RFC4301-4.4.1-12 positive -- a both-direction BYPASS entry installs one SPD-O entry whose selector is the outbound traffic: source 10.1.0.0/24, destination 10.2.0.0/24, source port 22, destination port any.
func TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry(t *testing.T) {
	out := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirBoth)), dataplane.SADirOut)
	if out == nil {
		t.Fatal("a both-direction bypass produced no SPD-O entry")
	}
	if out.Action != dataplane.SPActionBypass {
		t.Fatalf("SPD-O action = %d, want BYPASS", out.Action)
	}
	if out.Src.String() != "10.1.0.0/24" || out.Dst.String() != "10.2.0.0/24" {
		t.Fatalf("SPD-O selector = %v -> %v, want 10.1.0.0/24 -> 10.2.0.0/24", out.Src, out.Dst)
	}
	if out.SrcPort != dataplane.ExactPortMatch(22) || !out.DstPort.IsAny() {
		t.Fatalf("SPD-O ports = src %+v dst %+v, want src 22, dst any", out.SrcPort, out.DstPort)
	}
}

// VALIDATES: RFC4301-4.4.1-12. The SPD-O entry never carries the inbound orientation, and an
// entry the operator scoped to SPD-I alone never grows an SPD-O twin.
// PREVENTS: an outbound entry no outbound packet can match, and an outbound bypass the
// operator never wrote.
// RFC requirement: RFC4301-4.4.1-12 negative -- the SPD-O entry never carries the inbound traffic's selector values, and an in-only BYPASS entry installs no SPD-O entry.
func TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy(t *testing.T) {
	out := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirBoth)), dataplane.SADirOut)
	if out == nil {
		t.Fatal("a both-direction bypass produced no SPD-O entry")
	}
	if out.Src.String() == "10.2.0.0/24" || out.DstPort == dataplane.ExactPortMatch(22) {
		t.Fatal("the SPD-O entry carries the inbound orientation")
	}
	if got := rfc4301Half(spdPolicyParams(rfc4301BypassEntry(ipsec.SPDDirIn)), dataplane.SADirOut); got != nil {
		t.Fatalf("an in-only bypass grew an SPD-O entry: %+v", *got)
	}
}
