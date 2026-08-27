//go:build !linux

// Design: ai/rules/platform-linux.md -- Linux-only work keeps a refusal path
// Related: l2tpdiag_linux.go -- the implementation on Linux

package deployment

import "errors"

func executeL2TPPPoXDiagnostic(_ l2tpDiagnosticOptions) (L2TPDiagnosticReport, error) {
	return L2TPDiagnosticReport{Diagnostic: l2tpPPPoXDiagnosticName},
		errors.New("the L2TP PPPoL2TP diagnostic requires Linux")
}

func executeL2TPTunnelDiagnostic(_ l2tpDiagnosticOptions) (L2TPDiagnosticReport, error) {
	return L2TPDiagnosticReport{Diagnostic: l2tpTunnelDiagnosticName},
		errors.New("the L2TP tunnel diagnostic requires Linux")
}
