//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- namespace FWD default dependency.
// Related: doctor_forward_default_linux.go -- the read-only live host and claim check.

package engine

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	engineDoctorChecks = append(engineDoctorChecks, engineDoctorCheck{
		decl: rpc.DoctorCheckDecl{Name: "ipsec-forward-default", Phase: rpc.DoctorPhaseRuntime, Order: 744,
			Codes: []string{diagnostic.CodeDoctorIPsecForwardDefault}},
		answer: answerForwardDefault,
	})
}
