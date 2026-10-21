//go:build linux || darwin

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestKeepWorkingDirRestoresAfterAChdir proves a gok run that moves into the
// instance directory leaves the caller where it was, with the run's error
// kept. Method: run a function that chdirs into a directory and removes it,
// as gok and the prepared-instance cleanup do, then compare the working
// directory before and after.
func TestKeepWorkingDirRestoresAfterAChdir(t *testing.T) {
	start := t.TempDir()
	t.Chdir(start)
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	instance := filepath.Join(t.TempDir(), "instance")
	if err := os.Mkdir(instance, 0o750); err != nil {
		t.Fatal(err)
	}
	failed := errors.New("gok failed")
	got := keepWorkingDir(func() error {
		if err := os.Chdir(instance); err != nil {
			return err
		}
		if err := os.Remove(instance); err != nil {
			return err
		}
		return failed
	})
	if !errors.Is(got, failed) {
		t.Fatalf("keepWorkingDir answered %v, want the run's error %v", got, failed)
	}

	after, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory unreadable after the run: %v", err)
	}
	if after != want {
		t.Fatalf("working directory after the run is %s, want %s", after, want)
	}
}
