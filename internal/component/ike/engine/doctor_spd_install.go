// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_runtime.go -- engineDoctorChecks, where this runtime check is declared and dispatched
// Related: spd_policy.go -- installSPDPoliciesAtStart, which records the entries the start could not install
// Related: spd_policy.go -- spdReconcile, what each reconcile left in the dataplane, selector by selector
// RFC: rfc/short/rfc4301.md -- the SPD management interface (Section 4.4.1)

package engine

import (
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// spdInstallCheckName names the check in the engine's Stage 1 registration and in the
// doctor callback's dispatch.
const spdInstallCheckName = "ipsec-spd-install"

// spdInstallCheckDecl declares the check at the runtime phase, so show doctor asks the
// engine's own process: the record of what the start installed lives only there,
// internal and forked alike (ai/rules/plugins.md).
var spdInstallCheckDecl = rpc.DoctorCheckDecl{
	Name:  spdInstallCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 746,
	Codes: []string{diagnostic.CodeDoctorIPsecSPDInstallRefused, diagnostic.CodeDoctorIPsecSPDUndoIncomplete},
}

// recordSPDReconcile folds what one reconcile left in the dataplane into spdDiverged.
// The caller holds applyMu, as every apply does.
//
// The record is NOT replaced by each apply, because a reconcile rewrites only the
// selectors of the entries it changed: a divergence at a selector no later reconcile
// touches is still in the dataplane, so it stays recorded and reported until a restart
// rebuilds the SPD. A reconcile that leaves a selector holding what the running
// configuration means (an install, a restore or a removal the dataplane accepted)
// clears that selector's record, and one that fails there again replaces it. Settled
// selectors are applied before diverged ones, because the undo of a write runs after
// the write (spdReconcile).
func (s *ikeEngineState) recordSPDReconcile(outcome spdReconcile) {
	for _, sel := range outcome.settled {
		delete(s.spdDiverged, sel)
	}
	for _, d := range outcome.diverged {
		if s.spdDiverged == nil {
			s.spdDiverged = make(map[spdSelector]spdDivergence)
		}
		s.spdDiverged[d.selector] = d
	}
	if len(s.spdDiverged) == 0 {
		s.spdDiverged = nil
	}
}

// answerSPDInstall answers ipsec-spd-install from the engine's process: one error for
// each operator SPD entry of the running configuration that the dataplane refused at
// the start and that no later apply has installed, in entry name order; then one error
// for each selector at which a failed reconcile step left the dataplane disagreeing
// with the running configuration (recordSPDReconcile), in message order.
//
// A reload never leaves a refused entry behind: its refusal refuses the commit and
// keeps the running entries (reconcileSPDPolicies), so only the start, which nothing
// can refuse, records one (installSPDPoliciesAtStart). An accepted reload installs
// every entry it holds, a refused one included, and clears the record (applyConfig).
//
// Safe for concurrent use: applyMu guards both records, as it guards every member the
// apply writes.
func answerSPDInstall(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(s.spdRefused)+len(s.spdDiverged))
	for _, name := range slices.Sorted(maps.Keys(s.spdRefused)) {
		refusal := s.spdRefused[name]
		message := refusal.Problem() + ". " + refusal.Why() + "."
		if cause := refusal.Unwrap(); cause != nil {
			message += " The dataplane answered: " + cause.Error() + "."
		}
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecSPDInstallRefused,
			Severity: severityError,
			Message:  message,
			Fix:      refusal.Fix(),
		})
	}
	diverged := make([]string, 0, len(s.spdDiverged))
	for _, d := range s.spdDiverged {
		diverged = append(diverged, d.message())
	}
	slices.Sort(diverged)
	for _, message := range diverged {
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecSPDUndoIncomplete,
			Severity: severityError,
			Message:  message,
			Fix:      spdInstallFixRestart,
		})
	}
	return out
}
