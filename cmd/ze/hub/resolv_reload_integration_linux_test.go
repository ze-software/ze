//go:build integration && linux

package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// TestSessionCommitReloadWritesResolvConf is the AC-9 daemon-effect chain on
// a real Linux filesystem: a session editor sets a name-server leaf-list
// member, commits, and the reload-path system effect (applyResolvConf, the
// function doReload runs after every successful config reload) rewrites
// resolv.conf from the committed file — no restart involved.
//
// VALIDATES: AC-9 "set system name-server 8.8.8.8 + commit → resolv.conf
// written with 8.8.8.8 WITHOUT restart" (the linux-only effect; the
// commit→NotifyReload→doReload routing is covered by
// TestSessionEditorHasReloadNotifier and the leaflist-commit-reload.et test).
// PREVENTS: reloads that apply config to daemons but leave resolv.conf stale
// until restart.
func TestSessionCommitReloadWritesResolvConf(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	resolvPath := filepath.Join(dir, "resolv.conf")

	seed := "set system dns resolv-conf-path " + resolvPath + "\n"
	require.NoError(t, os.WriteFile(configPath, []byte(seed), 0o600))

	// SSH-shaped session editor: set one leaf-list member and commit.
	ed, err := cli.NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(cli.NewEditSession("thomas", "ssh"))

	require.NoError(t, ed.SetValue([]string{"system"}, "name-server", "8.8.8.8"))
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	require.Equal(t, 1, result.Applied)

	// Reload-path system effect: parse the committed file the way the
	// daemon's load path does and run the post-reload resolv.conf write.
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	schema, err := zeconfig.YANGSchema()
	require.NoError(t, err)
	content := string(data)
	if idx := strings.IndexByte(content, '\n'); strings.HasPrefix(content, "# ze-schema:") && idx >= 0 {
		content = content[idx+1:]
	}
	tree, _, err := zeconfig.NewSetParser(schema).ParseWithMeta(content)
	require.NoError(t, err)

	applyResolvConf(tree)

	written, err := os.ReadFile(resolvPath)
	require.NoError(t, err, "resolv.conf must be written by the reload effect")
	assert.Contains(t, string(written), "nameserver 8.8.8.8",
		"committed name-server member must reach resolv.conf without restart")
}
