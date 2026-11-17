// Design: docs/guide/config-editor.md -- remote file inputs use the configuration directories.

package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	_ "github.com/ze-software/ze/internal/component/config/cli" // Register the production caller-path policy.
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

const loadPathOriginal = `bgp { router-id 192.0.2.1; }`
const loadPathReplacement = `bgp { router-id 192.0.2.99; }`

// loadPathModel uses the public model and production path registration, without
// replacing the registered policy with a test-only directory list.
func loadPathModel(t *testing.T, authority cli.FilesystemAuthority) (cli.Model, *cli.Editor, string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	configDir := filepath.Join(root, "ze")
	require.NoError(t, os.Mkdir(configDir, 0o700))
	t.Setenv("ze.config.dir", configDir)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	current := filepath.Join(configDir, "router.conf")
	require.NoError(t, os.WriteFile(current, []byte(loadPathOriginal), 0o600))
	editor, err := cli.NewLooseFileEditor(nil, current)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, editor.Close()) })
	model, err := cli.NewModel(editor, authority)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	return model, editor, root, configDir
}

func loadPathEnter(t *testing.T, model cli.Model, line string) cli.Model {
	t.Helper()
	model.SetInput(line)
	next, run := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if run != nil {
		next, _ = next.Update(run())
	}
	result, ok := next.(cli.Model)
	require.True(t, ok)
	return result
}

// TestDaemonLoadConfinesFileInputs proves a remote operator cannot import a
// daemon-readable file outside the configuration roots, including via aliases
// and a symlink. Valid input distinguishes a refused read from a parse failure.
func TestDaemonLoadConfinesFileInputs(t *testing.T) {
	for _, prefix := range []string{"", "top ", "top top "} {
		for _, form := range []string{"absolute", "traversal", "symlink", "store-owned"} {
			t.Run(prefix+form, func(t *testing.T) {
				model, editor, root, configDir := loadPathModel(t, cli.FilesystemAuthorityDaemon)
				outside := filepath.Join(root, "outside.conf")
				require.NoError(t, os.WriteFile(outside, []byte(loadPathReplacement), 0o600))
				path := outside
				switch form {
				case "traversal":
					path = configDir + "/../outside.conf"
				case "symlink":
					path = filepath.Join(configDir, "linked.conf")
					require.NoError(t, os.Symlink(outside, path))
				case "store-owned":
					path = filepath.Join(configDir, "database.lock")
					require.NoError(t, os.WriteFile(path, []byte(loadPathReplacement), 0o600))
				case "absolute":
				}
				before := editor.WorkingContent()
				model = loadPathEnter(t, model, prefix+"load file absolute replace "+strconv.Quote(path))
				require.Error(t, model.Error())
				require.Contains(t, model.Error().Error(), "load terminal absolute replace")
				require.Equal(t, before, editor.WorkingContent())
				require.False(t, editor.Dirty())
			})
		}
	}
}

// TestLoadUnknownAuthorityRefusesBeforeReading pins the zero and invalid model
// states: neither grants the daemon's explicitly confined file-read capability.
func TestLoadUnknownAuthorityRefusesBeforeReading(t *testing.T) {
	for _, authority := range []cli.FilesystemAuthority{cli.FilesystemAuthorityUnknown, cli.FilesystemAuthority(255)} {
		t.Run(strconv.Itoa(int(authority)), func(t *testing.T) {
			model, editor, _, configDir := loadPathModel(t, authority)
			path := filepath.Join(configDir, "allowed.conf")
			require.NoError(t, os.WriteFile(path, []byte(loadPathReplacement), 0o600))
			before := editor.WorkingContent()
			model = loadPathEnter(t, model, "top load file absolute merge "+strconv.Quote(path))
			require.Error(t, model.Error())
			require.Equal(t, before, editor.WorkingContent())
		})
	}
}

// TestLoadKeepsAuthorizedFileInputs proves confinement preserves daemon imports
// inside its configuration roots and the local owner's normal OS permissions.
func TestLoadKeepsAuthorizedFileInputs(t *testing.T) {
	for _, authority := range []cli.FilesystemAuthority{cli.FilesystemAuthorityDaemon, cli.FilesystemAuthorityOperatorLocal} {
		t.Run(strconv.Itoa(int(authority)), func(t *testing.T) {
			model, editor, root, configDir := loadPathModel(t, authority)
			dir := configDir
			if authority == cli.FilesystemAuthorityOperatorLocal {
				dir = root
			}
			path := filepath.Join(dir, "allowed.conf")
			require.NoError(t, os.WriteFile(path, []byte(loadPathReplacement), 0o600))
			model = loadPathEnter(t, model, "load file absolute replace "+strconv.Quote(path))
			require.NoError(t, model.Error())
			require.Contains(t, editor.WorkingContent(), "192.0.2.99")
		})
	}
}

// A symlink whose resolved target is still permitted remains a usable import;
// the descriptor opener refuses substitutions after that resolution.
func TestDaemonLoadKeepsInsideSymlink(t *testing.T) {
	model, editor, _, configDir := loadPathModel(t, cli.FilesystemAuthorityDaemon)
	path := filepath.Join(configDir, "allowed.conf")
	require.NoError(t, os.WriteFile(path, []byte(loadPathReplacement), 0o600))
	alias := filepath.Join(configDir, "alias.conf")
	require.NoError(t, os.Symlink(path, alias))
	model = loadPathEnter(t, model, "load file absolute replace "+strconv.Quote(alias))
	require.NoError(t, model.Error())
	require.Contains(t, editor.WorkingContent(), "192.0.2.99")
}

// TestLoadBrowserKeepsSuppliedContent proves a browser's filename stays a label;
// its bytes, rather than an identically named router file, become the candidate.
func TestLoadBrowserKeepsSuppliedContent(t *testing.T) {
	_, editor, root, _ := loadPathModel(t, cli.FilesystemAuthorityUnknown)
	path := filepath.Join(root, "not-on-the-router.conf")
	content := loadPathReplacement
	answer, err := editor.ExecuteConfigCommand(contract.ConfigCommandRequest{
		Line: "load file absolute replace " + strconv.Quote(path), Content: &content,
	})
	require.NoError(t, err)
	require.Empty(t, answer.Input)
	require.Contains(t, editor.WorkingContent(), "192.0.2.99")
}

// TestDaemonLoadBoundsFileBytes keeps a permitted path from turning a small
// command into an unbounded read. Both accepted and refused files contain valid
// configuration, so the boundary cannot pass by refusing all imports.
func TestDaemonLoadBoundsFileBytes(t *testing.T) {
	for _, size := range []int{4 << 20, (4 << 20) + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			model, editor, _, configDir := loadPathModel(t, cli.FilesystemAuthorityDaemon)
			path := filepath.Join(configDir, "large.conf")
			content := loadPathReplacement + strings.Repeat(" ", size-len(loadPathReplacement))
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			before := editor.WorkingContent()
			model = loadPathEnter(t, model, "load file absolute replace "+strconv.Quote(path))
			if size > 4<<20 {
				require.Error(t, model.Error())
				require.Contains(t, model.Error().Error(), "4 MiB")
				require.Equal(t, before, editor.WorkingContent())
				return
			}
			require.NoError(t, model.Error())
			require.Contains(t, editor.WorkingContent(), "192.0.2.99")
		})
	}
}

// TestDaemonLoadRelativeStoredNameUsesConfigDirectory exercises the default
// daemon boot identity, a stored name rather than an absolute OS path. Relative
// inputs belong beside that stored configuration, not in the daemon's cwd.
func TestDaemonLoadRelativeStoredNameUsesConfigDirectory(t *testing.T) {
	_, _, _, configDir := loadPathModel(t, cli.FilesystemAuthorityUnknown)
	store, err := storage.Create(configDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("router.conf", []byte(loadPathOriginal), 0o600))
	editor, err := cli.NewEditorWithStorage(store, "router.conf")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, editor.Close()) })
	model, err := cli.NewModel(editor, cli.FilesystemAuthorityDaemon)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	require.NoError(t, os.Mkdir(filepath.Join(configDir, "topics"), 0o700))
	path := filepath.Join(configDir, "topics", "section.conf")
	require.NoError(t, os.WriteFile(path, []byte(loadPathReplacement), 0o600))
	model = loadPathEnter(t, model, "load file absolute replace topics/section.conf")
	require.NoError(t, model.Error())
	require.Contains(t, editor.WorkingContent(), "192.0.2.99")
}
