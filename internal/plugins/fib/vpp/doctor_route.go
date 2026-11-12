// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- the FIB source that marks Ze's IP routes in VPP
// Related: routesource.go -- the stale-route sweep whose failures this check reports
// Related: register_doctor.go -- registers the explanation of the code this check reports
//
// The fib-vpp-stale-routes check reports what only the writer knows: an IP
// route or labeled push a previous run left in VPP under fib-vpp's FIB source
// that the sweep could not remove, IP tables the writer could not read when it
// opened, and a writer that could not open at all because VPP would not give
// it its FIB source. It runs in the runtime phase through fib-vpp's own doctor
// callback, because the writer lives in fib-vpp's process, which may be a
// forked one.
package fibvpp

import (
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// staleRoutesCheckName is the runtime doctor check for fib-vpp's IP routes.
const staleRoutesCheckName = "fib-vpp-stale-routes"

// codeStaleRoute names an IP route a previous fib-vpp writer left in VPP that
// this writer could not remove or could not look for, or a writer that has no
// FIB source to mark its routes with.
const codeStaleRoute = "doctor-fib-vpp-stale-route"

// staleRouteFix is the command that runs the sweep again: the restarted
// daemon opens a new writer, which reads fib-vpp's FIB source and removes
// every route sysrib and the label sources do not re-announce.
const staleRouteFix = "ze signal restart"

// staleRoutesCheckDecl declares the runtime check in fib-vpp's Stage 1
// registration, so `show doctor` asks fib-vpp's process for it.
var staleRoutesCheckDecl = rpc.DoctorCheckDecl{
	Name:  staleRoutesCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Codes: []string{codeStaleRoute},
}

// staleRouteCodeMeta is the explanation `ze show diagnostic` prints for
// codeStaleRoute.
var staleRouteCodeMeta = diagnostic.CodeMeta{
	Code:  codeStaleRoute,
	Title: "An IP route a previous run installed in VPP is still forwarding",
	Description: "VPP keeps its FIB when Ze stops, so the IP routes and labeled pushes a previous fib-vpp writer " +
		"installed survive a Ze restart. fib-vpp installs every IP route under a VPP FIB source of its own, " +
		"`ze-fib-vpp`, so it can tell them from interface, DHCP, CLI and other API clients' routes. When it opens " +
		"its writer it lists every route under that source in every IP table, asks the system RIB and the label " +
		"sources to re-announce what they want, and after 30 seconds removes every listed route nobody " +
		"re-announced; the removal takes away only fib-vpp's source, never another owner's route for the same " +
		"prefix. This ERROR means VPP refused to remove one, the IP tables could not be read, or VPP would not " +
		"register the FIB source, in which case fib-vpp programs no route at all. Remedy: restart Ze with " +
		"`ze signal restart`, which opens a new writer and runs the sweep again; when the FIB source is refused, " +
		"run a VPP release that serves `fib_source_add` and `ip_route_add_del_v2` first.",
	Examples: []string{"ze show doctor", "ze show diagnostic doctor-fib-vpp-stale-route"},
}

// staleRouteDoctorFindings answers the fib-vpp-stale-routes check. With no
// writer it reports the open failure only when the FIB source caused it:
// VPP being unreachable is reported by the vpp component's own checks.
func staleRouteDoctorFindings(f *fibVPP, unreachable error) []rpc.DoctorCheckDiagnostic {
	if f == nil {
		if errors.Is(unreachable, errFibSource) {
			var tb textbuf.Buffer
			tb.Str("fib-vpp: VPP did not give the writer its FIB source (").Str(unreachable.Error()).
				Str("), so fib-vpp programs no route into VPP and removes none a previous run left")
			return []rpc.DoctorCheckDiagnostic{staleRouteFinding(tb.String())}
		}
		// refuse-by-default: guards answerDoctorCheck -- an unreachable VPP is the vpp component's finding, not this check's
		return nil
	}
	f.mu.Lock()
	routes := f.routes
	f.mu.Unlock()
	// refuse-by-default: guards answerDoctorCheck -- a writer over a mock backend holds no VPP, so it has no sweep to report
	if routes == nil {
		return nil
	}
	keys, errs, dumpErr := routes.staleFailures()
	var findings []rpc.DoctorCheckDiagnostic
	if dumpErr != nil {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: VPP's IP tables could not be read when the writer opened (").Str(dumpErr.Error()).
			Str("), so IP routes a previous run left in VPP are neither known nor removed and may still forward")
		findings = append(findings, staleRouteFinding(tb.String()))
	}
	shown := min(len(keys), labelFindingsMax)
	for _, k := range keys[:shown] {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: ").Str(k.prefix.String()).Str(" in IP table ").Int(int64(k.table)).
			Str(" was left in VPP by a previous run, nothing re-announced it, and VPP refused to remove it (").
			Str(errs[k].Error()).Str("), so it still forwards")
		findings = append(findings, staleRouteFinding(tb.String()))
	}
	if rest := len(keys) - shown; rest > 0 {
		var tb textbuf.Buffer
		tb.Str("fib-vpp: ").Int(int64(rest)).
			Str(" more IP routes left in VPP by a previous run could not be removed")
		findings = append(findings, staleRouteFinding(tb.String()))
	}
	return findings
}

// staleRouteFinding is one codeStaleRoute error carrying message.
func staleRouteFinding(message string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     codeStaleRoute,
		Severity: string(diagnostic.SeverityError),
		Message:  message,
		Fix:      staleRouteFix,
	}
}
