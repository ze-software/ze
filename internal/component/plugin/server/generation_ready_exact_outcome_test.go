// Design: docs/architecture/api/process-protocol.md -- exact replacement restoration outcome.
// Related: failure_policy.go -- an asynchronous supervisor clears historical stop records.
package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestGenerationReadyRestartIgnoresPreviousStopRecord recovers a genuinely
// stopped configured plugin on both transports. The real supervisor is held
// before ClearStopped, so a useful fast restoration cannot borrow an older
// diagnostic row as its result. The same replacement serves a reverse call
// after the supervisor is released.
func TestGenerationReadyRestartIgnoresPreviousStopRecord(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			var server *Server
			t.Cleanup(func() {
				if server != nil {
					server.wg.Wait()
				}
			})
			server, _ = newLifecycleStartupServer(t)
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			const name = "generation-ready-old-stop-test"
			var starts atomic.Uint64
			var calls atomic.Uint64
			posts := make(chan uint64, 2)
			require.NoError(t, server.Dispatcher().Register("ready-old-stop-probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
				calls.Add(1)
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "useful exact replacement runtime"))
			require.NoError(t, registry.Register(registry.Registration{
				Name: name, CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					generation := starts.Add(1)
					if !typed {
						conn = struct{ net.Conn }{conn}
					}
					provider := sdk.NewWithConn(name, conn)
					provider.OnAllPluginsReady(func() error { posts <- generation; return nil })
					provider.OnCurrentSelection(func(work context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
						if _, _, err := provider.DispatchCommand(work, "ready-old-stop-probe"); err != nil {
							return nil, err
						}
						output := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession, Cut: 1}
						for _, key := range input.Keys {
							output.Results = append(output.Results, rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI,
								Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent})
						}
						return output, nil
					})
					declaration := &sdk.Registration{}
					if generation > 1 {
						provider.OnDrain(func(context.Context, rpc.DrainInput) error { return nil })
						declaration.DrainBudget = 1
					}
					if err := provider.Run(server.Context(), declaration); err != nil && server.Context().Err() == nil {
						return 1
					}
					return 0
				},
			}))
			require.NoError(t, server.runPluginPhase([]plugin.PluginConfig{{Name: name, Run: name, Internal: true}}))
			server.Dispatcher().Registry().Freeze()
			server.signalStartupComplete()
			select {
			case generation := <-posts:
				require.Equal(t, uint64(1), generation)
			case <-ctx.Done():
				t.Fatal("original generation did not receive post-startup")
			}
			pm := server.ProcessManager()
			original := pm.GetProcess(name)
			require.NotNil(t, original)
			input := rpc.CurrentSelectionInput{Destination: "192.0.2.80", DestinationSession: 71,
				Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, 7}, Revision: 1}}}
			_, err := original.Conn().SendCurrentSelection(ctx, input)
			require.NoError(t, err, "the original supervisor must actually serve runtime before its retirement")
			require.Equal(t, uint64(1), calls.Load())
			server.rollbackReplacement(original)
			require.NoError(t, original.WaitRuntimeCleanupContext(ctx))
			require.Nil(t, pm.GetProcess(name))
			require.False(t, original.Running())
			pm.RecordStopped(name, process.PluginStop{Cause: process.StopRestartFailed, Detail: "the previous generation stopped"})
			_, recorded := pm.Stopped(name)
			require.True(t, recorded)

			// A replacement with DrainBudget must register its actual caller
			// before superviseProcess reaches ClearStopped. Holding that existing
			// leaf lock keeps the old diagnostic present without blocking the
			// handshake, no-op readiness, or SDK post-startup response reader.
			server.drainMu.Lock()
			locked := true
			t.Cleanup(func() {
				if locked {
					server.drainMu.Unlock()
				}
			})
			response, err := server.Dispatcher().Dispatch(&CommandContext{Server: server}, "request plugin restart "+name)
			require.NoError(t, err)
			require.NotNil(t, response)
			replacement := pm.GetProcess(name)
			require.NotNil(t, replacement)
			require.NotSame(t, original, replacement)
			require.True(t, replacement.Running())
			require.NoError(t, replacement.RequirePeerDeclarations())
			_, recorded = pm.Stopped(name)
			require.True(t, recorded, "the earlier name-level record must remain present at the command's decision")
			select {
			case generation := <-posts:
				require.Equal(t, uint64(2), generation, "the useful replacement already completed its post-startup")
			case <-ctx.Done():
				t.Fatal("fast replacement did not receive useful post-startup before its supervisor ran")
			}
			require.Equal(t, plugin.StatusDone, response.Status, "%+v", response)
			data, ok := response.Data.(plugin.Map)
			require.True(t, ok)
			require.Equal(t, false, data[fieldRestartWasRunning], "the command recovered the stopped configured plugin")
			locked = false
			server.drainMu.Unlock()
			_, err = replacement.Conn().SendCurrentSelection(ctx, input)
			require.NoError(t, err, "the exact successful replacement must serve useful runtime after release")
			require.Equal(t, uint64(2), calls.Load())
			require.False(t, replacement.BeginPostStartup(), "the replacement callback remains single-delivery")
		})
	}
}
