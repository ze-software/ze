// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_runtime.go -- engineDoctorChecks, where this runtime check is declared and dispatched
// Related: source_guard.go -- sourceGuards, which records the refusals this check reads
// Related: netlink_refusal.go -- sourceRuleRequest and nftRequest, the cause and fix of each refusal
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package engine

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourceGuardCheckName names the check in the engine's Stage 1 registration and in the
// doctor callback's dispatch.
const sourceGuardCheckName = "ipsec-source-guard"

// sourceGuardCheckDecl declares the check at the runtime phase, so show doctor asks the
// engine's own process: the guard state it reads lives only there (ai/rules/plugins.md).
var sourceGuardCheckDecl = rpc.DoctorCheckDecl{
	Name:  sourceGuardCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 748,
	Codes: []string{codeDoctorIPsecSourceGuardMissing},
}

// answerSourceGuard answers ipsec-source-guard from the engine's process. The guard is
// one per process (guards), so the engine state names nothing it reads.
func answerSourceGuard(*ikeEngineState) []rpc.DoctorCheckDiagnostic {
	return guards.findings()
}

// findings answers one error for each guarded address whose source exemption the guard
// keeps uninstalled because the kernel refused part of the guard, in interface and then
// address order: first when the last publish of the nft table failed and the table
// does not cover the address (publishRefused), then when the last attempt at its source
// rule failed (ruleRefused). Both name the cause and fix the log line named, from the
// same classifier (sourceRuleRequest, nftRequest). Safe for concurrent use.
func (g *sourceGuards) findings() []rpc.DoctorCheckDiagnostic {
	g.mu.Lock()
	defer g.mu.Unlock()

	var out []rpc.DoctorCheckDiagnostic
	for _, name := range sortedNames(g.held) {
		sources := slices.Clone(g.held[name].spec.sources)
		slices.SortFunc(sources, netip.Addr.Compare)
		for _, addr := range sources {
			if g.publishRefused != nil {
				if !g.published[addr] {
					out = append(out, sourceGuardPublishFinding(name, addr, g.publishRefused))
				}
			}
			if err, refused := g.ruleRefused[guardSourceKey{iface: name, addr: addr}]; refused {
				out = append(out, sourceGuardRuleFinding(name, addr, err))
			}
		}
	}
	return out
}

// sourceGuardRuleFinding is the finding for an address whose source rule the kernel
// refused with err.
func sourceGuardRuleFinding(iface string, addr netip.Addr, err error) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     codeDoctorIPsecSourceGuardMissing,
		Severity: severityError,
		Message: "Ze could not install the tunnel-source rule from " + addr.String() + " of XFRM interface " + iface +
			", so the address's IPv6 source exemption is not installed. " + sourceGuardMissingWhy(iface, addr, "rule") +
			" The cause: " + sourceRuleRequest.cause(err) + ". Once it is gone, a restart of Ze installs the guard. The error: " + oneLine(err) + ".",
		Fix: sourceRuleRequest.fix(err, ""),
	}
}

// sourceGuardPublishFinding is the finding for an address the nft table does not cover
// because its publish was refused with err.
func sourceGuardPublishFinding(iface string, addr netip.Addr, err error) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     codeDoctorIPsecSourceGuardMissing,
		Severity: severityError,
		Message: "Ze could not publish the tunnel-source guard's nft table " + sourceGuardTableName + " for " + addr.String() +
			" of XFRM interface " + iface + ", so the address's IPv6 source exemption is not installed. " +
			sourceGuardMissingWhy(iface, addr, "publication") +
			" The cause: " + nftRequest.cause(err) + ". Once it is gone, a restart of Ze installs the guard. The error: " + oneLine(err) + ".",
		Fix: nftRequest.fix(err, ""),
	}
}

// sourceGuardMissingWhy says why a missing source exemption matters, and what retries
// the refused step.
func sourceGuardMissingWhy(iface string, addr netip.Addr, step string) string {
	return "Under `unmatched discard` IPv6 host traffic from " + addr.String() + " routed into " + iface +
		" meets the node-wide DISCARD catch-all, so the tunnel bound to " + iface + " carries none of it." +
		" The next commit or address change retries the " + step + "."
}

// oneLine writes err on one line, because a finding is one line.
func oneLine(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", "; ")
}
