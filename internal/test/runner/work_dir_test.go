// VALIDATES: every .ci step runs in a directory of its own, and only a
// repository-anchored tool keeps the repository root.
// PREVENTS: the runner handing a child no working directory, so it inherits the
// one `./le` was started in. A ze daemon started there writes database.zefs,
// daemon.log, its rendered config, its host keys and its rollback/ and crash/
// trees into the checkout, where they show as untracked files beside real work.
// 1503 of 1789 .ci files declare no tmpfs block, so that was the common case.
// See plan/journal/test-artifacts-land-in-the-repository-root.md.

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestChildWorkingDirectoryAnchorsOnlyRepositoryTools pins the decision the
// runner makes per binary. `go test` takes ./... package patterns that mean
// nothing outside the module and `./le` IS a path in the repository root, so
// both keep that root. Everything else gets the test's own directory.
func TestChildWorkingDirectoryAnchorsOnlyRepositoryTools(t *testing.T) {
	r := &Runner{baseDir: "/repo"}
	rec := &Record{WorkDir: "/scratch/ze-work-1"}

	for _, binName := range []string{"go", "le", "./le"} {
		require.Equal(t, "/repo", r.childWorkingDirectory(binName, rec),
			"%s resolves its arguments against the module, so it keeps the repository root", binName)
	}
	for _, binName := range []string{"ze", "ze-test", "ze-peer", "tc", "curl"} {
		require.Equal(t, "/scratch/ze-work-1", r.childWorkingDirectory(binName, rec),
			"%s writes its runtime files into its cwd, so it gets the test's own directory", binName)
	}
}

// TestRecordWithoutTmpfsStillRunsOutsideTheCheckout is the discriminating half.
// It runs a record that declares NO tmpfs block, which is the population the
// defect lived in, and asks the child where it is. Restore the old
// `if rec.TmpfsTempDir != ""` guard and the answer becomes the repository root.
func TestRecordWithoutTmpfsStillRunsOutsideTheCheckout(t *testing.T) {
	if _, err := os.Stat("/bin/pwd"); err != nil {
		t.Skipf("no /bin/pwd to ask: %v", err)
	}

	baseDir := t.TempDir()
	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	// expect=exit is what makes the runner WAIT for the foreground child, so its
	// output is complete when the assertion below reads it. Without it the
	// default arm waits peers alone and the probe races its own answer.
	success := 0
	rec := &Record{
		Name:           "work-dir-probe",
		Extra:          map[string]string{"timeout": "30s"},
		Conf:           map[string]any{},
		ExpectExitCode: &success,
		RunCommands: []RunCommand{
			{Mode: modeForeground, Seq: 1, Exec: "/bin/pwd"},
		},
	}

	require.True(t, r.runTest(context.Background(), rec, &RunOptions{}),
		"the probe must run: %v", rec.Error)
	require.NotEmpty(t, rec.WorkDir, "a record with no tmpfs block still owns a working directory")
	require.Empty(t, rec.TmpfsTempDir, "a record that declares no tmpfs files declares no tmpfs directory")

	// The child's own answer. This is the assertion the fix exists for: restore
	// the guard and the child answers with the directory the runner was started
	// in instead.
	reported := strings.TrimSpace(rec.ClientOutput)
	require.Equal(t, rec.WorkDir, reported, "the child must run in the directory the record owns")
	require.True(t, strings.HasPrefix(filepath.Base(rec.WorkDir), workDirPrefix),
		"a directory left behind by a killed run must say what made it: %s", rec.WorkDir)
	require.NotEqual(t, r.baseDir, rec.WorkDir, "the test's directory is never the repository root")

	_, err = os.Stat(rec.WorkDir)
	require.True(t, os.IsNotExist(err), "the work directory is removed when the test ends")
}
