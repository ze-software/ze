// Design: docs/contributing/running-commands.md -- cleanup without verification.

package reclaim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/verify"
)

// TestStandaloneReclaimWorktrees reaches the registered command handler in an
// isolated repository. Only a directoryless marker is seeded: no verification
// action, worktree creation, build, or suite is part of this cleanup entry.
func TestStandaloneReclaimWorktrees(t *testing.T) {
	root := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "git", "init", "-q", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initialize isolated cleanup repository: %v: %s", err, output)
	}
	base := filepath.Join(root, "tmp", "verify-worktree")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(base, "directoryless.owner")
	if err := os.WriteFile(marker, []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lepath.RootKey, root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	handler := leroot.LookupCommand("reclaim")
	if handler == nil {
		t.Fatal("standalone reclaim command has no registered handler")
	}
	answer := handler(commandtest.Args("worktrees"))
	report, ok := answer.Payload.(verify.Report)
	if !ok || answer.Code != 0 || report.Action != "reclaim" || report.Verify != nil {
		t.Fatalf("standalone cleanup returned verification or failure: %+v", answer)
	}
	if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("standalone entry left its dead directoryless marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp", "verify")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("standalone cleanup created stage artifacts: %v", err)
	}
}
