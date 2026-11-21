// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: vpp_policy.go -- vppICMPSelectorError; icmp_selector_test.go -- MaskedBlocks
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Section 4.4.1.1)

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
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

// VALIDATES: the VPP refusal of a port selector on a protocol whose ports VPP does not
// compare names, as its fix, the set commands that make both ports any for the config
// that wrote the entry: the operator policy for an operator entry, the peer's traffic
// selector for a Child SA entry, and both when the role names neither.
// PREVENTS: a fix that is a config fragment rather than a command, and a fix that
// names the policy tree for a Child SA the peer's traffic selector configured.
func TestVPPConstrainedSelectorRefusalNamesTheSetCommand(t *testing.T) {
	_, local, _ := net.ParseCIDR("192.0.2.0/24")
	_, remote, _ := net.ParseCIDR("198.51.100.0/24")
	const (
		operatorFix = "set vpn ipsec policy <name> local port any; set vpn ipsec policy <name> remote port any"
		peerFix     = "set vpn ipsec site-to-site peer branch traffic-selector <n> local port any; " +
			"set vpn ipsec site-to-site peer branch traffic-selector <n> remote port any"
	)
	for _, tc := range []struct {
		role PolicyRole
		want string
	}{
		{PolicyRoleOperator, operatorFix},
		{PolicyRoleChildSA, peerFix},
		{PolicyRoleUnspecified, operatorFix + ", or " + peerFix},
	} {
		sp := SPParams{
			Src: local, Dst: remote, Dir: SADirOut, Action: SPActionBypass, Priority: 1000,
			UpperProto: 136, SrcPort: ExactPortMatch(5000), DstPort: AnyPortMatch(),
			Role: tc.role, Owner: "branch",
		}
		err := vppICMPSelectorError(sp)
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok || !errors.Is(err, ErrSAOptionRefused) {
			t.Fatalf("%s: err = %v, want an operator error wrapping ErrSAOptionRefused", tc.role, err)
		}
		if refusal.Fix() != tc.want {
			t.Errorf("%s: fix = %q, want %q", tc.role, refusal.Fix(), tc.want)
		}
	}
}
