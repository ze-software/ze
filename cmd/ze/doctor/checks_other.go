// Design: docs/features/ai-first.md — non-Linux readiness check stubs

//go:build !linux

package doctor

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func checkVPPSocket(_ string) []diagnostic.Diagnostic {
	return nil
}

func checkKernelModules(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkInterfaces(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkVPPVersion(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkKernelNexthop() []diagnostic.Diagnostic {
	return nil
}

func checkMPLSSupport(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkFirewallBackend(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkTelemetryProcfs(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkSysctlProcfs(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkConntrackProcfs(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkPolicyRouteNetlink(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}
