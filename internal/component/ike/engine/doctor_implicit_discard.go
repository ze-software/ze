// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the nominal final entry of the SPD
// Related: unmatched.go -- implicitDiscardInForce, refuseImplicitDiscard, warnImplicitDiscardAtStart
// Related: doctor.go -- the sibling ike readiness checks and their registration

package engine

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkIPsecImplicitDiscard reports the discard catch-all in force only because
// `vpn ipsec unmatched` is unset.
//
// refuseImplicitDiscard refuses a commit that enters this state, but a configuration
// the daemon starts with is never verified, so a box can boot into it. The start says
// so once in the log (warnImplicitDiscardAtStart); this check says it whenever the
// operator asks, with the same text and code, because the router's own BGP, SSH, DNS,
// NTP, ICMP and neighbor discovery outside a tunnel are dropped and the log line is
// long gone by the time someone looks.
//
// It is an error rather than a warning: traffic the operator never named is being
// dropped now, and only the operator writing the leaf says that is intended.
func checkIPsecImplicitDiscard(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		// The check cannot read the configuration, so it cannot answer: it says so
		// rather than read as "no discard in force", the convention of the ddos
		// flowspec orphan-record check.
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorIPsecImplicitDiscard,
			Severity: severityWarning,
			Message:  "vpn ipsec: this doctor run holds no parsed configuration, so it cannot say whether the SPD catch-all DISCARD is in force because `vpn ipsec unmatched` is not set; the command below prints the running configuration, whose `vpn ipsec unmatched` leaf answers it",
			Fix:      "show config running",
		}}
	}
	// refuse-by-default: guards ipsec.ParseIPsecConfig -- it answers a nil tree with EmptyConfig, whose bypass catch-all this check never reports, and the runner reports a missing config before this phase
	if tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	// refuse-by-default: guards checkIPsecInterface -- it already reports an unparseable vpn ipsec section, with the same tree and in the same phase, and a second report would make one config error read as two unrelated faults
	if err != nil {
		return nil
	}
	// refuse-by-default: guards implicitDiscardInForce -- it reads the config's fields, and ParseIPsecConfig answers a nil config only beside an error
	if cfg == nil {
		return nil
	}
	if !implicitDiscardInForce(cfg) {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecImplicitDiscard,
		Severity: severityError,
		Message:  implicitDiscardInForceText,
		// The confirming command: the RFC default stays, now written. The message names
		// the bypass entries to add first and the other way forward.
		Fix: "set vpn ipsec unmatched discard",
	}}
}
