package appliance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGokBuildsWithTheHostToolchain proves that the go subprocesses gok spawns
// run the go on PATH and never switch toolchain.
//
// VALIDATES: runGokInProcess sets GOTOOLCHAIN=local before gok runs.
// PREVENTS: the arm64 build of 2026-10-10, where le's pin (GOTOOLCHAIN=go1.27.0,
// from the go directive) met a go1.27.1 host. Go then fetched go1.27.0 from a
// GOMODCACHE that is the checked-in gokrazy/modcache with GOPROXY=off, answered
// "toolchain not available" to every command, and gok reported the failed
// `go list` as "go get ze.invalid/kernel", which reads like a missing replace.
//
// Method: run runGokInProcess under the pin le exports, with a fake gok that
// records the GOTOOLCHAIN its subprocesses would inherit.
func TestGokBuildsWithTheHostToolchain(t *testing.T) {
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "gokrazy", "modcache"), 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	t.Setenv("GOTOOLCHAIN", "go1.27.0")
	t.Setenv("GOMODCACHE", "")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPROXY", "")

	saved := gokExecuteFn
	t.Cleanup(func() { gokExecuteFn = saved })
	seen := ""
	gokExecuteFn = func(context.Context, []string) error {
		seen = os.Getenv("GOTOOLCHAIN")
		return nil
	}

	if err := runGokInProcess(nil); err != nil {
		t.Fatalf("runGokInProcess: %v", err)
	}
	if seen != "local" {
		t.Fatalf("gok ran with GOTOOLCHAIN=%q, want local: a pinned toolchain the checked-in modcache lacks cannot be fetched under GOPROXY=off", seen)
	}
}
