package appliance

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// This file exposes the `ze appliance kernel` entry to the external test
// package appliance_test. That package exists because the cancellation proof
// issues its own docker commands through interoplab.Docker, and interoplab
// imports this package, so an in-package test importing it is an import cycle.

// ExitErrorForTest is the exit status `ze appliance kernel` answers a failed
// or stopped build with.
const ExitErrorForTest = exitError

// RunKernelForTest runs `ze appliance kernel` with args, as cmdKernel does.
func RunKernelForTest(args []string) int { return runKernel(args) }

// WriteInstallerKernelRegistryForTest lays down the minimal installer kernel
// registry under the current directory (writeInstallerKernelRegistry).
func WriteInstallerKernelRegistryForTest(t *testing.T) {
	t.Helper()
	writeInstallerKernelRegistry(t)
}

// UseNativeKernelBuildForTest keeps the real build route (defaultKernelBuild,
// which owns the interrupt handling and the build limit) and hands the native
// request to fn instead of kernelbuilder.Build, until the test ends.
func UseNativeKernelBuildForTest(t *testing.T, fn func(context.Context, kernelbuilder.Request) error) {
	t.Helper()
	setTestKernelBuild(t, defaultKernelBuild)
	old := nativeKernelBuildFn
	nativeKernelBuildFn = fn
	t.Cleanup(func() { nativeKernelBuildFn = old })
}
