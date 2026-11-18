// Design: docs/architecture/api/process-protocol.md -- one Stage-1 value on both transports.
package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Compiled metadata deliberately declares no fence here. The actual SDK's
// Stage-1 value must provide it, on typed and framed post-startup transports.
func TestReplayFenceDeclarationSDKTransports(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			t.Cleanup(cancel)
			const implementation = "stage-one-replay-fence-only"
			require.NoError(t, registry.Register(registry.Registration{Name: implementation, CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
				if !typed {
					conn = struct{ net.Conn }{conn}
				}
				p := sdk.NewWithConn(implementation, conn)
				defer p.Close() //nolint:errcheck // Run owns the test connection lifetime.
				var declaration sdk.Registration
				if err := json.Unmarshal([]byte(`{"signals-session-ready":true,"fences-live-forwards":true}`), &declaration); err != nil {
					return 1
				}
				if err := p.Run(ctx, &declaration); err != nil && ctx.Err() == nil {
					return 1
				}
				return 0
			}}))
			host, err := NewServer(&ServerConfig{Plugins: []plugin.PluginConfig{{Name: "runtime-fence-alias", Run: implementation, Internal: true}}, ConfigNeedHolds: holdsForEmptyConfig}, nil)
			require.NoError(t, err)
			host.SetProcessSpawner(&lifecycleTestSpawner{ctx: ctx})
			t.Cleanup(host.Stop)
			require.NoError(t, host.StartWithContext(ctx))
			require.NoError(t, host.WaitForStartupComplete(ctx))
			proc := host.ProcessManager().GetProcess("runtime-fence-alias")
			require.NotNil(t, proc)
			require.Equal(t, typed, proc.Conn().HasBridge(), "the intended post-startup transport must actually be active")
			require.True(t, registry.FencesLiveForwards(proc.Name()), "Stage 1, rather than compiled fallback, owns this fence")
		})
	}
}
