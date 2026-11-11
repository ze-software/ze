// Design: docs/contributing/terminal-demos.md -- the poster proof drives the ze of the tree under test.
// Related: web_config_poster_integration_test.go -- the proof that builds and checks its ze.
package siteterminaldemo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// posterZeIsOwned refuses a poster child whose `ze` on PATH is not the copy
// its parent built from the tree under test (want). A ze found earlier on PATH,
// such as the hand-built shared tmp/terminal-demos/bin copy, would judge code
// older than the tree.
func posterZeIsOwned(want string) error {
	got, err := exec.LookPath("ze")
	if err != nil {
		return fmt.Errorf("poster child finds no ze on PATH (want %s): %w", want, err)
	}
	if got != want {
		return fmt.Errorf("poster child drives %s, not %s, the ze its parent built from the tree under test; "+
			"a binary older than the tree would judge old code, so put the built copy first on PATH", got, want)
	}
	return nil
}

// TestPosterZeIsOwnedRefusesAnotherZe proves the poster child refuses to drive
// a ze it was not handed. Method: two directories each hold an executable ze;
// with the stale one first on PATH the check must refuse and name it, and with
// the owned one first it must pass.
// MUTATION: making posterZeIsOwned return nil after LookPath passes the stale
// PATH; dropping the PATH order from the container command puts the shared
// stale copy first and the child refuses it.
func TestPosterZeIsOwnedRefusesAnotherZe(t *testing.T) {
	stale := t.TempDir()
	owned := t.TempDir()
	for _, dir := range []string{stale, owned} {
		if err := os.WriteFile(filepath.Join(dir, "ze"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(owned, "ze")

	t.Setenv("PATH", stale+string(os.PathListSeparator)+owned)
	err := posterZeIsOwned(want)
	if err == nil {
		t.Fatal("a ze earlier on PATH than the built copy was accepted")
	}
	if !strings.Contains(err.Error(), filepath.Join(stale, "ze")) {
		t.Fatalf("the refusal does not name the ze it found: %v", err)
	}

	t.Setenv("PATH", owned+string(os.PathListSeparator)+stale)
	if err := posterZeIsOwned(want); err != nil {
		t.Fatalf("the built copy first on PATH was refused: %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if err := posterZeIsOwned(want); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("a PATH with no ze was not refused as missing: %v", err)
	}
}
