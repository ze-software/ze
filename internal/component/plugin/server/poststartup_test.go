package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestMidLifeAutoLoadDeliversPostStartup verifies that a plugin auto-loaded by a
// config reload receives the post-startup callback, so its OnAllPluginsReady
// handler runs and can tell an already-running plugin that it has taken an
// exclusive role over.
//
// VALIDATES: AC-12 -- a claimant joining mid-life reaches the plugin that must
// stand its own default behavior down, so exactly one of the two performs the
// role afterwards.
// PREVENTS: the mid-life join reaching NEITHER channel. The declarative Stage-2
// claim cannot reach a plugin that is already configured, because Stage 2 runs
// once per handshake; and the runtime corrective is dispatched from
// OnAllPluginsReady, which is driven by the post-startup callback that
// signalStartupComplete fans out ONCE, before any reload can run. Before this,
// autoLoadForNewConfigPaths signaled only the reactor, under a comment that
// said it signaled the plugins.
func TestMidLifeAutoLoadDeliversPostStartup(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const holder = "poststartup-role-holder"
	const lateJoiner = "poststartup-late-claimant"
	const claimCommand = "request poststartup claim-role"
	const configRoot = "poststartupjoin"

	var claimed atomic.Int64

	require.NoError(t, registry.Register(registry.Registration{
		Name:        holder,
		Description: "post-startup test role holder",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(holder, conn)
			p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
				if command == claimCommand {
					claimed.Add(1)
				}
				return "done", map[string]any{"claimed": true}, nil
			})
			err := p.Run(context.Background(), sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: claimCommand}},
			})
			if err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        lateJoiner,
		Description: "post-startup test late claimant",
		ConfigRoots: []string{configRoot},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(lateJoiner, conn)
			p.OnAllPluginsReady(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _, err := p.DispatchCommand(ctx, claimCommand)
				return err
			})
			if err := p.Run(context.Background(), sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: holder, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	// Startup settles: registries freeze and the one daemon-wide post-startup
	// fan-out happens here, with the late joiner not yet loaded.
	s.signalStartupComplete()
	require.Equal(t, int64(0), claimed.Load(), "nothing claims the role before the reload")

	started, err := s.autoLoadForNewConfigPaths(
		context.Background(),
		map[string]any{configRoot: map[string]any{}},
		[]string{configRoot},
	)
	require.NoError(t, err)
	require.Equal(t, []string{lateJoiner}, started)

	assert.Eventually(t, func() bool { return claimed.Load() >= 1 }, 5*time.Second, 10*time.Millisecond,
		"the mid-life plugin's OnAllPluginsReady must run and reach the role holder")
}
