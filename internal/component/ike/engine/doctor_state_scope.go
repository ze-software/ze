// Design: docs/architecture/doctor-and-health-checks.md -- runtime kernel dependencies.
// Related: doctor_runtime.go -- engine-process declaration and callback dispatch.
// Related: dataplane/xfrm_state_scope_linux.go -- the shared live capability check.

package engine

import (
	"errors"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const stateScopeCheckName = "ipsec-state-scope"

var stateScopeCheckDecl = rpc.DoctorCheckDecl{
	Name:  stateScopeCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 745,
	Codes: []string{codeDoctorIPsecStateScope},
}

// answerStateScope reads the loaded backend in the engine's own process, even
// when no peer session or Child SA has started. Safe for concurrent use: applyMu
// protects the configured-peer snapshot, and the backend owns its live check.
func answerStateScope(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	s.applyMu.Lock()
	configured := s.applied != nil
	if configured {
		configured = len(s.applied.Peers) > 0
	}
	s.applyMu.Unlock()
	if !configured {
		// refuse-by-default: guards the runtime dependency -- no configured peer
		// can create a Child SA that needs a complete state scope.
		return nil
	}
	checker, ok := dataplane.Get().(dataplane.SAStateScopeChecker)
	if !ok {
		// refuse-by-default: guards backend scope capability -- undeclared backends
		// have no runtime scope dependency this check can inspect.
		return nil
	}
	err := checker.CheckSAStateScope()
	if err == nil {
		return nil
	}
	failure, canonical := errors.AsType[*opererr.Error](err)
	if !canonical {
		failure = opererr.Wrap(err,
			"Ze cannot inspect the IPsec SA's complete traffic selector scope",
			"The loaded dataplane could not verify the runtime checks needed before installing a Child SA; inspect the IKE engine's warnings.",
			childInstallFixLog)
	}
	message := failure.Problem() + " " + failure.Why()
	if cause := failure.Unwrap(); cause != nil {
		message += " (" + cause.Error() + ")"
	}
	return []rpc.DoctorCheckDiagnostic{{Code: codeDoctorIPsecStateScope,
		Severity: severityError, Message: message, Fix: failure.Fix()}}
}
