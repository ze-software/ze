// Design: docs/features/ai-first.md — non-Linux readiness check stubs

//go:build !linux

package doctor

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
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

func checkNTPClockPrivilege(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkMachineID(_ *host.PlatformInfo, _ storage.Storage) []diagnostic.Diagnostic {
	return nil
}

func checkVPPDPDK(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkRandomSeed(_ *host.PlatformInfo) []diagnostic.Diagnostic {
	return nil
}

func checkSmartEnabled(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}
