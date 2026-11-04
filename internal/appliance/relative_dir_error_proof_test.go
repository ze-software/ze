//go:build linux

package appliance

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunRefusesAnUnresolvableApplianceDirectory removes the invocation's cwd
// before resolving a relative --dir. The actual CLI must refuse the getcwd
// failure instead of dispatching with a relative path or a guessed directory.
func TestRunRefusesAnUnresolvableApplianceDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "vanished")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	})
	if err := os.Remove(directory); err != nil {
		t.Fatal(err)
	}
	previousList, previousBase := cmdList, baseDir
	defer func() { cmdList, baseDir = previousList, previousBase }()
	dispatched := false
	cmdList = func([]string) int {
		dispatched = true
		return exitOK
	}
	if code := Run([]string{"--dir", "relative-appliances", "list"}); code != exitError {
		t.Errorf("unresolvable appliance directory answered %d, want refusal", code)
	}
	if dispatched {
		t.Error("CLI dispatched after its appliance directory could not be resolved")
	}
}
