package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// kernelImageMagic is where a vmlinuz of each arch carries its header magic.
var kernelImageMagic = map[string]struct {
	offset int
	magic  string
}{
	archAMD64: {offset: 0x202, magic: "HdrS"},
	archARM64: {offset: 0x38, magic: "ARMd"},
}

// writeRuntimeKernelTree lays out a runtime kernel tree for arch in dir.
func writeRuntimeKernelTree(t *testing.T, dir, arch string) {
	t.Helper()
	release := filepath.Join(dir, "lib", "modules", defaultKernelVersion+"-ze")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	header := kernelImageMagic[arch]
	image := make([]byte, 0x400)
	copy(image[header.offset:], header.magic)
	if err := os.WriteFile(filepath.Join(dir, "vmlinuz"), image, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(release, "modules.builtin"), []byte("kernel/net/mpls/mpls_router.ko\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The provenance a build writes; Prepare derives the image's GPLv2 notice
	// from it and refuses a tree without one.
	digest, tracked := kernelbuilder.SourceDigest(defaultKernelVersion)
	if !tracked {
		t.Fatalf("kernel %s has no tracked digest", defaultKernelVersion)
	}
	provenance := "version=" + defaultKernelVersion + "\ntarget=runtime\nprofile=runtime\narch=" + arch + "\nmodules=yes\nbuilder=docker\n" +
		"source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-" + defaultKernelVersion + ".tar.xz\nsource-sha256=" + digest + "\n"
	if err := os.WriteFile(filepath.Join(dir, kernelbuilder.ProvenanceName), []byte(provenance), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setTestRuntimeKernelTree makes runtimeKernelTreeFn answer a fixture tree for
// the arch asked, and answers the list of arches it was asked for.
func setTestRuntimeKernelTree(t *testing.T) *[]string {
	t.Helper()
	asked := &[]string{}
	old := runtimeKernelTreeFn
	runtimeKernelTreeFn = func(arch string) (string, error) {
		*asked = append(*asked, arch)
		tree := filepath.Join(t.TempDir(), "runtime-"+arch)
		writeRuntimeKernelTree(t, tree, arch)
		return tree, nil
	}
	t.Cleanup(func() { runtimeKernelTreeFn = old })
	return asked
}

// TestResolveBuildParentDirUsesRuntimeKernel verifies `ze appliance build`
// resolves the runtime kernel for the image's arch and prepares an instance
// whose kernel package is that tree, replacing the builddir's kernel module.
//
// VALIDATES: AC-1 wiring, AC-11 arch keying.
// PREVENTS: an image prepared with no kernel, or with another arch's kernel.
func TestResolveBuildParentDirUsesRuntimeKernel(t *testing.T) {
	writeGokrazyFixture(t)
	asked := setTestRuntimeKernelTree(t)

	parent, cleanup, err := resolveBuildParentDir(&applianceConfig{Image: ImageConfig{Arch: archARM64}})
	if err != nil {
		t.Fatalf("resolveBuildParentDir: %v", err)
	}
	defer cleanup()

	if len(*asked) != 1 || (*asked)[0] != archARM64 {
		t.Fatalf("runtime kernel resolved for %v, want exactly [arm64]", *asked)
	}
	vmlinuz, err := os.ReadFile(filepath.Join(parent, "kernel", "vmlinuz"))
	if err != nil {
		t.Fatalf("prepared instance carries no kernel package: %v", err)
	}
	if got := string(vmlinuz[0x38:0x3c]); got != "ARMd" {
		t.Errorf("kernel package vmlinuz is not the arm64 tree (magic %q)", got)
	}
	mod, err := os.ReadFile(filepath.Join(parent, "ze", "builddir", filepath.FromSlash(instance.KernelModule), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "replace "+instance.KernelModule+" => "+filepath.Join(parent, "kernel")) {
		t.Errorf("prepared kernel module does not point at the assembled package:\n%s", mod)
	}
}

// TestResolveBuildParentDirRefusesUnresolvedKernel verifies a kernel that cannot
// be resolved fails the build before an instance is prepared.
//
// VALIDATES: AC-5 at the build entry point.
func TestResolveBuildParentDirRefusesUnresolvedKernel(t *testing.T) {
	root := writeGokrazyFixture(t)
	old := runtimeKernelTreeFn
	runtimeKernelTreeFn = func(string) (string, error) { return "", errors.New("no builder") }
	t.Cleanup(func() { runtimeKernelTreeFn = old })

	_, cleanup, err := resolveBuildParentDir(&applianceConfig{Image: ImageConfig{Arch: archAMD64}})
	cleanup()
	if err == nil || !strings.Contains(err.Error(), "resolve the amd64 runtime kernel") {
		t.Fatalf("err = %v, want the runtime kernel resolution failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(statErr) {
		t.Errorf("an instance was prepared though the kernel was not resolved")
	}
}

// TestRuntimeKernelTreeServesCacheWithoutBuilding verifies a cached runtime
// kernel is answered from its cache entry and no builder is started.
//
// VALIDATES: AC-7.
// PREVENTS: a warm cache paying for a container or VM per image build.
func TestRuntimeKernelTreeServesCacheWithoutBuilding(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestKernelBuild(t, func(kernelBuildSpec) error {
		t.Fatal("a cached runtime kernel started a build")
		return nil
	})
	cached, err := kernelCachePathFor(defaultKernelVersion, archARM64, runtimeKernelProfile, kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}
	writeRuntimeKernelTree(t, cached, archARM64)

	tree, err := RuntimeKernelTree(archARM64)
	if err != nil {
		t.Fatalf("RuntimeKernelTree: %v", err)
	}
	if tree != cached {
		t.Errorf("tree = %s, want the cache entry %s", tree, cached)
	}
}

// TestColdOfflineCacheNamesRemedy verifies a cold cache whose build fails (no
// network for the source tarball) names the cache path and the command that
// fills it.
//
// VALIDATES: AC-10.
func TestColdOfflineCacheNamesRemedy(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestKernelBuild(t, func(kernelBuildSpec) error {
		return errors.New("download linux-7.2.9.tar.xz: dial tcp: lookup cdn.kernel.org: no such host")
	})
	cached, err := kernelCachePathFor(defaultKernelVersion, runtime.GOARCH, runtimeKernelProfile, kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RuntimeKernelTree(runtime.GOARCH)
	if err == nil {
		t.Fatal("a cold cache with no network resolved a kernel")
	}
	for _, want := range []string{cached, "on an " + runtime.GOARCH + " host", "ze appliance kernel --target runtime --arch " + runtime.GOARCH, "no such host"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q:\n%v", want, err)
		}
	}
}
