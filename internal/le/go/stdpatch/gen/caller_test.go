// VALIDATES: CallerEnvironment gives a program outside the checkout the
// GOROOT, GOTOOLCHAIN and PATH its caller had (spec-le-one-go-cache-per-host O-5).
// PREVENTS: Ze's patched GOROOT leaking into another project's tools (R-8).

package gen

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestCallerEnvironmentRestoresWhatActivateReplaced proves the half of O-5
// that le's own environment cannot show: after Activate replaced GOROOT,
// GOTOOLCHAIN and PATH, CallerEnvironment answers the values the caller had,
// and drops a variable the caller never set. It stands in for one Activate by
// writing the record Activate keeps, and restores it after, because a real
// activation would build a tree and change this test process's environment.
func TestCallerEnvironmentRestoresWhatActivateReplaced(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	activation.Lock()
	savedDone, savedTree, savedErr, savedCaller := activation.done, activation.tree, activation.err, activation.caller
	activation.done = true
	activation.caller = callerValues{
		{value: "/usr/local/go", set: true}, // GOROOT
		{value: "", set: false},             // GOTOOLCHAIN
		{value: "/usr/bin:/bin", set: true}, // PATH
	}
	activation.Unlock()
	t.Cleanup(func() {
		activation.Lock()
		activation.done, activation.tree, activation.err, activation.caller = savedDone, savedTree, savedErr, savedCaller
		activation.Unlock()
	})

	activated := []string{
		"HOME=/home/someone",
		"GOROOT=" + filepath.Join(root, "cache", "stdpatch", "go1.99"),
		"GOTOOLCHAIN=local",
		"PATH=" + filepath.Join(root, "cache", "stdpatch", "go1.99", "bin") + ":/usr/bin:/bin",
	}
	got, err := CallerEnvironment(root, activated)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"HOME=/home/someone", "GOROOT=/usr/local/go", "PATH=/usr/bin:/bin"}
	if !slices.Equal(got, want) {
		t.Errorf("CallerEnvironment = %q, want %q", got, want)
	}
}
