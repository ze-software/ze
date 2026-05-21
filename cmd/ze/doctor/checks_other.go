// Design: docs/features/ai-first.md — non-Linux readiness check stubs

//go:build !linux

package doctor

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func checkVPPSocket() []diagnostic.Diagnostic {
	return nil
}

func checkKernelModules(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}

func checkInterfaces(_ *config.Tree) []diagnostic.Diagnostic {
	return nil
}
