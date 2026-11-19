// Design: docs/architecture/appliance/build-artifacts.md -- a test boots the cached runtime kernel and never builds one
// Related: runtimekernel.go -- RuntimeKernelTree, the resolver an explicit build runs
// Related: runtimekernel_publish.go -- the locked, atomic publication a cold build ends in

package appliance

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CachedRuntimeKernelTree answers the runtime kernel cache entry for arch that
// RuntimeKernelTree would serve, under the same key, and never builds one. A
// test, a record run or a lab calls it: a cold build takes about thirty minutes,
// and inside a test's deadline a kill can land before the build is published,
// so the same key is then built again. On a miss it refuses with the entry's
// path and kernelBuild(arch), which MUST answer the one command that builds the
// kernel for arch, run on its own.
//
// An entry is published by one rename after it is complete
// (publishRuntimeKernel), so an entry whose vmlinuz exists is whole.
func CachedRuntimeKernelTree(root, arch string, kernelBuild func(arch string) string) (string, error) {
	absolute, err := kernelSourceRoot(root)
	if err != nil {
		return "", err
	}
	if err := refuseAnotherRootsRelease(absolute); err != nil {
		return "", err
	}
	if !kernelbuilder.ArchSupported(arch) {
		return "", errors.New(unsupportedArch("the runtime kernel arch", arch,
			"name this host's arch, "+kernelbuilder.HostArch()+", instead"))
	}
	if err := enforceReserveMemKernelFloor(defaultKernelVersion); err != nil {
		return "", err
	}
	cachedDir, err := kernelCachePathFor(absolute, defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime)
	if err != nil {
		return "", err
	}
	vmlinuz := filepath.Join(cachedDir, runtimeKernelArtifact)
	_, err = os.Stat(vmlinuz)
	if err == nil {
		return cachedDir, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", opererr.Wrap(err, "cannot read the runtime kernel "+vmlinuz,
			"a test boots only a cached kernel it can read, and this one is there but not readable by this user",
			"ls -l "+shellArg(vmlinuz))
	}
	return "", coldRuntimeKernel(arch, cachedDir, kernelBuild(arch))
}

// coldRuntimeKernel answers the refusal for a runtime kernel the cache does not
// hold at cachedDir, ending with build, the command that builds it.
func coldRuntimeKernel(arch, cachedDir, build string) error {
	var tb textbuf.Buffer
	problem := tb.Str("runtime kernel ").Str(defaultKernelVersion).Str(" for ").Str(arch).
		Str(" is not in the cache at ").Str(cachedDir).String()
	why := "a test, a record run or a lab boots the cached kernel and never builds one: a build takes about thirty minutes " +
		"and runs on its own, outside any test deadline. Build it once with the command below, then run this command again"
	if host := kernelbuilder.HostArch(); arch != host {
		why = "a test, a record run or a lab boots the cached kernel and never builds one, and this " + host +
			" host cannot build an " + arch + " kernel: a kernel built on another host fills that host's cache. " +
			"Run this command on an " + arch + " host, after you build the kernel there once with the command below"
	}
	return opererr.New(problem, why, build)
}
