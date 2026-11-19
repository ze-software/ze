// Design: docs/architecture/testing/qemu-integration.md -- an appliance image proof boots the cached runtime kernel
// Related: install_build.go -- Installer.buildImage, an image build this guards
// Related: install_iso.go -- Installer.prepareISOImageInputs, an image build this guards
// Related: hugepages.go -- Hugepages.buildImage, an image build this guards
// Related: crashcapture.go -- CrashCapture.buildImage, an image build this guards
// Related: mplsboot.go -- buildSeededImage, the MPLS boot and hostname image build this guards
package testqemu

import (
	"github.com/ze-software/ze/internal/appliance"
	buildhostdriver "github.com/ze-software/ze/internal/le/build/hostdriver"
)

// cachedRuntimeKernel is the lookup requireCachedRuntimeKernel makes. It is a
// variable only so a unit test whose fake runner never spawns a real build can
// answer it; nothing else replaces it.
var cachedRuntimeKernel = cachedRuntimeKernelTree

// cachedRuntimeKernelTree answers the runtime kernel cache entry for arch that
// `ze appliance build`, run from tree, embeds in its image, and never builds
// it.
func cachedRuntimeKernelTree(tree, arch string) (string, error) {
	return appliance.CachedRuntimeKernelTree(tree, arch, runtimeKernelBuildCommand)
}

// runtimeKernelBuildCommand answers the commands that build the runtime kernel
// for arch from this checkout, the remedy every le refusal of a missing runtime
// kernel prints (buildhostdriver.RuntimeKernelBuild).
func runtimeKernelBuildCommand(arch string) string {
	return buildhostdriver.RuntimeKernelBuild(arch, "")
}

// requireCachedRuntimeKernel refuses an appliance image build whose runtime
// kernel the cache does not hold, before the build is spawned. The child
// `ze appliance build` resolves its kernel through appliance.RuntimeKernelTree,
// which builds a missing one for about thirty minutes inside this proof's
// deadline; a test boots the cached kernel and never builds one, so the
// refusal names the entry's path and the command that fills it, which runs on
// its own. The image itself is still built here: it is this run's artifact,
// carrying this run's seed, credentials and boot settings, so no cache key
// could serve it to another run.
func requireCachedRuntimeKernel(tree, arch string) error {
	_, err := cachedRuntimeKernel(tree, arch)
	return err
}
