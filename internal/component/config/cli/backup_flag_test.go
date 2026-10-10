package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
)

const backupTestConfig = `bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4;
}
`

// newBackupArtifact writes an artifact holding router.conf and closes it, so
// the command under test opens it the way an operator's backup is opened.
func newBackupArtifact(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zefs")
	created, err := storage.CreateBlob(path)
	require.NoError(t, err)
	require.NoError(t, created.WriteFile("router.conf", []byte(backupTestConfig), 0))
	require.NoError(t, created.Close())
	return path
}

// readBackupActive answers the config every pointer-following reader sees in
// the artifact, and the file/active mirror, which MUST agree after a commit.
func readBackupActive(t *testing.T, path string) (active, mirror string, versions int) {
	t.Helper()
	store, err := storage.OpenBlob(path, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	data, err := storage.ReadActiveConfig(store, "router.conf")
	require.NoError(t, err)
	file, err := store.ReadFile("router.conf")
	require.NoError(t, err)
	list, err := store.ListVersions("router.conf")
	require.NoError(t, err)
	return string(data), string(file), len(list)
}

// TestEditBackupFlagConflicts proves `--backup` refuses the flags that need a
// loose file or a daemon, naming both flags, before anything is opened.
//
// VALIDATES: spec-storage-2 AC-14.
// PREVENTS: `-f --backup` silently editing one of the two sources.
func TestEditBackupFlagConflicts(t *testing.T) {
	path := newBackupArtifact(t)
	for _, tc := range []struct {
		name  string
		args  []string
		names []string
	}{
		{"loose file", []string{"-f", "--backup", path}, []string{"--backup", "-f"}},
		{"web", []string{"--backup", path, "--web", "8080"}, []string{"--backup", "--web"}},
		{"insecure web", []string{"--backup", path, "--insecure-web"}, []string{"--backup", "--insecure-web"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stderr := captureStderr(t, func() int { return cmdEditWithStorage(nil, tc.args) })
			assert.Equal(t, exitError, code)
			for _, name := range tc.names {
				assert.Contains(t, stderr, name)
			}
		})
	}
	code, stderr := captureStderr(t, func() int {
		return cmdSetImpl(nil, []string{"--backup", path, "--reload", "router.conf", "bgp", "router-id", "5.6.7.8"})
	})
	assert.Equal(t, exitError, code)
	assert.Contains(t, stderr, "--backup and --reload")
}

// TestEditBackupNoDaemon drives `ze config edit --backup` with no store, no
// credentials and no daemon anywhere, and commits inside the session.
//
// The session MUST open on the artifact in session mode, publish its commit
// into the artifact (a new version, the active pointer, and the file/active
// mirror agreeing with it), and refuse a second editor both before and after
// that commit rewrote the artifact.
//
// VALIDATES: spec-storage-2 AC-12, R-7, R-9, R-11.
// PREVENTS: `--backup` reaching the daemon probe or the ephemeral daemon, a
// commit that moves the mirror but leaves the pointer on the old version, and
// a lock that dies with the inode the first commit replaced.
func TestEditBackupNoDaemon(t *testing.T) {
	path := newBackupArtifact(t)
	t.Setenv("ZE_CONFIG_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	secondBefore, secondAfter := -1, -1
	var sessionStderr string
	ran := false
	saved := runBackupSession
	t.Cleanup(func() { runBackupSession = saved })
	runBackupSession = func(ed *editor.Editor) int {
		defer ed.Close() //nolint:errcheck // Test session end.
		ran = true
		require.True(t, ed.HasSession(), "the backup editor MUST be a session editor")
		require.True(t, ed.HasReloadNotifier(), "a session commit MUST take the staged-candidate path")
		secondBefore, _ = captureStderr(t, func() int { return cmdEditWithStorage(nil, []string{"--backup", path}) })

		require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65001"))
		result, content, err := ed.CommitSessionCandidate(time.Now())
		require.NoError(t, err)
		require.Equal(t, 1, result.Applied)
		require.NoError(t, ed.NotifyReload())
		if err := ed.MarkCommittedContent(content); err != nil {
			t.Fatal(err)
		}

		secondAfter, sessionStderr = captureStderr(t, func() int { return cmdEditWithStorage(nil, []string{"--backup", path, "router.conf"}) })
		return exitOK
	}

	code := cmdEditWithStorage(nil, []string{"--backup", path, "router.conf"})
	require.Equal(t, exitOK, code)
	require.True(t, ran, "the session never started")
	assert.Equal(t, exitError, secondBefore, "a second editor MUST be refused")
	assert.Equal(t, exitError, secondAfter, "a second editor MUST still be refused after the commit rewrote the artifact")
	assert.Contains(t, sessionStderr, path, "the refusal names the artifact")

	active, mirror, versions := readBackupActive(t, path)
	assert.Contains(t, active, "65001", "the active pointer MUST follow the commit")
	assert.Equal(t, active, mirror, "file/active MUST mirror the active version")
	assert.Equal(t, 2, versions, "the previous config becomes the rollback version, the commit the active one")
}

// TestConfigBackupFamily runs the offline family over one artifact: show
// reads its config, set publishes into it, diff compares against its history,
// and list names what it holds.
//
// VALIDATES: spec-storage-2 AC-13.
// PREVENTS: a --backup command that reads the live store, or a set that
// writes the mirror while the active pointer stays on the old version.
func TestConfigBackupFamily(t *testing.T) {
	path := newBackupArtifact(t)

	var shown bytes.Buffer
	require.Equal(t, exitOK, showConfig(&shown, []string{"--backup", path, "router.conf", "bgp"}))
	assert.Contains(t, shown.String(), "65000")

	for _, asn := range []string{"65010", "65020"} {
		code, stderr := captureStderr(t, func() int {
			return cmdSetImpl(nil, []string{"--backup", path, "router.conf", "bgp", "session", "asn", "local", asn})
		})
		require.Equal(t, exitOK, code, stderr)
		active, mirror, _ := readBackupActive(t, path)
		assert.Contains(t, active, asn)
		assert.Equal(t, active, mirror)
	}

	code, diff := captureDiffStdout(t, func() int {
		return cmdDiffImpl(nil, []string{"--backup", path, "2", "router.conf"})
	})
	require.Equal(t, exitOK, code)
	// Revision 1 is the active version itself, as in a daemon's store;
	// revision 2 is the rollback the second set left behind, and revision 3
	// the seed config the first set adopted, stamped before its candidate.
	assert.Contains(t, diff, "65010")
	assert.Contains(t, diff, "65020")

	code, listed := captureDiffStdout(t, func() int { return cmdListWithStorage(nil, []string{"--backup", path}) })
	require.Equal(t, exitOK, code)
	assert.Equal(t, "[data] file/active/router.conf", strings.TrimSpace(listed))
}
