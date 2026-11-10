// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: vpp_policy.go -- vppICMPSelectorError; icmp_selector_test.go -- MaskedBlocks
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Section 4.4.1.1)

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"testing"
)

// VALIDATES: the VPP backend refuses an ICMP policy that names a type or code, and
// accepts an ICMP policy with neither.
// PREVENTS: VPP installing an ICMP type entry that its SPD match would apply to every
// ICMP message, because it compares ports only for TCP, UDP and SCTP, and the refusal
// reading as a platform without IPsec (ErrNotSupported), which keeps a Child SA degraded.
func TestVPPRefusesAnICMPTypeSelector(t *testing.T) {
	_, local, _ := net.ParseCIDR("192.0.2.0/24")
	_, remote, _ := net.ParseCIDR("198.51.100.0/24")
	sp := SPParams{
		Src: local, Dst: remote, Dir: SADirIn, Action: SPActionDiscard,
		Priority: 1000, UpperProto: UpperProtoICMP,
		SrcPort: ExactPortMatch(8), DstPort: AnyPortMatch(),
	}
	if _, err := vppSPDSelector(sp); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Fatalf("ICMP type 8 on VPP: err = %v, want ErrSAOptionRefused and not ErrNotSupported", err)
	}
	sp.UpperProto = UpperProtoICMPv6
	if _, err := vppSPDSelector(sp); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Fatalf("ICMPv6 type 8 on VPP: err = %v, want ErrSAOptionRefused and not ErrNotSupported", err)
	}
	sp.UpperProto, sp.SrcPort = UpperProtoICMP, AnyPortMatch()
	if _, err := vppSPDSelector(sp); err != nil {
		t.Fatalf("ICMP with any type and code on VPP was refused: %v", err)
	}
}
