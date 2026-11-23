// Design: docs/architecture/fib/fib-depth-4-srv6.md -- VPP SRv6 refusal and ownership-restore doctor check
// Related: srv6_state.go -- durable ownership restore and store-unavailable classification
// Related: srv6.go -- SRv6 steering lifecycle whose refusals this check reports
// Related: register_doctor.go -- registers the explanation of the code this check reports
//
// The fib-vpp-srv6 check reports what only the writer knows: an SRv6 ownership
// reconciliation error that blocks route mutations to preserve forwarding, and
// each SRv6 service route fib-vpp refused (for example because no persistent
// daemon store is initialized or VPP steering failed). When no persistent store
// exists and no SRv6 service route was requested, plain IP and MPLS routes are
// programmed normally and this check stays silent. It runs in the runtime phase
// through fib-vpp's own doctor callback, because the writer lives in fib-vpp's
// process, which may be a forked one.
package fibvpp

import (
	"cmp"
	"slices"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// srv6CheckName is the runtime doctor check for fib-vpp's SRv6 ownership and
// route refusals.
const srv6CheckName = "fib-vpp-srv6"

// codeSRv6Refused names an SRv6 ownership reconciliation failure or an SRv6
// service route that fib-vpp refused to program into VPP.
const codeSRv6Refused = "doctor-fib-vpp-srv6-refused"

// srv6CheckDecl declares the runtime check in fib-vpp's Stage 1 registration,
// so `show doctor` asks fib-vpp's process for it.
var srv6CheckDecl = rpc.DoctorCheckDecl{
	Name:  srv6CheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{codeSRv6Refused},
}

// srv6RefusedCodeMeta is the explanation `ze show diagnostic` prints for
// codeSRv6Refused.
var srv6RefusedCodeMeta = diagnostic.CodeMeta{
	Code:  codeSRv6Refused,
	Title: "VPP SRv6 service route or ownership restore was refused",
	Description: "fib-vpp records VPP SRv6 policies, steering entries and ordinary fallback routes in " +
		"daemon-owned durable state so a restart can reconcile ownership without touching another writer's " +
		"entries. When no persistent store is configured, plain IP and MPLS routes are still programmed, " +
		"while any SRv6 service route is refused and reported here. This ERROR also fires when durable SRv6 " +
		"ownership cannot be restored (corrupt state, unreadable store, conflicting VPP policy, or an " +
		"unconfirmed mutation), in which case fib-vpp blocks further route mutations to preserve forwarding, " +
		"or when an SRv6 steering mutation fails. Remedy: when durable state is unavailable, initialize a " +
		"persistent store with `ze init`, then restart Ze; when ownership restore failed, run `ze show data check`, " +
		"then restart Ze; otherwise restart Ze using the command in the finding's fix.",
	Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-fib-vpp-srv6-refused"},
}

// srv6RefusalFix returns the remediation command for one refused SRv6 route.
func srv6RefusalFix(err error) string {
	if isSRv6StoreUnavailable(err) {
		return "ze init, then " + privilege.RestartRemedy()
	}
	return privilege.RestartRemedy()
}

// srv6DoctorFindings answers the fib-vpp-srv6 runtime check. With no writer
// there is no SRv6 state to judge: VPP being unreachable is reported by the
// vpp component's own checks. Safe for concurrent use: takes f.mu.
func srv6DoctorFindings(f *fibVPP) []rpc.DoctorCheckDiagnostic {
	if f == nil {
		// refuse-by-default: guards answerDoctorCheck -- an unreachable VPP is the vpp component's finding, not this check's
		return nil
	}
	f.mu.Lock()
	restoreErr := f.srv6RestoreErr
	keys := make([]srv6RouteKey, 0, len(f.srv6Refusals))
	refusals := make(map[srv6RouteKey]srv6Refusal, len(f.srv6Refusals))
	for k, ref := range f.srv6Refusals {
		keys = append(keys, k)
		refusals[k] = ref
	}
	f.mu.Unlock()

	var findings []rpc.DoctorCheckDiagnostic
	if restoreErr != nil {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: SRv6 ownership reconciliation failed (").Str(restoreErr.Error()).
			Str("), so fib-vpp blocks route mutations to preserve forwarding")
		findings = append(findings, srv6RefusedFinding(tb.String(), "ze show data check, then "+privilege.RestartRemedy()))
	}
	slices.SortFunc(keys, func(a, b srv6RouteKey) int {
		if c := cmp.Compare(a.table, b.table); c != 0 {
			return c
		}
		return a.prefix.Compare(b.prefix)
	})
	shown := min(len(keys), labelFindingsMax)
	for _, k := range keys[:shown] {
		ref := refusals[k]
		var tb textbuf.Buffer
		tb.Str("fib-vpp: SRv6 route ").Str(k.prefix.String()).Str(" in IP table ").Int(int64(k.table))
		if ref.sid.IsValid() {
			tb.Str(" (SID ").Str(ref.sid.String()).Byte(')')
		}
		tb.Str(" was refused (").Str(ref.err.Error()).Str("), so it is not forwarding via SRv6")
		findings = append(findings, srv6RefusedFinding(tb.String(), srv6RefusalFix(ref.err)))
	}
	if rest := len(keys) - shown; rest > 0 {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: ").Int(int64(rest)).Str(" more SRv6 routes were refused and are not forwarding via SRv6")
		var overflowErr error
		for _, k := range keys[shown:] {
			if isSRv6StoreUnavailable(refusals[k].err) {
				overflowErr = refusals[k].err
				break
			}
		}
		findings = append(findings, srv6RefusedFinding(tb.String(), srv6RefusalFix(overflowErr)))
	}
	return findings
}

// srv6RefusedFinding is one codeSRv6Refused error carrying message and fix.
func srv6RefusedFinding(message, fix string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     codeSRv6Refused,
		Severity: string(diagnostic.SeverityError),
		Message:  message,
		Fix:      fix,
	}
}
