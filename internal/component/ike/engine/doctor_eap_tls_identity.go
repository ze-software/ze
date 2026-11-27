// Design: docs/architecture/doctor-and-health-checks.md -- post-config readiness checks
// RFC: rfc/short/rfc5216.md -- Section 5.3, peer and server identity authorization
// Related: register.go -- DoctorChecks registration; register_doctor_codes.go -- diagnostic code

package engine

import (
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const fixShowConfigRunning = "show config running"

// checkIPsecEAPTLSIdentity reports site-to-site eap-tls peers that lack a valid
// certificate identity constraint (allowed-identities on respond peers, or
// server-identity / unambiguous FQDN remote-address on initiate peers).
//
// Validation, commit, and reload refuse these configurations fail-closed, but a
// daemon started from an unvalidated config file can reach runtime with an
// unconstrained peer. Both runtime builders refuse the handshake fail-closed,
// and this post-config check surfaces every affected peer with its exact fix.
func checkIPsecEAPTLSIdentity(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     codeDoctorIPsecEAPTLSIdentity,
			Severity: severityWarning,
			Message:  "vpn ipsec: this doctor run holds no parsed configuration, so it cannot check whether EAP-TLS peers constrain remote certificate identities; the command below prints the running configuration",
			Fix:      fixShowConfigRunning,
		}}
	}
	// refuse-by-default: guards ipsec.ParseIPsecConfig -- it answers a nil tree with EmptyConfig, which holds no peers, and the runner reports a missing config before this phase.
	if tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	// refuse-by-default: guards checkIPsecInterface -- it already reports an unparseable vpn ipsec section with the same tree and in the same phase.
	if err != nil {
		return nil
	}
	// refuse-by-default: guards cfg.Peers -- ParseIPsecConfig answers a nil config only beside an error.
	if cfg == nil {
		return nil
	}

	var diags []rpc.DoctorCheckDiagnostic
	for _, name := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[name]
		if peer.Auth.Mode != ipsec.AuthEAPTLS {
			continue
		}
		refusal := ipsec.ValidatePeerEAPTLSIdentity(name, &peer)
		if refusal == nil {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(refusal.Problem()).Str("; ").Str(refusal.Why())
		if cause := refusal.Unwrap(); cause != nil {
			tb.Str(": ").Err(cause)
		}
		diags = append(diags, rpc.DoctorCheckDiagnostic{
			Code:     codeDoctorIPsecEAPTLSIdentity,
			Severity: severityError,
			Message:  tb.String(),
			Fix:      refusal.Fix(),
		})
	}
	return diags
}
