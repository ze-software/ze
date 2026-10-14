package appliance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGokBuildsWithTheHostToolchain proves that the go subprocesses gok spawns
// run the go of the patched GOROOT and never switch toolchain.
//
// VALIDATES: AC-28, runGokInProcess refuses under le's old pin
// (GOTOOLCHAIN=go1.27.0) before gok runs, naming GOTOOLCHAIN and ending with
// the command line; under the patched tree gok runs with the GOTOOLCHAIN=local
// that environment carries, which SetGokGoEnv no longer sets itself.
// PREVENTS: the arm64 build of 2026-10-10, where le's pin (GOTOOLCHAIN=go1.27.0,
// from the go directive) met a go1.27.1 host. Go then fetched go1.27.0 from a
// GOMODCACHE that is the checked-in gokrazy/modcache with GOPROXY=off, answered
// "toolchain not available" to every command, and gok reported the failed
// `go list` as "go get ze.invalid/kernel", which reads like a missing replace.
// That build is now stopped by the refusal rather than by an override.
//
// Method: run runGokInProcess with a fake gok that records the GOTOOLCHAIN its
// subprocesses would inherit.
func TestGokBuildsWithTheHostToolchain(t *testing.T) {
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "gokrazy", "modcache"), 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	t.Setenv("GOMODCACHE", "")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPROXY", "")

	saved := gokExecuteFn
	t.Cleanup(func() { gokExecuteFn = saved })
	ran := false
	seen := ""
	gokExecuteFn = func(context.Context, []string) error {
		ran = true
		seen = os.Getenv("GOTOOLCHAIN")
		return nil
	}

	t.Run("refuses-under-the-pin", func(t *testing.T) {
		underPatchedTree(t, patchedTreeFixture(t, true))
		t.Setenv("GOTOOLCHAIN", "go1.27.0")
		err := runGokInProcess(nil)
		if err == nil {
			t.Fatal("runGokInProcess ran under GOTOOLCHAIN=go1.27.0")
		}
		if !strings.Contains(err.Error(), "GOTOOLCHAIN") || !strings.HasSuffix(err.Error(), patchedTreeCommand) {
			t.Errorf("refusal does not name GOTOOLCHAIN and end with the command line:\n%v", err)
		}
		if ran {
			t.Error("gok ran before the refusal")
		}
	})
	t.Run("runs-under-the-patched-tree", func(t *testing.T) {
		underPatchedTree(t, patchedTreeFixture(t, true))
		if err := runGokInProcess(nil); err != nil {
			t.Fatalf("runGokInProcess: %v", err)
		}
		if !ran || seen != "local" {
			t.Fatalf("gok ran=%v with GOTOOLCHAIN=%q, want local: a pinned toolchain the checked-in modcache lacks cannot be fetched under GOPROXY=off", ran, seen)
		}
	})
}
