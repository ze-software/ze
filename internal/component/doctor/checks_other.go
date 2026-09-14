// Design: docs/features/ai-first.md — non-Linux readiness check stubs
//
// The stubs here answer for the checks this component registers itself
// (doctor_checks.go). Every other Linux probe moved to the package that owns
// the dependency it reads, and each owner carries its own platform split
// (checks_linux.go names them).

//go:build !linux

package doctor

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func checkKernelModules(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkMachineID(_ *host.PlatformInfo, _ storage.Storage) []diagnostic.Diagnostic {
	return nil
}

func checkRandomSeed(_ *host.PlatformInfo) []diagnostic.Diagnostic {
	return nil
}
