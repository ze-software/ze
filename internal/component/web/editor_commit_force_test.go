package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestWebTerminalCommitForceOverridesConflict is AC-32 of
// spec-session-editor-file-mode-parity at the web terminal.
//
// GOAL: `commit now force` typed in the web terminal applies over another
// user's LIVE conflict, through the same editor code the SSH editor runs.
// METHOD: bob and alice each set a different router-id through the manager;
// alice types `commit now`, then `commit now force`, once against a manager
// with the daemon's commit hook (candidate path) and once without (direct).
//
// VALIDATES: `commit now` reports the conflict naming bob; `commit now force`
// answers success, alice's value is committed, and bob's overridden change is
// gone from his change file.
// PREVENTS: the web terminal refusing force over a conflict, or forcing with
// a copy of the commit code that leaves bob's change pending to be committed
// over alice's.
func TestWebTerminalCommitForceOverridesConflict(t *testing.T) {
	t.Run("commit hook", func(t *testing.T) {
		mgr, schema := newPromotingEditorManager(t, "# ze config\n")
		checkWebCommitForce(t, mgr, schema)
	})
	t.Run("no commit hook", func(t *testing.T) {
		installPeerValidator(t, func(*config.Tree) error { return nil })
		configPath := filepath.Join(t.TempDir(), "test.conf")
		require.NoError(t, os.WriteFile(configPath, []byte("# ze config\n"), 0o600))
		schema, err := config.YANGSchema()
		require.NoError(t, err)
		mgr := NewEditorManager(testConfigStore(t, configPath), configPath, schema,
			validatingEditorFactory(), testEditSessionFactory())
		checkWebCommitForce(t, mgr, schema)
	})
}

func checkWebCommitForce(t *testing.T, mgr *EditorManager, schema *config.Schema) {
	t.Helper()
	require.NoError(t, mgr.SetValue("bob", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "session", "asn"}, "local", "65000"))

	_, output := executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verbCommit, Args: []string{"now"}})
	assert.Contains(t, output, "commit conflicts", "commit now alone is refused")
	assert.Contains(t, output, "bob", "the conflict names the other user")

	_, output = executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verbCommit, Args: []string{"now", "force"}})
	require.Equal(t, terminalOutputCommitSuccessful, output)

	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Contains(t, string(committed), "10.0.0.9", "alice's forced value is committed")

	bobChange, err := mgr.store.ReadFile(cli.ChangePath(mgr.configPath, "bob"))
	if err == nil {
		assert.NotContains(t, string(bobChange), "10.0.0.2", "bob's overridden change is gone")
	}
	notice, err := mgr.store.ReadFile(cli.DiscardNoticePath(mgr.configPath, "bob"))
	require.NoError(t, err, "bob is left a discard notice")
	assert.Contains(t, string(notice), "alice")
}
