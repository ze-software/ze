// Design: docs/architecture/appliance/build-artifacts.md -- a test boots the cached runtime kernel and never builds one
// Related: runtimekernel.go -- RuntimeKernelTree, the resolver an explicit build runs
// Related: runtimekernel_publish.go -- the locked, atomic publication a cold build ends in
// Related: cmd_kernel.go -- resolveRuntimeKernel, which tries the same entries before it builds

package appliance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CachedRuntimeKernelTree answers the runtime kernel cache entry for arch that
// RuntimeKernelTree would serve, under the same keys, and never builds one. A
// test, a record run or a lab calls it: a cold build takes about thirty minutes,
// and inside a test's deadline a kill can land before the build is published,
// so the same key is then built again. It tries the entry of the backend a
// build here would choose, then the other backend's entry of the same inputs
// (runtimeKernelBackends). When neither holds a kernel it refuses with the
// preferred entry's path, the other's, and kernelBuild(arch), which MUST answer
// the one command that builds the kernel for arch, run on its own.
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
	entries, err := defaultRuntimeKernelEntries(absolute, arch)
	if err != nil {
		return "", err
	}
	entry, found, err := publishedRuntimeKernel(entries, runtimeKernelArtifact)
	if err != nil {
		return "", err
	}
	if found {
		return entry, nil
	}
	return "", coldRuntimeKernel(arch, entries, kernelBuild(arch))
}

// defaultRuntimeKernelEntries answers runtimeKernelEntries for the default
// runtime kernel of arch with no backend requested: the lookup the image
// build, `./le build gokrazy`, the doctor check and every test make.
func defaultRuntimeKernelEntries(root, arch string) ([]string, error) {
	td, err := kernelTargetFor(kernelTargetRuntime)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveKernelProfile(root, td.configDir, runtimeKernelProfile)
	if err != nil {
		return nil, err
	}
	return runtimeKernelEntries(root, defaultKernelVersion, arch, "", resolved)
}

// runtimeKernelBackends answers the backends whose cache entry may serve a
// runtime kernel, in the order they are tried. A requested backend names only
// itself: `--builder` asks for that backend's kernel. With none requested, the
// backend a build here would choose (kernelbuilder.PreferredBackend) comes
// first, then the other. Both backends compile the same inputs, the ones the
// entry's two hashes key, so the other backend's entry is a build of those
// inputs, and building the preferred one beside it would build them twice
// (ai/META-DECISIONS.md, "Test systems and labs"). The order keeps today's
// choice whenever both entries exist.
func runtimeKernelBackends(requested string) []string {
	if requested != "" {
		return []string{requested}
	}
	if kernelbuilder.PreferredBackend("") == builderDocker {
		return []string{builderDocker, builderQEMU}
	}
	return []string{builderQEMU, builderDocker}
}

// runtimeKernelEntries answers the cache directories of the runtime kernel of
// version, arch and resolved, one per backend runtimeKernelBackends(builder)
// names, in its order. The first is the entry a build here publishes.
func runtimeKernelEntries(root, version, arch, builder string, resolved kernelProfileResolution) ([]string, error) {
	backends := runtimeKernelBackends(builder)
	entries := make([]string, 0, len(backends))
	for _, backend := range backends {
		variant, err := kernelCacheVariantFor(root, kernelTargetRuntime, arch, backend, resolved)
		if err != nil {
			return nil, err
		}
		entry, err := filepath.Abs(kernelTreeCachePath(version, variant))
		if err != nil {
			return nil, fmt.Errorf("resolve runtime kernel cache entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// publishedRuntimeKernel answers the first of entries whose artifact exists,
// and found false when none holds it. An entry is published by one rename after
// it is complete (publishRuntimeKernel), so an entry whose artifact exists is
// whole. An artifact that exists but cannot be read is refused rather than
// passed over: it is a built kernel, and trying the next entry would hide it.
func publishedRuntimeKernel(entries []string, artifact string) (entry string, found bool, err error) {
	for _, entry := range entries {
		kernel := filepath.Join(entry, artifact)
		_, err := os.Stat(kernel)
		if err == nil {
			return entry, true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", false, opererr.Wrap(err, "cannot read the runtime kernel "+kernel,
				"a run boots only a cached kernel it can read, and this one is there but not readable by this user",
				"ls -l "+shellArg(kernel))
		}
	}
	return "", false, nil
}

// RuntimeKernelBuilder answers the backend that built the runtime kernel cache
// entry at tree, from the provenance record its build wrote last
// (kernelbuilder.ProvenanceName). A run report names it, because a default
// resolution may boot either backend's kernel (runtimeKernelBackends).
func RuntimeKernelBuilder(tree string) (string, error) {
	fields, err := existingRuntimeProvenance(filepath.Join(tree, kernelbuilder.ProvenanceName))
	if err != nil {
		return "", fmt.Errorf("read which backend built the runtime kernel %s: %w", tree, err)
	}
	builder := fields["builder"]
	switch builder {
	case builderDocker, builderQEMU:
		return builder, nil
	default:
		return "", fmt.Errorf("the runtime kernel %s records builder %q, which is neither %s nor %s. Fix: %s",
			tree, builder, builderDocker, builderQEMU, "rebuild it with ze appliance kernel --target runtime")
	}
}

// coldRuntimeKernel answers the refusal for a runtime kernel the cache holds
// at none of entries, ending with build, the command that builds it. The first
// entry is the one that command publishes.
func coldRuntimeKernel(arch string, entries []string, build string) error {
	var tb textbuf.Buffer
	tb.Str("runtime kernel ").Str(defaultKernelVersion).Str(" for ").Str(arch).
		Str(" is not in the cache at ").Str(entries[0])
	for _, other := range entries[1:] {
		tb.Str(", nor at ").Str(other).Str(", the same inputs built by the other backend")
	}
	problem := tb.String()
	why := "a test, a record run or a lab boots the cached kernel and never builds one: a build takes about thirty minutes " +
		"and runs on its own, outside any test deadline. Build it once with the command below, then run this command again"
	if host := kernelbuilder.HostArch(); arch != host {
		why = "a test, a record run or a lab boots the cached kernel and never builds one, and this " + host +
			" host cannot build an " + arch + " kernel: a kernel built on another host fills that host's cache. " +
			"Run this command on an " + arch + " host, after you build the kernel there once with the command below"
	}
	return opererr.New(problem, why, build)
}
