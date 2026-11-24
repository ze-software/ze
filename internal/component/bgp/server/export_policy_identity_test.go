// Design: docs/architecture/api/process-protocol.md -- exact policy event incarnation.
// Related: export_policy_event_test.go -- deferred final-output settlement.
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
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Capture an old live owner, then replace the manager and subscription target
// with an actual ready SDK Process bearing that same configured name. The old
// owner must neither send the event to its successor nor acknowledge success.
// The successor's own token must remain useful on the same event rails.
// MUTATION: omit the SameIncarnation check in policyEventProcess; name-only
// discovery then delivers and acknowledges the stale request on the successor.
func TestExportPolicyEventRejectsSameNameReplacement(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			t.Cleanup(cancel)
			const name = "policy-same-name"
			const address = "192.0.2.186"
			var oldCalls, newCalls atomic.Int32
			receives := []rpc.MessageReceiveDecl{{Message: bgpevents.Namespace + ":" + bgpevents.EventState}}
			start := func(implementation string, calls *atomic.Int32) *pluginserver.Server {
				require.NoError(t, registry.Register(registry.Registration{Name: implementation, Receives: receives, CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
					if !typed {
						conn = struct{ net.Conn }{conn}
					}
					p := sdk.NewWithConn(implementation, conn)
					defer p.Close() //nolint:errcheck // The owning server stops and joins this SDK.
					p.SetEventFormat("json")
					p.OnEvent(func(string) error { calls.Add(1); return nil })
					p.OnStructuredEvent(func(batch []any) error { calls.Add(int32(len(batch))); return nil })
					p.OnEventSettlement(func(context.Context) error { return nil })
					if err := p.Run(ctx, &sdk.Registration{Receives: receives}); err != nil && ctx.Err() == nil {
						return 1
					}
					return 0
				}}))
				host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, CompiledPlugin: implementation, Run: implementation, Internal: true, Encoder: plugin.EncodingJSON}}, ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
				require.NoError(t, err)
				host.SetProcessSpawner(&openObserverSpawner{ctx: ctx})
				host.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{Addr: address, Path: "/bgp/ip/" + address}})
				t.Cleanup(host.Stop)
				require.NoError(t, host.StartWithContext(ctx))
				require.NoError(t, host.WaitForStartupComplete(ctx))
				return host
			}
			originalHost := start("policy-old-implementation", &oldCalls)
			original := originalHost.ProcessManager().GetProcess(name)
			require.NotNil(t, original)
			captured, err := original.SelectedOwner()
			require.NoError(t, err)
			replacementHost := start("policy-new-implementation", &newCalls)
			replacement := replacementHost.ProcessManager().GetProcess(name)
			require.NotNil(t, replacement)
			successor, err := replacement.SelectedOwner()
			require.NoError(t, err)
			require.False(t, captured.SameIncarnation(successor))
			// Restore runtime cleanup ownership before the two owning servers
			// stop. The superseded original stays live through the assertion,
			// so a canceled token cannot accidentally prove this question.
			t.Cleanup(func() { require.NoError(t, originalHost.ProcessManager().AddProcess(name, original)) })
			require.NoError(t, originalHost.ProcessManager().AddProcess(name, replacement))
			originalHost.Subscriptions().Add(replacement, &pluginserver.Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth})
			require.NoError(t, captured.Current(), "the old process remains alive across the exact name-rebind cut")
			peer := &plugin.PeerInfo{Address: netip.MustParseAddr(address), SelectedOwner: name, DestinationSession: 886}
			dispatcher := NewEventDispatcher(originalHost)
			require.ErrorContains(t, dispatcher.OnPeerExportPolicy(ctx, peer, captured, false), "incarnation changed")
			require.Zero(t, oldCalls.Load(), "stale reoffer must not reach its old live process")
			require.Zero(t, newCalls.Load(), "old token must not reoffer through the successor")
			require.NoError(t, dispatcher.OnPeerExportPolicy(ctx, peer, successor, false))
			require.Equal(t, int32(1), newCalls.Load(), "the same ready successor must remain useful with its own token")
			// The exact owner alone is no state grant. Narrowing the peer via
			// send > to none refuses it, while the separate healthy host remains useful.
			originalHost.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{
				Addr: address,
				Path: "/bgp/ip/" + address,
				Bus:  plugin.BusBlock{Send: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true}},
			}})
			require.ErrorContains(t, dispatcher.OnPeerExportPolicy(ctx, peer, successor, false), "granted destination state")
			require.Equal(t, int32(1), newCalls.Load())
			require.NoError(t, NewEventDispatcher(replacementHost).OnPeerExportPolicy(ctx, peer, successor, false))
			require.Equal(t, int32(2), newCalls.Load())
			require.Zero(t, oldCalls.Load())
		})
	}
}
