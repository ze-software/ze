// Design: docs/architecture/api/architecture.md -- the readiness check this transport owns
// Overview: register.go -- the init() that installs the registration below
// Related: server.go -- the listener that serves the TLS pair this check reads
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check over environment/api-server/grpc/tls-cert and
// tls-key belongs to the transport that serves that pair
// (ai/patterns/registration.md, "Doctor Check Registry"), so it is owned here
// now and dropping this transport drops its check with it.

package grpc

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// doctorServiceName opens every message the check prints, so the operator
// knows which listener the certificate belongs to.
const doctorServiceName = "api-grpc"

// grpcTLSDoctorCheck describes the check. register.go registers it.
//
// Order 165 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: one runner call reported the mcp pair and
// then this one, so it sorts right after mcp-tls-material at 160
// (internal/component/mcp/doctor.go) and before the web material at 170
// (internal/component/web/doctor.go).
var grpcTLSDoctorCheck = diagnostic.DoctorCheck{
	Name:         "api-grpc-tls-material",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        165,
	Component:    "api-grpc",
	Dependencies: []string{"filesystem"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		diagnostic.CodeDoctorTLSMissing,
		diagnostic.CodeDoctorTLSExpired,
		diagnostic.CodeDoctorTLSInvalid,
	},
	Check: checkGRPCTLSMaterial,
}

// checkGRPCTLSMaterial reports on the certificate and key an enabled
// api-server block names for gRPC: a file that is not there, material that
// does not parse, and a validity window that has closed or is about to.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkGRPCTLSMaterial(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg, ok := config.ExtractAPIConfig(tree)
	if !ok {
		return nil
	}
	return diagnostic.DoctorCertPair(doctorServiceName, cfg.GRPCTLSCert, cfg.GRPCTLSKey, ctx.ConfigDir)
}
