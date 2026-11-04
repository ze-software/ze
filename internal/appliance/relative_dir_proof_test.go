//go:build linux || darwin

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestRunPinsRelativeApplianceDirectory follows the actual CLI resolver into a
// handler whose in-process gok phase changes cwd. The real config reader must
// still read the original appliance, not the decoy at the new working directory.
func TestRunPinsRelativeApplianceDirectory(t *testing.T) {
	for _, source := range []string{"flag", "environment", "XDG default"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("ZE_APPLIANCE_DIR", "")
			t.Setenv("XDG_CONFIG_HOME", "settings")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			directory := "appliances"
			args := []string{"build", "lab"}
			if source == "flag" {
				args = append([]string{"--dir", directory}, args...)
			}
			if source == "environment" {
				t.Setenv("ZE_APPLIANCE_DIR", directory)
				env.ResetCache()
			}
			if source == "XDG default" {
				directory = filepath.Join("settings", defaultSubdir)
			}
			instance := filepath.Join(root, "gok-instance")
			if err := os.Mkdir(instance, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, fixture := range []struct {
				base string
				name string
			}{
				{base: filepath.Join(root, directory), name: "original-appliance"},
				{base: filepath.Join(instance, directory), name: "cwd-decoy"},
			} {
				path := ConfigPath(fixture.base, "lab")
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				config := DefaultConfig(fixture.name)
				if err := saveConfig(path, &config); err != nil {
					t.Fatal(err)
				}
			}
			previousBuild := cmdBuild
			previousBase := baseDir
			defer func() { cmdBuild, baseDir = previousBuild, previousBase }()
			var readErr error
			cmdBuild = func(_ []string) int {
				readErr = keepWorkingDir(func() error {
					if err := os.Chdir(instance); err != nil {
						return err
					}
					config, err := LoadConfig(ConfigPath(getBaseDir(), "lab"))
					if err != nil {
						return err
					}
					if config.Identity.Name != "original-appliance" {
						return errors.New("appliance directory followed gok cwd into the decoy")
					}
					return nil
				})
				if readErr != nil {
					return exitError
				}
				return exitOK
			}
			if code := Run(args); code != exitOK {
				t.Fatalf("CLI could not read the invocation's appliance after gok chdir: code %d: %v", code, readErr)
			}
		})
	}
}
