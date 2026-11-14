package testweakened

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitState is what a read of the repository must leave as it found it: the
// index's bytes and modification time, and every lock file under .git.
type gitState struct {
	index    []byte
	modified time.Time
	locks    []string
}

func readGitState(t *testing.T, root string) gitState {
	t.Helper()
	index := filepath.Join(root, ".git", "index")
	content, err := os.ReadFile(index) //nolint:gosec // the fixture repository's own index
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	state := gitState{index: content, modified: info.ModTime()}
	walkErr := filepath.WalkDir(filepath.Join(root, ".git"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(entry.Name(), ".lock") {
			state.locks = append(state.locks, path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	return state
}

func sameGitState(t *testing.T, step string, before, after gitState) {
	t.Helper()
	if !bytes.Equal(before.index, after.index) || !before.modified.Equal(after.modified) {
		t.Errorf("%s rewrote .git/index", step)
	}
	if len(after.locks) != 0 {
		t.Errorf("%s left lock files under .git: %v", step, after.locks)
	}
}

// TestProposedAndApproveLeaveGitUntouched proves the edit hook's baseline read
// and the approval writer only read the repository. The edit hook runs on
// every Write and Edit of every session, which share one index, so a refresh
// of the index, an optional lock or a detached auto-maintenance run would
// race another session's commit script.
// Method: a committed fixture whose working-tree file is written in the same
// second as the commit (the racily clean case `git diff` refreshes), then a
// whole-file Edit proposal, a refused approval and an accepted approval, each
// followed by a comparison of .git/index and a search for lock files. The
// pause lets any detached maintenance the read started reach the disk.
// VALIDATES: proposedCommittedUnits and the approval resolver use read-only git.
// PREVENTS: a guard that rewrites or locks the shared index on each edit.
// Limit: none of the commands the hook path runs today refreshes the index,
// so this test cannot go red if gitenv.ReadOnlyQuery is dropped;
// TestEveryHookGitQueryIsReadOnly is the test that does. This one is kept
// as the end-to-end backstop: it observes the index and the lock files
// themselves, so a future query whose side effect no setting suppresses is
// caught here.
func TestProposedAndApproveLeaveGitUntouched(t *testing.T) {
	root := headBaselineRepository(t)
	// The fixture writes after its commit, so a status-like read finds a
	// racily clean entry and would write the refreshed index back.
	writeProposedFile(t, root, "pkg/split_test.go", "package pkg\nfunc TestSplit(t *testing.T) {\n\trequire.NoError(t, err)\n}\n")
	settle := func() { time.Sleep(300 * time.Millisecond) } // sleep(no-signal): detached git maintenance announces nothing
	settle()
	before := readGitState(t, root)

	if _, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/intent_test.go", Tool: "Edit", ToolInput: ProposedToolInput{
			OldString: "\trequire.NoError(t, err)\n", NewString: commentedOut,
		},
	}); err != nil {
		t.Fatal(err)
	}
	settle()
	sameGitState(t, "a whole-file Edit proposal", before, readGitState(t, root))

	if _, err := Approve(root, fixtureSession, "pkg.TestMissing", "verdict"); err == nil {
		t.Fatal("an approval of an undeclared unit was accepted")
	}
	settle()
	sameGitState(t, "a refused approval", before, readGitState(t, root))

	if _, err := Approve(root, fixtureSession, "pkg.TestKept", "verdict"); err != nil {
		t.Fatal(err)
	}
	settle()
	sameGitState(t, "an accepted approval", before, readGitState(t, root))
}
