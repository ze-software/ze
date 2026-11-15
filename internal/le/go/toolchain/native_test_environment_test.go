// Design: docs/architecture/core-design.md -- test environments carry the launching native le.
// Related: gotoolchain.go -- test environments carry the launching native le.
// VALIDATES: scoped unit actions and the CI race action share the toolchain's
// test environment, so tool fixtures resolve this run's native le by bare name.
// PREVENTS: CI depending on an unrelated le installed in its ambient PATH.
package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestNativeTestEnvironmentResolvesTheLaunchingLE(t *testing.T) {
	native := filepath.Join(t.TempDir(), "le")
	other := filepath.Join(t.TempDir(), "le")
	for _, path := range []string{native, other} {
		if err := os.WriteFile(path, nil, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	inherited := []string{"PATH=" + filepath.Dir(other), "KEEP=fixture"}
	environment := nativeTestEnvironment(inherited, native)
	if !slices.Contains(environment, "KEEP=fixture") {
		t.Fatal("native test environment lost another variable")
	}
	wantPath := filepath.Dir(native) + string(os.PathListSeparator) + filepath.Dir(other)
	if got := valueOf(environment, "PATH"); got != wantPath {
		t.Fatalf("child PATH = %q, want %q", got, wantPath)
	}
	t.Setenv("PATH", valueOf(environment, "PATH"))
	resolved, err := exec.LookPath("le")
	if err != nil {
		t.Fatalf("fixture cannot resolve le: %v", err)
	}
	if resolved != native {
		t.Fatalf("fixture resolves le to %q (%v), want launching binary %q", resolved, err, native)
	}
	if !slices.Equal(inherited, []string{"PATH=" + filepath.Dir(other), "KEEP=fixture"}) {
		t.Fatal("native test environment mutated its caller's environment")
	}
}

func TestNativeTestEnvironmentLeavesOtherExecutablesAlone(t *testing.T) {
	inherited := []string{"PATH=/fixture/bin", "KEEP=fixture"}
	for _, executable := range []string{"le", "/fixture/unit.test", "/fixture/ze"} {
		if got := nativeTestEnvironment(inherited, executable); !slices.Equal(got, inherited) {
			t.Errorf("executable %q changed its fixture environment to %q", executable, got)
		}
	}
}
