//go:build ze_ssh

package hub

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestMergePluginCommandsNilSafe verifies the SSH per-session merge is a no-op
// (never panics, never mutates) when the dispatcher is not yet reachable. This
// is the early-startup race: a session factory is built before the reactor wires
// the API server, so params.APIServer / the server may be nil on the first tab.
//
// VALIDATES: R-2 -- completion degrades gracefully during the startup window.
// PREVENTS: a nil-deref crash on the SSH session goroutine at daemon start.
func TestMergePluginCommandsNilSafe(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{}}

	mergePluginCommands(tree, infra.HookParams{}) // APIServer field nil
	mergePluginCommands(tree, infra.HookParams{   // APIServer returns nil server
		APIServer: func() *pluginserver.Server { return nil },
	})

	if len(tree.Children) != 0 {
		t.Errorf("nil command source must not add nodes, got %v", tree.Children)
	}
}

// TestDashboardFactoryUsesPublicSummaryCommand verifies that SSH dashboard
// polling uses the registered CLI path rather than the internal RPC nickname.
//
// VALIDATES: monitor bgp reaches the live BGP summary over an SSH session.
// PREVENTS: a healthy dashboard rendering an empty peer table.
func TestDashboardFactoryUsesPublicSummaryCommand(t *testing.T) {
	var command string
	factory := dashboardFactoryFromExecutor(func(input string) (*plugin.RenderedResponse, error) {
		command = input
		return &plugin.RenderedResponse{Output: `{"peers-configured":3}`}, nil
	})

	poller, err := factory()
	require.NoError(t, err)
	output, err := poller()
	require.NoError(t, err)

	assert.Equal(t, "show bgp", command)
	assert.JSONEq(t, `{"peers-configured":3}`, output)
}

// TestSessionFactoryModelRefusesSaveBeforeDispatch checks the hub authority
// boundary. buildSessionModelFactory creates the daemon-hosted model. The model
// refuses save and does not call the daemon executor.
//
// VALIDATES: IR2-1 -- the production SSH PTY factory selects remote authority.
// PREVENTS: a safe generic Model existing while the hub constructs it as local.
func TestSessionFactoryModelRefusesSaveBeforeDispatch(t *testing.T) {
	server, err := zessh.NewServer(zessh.Config{
		HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
	})
	require.NoError(t, err)

	var dispatched atomic.Bool
	server.SetExecutorFactory(
		func(_, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
			return func(string) (*plugin.RenderedResponse, error) {
				dispatched.Store(true)
				return &plugin.RenderedResponse{Output: `{"version":"test"}`}, nil
			}
		},
	)
	factory := buildSessionModelFactory(server, infra.HookParams{}, nil, nil)
	created := factory("operator", "192.0.2.10:2222", nil)
	model, ok := created.(cli.Model)
	require.True(t, ok, "session model type = %T, want cli.Model", created)

	path := filepath.Join(t.TempDir(), "hub-ssh-save.json")
	var input textbuf.Buffer
	model.SetInput(input.Str("show version | json compact | save ").Str(path).String())
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "operational command produced no execution command")
	updated, _ = updated.Update(cmd())
	model, ok = updated.(cli.Model)
	require.True(t, ok, "updated session model type = %T, want cli.Model", updated)

	output := model.View().Content
	assert.Contains(t, output, "save")
	assert.Contains(t, output, "refused")
	assert.False(t, dispatched.Load(), "a refused hub PTY save must not reach the daemon dispatcher")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestSessionFactoryEditorModelRefusesSaveBeforeDispatch reaches the
// storage-backed NewModel branch, rather than the command-model fallback. The
// first run proves editor-mode mutation of the `run` command, then save must be
// refused without a second executor call.
func TestSessionFactoryEditorModelRefusesSaveBeforeDispatch(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, nil, 0o600))
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	server, err := zessh.NewServer(zessh.Config{
		HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
	})
	require.NoError(t, err)

	var dispatches atomic.Int32
	var dispatchedInput string
	server.SetExecutorFactory(
		func(_, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
			return func(input string) (*plugin.RenderedResponse, error) {
				dispatchedInput = input
				dispatches.Add(1)
				return &plugin.RenderedResponse{Output: `{"version":"test"}`}, nil
			}
		},
	)
	factory := buildSessionModelFactory(server, infra.HookParams{
		ConfigPath: configPath,
		Store:      store,
	}, nil, nil)
	created := factory("operator", "192.0.2.10:2222", nil)
	model, ok := created.(cli.Model)
	require.True(t, ok, "session model type = %T, want cli.Model", created)
	assert.Contains(t, model.View().Content, "ze#", "storage-backed sessions must start in editor config mode")

	model.SetInput("run show version")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	updated, _ = updated.Update(cmd())
	model, ok = updated.(cli.Model)
	require.True(t, ok)
	require.Equal(t, int32(1), dispatches.Load())
	assert.Equal(t, "show version", dispatchedInput, "editor run prefix was not removed")

	path := filepath.Join(dir, "editor-ssh-save.json")
	var input textbuf.Buffer
	model.SetInput(input.Str("run show version | json compact | save ").Str(path).String())
	updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "refused editor run produced no result command")
	updated, _ = updated.Update(cmd())
	model, ok = updated.(cli.Model)
	require.True(t, ok)

	output := model.View().Content
	assert.Contains(t, output, "save")
	assert.Contains(t, output, "refused")
	assert.Equal(t, int32(1), dispatches.Load(), "a refused editor PTY save reached the executor")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
