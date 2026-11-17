//go:build linux || darwin || freebsd

// Design: docs/guide/config-editor.md -- an import cannot follow a replaced path.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/command"
)

// TestLoadOpenPinsValidatedPath swaps a file or ancestor after the path policy
// accepted a regular input. Reopening must refuse the link rather than reading
// an outside file or the configuration store's private namespace.
func TestLoadOpenPinsValidatedPath(t *testing.T) {
	for _, replacement := range []string{"file", "ancestor", "private"} {
		t.Run(replacement, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", root)
			configDir := filepath.Join(root, "ze")
			publicDir := filepath.Join(configDir, "public")
			require.NoError(t, os.MkdirAll(publicDir, 0o700))
			path := filepath.Join(publicDir, "section.conf")
			require.NoError(t, os.WriteFile(path, []byte("inside"), 0o600))
			resolved, err := command.CallerPath(command.RouteRemote, "config edit", nil, path)
			require.NoError(t, err)
			targetDir := filepath.Join(root, "outside")
			if replacement == "private" {
				targetDir = filepath.Join(configDir, "database")
			}
			require.NoError(t, os.Mkdir(targetDir, 0o700))
			target := filepath.Join(targetDir, "section.conf")
			require.NoError(t, os.WriteFile(target, []byte("must not be read"), 0o600))
			if replacement == "file" {
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Symlink(target, path))
			} else {
				require.NoError(t, os.Rename(publicDir, publicDir+"-old"))
				require.NoError(t, os.Symlink(targetDir, publicDir))
			}
			file, err := openLoadFile(resolved)
			if file != nil {
				require.NoError(t, file.Close())
			}
			require.Error(t, err, "a path replaced after validation must not open")
		})
	}
}

// TestLoadOpenKeepsRegularInput prevents a refusal-only implementation from
// passing the replacement probes: the unchanged regular file stays readable.
func TestLoadOpenKeepsRegularInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "section.conf")
	require.NoError(t, os.WriteFile(path, []byte("inside"), 0o600))
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	file, err := openLoadFile(resolved)
	require.NoError(t, err)
	require.NoError(t, file.Close())
}

func TestLoadOpenRefusesNonRegularInput(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	fifo := filepath.Join(root, "pipe")
	require.NoError(t, unix.Mkfifo(fifo, 0o600))
	for _, path := range []string{root, fifo} {
		file, err := openLoadFile(path)
		if file != nil {
			require.NoError(t, file.Close())
		}
		require.Error(t, err)
	}
}
