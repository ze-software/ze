package instance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// writeKernelTree lays out a runtime kernel cache entry for arch: a vmlinuz
// carrying the arch's header magic, a modules tree with a dangling build link
// (as modules_install leaves it), a device tree and an overlay.
func writeKernelTree(t *testing.T, arch string) string {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "7.2.9-runtime-"+arch)
	release := filepath.Join(tree, "lib", "modules", "7.2.9-ze")
	for _, dir := range []string{release, filepath.Join(tree, overlaysName)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	header := kernelHeaders[arch]
	image := make([]byte, 0x400)
	copy(image[header.offset:], header.magic)
	write := func(path string, data []byte) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(tree, vmlinuzName), image)
	write(filepath.Join(release, "modules.builtin"), []byte("kernel/net/mpls/mpls_router.ko\n"))
	write(filepath.Join(tree, "board.dtb"), []byte("dtb"))
	write(filepath.Join(tree, overlaysName, "overlay_map.dtb"), []byte("map"))
	if err := os.Symlink("/nonexistent/kernel/source", filepath.Join(release, "build")); err != nil {
		t.Fatal(err)
	}
	return tree
}

// testOptions answers Options for a host-arch image whose kernel tree is a
// fixture, with extraArgs appended to the cmdline.
func testOptions(t *testing.T, extraArgs ...string) Options {
	t.Helper()
	return Options{ExtraKernelArgs: extraArgs, KernelTree: writeKernelTree(t, runtime.GOARCH), Arch: runtime.GOARCH}
}

// TestAssembleKernelPackage verifies the assembled package is what gokrazy reads
// a kernel from: a module named KernelModule with one Go file, plus the tree's
// vmlinuz, modules (symlinks kept), device trees and overlays.
//
// VALIDATES: the one assembler, with no upstream kernel module copied in.
// PREVENTS: a package gok cannot `go list`, or one missing the modules tree.
func TestAssembleKernelPackage(t *testing.T) {
	tree := writeKernelTree(t, "arm64")
	pkg := filepath.Join(t.TempDir(), "kernel")

	if err := assembleKernelPackage(tree, pkg, "arm64"); err != nil {
		t.Fatalf("assembleKernelPackage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(pkg, GoModName))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := modfile.Parse(GoModName, data, nil)
	if err != nil {
		t.Fatalf("assembled go.mod does not parse: %v", err)
	}
	if mod.Module.Mod.Path != KernelModule {
		t.Errorf("module = %q, want %q", mod.Module.Mod.Path, KernelModule)
	}
	for _, rel := range []string{"kernel.go", vmlinuzName, "board.dtb", "overlays/overlay_map.dtb", "lib/modules/7.2.9-ze/modules.builtin"} {
		if _, err := os.Stat(filepath.Join(pkg, rel)); err != nil {
			t.Errorf("assembled package lacks %s: %v", rel, err)
		}
	}
	link, err := os.Readlink(filepath.Join(pkg, "lib", "modules", "7.2.9-ze", "build"))
	if err != nil || link != "/nonexistent/kernel/source" {
		t.Errorf("modules build link = %q, %v; want it kept as a symlink", link, err)
	}
}

// TestAssembleKernelPackageWritesCmdline verifies the package carries the two
// boot files gokrazy's packer reads: the kernel command line and config.txt.
//
// VALIDATES: writeBoot finds <kernel package>/cmdline.txt, and its root= is the
// spelling the packer rewrites to the image's PARTUUID.
// PREVENTS: `ze appliance build` failing at "Creating boot file system" with
// "open .../kernel/cmdline.txt: no such file or directory", which is what the
// first arm64 build of ze's own kernel answered on 2026-10-10, and then the
// same for config.txt.
func TestAssembleKernelPackageWritesCmdline(t *testing.T) {
	tree := writeKernelTree(t, "arm64")
	pkg := filepath.Join(t.TempDir(), "kernel")

	if err := assembleKernelPackage(tree, pkg, "arm64"); err != nil {
		t.Fatalf("assembleKernelPackage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(pkg, "cmdline.txt"))
	if err != nil {
		t.Fatalf("assembled package lacks cmdline.txt: %v", err)
	}
	const want = "root=/dev/sda2 ro init=/gokrazy/init panic=10 oops=panic"
	if got := strings.TrimSpace(string(data)); got != want {
		t.Errorf("cmdline.txt = %q, want %q", got, want)
	}

	// writeConfig reads config.txt too; it is the Raspberry Pi bootloader's file,
	// which no ze target reads, so it is empty as rtr7's was.
	config, err := os.ReadFile(filepath.Join(pkg, "config.txt"))
	if err != nil {
		t.Fatalf("assembled package lacks config.txt: %v", err)
	}
	if len(config) != 0 {
		t.Errorf("config.txt = %q, want it empty", config)
	}
}

// TestAssembleKernelPackageRefusesWrongArch verifies an amd64 vmlinuz is refused
// for an arm64 image before any file is written.
//
// VALIDATES: R-4, the arch keyed end to end.
// PREVENTS: gok failing late on a kernel from the wrong cache entry.
func TestAssembleKernelPackageRefusesWrongArch(t *testing.T) {
	tree := writeKernelTree(t, "amd64")
	pkg := filepath.Join(t.TempDir(), "kernel")

	err := assembleKernelPackage(tree, pkg, "arm64")
	if err == nil {
		t.Fatal("an amd64 kernel was accepted for an arm64 image")
	}
	if !strings.Contains(err.Error(), "not an arm64 kernel") {
		t.Errorf("error does not say which arch was wanted: %v", err)
	}
	if _, statErr := os.Stat(pkg); !os.IsNotExist(statErr) {
		t.Errorf("a refused assembly left %s behind", pkg)
	}
}

// TestPrepareReplacesZeKernelModule verifies the kernel package is assembled
// inside the PREPARED parent and the prepared builddir module's require of
// KernelModule is replaced to it, as an absolute path, while the tracked module
// stays byte-identical.
//
// VALIDATES: the image kernel comes from the resolved tree, per build.
// PREVENTS: a build writing a tracked go.mod, or two builds sharing one package.
func TestPrepareReplacesZeKernelModule(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	trackedKernel := filepath.Join(srcParent, "ze", "builddir", filepath.FromSlash(KernelModule), GoModName)

	prepared, cleanup, err := Prepare(srcParent, testOptions(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	preparedKernel := filepath.Join(prepared, "ze", "builddir", filepath.FromSlash(KernelModule), GoModName)
	data, err := os.ReadFile(preparedKernel)
	if err != nil {
		t.Fatalf("read prepared kernel go.mod: %v", err)
	}
	mod, err := modfile.Parse(preparedKernel, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, r := range mod.Replace {
		if r.Old.Path == KernelModule {
			target = r.New.Path
		}
	}
	if want := filepath.Join(prepared, kernelPackageDir); target != want {
		t.Errorf("kernel replace = %q, want %q (inside the prepared parent)", target, want)
	}
	if _, err := os.Stat(filepath.Join(target, vmlinuzName)); err != nil {
		t.Errorf("replace target holds no vmlinuz: %v", err)
	}

	tracked, err := os.ReadFile(trackedKernel)
	if err != nil {
		t.Fatal(err)
	}
	if string(tracked) != trackedKernelMod {
		t.Errorf("the tracked kernel module was modified by a build:\n%s", tracked)
	}
}

// TestPrepareRefusesWithoutKernelPackage verifies Prepare with no resolved
// kernel tree refuses, naming the kernel package, and creates no prepared dir.
//
// VALIDATES: AC-5.
// PREVENTS: gok resolving the kernel module on its own, from the network.
func TestPrepareRefusesWithoutKernelPackage(t *testing.T) {
	root, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))

	_, cleanup, err := Prepare(srcParent, Options{})
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("Prepare accepted an image with no kernel package")
	}
	if !strings.Contains(err.Error(), "no kernel package") || !strings.Contains(err.Error(), KernelModule) {
		t.Errorf("refusal does not name the kernel package: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(statErr) {
		t.Errorf("a refused Prepare created project tmp/: %v", statErr)
	}
}

// TestPrepareRefusesWithoutKernelModule verifies a builddir with no module
// requiring KernelModule fails the build rather than leaving the kernel to gok.
//
// VALIDATES: R-3, fail closed on a missing replace target.
func TestPrepareRefusesWithoutKernelModule(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	if err := os.RemoveAll(filepath.Join(srcParent, "ze", "builddir", "ze.invalid")); err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := Prepare(srcParent, testOptions(t))
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("Prepare built an instance whose builddir cannot carry the kernel replace")
	}
	if !strings.Contains(err.Error(), KernelModule) {
		t.Errorf("error does not name the kernel module: %v", err)
	}
}
