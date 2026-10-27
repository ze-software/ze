// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel
// target resolves from the cache, else a native build; the appliance image boots
// that kernel and no other.
// Related: cmd_kernel.go -- resolveRuntimeKernel, the one resolver.
// Related: instance/kernelpkg.go -- the assembler that turns the tree into a gokrazy kernel package.

package appliance

import "errors"

// RuntimeKernelTree resolves ze's runtime kernel for arch and answers its
// arch-keyed cache directory: the cache entry when one is present, else a native
// build (Docker or QEMU, on a host of that arch) whose output is checked against
// the kernel requirements and then cached. The answer is the cache entry, never
// tmp/kernel/build, so two concurrent image builds never read a tree a third
// resolver call is rewriting.
func RuntimeKernelTree(arch string) (string, error) {
	return resolveKernel(defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime, "")
}

// ImageRuntimeKernelTree is RuntimeKernelTree for a caller that builds an
// image of arch. Its refusal for a kernel this host cannot build names
// imageBuild(arch) on an arch host, or imageBuild(host) here, never a bare
// kernel build: a kernel built on another host fills that host's cache, so the
// image build here would stay refused. imageBuild MUST answer the exact
// command that builds the image for the arch it is given.
func ImageRuntimeKernelTree(arch string, imageBuild func(arch string) string) (string, error) {
	tree, err := RuntimeKernelTree(arch)
	if foreign, ok := errors.AsType[*foreignArchKernelError](err); ok {
		return "", foreign.forImage(imageBuild)
	}
	return tree, err
}

// GokrazyRuntimeKernelReachable answers the refusal `./le build gokrazy` with
// args meets at the runtime kernel for arch, before anything long starts: nil
// when this host can build arch or the cache holds its kernel, else a refusal
// whose fix is a `./le build gokrazy` command, not a kernel build for another
// host's cache.
func GokrazyRuntimeKernelReachable(arch string, args []string) error {
	err := runtimeKernelReachable(arch)
	if foreign, ok := errors.AsType[*foreignArchKernelError](err); ok {
		return foreign.forGokrazy(args)
	}
	return err
}

// runtimeKernelTreeFn is RuntimeKernelTree, as a package var so unit tests of
// the image build resolve a fixture tree instead of building a kernel.
var runtimeKernelTreeFn = RuntimeKernelTree
