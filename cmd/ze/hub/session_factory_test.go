//go:build ze_ssh

package hub

import (
	"context"
	"io"
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
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
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
		func(_ context.Context, _, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
			return func(string) (*plugin.RenderedResponse, error) {
				dispatched.Store(true)
				return &plugin.RenderedResponse{Output: `{"version":"test"}`}, nil
			}
		},
	)
	factory := buildSessionModelFactory(server, infra.HookParams{}, nil, nil)
	created, createErr := factory(t.Context(), "operator", "192.0.2.10:2222", nil, zessh.SessionRequest{})
	require.NoError(t, createErr)
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
	store, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.WriteFile(configPath, nil, 0o600))

	server, err := zessh.NewServer(zessh.Config{
		HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
	})
	require.NoError(t, err)

	var dispatches atomic.Int32
	var dispatchedInput string
	server.SetExecutorFactory(
		func(_ context.Context, _, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
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
	created, createErr := factory(t.Context(), "operator", "192.0.2.10:2222", nil, zessh.SessionRequest{})
	require.NoError(t, createErr)
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

// A requested edit target is not permission to silently fall back to the
// daemon's own config or to a command-only session.
func TestSessionFactoryRefusesUnavailableEditTarget(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.WriteFile("router.conf", nil, 0o600))
	server, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	factory := buildSessionModelFactory(server, infra.HookParams{Store: store, ConfigPath: "router.conf"}, nil, nil)
	model, err := factory(t.Context(), "operator", "192.0.2.10:2222", nil, zessh.SessionRequest{ConfigName: "missing.conf", Mode: "edit"})
	require.Error(t, err)
	require.Nil(t, model)
	storeless := buildSessionModelFactory(server, infra.HookParams{}, nil, nil)
	model, err = storeless(t.Context(), "operator", "192.0.2.10:2222", nil, zessh.SessionRequest{Mode: "edit"})
	require.Error(t, err)
	require.Nil(t, model)
}

// clientSessionFactory builds the session factory of a daemon whose own config
// is router.conf and whose store also holds the client configs named, so a
// refusal under test is never the one for a config that does not exist.
func clientSessionFactory(t *testing.T, clientConfigs ...string) zessh.SessionModelFactory {
	t.Helper()
	store := newTestStore(t)
	require.NoError(t, store.WriteFile("router.conf", nil, 0o600))
	for _, name := range clientConfigs {
		require.NoError(t, store.WriteFile(name, nil, 0o600))
	}
	server, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	return buildSessionModelFactory(server, infra.HookParams{Store: store, ConfigPath: "router.conf"}, nil, nil)
}

// TestSessionEditorRefusesUndeclaredClient opens the session editor on
// client-ghost.conf, which the store holds, on three daemons: a hub whose
// serving blocks declare another client, a daemon that serves no client, and a
// daemon that has not published what it serves. Each MUST refuse the session
// with an operator error whose reason and fix are the import's.
//
// VALIDATES: spec-fleet-6 AC-20 -- the session editor refuses a client config
// no serving block declares, as the import, the restore and the rollback do.
// PREVENTS: a commit that records a version of a config no client ever fetches.
func TestSessionEditorRefusesUndeclaredClient(t *testing.T) {
	cases := []struct {
		name   string
		target *storagecli.DataRPCTarget
		why    string
		fix    string
	}{
		{
			name:   "hub declares another client",
			target: &storagecli.DataRPCTarget{ServesClient: func(name string) bool { return name == "edge" }},
			why:    "the hub has no client entry named ghost under plugin hub server, so no client would ever fetch it",
			fix:    "ze config edit",
		},
		{
			name:   "daemon serves no client",
			target: &storagecli.DataRPCTarget{},
			why:    "this daemon serves no managed client, so no client would ever fetch it",
			fix:    "ze config edit",
		},
		{
			name:   "daemon has not published what it serves",
			target: nil,
			why:    "cannot tell whether a client would ever fetch it",
			fix:    "ze doctor",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storagecli.InstallDataRPC(tc.target)
			t.Cleanup(func() { storagecli.InstallDataRPC(nil) })
			factory := clientSessionFactory(t, "client-ghost.conf")

			model, err := factory(t.Context(), "operator", "192.0.2.10:2222", nil,
				zessh.SessionRequest{ConfigName: "client-ghost.conf", Mode: "edit"})

			require.Nil(t, model)
			var refusal *opererr.Error
			require.ErrorAs(t, err, &refusal, "the refusal must be an operator error")
			assert.Equal(t, "config edit: client-ghost.conf was not opened", refusal.Problem())
			assert.Contains(t, refusal.Why(), tc.why)
			assert.Equal(t, tc.fix, refusal.Fix())
		})
	}
}

// TestSessionEditorOpensDeclaredClient opens the session editor on
// client-edge.conf on a hub whose serving blocks declare edge. No managed
// client is connected: the predicate the hub installs answers "declared", and
// nothing in the factory asks for a connection. The session MUST open in
// config mode.
//
// VALIDATES: spec-fleet-6 AC-20 survivor and D-2 -- a declared client's config
// opens whether or not that client is connected.
// PREVENTS: the declared-client guard growing into a connected-only rule, which
// would refuse provisioning a client that has never connected.
func TestSessionEditorOpensDeclaredClient(t *testing.T) {
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{ServesClient: func(name string) bool { return name == "edge" }})
	t.Cleanup(func() { storagecli.InstallDataRPC(nil) })
	factory := clientSessionFactory(t, "client-edge.conf")

	created, err := factory(t.Context(), "operator", "192.0.2.10:2222", nil,
		zessh.SessionRequest{ConfigName: "client-edge.conf", Mode: "edit"})

	require.NoError(t, err)
	model, ok := created.(cli.Model)
	require.True(t, ok, "session model type = %T, want cli.Model", created)
	if closer, closes := created.(io.Closer); closes {
		t.Cleanup(func() { _ = closer.Close() })
	}
	assert.Contains(t, model.View().Content, "ze#", "a declared client's config must open in editor config mode")
}

// typeSessionLine types line into a session model, presses Enter, and runs
// the command the key answered with, as the bubbletea program would.
func typeSessionLine(t *testing.T, model cli.Model, line string) cli.Model {
	t.Helper()
	model.SetInput(line)
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		updated, _ = updated.Update(cmd())
	}
	next, ok := updated.(cli.Model)
	require.True(t, ok, "updated session model type = %T, want cli.Model", updated)
	return next
}

// TestSessionFactoryFailsClosedWithoutAuthorizer builds an editor session
// whose connection carries no authorizer, which only an SSH server built
// without one produces. The factory MUST install a gate that denies every
// config-mode mutation, as the SSH executor and the SSH upload do, while
// display keeps answering. Method: the factory's real model over a real
// store; the refusal is read from the view, the candidate from the session's
// change file and the shared draft.
//
// VALIDATES: spec-ssh-editor-authorization AC-11.
// PREVENTS: a missing authorizer opening the editor as if authorization were
// unconfigured.
func TestSessionFactoryFailsClosedWithoutAuthorizer(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.WriteFile("router.conf", []byte(authzSeedConfig), 0o600))
	server, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	factory := buildSessionModelFactory(server, infra.HookParams{Store: store, ConfigPath: "router.conf"}, nil, nil)
	created, err := factory(t.Context(), "operator", "192.0.2.10:2222", nil, zessh.SessionRequest{})
	require.NoError(t, err)
	model, ok := created.(cli.Model)
	require.True(t, ok, "session model type = %T, want cli.Model", created)
	t.Cleanup(func() { _ = model.Close() })

	model = typeSessionLine(t, model, "set system host denied-host")
	assert.Contains(t, model.View().Content, plugin.UnauthorizedMessage+": "+plugin.ConfigAuthSet)
	for _, key := range []string{cli.ChangePath("router.conf", "operator"), cli.DraftPath("router.conf")} {
		if !store.Exists(key) {
			continue
		}
		data, readErr := store.ReadFile(key)
		require.NoError(t, readErr)
		assert.NotContains(t, string(data), "denied-host", "a refused set reached the candidate in %s", key)
	}

	model = typeSessionLine(t, model, "show system")
	assert.Contains(t, model.View().Content, authzSeedHost, "display must keep answering without an authorizer")
}

// TestSessionFactoryRecordsTranscript drives AC-15 and review finding I5: the
// daemon owns a remote session's model, so with `cli { transcript enabled }` it
// records every operational command and its answer into its own
// `$XDG_DATA_HOME/ze/transcripts/`, and the model's Close releases the file.
// The username comes back from the authenticator unsanitized, so it must not
// choose the file's path: `../../x` lands inside the transcript directory.
func TestSessionFactoryRecordsTranscript(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	require.NoError(t, env.Set("ze.cli.transcript", "enabled"))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.cli.transcript", "false")) })

	server, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	server.SetExecutorFactory(func(_ context.Context, _, _ string, _ plugin.Authorizer) zessh.CommandExecutor {
		return func(string) (*plugin.RenderedResponse, error) {
			return &plugin.RenderedResponse{Output: `{"version":"transcribed"}`}, nil
		}
	})
	factory := buildSessionModelFactory(server, infra.HookParams{}, nil, nil)
	created, createErr := factory(t.Context(), "../../x", "192.0.2.10:2222", nil, zessh.SessionRequest{})
	require.NoError(t, createErr)
	model, ok := created.(cli.Model)
	require.True(t, ok, "session model type = %T, want cli.Model", created)

	model.SetInput("show version")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "operational command produced no execution command")
	updated.Update(cmd())
	closer, ok := created.(io.Closer)
	require.True(t, ok, "the session model must close its transcript")
	require.NoError(t, closer.Close())

	transcripts := filepath.Join(dataHome, "ze", "transcripts")
	files, globErr := filepath.Glob(filepath.Join(transcripts, "transcript-*.log"))
	require.NoError(t, globErr)
	require.Len(t, files, 1, "one session writes one transcript")
	assert.NotContains(t, filepath.Base(files[0]), "x", "the username must not name the file")
	content, readErr := os.ReadFile(files[0])
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "# User: ../../x")
	assert.Contains(t, string(content), "# Host: 192.0.2.10:2222")
	assert.Contains(t, string(content), "> show version")
	assert.Contains(t, string(content), `{"version":"transcribed"}`)
	_, statErr := os.Stat(filepath.Join(dataHome, "x"))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "nothing may land outside the transcript directory")
}
