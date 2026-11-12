// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: register.go -- runEngine declares these checks at Stage 1 and answers them through OnDoctorCheck
// Related: doctor_esp_form_fast_path.go -- the ipsec-esp-form-fast-path check
// Related: doctor_child_install.go -- the ipsec-child-install check

package engine

import (
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// engineDoctorCheck is one runtime doctor check the engine answers from its own process.
type engineDoctorCheck struct {
	// decl is what the Stage 1 registration declares.
	decl rpc.DoctorCheckDecl
	// answer reads the engine's live state and answers the findings. It only reads.
	answer func(s *ikeEngineState) []rpc.DoctorCheckDiagnostic
}

// engineDoctorChecks holds every runtime check the engine declares at Stage 1. Each is
// declared once, here: the registration's DoctorChecks (engineDoctorCheckDecls) and the
// doctor callback's dispatch (answerDoctorCheck) both read this list, so a check that is
// declared is a check that is answered.
//
// A check here MUST NOT also be a registry.DoctorCheckDef in the registry row
// (register.go): Stage 1 refuses a name declared on both routes
// (refuseDoctorCheckDeclaredTwice, internal/component/plugin/server/startup.go).
var engineDoctorChecks = []engineDoctorCheck{{
	decl:   espFormFastPathCheckDecl,
	answer: answerESPFormFastPath,
}, {
	decl:   childInstallCheckDecl,
	answer: answerChildInstall,
}}

// engineDoctorCheckDecls answers the declarations the engine sends at Stage 1.
func engineDoctorCheckDecls() []rpc.DoctorCheckDecl {
	decls := make([]rpc.DoctorCheckDecl, 0, len(engineDoctorChecks))
	for i := range engineDoctorChecks {
		decls = append(decls, engineDoctorChecks[i].decl)
	}
	return decls
}

// answerDoctorCheck answers show doctor's request for the check named name. A name the
// engine did not declare is refused: the daemon asks only for declared checks, so only
// a Ze defect reaches that branch, and an empty answer would read as a healthy box.
func (s *ikeEngineState) answerDoctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	for i := range engineDoctorChecks {
		if engineDoctorChecks[i].decl.Name == name {
			return engineDoctorChecks[i].answer(s), nil
		}
	}
	return nil, fmt.Errorf("ike: the engine declares no doctor check named %q, so it cannot answer it; this is a Ze defect", name)
}

// answerESPFormFastPath answers ipsec-esp-form-fast-path from the engine's process. It
// reads the outcome of the engine's last attempt to open the TUN clone device
// (recordESPFormTUN, apply.go) and never makes one: a runtime check only reads.
func answerESPFormFastPath(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	return checkIPsecESPFormFastPath(s.espFormTUN.Load())
}
