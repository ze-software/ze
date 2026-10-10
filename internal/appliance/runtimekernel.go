// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel
// target resolves from the cache, else a native build; the appliance image boots
// that kernel and no other.
// Related: cmd_kernel.go -- resolveRuntimeKernel, the one resolver.
// Related: instance/kernelpkg.go -- the assembler that turns the tree into a gokrazy kernel package.

package appliance

// RuntimeKernelTree resolves ze's runtime kernel for arch and answers its
// arch-keyed cache directory: the cache entry when one is present, else a native
// build (Docker or QEMU, on a host of that arch) whose output is checked against
// the kernel requirements and then cached. The answer is the cache entry, never
// tmp/kernel/build, so two concurrent image builds never read a tree a third
// resolver call is rewriting.
func RuntimeKernelTree(arch string) (string, error) {
	return resolveKernel(defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime)
}

// runtimeKernelTreeFn is RuntimeKernelTree, as a package var so unit tests of
// the image build resolve a fixture tree instead of building a kernel.
var runtimeKernelTreeFn = RuntimeKernelTree
