package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBareStepProgramResolvesBesideZe proves a step that names a program bare
// runs the one beside ze, which the step's PATH holds, even when the runner's
// own PATH does not: the guest run of the container suite failed
// "ze-containerprobe: executable file not found in $PATH" although the probe
// sat beside ze.
//
// Method: a program in the ze directory and a runner PATH that lacks it.
// exec.LookPath, which exec.Command uses, misses it; childLookPath finds it.
func TestBareStepProgramResolvesBesideZe(t *testing.T) {
	binaries := t.TempDir()
	program := filepath.Join(binaries, "ze-probe-fixture")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // an executable test fixture
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	r := &Runner{zePath: filepath.Join(binaries, binNameZe)}

	if _, err := exec.LookPath("ze-probe-fixture"); err == nil {
		t.Fatal("the runner's own PATH finds the program, so this test cannot show the step's PATH is what found it")
	}
	got, err := r.childLookPath("ze-probe-fixture")
	if err != nil {
		t.Fatalf("childLookPath: %v", err)
	}
	if got != program {
		t.Errorf("childLookPath answered %q, want %q", got, program)
	}
}

// TestBareStepProgramMissNamesTheSearchedPath proves a program on none of the
// step's PATH directories, or one present without an execute bit, is a miss
// that names the PATH it searched and leaves the name unchanged, and that a
// name holding a separator is a path and is not looked up.
func TestBareStepProgramMissNamesTheSearchedPath(t *testing.T) {
	binaries := t.TempDir()
	if err := os.WriteFile(filepath.Join(binaries, "ze-not-executable"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	r := &Runner{zePath: filepath.Join(binaries, binNameZe)}

	for _, name := range []string{"ze-absent", "ze-not-executable"} {
		got, err := r.childLookPath(name)
		if err == nil {
			t.Errorf("childLookPath(%q) found %q, want a miss", name, got)
			continue
		}
		if got != name {
			t.Errorf("childLookPath(%q) answered %q on a miss, want the name unchanged", name, got)
		}
		if !strings.Contains(err.Error(), binaries) {
			t.Errorf("the miss for %q does not name the searched directory %s: %v", name, binaries, err)
		}
	}

	path := filepath.Join("some", "where", "program")
	if got, err := r.childLookPath(path); err != nil || got != path {
		t.Errorf("childLookPath(%q) = %q, %v; want the path unchanged and no error", path, got, err)
	}
}
