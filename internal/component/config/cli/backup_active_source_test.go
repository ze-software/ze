// Design: docs/guide/config-editor.md -- committed backup reads follow active versions.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func backupActiveSource(t *testing.T, state string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "versioned.zefs")
	store, err := storage.CreateBlob(path)
	require.NoError(t, err)
	for _, name := range []string{"router.conf", "other.conf"} {
		content := backupTestConfig
		if name == "other.conf" {
			content = strings.ReplaceAll(content, "1.2.3.4", "203.0.113.9")
		}
		_, err := storage.RestoreConfig(store, name, []byte(content))
		require.NoError(t, err)
		if state == "pointer-only" {
			require.NoError(t, store.Remove(name))
		} else {
			stale := strings.ReplaceAll(backupTestConfig, "1.2.3.4", "192.0.2.199")
			stale = strings.ReplaceAll(stale, "65000", "65099")
			require.NoError(t, store.WriteFile(name, []byte(stale), 0o600))
		}
	}
	if state == "unresolved" {
		require.NoError(t, store.WriteKey(zefs.KeyConfigActive.Key("router.conf"), []byte("20000101-000000.000\n")))
	}
	require.NoError(t, store.Close())
	return path
}

// TestBackupCommandsReadTheActiveVersion drives the actual show, resolved
// comparison, set and offline session entry points against pointer-only and
// stale-mirror artifacts. A corrupt pointer refuses every route unchanged.
func TestBackupCommandsReadTheActiveVersion(t *testing.T) {
	for _, state := range []string{"pointer-only", "stale-mirror", "unresolved"} {
		for _, operation := range []string{"show", "diff", "set", "edit"} {
			t.Run(state+"/"+operation, func(t *testing.T) {
				path := backupActiveSource(t, state)
				original, err := os.ReadFile(path)
				require.NoError(t, err)
				var shown bytes.Buffer
				var code int
				var output string
				switch operation {
				case "show":
					code = showConfig(&shown, []string{"--backup", path, "router.conf", "bgp"})
					output = shown.String()
				case "diff":
					code, output = captureDiffStdout(t, func() int {
						return cmdDiffImpl(nil, []string{"--backup", path, "router.conf", "other.conf"})
					})
				case "set":
					code, output = captureStderr(t, func() int {
						return cmdSetImpl(nil, []string{"--backup", path, "router.conf", "bgp", "router-id", "192.0.2.88"})
					})
				case "edit":
					before := runBackupSession
					t.Cleanup(func() { runBackupSession = before })
					runBackupSession = func(ed *editor.Editor) int {
						output = ed.OriginalContent()
						require.NoError(t, ed.Close())
						return exitOK
					}
					code = cmdEditWithStorage(nil, []string{"--backup", path, "router.conf"})
				}
				if state == "unresolved" {
					require.NotEqual(t, exitOK, code, output)
					kept, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, original, kept, "a failed active pointer must not edit the fallback mirror")
					return
				}
				require.Equal(t, exitOK, code, output)
				if operation == "set" {
					active, mirror, _ := readBackupActive(t, path)
					require.Equal(t, active, mirror)
					require.Contains(t, active, "192.0.2.88")
					require.Contains(t, active, "65000")
					require.NotContains(t, active, "65099")
					return
				}
				require.Contains(t, output, "1.2.3.4")
				require.NotContains(t, output, "192.0.2.199")
				if operation == "diff" {
					require.Contains(t, output, "203.0.113.9")
				}
			})
		}
	}
}

// TestBackupEditorDefaultFindsTheActivePointer proves the instance's existing
// versioned configuration is selected without creating a fallback mirror.
func TestBackupEditorDefaultFindsTheActivePointer(t *testing.T) {
	path := backupActiveSource(t, "pointer-only")
	source, err := openBackup(path, true)
	require.NoError(t, err)
	require.NoError(t, source.WriteKey(zefs.KeyInstanceName.Key(), []byte("router")))
	require.NoError(t, source.Close())
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	original := runBackupSession
	t.Cleanup(func() { runBackupSession = original })
	called := false
	runBackupSession = func(ed *editor.Editor) int {
		called = true
		require.Equal(t, backupTestConfig, ed.OriginalContent())
		require.NoError(t, ed.Close())
		return exitOK
	}
	require.Equal(t, exitOK, cmdEditWithStorage(nil, []string{"--backup", path}))
	require.True(t, called)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after, "opening the selected active version must not manufacture a raw mirror")
}
