// Design: docs/architecture/appliance/iso-installer.md -- named fallback target selection.

package appliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestISOCompleteBuilderRepairRetainsNamedForeignTargetAndDirectory(t *testing.T) {
	for _, mode := range []string{"check", "build"} {
		for _, failure := range []string{"unknown-host", "unreadable-modules"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				oldBase := baseDir
				baseDir = filepath.Join(t.TempDir(), "appliance directory")
				t.Cleanup(func() { baseDir = oldBase })
				appDir := filepath.Join(baseDir, isoTestApplianceName)
				if err := os.MkdirAll(appDir, 0o700); err != nil {
					t.Fatal(err)
				}
				arch := archARM64
				if runtime.GOARCH == archARM64 {
					arch = archAMD64
				}
				rewriteIsoTestConfig(t, appDir, func(config *applianceConfig) { config.Image.Arch = arch })
				image := filepath.Join(appDir, "ze-selection.img")
				writeIsoTestFile(t, image, "image")
				writeIsoTestChecksum(t, image)
				kernel, initrd, _ := setupIsoBuilderTest(t)
				checked := ""
				isoModuleCheckFn = func(builder, target string) (string, error) {
					checked = target
					if failure == "unknown-host" {
						_, err := isoGRUBInstallCommand("unrecognized", target)
						return "", err
					}
					return "", isoGRUBModuleError(os.ErrPermission, builder, target, "/blocked/modinfo.sh")
				}
				args := []string{"--kernel", kernel, "--initrd", initrd}
				if mode == "check" {
					args = append(args, "--check")
				}
				args = append(args, isoTestApplianceName)
				code := exitOK
				output := captureStderr(t, func() {
					oldOut := os.Stdout
					os.Stdout = os.Stderr
					defer func() { os.Stdout = oldOut }()
					code = runIso(args)
				})
				if code != exitError {
					t.Fatalf("missing foreign-target modules returned %d", code)
				}
				target, _, err := isoGRUBTarget(arch)
				if err != nil {
					t.Fatal(err)
				}
				if checked != target {
					t.Fatalf("module check selected %q, want non-host target %q", checked, target)
				}
				selection := "./ze-host appliance --dir " + shellArg(baseDir) + " iso --check --builder "
				builder := strings.TrimPrefix(isoGRUBOverride, "./ze-host appliance iso --check --builder ")
				want := "fix: " + selection + builder + " " + isoTestApplianceName + "\n"
				if !strings.Contains(output, want) {
					t.Errorf("complete-builder repair lost the selected appliance or quoted directory; want %q in:\n%s", want, output)
				}
			})
		}
	}
}
