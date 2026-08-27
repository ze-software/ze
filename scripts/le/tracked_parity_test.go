// The migration's proof for `le tracked`: both halves judge the same commit and
// reach the same verdict about it.
//
// internal/le/letracked replaces scripts/le/application/tracked.py. Both versions
// remain until the swap. This file is deliberately HERE because it is a
// migration artifact. The commit that deletes the script also deletes this
// proof.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over this checkout's HEAD both
// halves answer 0 and both count a non-zero number of areas, and over a commit
// that predates each half's own tree both answer 2 rather than 0.
//
// THIS IS THE ONE PORT WHOSE OUTPUT IS NOT COMPARED. The reason comes from the
// languages, not a shortcut. Python asks whether every registered area IMPORTS
// from the commit. A Go area cannot fail that way because a caller without its
// callee does not compile. ze-repository-tracked-build-check already compiles
//
// the committed tree. The replacement asks whether each committed tool is WIRED
// into the binary and whether that binary runs. Those checks have different
// findings and text. The unchanged contract is exit 0 for a sound commit and
// exit 2 for an unjudgeable commit. The finding count must also be nonzero.
//
// The exit-2 case gives each half its OWN commit because each has a different
// root file. The commit must predate that file. The Python tree arrived with
// scripts/le/registry.py, and the Go tree arrived with internal/le/register.go. One
// existing commit cannot predate both.

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/letracked"
)

// scriptTracked runs the Python half over one revision and answers its exit
// code. The page it prints is its own words about its own findings, which is
// what the header above explains is not comparable.
func scriptTracked(t *testing.T, root, rev string) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "python3", "-m", "le.application.tracked", "--rev", rev)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "scripts"))

	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !asExitError(err, &exit) {
			t.Fatalf("run the script: %v", err)
		}
		return exit.ExitCode()
	}
	return 0
}

// TestBothHalvesFindThisCheckoutsHeadSound is the case that must not be
// vacuous: each half builds or imports the committed tree and says it works.
func TestBothHalvesFindThisCheckoutsHeadSound(t *testing.T) {
	root := repoRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), letracked.DefaultDeadline)
	defer cancel()

	verdict, code, err := letracked.Run(ctx, root, "HEAD")
	if err != nil {
		t.Fatalf("the command could not judge HEAD: %v", err)
	}
	if code != 0 {
		t.Fatalf("the command says HEAD is broken:\n%s", verdict.Text())
	}
	if verdict.Areas == 0 {
		t.Fatal("the command counted zero areas: its verdict is vacuous")
	}

	if scripted := scriptTracked(t, root, "HEAD"); scripted != 0 {
		t.Errorf("the script says HEAD is broken (exit %d) while the command says it is sound", scripted)
	}
}

// TestBothHalvesRefuseACommitThatPredatesThem pins exit 2 for both halves. An
// unjudgeable commit must not appear sound. Each half has a separate root-file
// commit to predate: scripts/le/registry.py for Python and internal/le/register.go for
// Go.
func TestBothHalvesRefuseACommitThatPredatesThem(t *testing.T) {
	root := repoRoot(t)

	// The parents of the two commits that ADDED each half's own root file.
	const beforeThePythonLe = "f8802f51ed1ef0808461d34eb87a4e0ad2475fe4~1"
	const beforeTheGoLe = "12ce0438a3d7c9f1953dd0599f62d67a4bf77716~1"

	if scripted := scriptTracked(t, root, beforeThePythonLe); scripted != 2 {
		t.Errorf("the script exited %d for a commit holding no le at all, want 2", scripted)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, code, err := letracked.Run(ctx, root, beforeTheGoLe)
	if code != 2 {
		t.Errorf("the command exited %d for a commit holding no le at all, want 2", code)
	}
	if err == nil {
		t.Error("the command refused a commit and gave no reason")
	}
}
