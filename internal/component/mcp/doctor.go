// Design: docs/architecture/mcp/overview.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: streamable.go -- the listener that serves the TLS pair this check reads
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check over environment/mcp/tls belongs to the
// component that serves that pair (ai/patterns/registration.md, "Doctor Check
// Registry"), so it is owned here now and dropping this component drops its
// check with it.

package mcp

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// doctorServiceName opens every message the check prints, so the operator
// knows which listener the certificate belongs to.
const doctorServiceName = "mcp"

// mcpTLSDoctorCheck describes the check. register.go registers it.
//
// Order 160 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran before the runner's own phase
// dispatch, after the kernel-modules check at 150
// (internal/component/doctor/doctor_checks.go) and before the gRPC pair at 165
// (internal/component/api/grpc/doctor.go), which the same runner call used to
// report right after it.
var mcpTLSDoctorCheck = diagnostic.DoctorCheck{
	Name:         "mcp-tls-material",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        160,
	Component:    "mcp",
	Dependencies: []string{"filesystem"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		diagnostic.CodeDoctorTLSMissing,
		diagnostic.CodeDoctorTLSExpired,
		diagnostic.CodeDoctorTLSInvalid,
	},
	Check: checkMCPTLSMaterial,
}

// checkMCPTLSMaterial reports on the certificate and key an enabled mcp block
// names: a file that is not there, material that does not parse, and a
// validity window that has closed or is about to.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkMCPTLSMaterial(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg, ok := config.ExtractMCPConfig(tree)
	if !ok {
		return nil
	}
	return diagnostic.DoctorCertPair(doctorServiceName, cfg.TLS.Cert, cfg.TLS.Key, ctx.ConfigDir)
}
