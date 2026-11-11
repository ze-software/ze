// Design: docs/architecture/appliance/iso-installer.md -- checked modules reach GRUB.

package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestISOBuildUsesTheAdmittedModuleDirectory(t *testing.T) {
	appDir := setupIsoTestAppliance(t)
	image := filepath.Join(appDir, "ze-module-proof.img")
	writeIsoTestFile(t, image, "appliance image")
	writeIsoTestChecksum(t, image)
	kernel, initrd := filepath.Join(t.TempDir(), "Image"), filepath.Join(t.TempDir(), "initrd")
	writeIsoTestKernel(t, kernel, archAMD64)
	writeIsoTestFile(t, initrd, "initrd")

	linkPrefix, realPrefix := t.TempDir(), t.TempDir()
	moduleDir := filepath.Join(realPrefix, "lib", "grub", isoGRUBTargetAMD64)
	for _, dir := range []string{filepath.Join(linkPrefix, "bin"), filepath.Join(realPrefix, "bin"), moduleDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeIsoTestFile(t, filepath.Join(moduleDir, "modinfo.sh"), "fixture modules")
	logPath := filepath.Join(realPrefix, "selected-modules")
	builder := filepath.Join(realPrefix, "bin", "grub-mkstandalone")
	grub := "#!/bin/sh\nset -eu\ndirectory=\nout=\nwhile [ $# -gt 0 ]; do\n" +
		"case \"$1\" in\n--directory) directory=$2; shift 2;;\n--directory=*) directory=${1#*=}; shift;;\n-o) out=$2; shift 2;;\n*) shift;;\nesac\ndone\n" +
		"test \"$directory\" = " + shellArg(moduleDir) + " || { echo 'checked module directory was not passed to GRUB' >&2; exit 53; }\n" +
		"test -f \"$directory/modinfo.sh\"\nprintf '%s' \"$directory\" > " + shellArg(logPath) + "\nprintf efi > \"$out\"\n"
	if err := os.WriteFile(builder, []byte(grub), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkPrefix, "bin", "grub-mkstandalone")
	if err := os.Symlink(builder, link); err != nil {
		t.Fatal(err)
	}
	xorriso := filepath.Join(realPrefix, "bin", "xorriso")
	isoScript := "#!/bin/sh\nset -eu\nout=\nwhile [ $# -gt 0 ]; do\ncase \"$1\" in\n-o) out=$2; shift 2;;\n*) shift;;\nesac\ndone\nprintf iso > \"$out\"\n"
	if err := os.WriteFile(xorriso, []byte(isoScript), 0o700); err != nil {
		t.Fatal(err)
	}
	oldLook, oldBuilder, oldModules := isoLookPathFn, isoBuilderFn, isoModuleCheckFn
	isoLookPathFn = func(name string) (string, error) {
		if name == "xorriso" {
			return xorriso, nil
		}
		return link, nil
	}
	isoBuilderFn, isoModuleCheckFn = runISOBuilder, checkISOGRUBModules
	t.Cleanup(func() { isoLookPathFn, isoBuilderFn, isoModuleCheckFn = oldLook, oldBuilder, oldModules })
	out := filepath.Join(t.TempDir(), "ze.iso")
	if code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--image", filepath.Base(image), "--builder", link, "--output", out, isoTestApplianceName}); code != exitOK {
		t.Fatalf("ISO builder did not consume the checked target modules: exit %d", code)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, logPath))); got != moduleDir {
		t.Errorf("GRUB consumed module directory %q, want %q", got, moduleDir)
	}
	if got := string(mustReadFile(t, out)); got != "iso" {
		t.Errorf("ISO assembly did not complete: %q", got)
	}
}
