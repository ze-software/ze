// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel
// target resolves from the cache, else a native build; the appliance image boots
// that kernel and no other.
// Related: cmd_kernel.go -- resolveRuntimeKernel, the one resolver.
// Related: instance/kernelpkg.go -- the assembler that turns the tree into a gokrazy kernel package.

package appliance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RuntimeKernelTree resolves ze's runtime kernel for arch and answers its
// arch-keyed cache directory: the cache entry when one is present, else a native
// build (Docker or QEMU, on a host of that arch) whose output is checked against
// the kernel requirements and then cached. root names the selected source tree;
// the caller's working directory does not select its inputs. Library resolution
// never materializes the shared tmp/kernel/build stage.
func RuntimeKernelTree(root, arch string) (string, error) {
	absolute, err := kernelSourceRoot(root)
	if err != nil {
		return "", err
	}
	if err := refuseAnotherRootsRelease(absolute); err != nil {
		return "", err
	}
	return resolveKernel(absolute, defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime, "")
}

// kernelVersionRel is where a source root declares the Linux release it pins.
const kernelVersionRel = "internal/appliance/kernel.version"

// refuseAnotherRootsRelease refuses a root that pins another Linux release than
// this binary. The release is compiled in with its tracked source digest
// (kernelbuilder.SourceDigest), so the binary cannot build the root's release;
// going on would build its own release from that root's config, and cache the mix
// under a name that hides it.
func refuseAnotherRootsRelease(root string) error {
	path := filepath.Join(root, filepath.FromSlash(kernelVersionRel))
	raw, err := os.ReadFile(path) //nolint:gosec // a fixed file under the caller's selected source root
	if errors.Is(err, fs.ErrNotExist) {
		// refuse-by-default: guards RuntimeKernelTree -- a root with no version file pins no other release, and its profile files still decide
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the kernel release of source root %s: %w", root, err)
	}
	pinned := strings.TrimSpace(string(raw))
	if pinned == defaultKernelVersion {
		return nil
	}
	return fmt.Errorf("source root %s pins Linux %s, but this binary was built for Linux %s, so it would build %s from that root's config. "+
		"Fix: rebuild this tool from that root (commit the kernel.version change so ./le rebuilds, or ./le build host-driver run for ze-host), then rerun",
		root, pinned, defaultKernelVersion, defaultKernelVersion)
}

// ImageRuntimeKernelTree is RuntimeKernelTree for a caller that builds an
// image of arch. Its refusal for a kernel this host cannot build names
// imageBuild(arch) on an arch host, or imageBuild(host) here, never a bare
// kernel build: a kernel built on another host fills that host's cache, so the
// image build here would stay refused. imageBuild MUST answer the exact
// command that builds the image for the arch it is given.
func ImageRuntimeKernelTree(root, arch string, imageBuild func(arch string) string) (string, error) {
	tree, err := RuntimeKernelTree(root, arch)
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
func GokrazyRuntimeKernelReachable(root, arch string, args []string) error {
	absolute, err := kernelSourceRoot(root)
	if err != nil {
		return err
	}
	err = runtimeKernelReachable(absolute, arch)
	if foreign, ok := errors.AsType[*foreignArchKernelError](err); ok {
		return foreign.forGokrazy(args)
	}
	return err
}

// runtimeKernelTreeFn is RuntimeKernelTree, as a package var so unit tests of
// the image build resolve a fixture tree instead of building a kernel.
var runtimeKernelTreeFn = RuntimeKernelTree

// kernelSourceRoot normalizes an explicitly selected source tree without
// rediscovering another checkout or changing this process's working directory.
func kernelSourceRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("kernel source root is empty")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve kernel source root %s: %w", root, err)
	}
	return absolute, nil
}
