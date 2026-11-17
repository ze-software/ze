// Design: docs/architecture/api/process-protocol.md -- configured origin-owner gaps.
package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Each fixture completes a real SDK/server startup. The optional handler is the
// only difference between a declared intake provider and a known legacy owner.
func newOriginOwnerGapFixture(t *testing.T, intake bool) (*selectedWriterFixture, string, *atomic.Int32) {
	t.Helper()
	f := newSelectedWriterFixture(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	const name = "origin-owner-gap-provider-test"
	declarations := registry.Lookup(selectedWriterProcess).Dissemination
	callbacks := new(atomic.Int32)
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 }, Dissemination: declarations,
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if intake {
				p.OnOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
					callbacks.Add(1)
					return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI,
						Destination: in.Destination, PeerOwner: in.PeerOwner, Generation: in.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}, nil
				})
			}
			if err := p.Run(ctx, &sdk.Registration{Dissemination: declarations}); err != nil {
				t.Errorf("owner fixture SDK: %v", err)
				return 1
			}
			return 0
		}}))
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Run: name, Internal: true}}}, nil)
	require.NoError(t, err)
	t.Cleanup(cancel)
	bindings := []plugin.PeerProcessBinding{{PluginName: name, Explicit: true,
		Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent}, Send: map[string]bool{"update": true}}}
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		{Addr: "192.0.2.79", Bindings: []plugin.PeerProcessBinding{{PluginName: name, Explicit: true, Receive: map[string]events.Direction{"state": events.DirBoth, "update": events.DirReceived}}}},
		{Addr: f.peer.addrString, Bindings: bindings},
	})
	startOriginIntakeTestHost(t, ctx, host)
	require.NoError(t, host.WaitForStartupComplete(ctx))
	original := host.ProcessManager().GetProcess(name)
	require.NotNil(t, original)
	t.Cleanup(func() {
		cancel()
		original.Stop() // Respawn/RemoveProcess may deliberately detach this exact Process.
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, original.Wait(join))
	})
	f.api.r.api = host
	f.peer.settings.ProcessBindings = []ProcessBinding{{PluginName: name, Explicit: true, Receive: bindings[0].Receive, Send: bindings[0].Send}}
	owner, err := host.ResolveDisseminationOwnerForPeer(pluginserver.DeliveryPeer{Addr: f.peer.addrString, Bindings: bindings}, SelectedDisseminationRole)
	require.NoError(t, err)
	f.peer.settings.DisseminationOwner = owner
	f.peer.refreshForwardFacts()
	if intake {
		f.api.r.originReceipts, err = newOriginReceiptStore(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			join, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			require.NoError(t, f.api.r.originReceipts.close(join))
		})
	}
	return f, name, callbacks
}

func originGapBatch() bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, false)}, NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.9"))}
}

// Original B passes a configured owner through direct output when Respawn has
// removed it or installed a not-yet-declared replacement. This root reaches
// actual Announce and checks both the refusal and an unrelated useful wire rail.
func TestOriginIntakeConfiguredOwnerGapRefusesDirectOutput(t *testing.T) {
	for _, gap := range []string{"removed", "pre-stage1"} {
		t.Run(gap, func(t *testing.T) {
			f, name, callbacks := newOriginOwnerGapFixture(t, true)
			batch := originGapBatch()
			selected := selector.ParseDefault(f.peer.addrString)
			first, err := f.api.AnnounceNLRIBatchResult(t.Context(), selected, batch, plugin.OperatorSender())
			require.NoError(t, err)
			require.True(t, first.Retained())
			require.Zero(t, first.Announced)
			require.Equal(t, int32(1), callbacks.Load())
			require.Empty(t, f.conn.written())
			manager := f.api.r.api.ProcessManager()
			original := manager.GetProcess(name)
			require.NotNil(t, original)
			manager.RemoveProcess(name)
			if gap == "pre-stage1" {
				replacement := process.NewProcess(original.Config())
				require.NoError(t, manager.AddProcess(name, replacement))
				t.Cleanup(func() { manager.RemoveProcess(name); replacement.Stop() })
			}
			f.conn.reset()
			before := f.api.r.originReceipts.next
			result, err := f.api.AnnounceNLRIBatchResult(t.Context(), selected, batch, plugin.OperatorSender())
			require.Error(t, err, "unavailable configured owner must not become direct origination")
			require.NotNil(t, result)
			require.Zero(t, result.Announced)
			require.Empty(t, result.Admissions)
			require.NotEmpty(t, result.Failure)
			require.Empty(t, f.conn.written(), "no direct UPDATE may bypass the absent declaring owner")
			require.Equal(t, int32(1), callbacks.Load(), "no callback was available during the gap")
			require.Equal(t, before, f.api.r.originReceipts.next, "unavailable owner issued no candidate receipt")

			healthy, wire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
			healthy.settings.ExportPolicy = f.peer.settings.ExportPolicy
			healthy.reactor = f.api.r
			f.api.r.mu.Lock()
			f.api.r.peers[healthy.Settings().PeerKey()] = healthy
			f.api.r.mu.Unlock()
			healthy.refreshForwardFacts()
			control, err := f.api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(healthy.addrString), batch, plugin.OperatorSender())
			require.NoError(t, err)
			require.Equal(t, uint32(1), control.Announced)
			require.Empty(t, control.Admissions)
			require.NotEmpty(t, wire.written(), "an unrelated no-owner peer still has useful direct output")
			require.Empty(t, f.conn.written(), "healthy control must not send to the unavailable owner scope")
		})
	}
}

func TestOriginIntakeKnownLiveLegacyOwnerKeepsDirectOutput(t *testing.T) {
	f, _, callbacks := newOriginOwnerGapFixture(t, false)
	output, err := f.api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(f.peer.addrString), originGapBatch(), plugin.OperatorSender())
	require.NoError(t, err)
	require.Equal(t, uint32(1), output.Announced)
	require.Empty(t, output.Admissions)
	require.NotEmpty(t, f.conn.written())
	require.Zero(t, callbacks.Load())
	require.Nil(t, f.api.r.originReceipts, "known legacy support does not instantiate origin authority")
}
