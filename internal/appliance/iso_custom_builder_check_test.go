// Design: docs/architecture/appliance/iso-installer.md -- custom builder readiness.

package appliance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestISOCheckCustomBuilderRequiresExecutable checks the public prerequisite
// command with readable target modules beside each private builder candidate.
func TestISOCheckCustomBuilderRequiresExecutable(t *testing.T) {
	for _, arch := range []string{archAMD64, archARM64} {
		for _, test := range []struct {
			name      string
			mode      os.FileMode
			directory bool
			ready     bool
		}{
			{name: "missing"},
			{name: "nonexecutable", mode: 0o600},
			{name: "directory", mode: 0o700, directory: true},
			{name: "executable", mode: 0o700, ready: true},
		} {
			t.Run(arch+"/"+test.name, func(t *testing.T) {
				oldBase, oldLookup, oldModules := baseDir, isoLookPathFn, isoModuleCheckFn
				t.Cleanup(func() { baseDir, isoLookPathFn, isoModuleCheckFn = oldBase, oldLookup, oldModules })
				baseDir = filepath.Join(t.TempDir(), "appliance directory")
				appDir := filepath.Join(baseDir, isoTestApplianceName)
				if err := os.MkdirAll(appDir, 0o700); err != nil {
					t.Fatal(err)
				}
				rewriteIsoTestConfig(t, appDir, func(config *applianceConfig) { config.Image.Arch = arch })
				kernel := filepath.Join(appDir, "Image")
				initrd := filepath.Join(appDir, "initrd.img.gz")
				writeIsoTestKernel(t, kernel, arch)
				writeIsoTestFile(t, initrd, "initrd")

				prefix := filepath.Join(t.TempDir(), "GRUB installation")
				builder := filepath.Join(prefix, "bin", "grub-mkstandalone")
				if err := os.MkdirAll(filepath.Dir(builder), 0o700); err != nil {
					t.Fatal(err)
				}
				if test.directory {
					if err := os.Mkdir(builder, test.mode); err != nil {
						t.Fatal(err)
					}
				} else if test.mode != 0 {
					if err := os.WriteFile(builder, []byte("fixture executable; never invoked"), test.mode); err != nil {
						t.Fatal(err)
					}
				}
				target, _, err := isoGRUBTarget(arch)
				if err != nil {
					t.Fatal(err)
				}
				modules := filepath.Join(prefix, "lib", "grub", target)
				if err := os.MkdirAll(modules, 0o700); err != nil {
					t.Fatal(err)
				}
				writeIsoTestFile(t, filepath.Join(modules, "modinfo.sh"), "fixture modules")
				if admitted, err := checkISOGRUBModules(builder, target); err != nil {
					t.Fatalf("matching module fixture is unusable: %v", err)
				} else if admitted != modules {
					t.Fatalf("matching module fixture selected %q, want %q", admitted, modules)
				}
				isoModuleCheckFn = checkISOGRUBModules
				isoLookPathFn = func(name string) (string, error) {
					if name == "xorriso" {
						return "/fixture/xorriso", nil
					}
					return "", exec.ErrNotFound
				}
				code := exitOK
				output := captureStderr(t, func() {
					oldOut := os.Stdout
					os.Stdout = os.Stderr
					defer func() { os.Stdout = oldOut }()
					code = runIso([]string{"--check", "--kernel", kernel, "--initrd", initrd, "--builder", builder, isoTestApplianceName})
				})
				if test.ready {
					if code != exitOK {
						t.Fatalf("executable custom builder rejected: code=%d\n%s", code, output)
					}
					want := "grub:     ready (" + builder + ", modules " + modules + ")"
					if !strings.Contains(output, want) {
						t.Errorf("readiness lost the selected executable or %s modules; want %q in:\n%s", target, want, output)
					}
					return
				}
				if code != exitError {
					t.Errorf("unusable custom builder accepted despite readable matching modules: code=%d\n%s", code, output)
				}
				if strings.Contains(output, "grub:     ready") {
					t.Errorf("unusable custom builder reported ready:\n%s", output)
				}
				if !strings.Contains(output, builder) {
					t.Errorf("custom builder refusal lost its path %q:\n%s", builder, output)
				}
				selection := "./ze-host appliance --dir " + shellArg(baseDir) + " iso --check --builder "
				override := strings.TrimPrefix(isoGRUBOverride, "./ze-host appliance iso --check --builder ")
				if want := "fix: " + selection + override + " " + isoTestApplianceName + "\n"; !strings.Contains(output, want) {
					t.Errorf("custom builder repair lost named appliance or directory; want %q in:\n%s", want, output)
				}
			})
		}
	}
}
