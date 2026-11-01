package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestReloadRefusesAChainItsAutoLoadedPluginRefuses verifies that a reload
// which starts a plugin whose chain reference was pending, and whose Stage 1
// declaration the filter judge refuses for that chain, is refused as a whole:
// the previous config stays in force and ze keeps running.
//
// Method: a running daemon (startup complete) reloads a tree that adds the
// config root of a registered plugin. The plugin declares a filter and asks
// that its own failure stop ze; the judge refuses the loaded chain the way the
// BGP judge does (registry.RefuseLoadedChain). The reload answer, the running
// tree, the plugin process and the shutdown hook are then read.
//
// VALIDATES: spec-filter-wire-0-umbrella AC-5 and AC-21 on reload: a config
// whose chain names a filter its plugin refuses is never left applied. The
// reload error carries the judge's refusal, says the running config stays in
// force, and names the fix.
// PREVENTS: a reload that only fails the plugin and so stops ze through the
// plugin's fatal failure policy, when the defect is the config, and an error
// that names no fix.
func TestReloadRefusesAChainItsAutoLoadedPluginRefuses(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })
	previous := registry.SetFilterDeclarationCheck(func(string, []registry.DeclaredFilter) error {
		return registry.RefuseLoadedChain(errors.New(chainRefusedMessage))
	})
	t.Cleanup(func() { registry.SetFilterDeclarationCheck(previous) })

	const name = "chain-refused"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares one filter, and that its own failure stops ze",
		ConfigRoots: []string{"chainroot"},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{
				Filters:       []sdk.FilterDecl{{Name: "g", Direction: rpc.FilterImport}},
				FailurePolicy: rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	reactor := &mockReloadReactor{tree: map[string]any{}}
	serverCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(serverCtx)
	t.Cleanup(s.cancel)
	var stopped atomic.Bool
	s.shutdownFunc = func() { stopped.Store(true) }

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.StartWithContext(s.ctx))
	t.Cleanup(pm.Stop)
	s.procManager.Store(pm)
	spawner := &lifecycleTestSpawner{ctx: s.ctx, pm: pm}
	s.SetProcessSpawner(spawner)
	// A running daemon: the reload comes after startup.
	s.signalStartupComplete()

	err = s.ReloadConfig(context.Background(), map[string]any{"chainroot": map[string]any{"enabled": "true"}})

	require.Error(t, err, "a reload whose chain the new plugin refuses must be refused")
	assert.ErrorIs(t, err, registry.ErrLoadedChainRefused)
	assert.Contains(t, err.Error(), chainRefusedMessage, "the reload error carries the judge's refusal")
	assert.Contains(t, err.Error(), "the reload is refused and the running config stays in force",
		"the reload error says what ze did")
	assert.Contains(t, err.Error(), "correct the chain reference", "the reload error names the fix")
	assert.Contains(t, err.Error(), "then reload again", "the reload error names the next step")
	assert.Nil(t, reactor.setTree, "a refused reload must not update the running config")
	assert.Nil(t, spawner.pm.GetProcess(name), "the plugin the refused reload started must be stopped")
	assert.False(t, stopped.Load(), "a refused chain is the config's defect: ze must keep running")
	assert.NoError(t, s.startupErr, "a reload refusal is not a startup error")
}
