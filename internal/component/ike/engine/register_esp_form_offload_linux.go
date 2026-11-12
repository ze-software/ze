// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- detect a loaded ESP GRO module before calling both receive paths usable
// Related: doctor_esp_form_offload_linux.go -- the engine's read-only runtime check

//go:build linux

package engine

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The module list is the engine process's view of the host, so this check uses the
// same runtime callback as the receiver's state, including when IKE runs forked.
func init() {
	engineDoctorChecks = append(engineDoctorChecks, engineDoctorCheck{
		decl: rpc.DoctorCheckDecl{
			Name:  "ipsec-esp-form-offload",
			Phase: rpc.DoctorPhaseRuntime,
			Order: 743,
			Codes: []string{
				diagnostic.CodeDoctorIPsecESPFormOffload,
				diagnostic.CodeDoctorIPsecESPFormOffloadUnknown,
			},
		},
		answer: answerESPFormOffload,
	})
}
