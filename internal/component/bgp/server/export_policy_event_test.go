// Design: docs/architecture/api/process-protocol.md -- selected policy settlement.
package server

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// MUTATION: return DrainEventsApplied instead of SettleEventsApplied. The event
// applies, but this production caller returns before the held output callback.
// This is a delivery/settlement test; role eligibility is supplied by the
// reactor's separately tested authenticated PeerInfo projection.
func TestExportPolicyEventJoinsCapturedOwnerDeferredOutput(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)
			entered, release := make(chan struct{}), make(chan struct{})
			var observed atomic.Int32
			var plugins []plugin.PluginConfig
			const owner, observer = "policy-event-owner", "policy-event-observer"
			receives := []rpc.MessageReceiveDecl{{Message: bgpevents.Namespace + ":" + bgpevents.EventState}}
			for _, name := range []string{owner, observer} {
				require.NoError(t, registry.Register(registry.Registration{Name: name, Receives: receives,
					CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
						if !typed {
							conn = struct{ net.Conn }{conn}
						}
						p := sdk.NewWithConn(name, conn)
						defer p.Close() //nolint:errcheck // Test SDK connection ownership.
						p.SetEventFormat("json")
						p.OnEvent(func(string) error {
							if name == observer {
								observed.Add(1)
							}
							return nil
						})
						p.OnStructuredEvent(func([]any) error {
							if name == observer {
								observed.Add(1)
							}
							return nil
						})
						if name == owner {
							p.OnEventSettlement(func(work context.Context) error {
								close(entered)
								select {
								case <-release:
									return nil
								case <-work.Done():
									return work.Err()
								}
							})
						}
						if err := p.Run(ctx, &sdk.Registration{Receives: receives}); err != nil && ctx.Err() == nil {
							return 1
						}
						return 0
					}}))
				plugins = append(plugins, plugin.PluginConfig{Name: name, CompiledPlugin: name, Run: "ze." + name, Internal: true, Encoder: plugin.EncodingJSON})
			}
			srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: plugins,
				ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
			require.NoError(t, err)
			srv.SetProcessSpawner(&openObserverSpawner{ctx: ctx})
			srv.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{Addr: "192.0.2.185", Path: "/bgp/ip/192.0.2.185"}})
			require.NoError(t, srv.StartWithContext(ctx))
			t.Cleanup(srv.Stop)
			require.NoError(t, srv.WaitForStartupComplete(ctx))
			peer := &plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.185"), SelectedOwner: owner, DestinationSession: 885}
			capturedOwner, err := srv.ProcessManager().GetProcess(owner).SelectedOwner()
			require.NoError(t, err)
			result := make(chan error, 1)
			go func() { result <- NewEventDispatcher(srv).OnPeerExportPolicy(ctx, peer, capturedOwner, false) }()
			select {
			case <-entered:
			case err := <-result:
				t.Fatalf("policy event returned before deferred output entered: %v", err)
			case <-ctx.Done():
				t.Fatal("deferred policy output did not enter", ctx.Err())
			}
			select {
			case err := <-result:
				t.Fatalf("policy event returned while deferred output was held: %v", err)
			default:
			}
			close(release)
			select {
			case err := <-result:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("policy event did not settle released output", ctx.Err())
			}
			require.Zero(t, observed.Load(), "same-encoding state observer must receive no synthetic UP")
		})
	}
}
