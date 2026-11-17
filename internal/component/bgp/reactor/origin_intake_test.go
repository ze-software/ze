// Design: docs/architecture/api/process-protocol.md -- real engine origin offer path.
package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The production PluginManager owns spawning; the server discovers processes
// and joins their real SDK handshake through the manager startup runner, as hub
// startup does. No manager is published directly to bypass the protocol.
func startOriginIntakeTestHost(t *testing.T, ctx context.Context, host *pluginserver.Server) {
	t.Helper()
	manager := pluginmgr.NewManager()
	host.SetProcessSpawner(manager)
	t.Cleanup(func() {
		host.Stop() // Seal publication and join server-selected process cleanup first.
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, manager.StopAll(join))
	})
	require.NoError(t, manager.SetStartupRunner(func(work context.Context) error {
		if err := host.StartWithContext(work); err != nil {
			return err
		}
		return host.WaitForStartupComplete(work)
	}))
	require.NoError(t, manager.StartAll(ctx, nil, nil), "real provider startup must finish before origin admission")
	require.NotNil(t, host.ProcessManager(), "real startup must publish its process owner")
}

// The test deciding plugin owns a retained contribution map; it emits no selected
// route. This proves offer/receipt wiring, not the future RIB election or writer.
func TestOriginIntakeActualBatchEntryPoints(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newSelectedWriterFixture(t, false)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			const name = "origin-intake-provider-test"
			declarations := []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
				SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}}
			started := make(chan struct{})
			accepted := make(map[uint64]rpc.OriginIntakeInput)
			offers := make(chan rpc.OriginIntakeInput, 4)
			require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 }, Dissemination: declarations,
				RunEngine: func(conn net.Conn) int {
					if !typed {
						conn = struct{ net.Conn }{conn}
					}
					decider := sdk.NewWithConn(name, conn)
					decider.OnOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
						if in.Operation == rpc.OriginIntakeWithdraw {
							if _, found := accepted[in.OriginReceipt]; !found {
								t.Error("withdraw lost its exact contribution receipt")
							}
							delete(accepted, in.OriginReceipt)
							return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
						}
						clear(accepted) // This fixture owns one exact source/key/scope.
						accepted[in.OriginReceipt] = rpc.CloneOriginIntake(in)
						offers <- in
						return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI,
							Destination: in.Destination, PeerOwner: in.PeerOwner, Generation: in.OriginReceipt,
							Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}, nil
					})
					decider.OnStarted(func(context.Context) error { close(started); return nil })
					if err := decider.Run(ctx, &sdk.Registration{Dissemination: declarations}); err != nil {
						t.Errorf("deciding plugin: %v", err)
						return 1
					}
					return 0
				}}))
			host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Run: name, Internal: true}}}, nil)
			require.NoError(t, err)
			bindings := []plugin.PeerProcessBinding{{PluginName: name, Explicit: true,
				Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent}, Send: map[string]bool{"update": true}}}
			host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
				{Addr: "192.0.2.79", Bindings: []plugin.PeerProcessBinding{{PluginName: name, Explicit: true, Receive: map[string]events.Direction{"state": events.DirBoth, "update": events.DirReceived}}}},
				{Addr: f.peer.addrString, Bindings: bindings},
			})
			startOriginIntakeTestHost(t, ctx, host)
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("registered provider did not finish startup")
			}
			require.NoError(t, host.WaitForStartupComplete(ctx), "provider must finish its real runtime-ready publication")
			f.api.r.api = host
			f.peer.settings.ProcessBindings = []ProcessBinding{{PluginName: name, Explicit: true, Receive: bindings[0].Receive, Send: bindings[0].Send}}
			configured, err := host.ResolveDisseminationOwnerForPeer(pluginserver.DeliveryPeer{Addr: f.peer.addrString, Bindings: bindings}, SelectedDisseminationRole)
			require.NoError(t, err)
			f.peer.settings.DisseminationOwner = configured
			f.peer.refreshForwardFacts()
			f.api.r.originReceipts, err = newOriginReceiptStore(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, f.api.r.originReceipts.close(context.WithoutCancel(ctx))) })
			original := []byte{64, 1, 1, 0}
			sourceContext, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(false))
			require.NoError(t, err, "actual ingress requires its original registered encoding context")
			batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, true)},
				NextHop: bgptypes.NewNextHopSelf(), Wire: attribute.NewAttributesWire(original, sourceContext)}
			f.conn.reset()
			output, err := f.api.AnnounceNLRIBatchResult(ctx, selector.ParseDefault(f.peer.addrString), batch, plugin.OperatorSender())
			require.NoError(t, err)
			require.True(t, output.Retained())
			require.Zero(t, output.Announced)
			require.Empty(t, f.conn.written(), "ordinary direct output ran before the deciding plugin")
			require.Len(t, accepted, 1)
			<-offers
			value := accepted[output.Admissions[0].Generation]
			require.Equal(t, f.peer.sourceOwner, value.PeerOwner)
			require.Equal(t, uint16(sourceContext), value.Route.SourceContext)
			require.True(t, value.Route.SourceAddPath)
			require.Zero(t, value.Route.SourcePathID)
			require.Equal(t, []byte{64, 1, 1, 0}, value.Route.Attributes)
			firstReceipt := output.Admissions[0].Generation
			store := f.api.r.originReceipts
			store.mu.Lock()
			record := store.receipts[firstReceipt]
			store.mu.Unlock()
			guard, err := store.admit(firstReceipt, record.provider, f.peer, f.peer.selectedOwnerSnapshot(), record.build.key)
			require.NoError(t, err)
			defer guard.complete()
			replaced := make(chan *rpc.UpdateRouteOutput, 1)
			replacementError := make(chan error, 1)
			go func() {
				result, err := f.api.AnnounceNLRIBatchResult(ctx, selector.ParseDefault(f.peer.addrString), batch, plugin.OperatorSender())
				replaced <- result
				replacementError <- err
			}()
			select {
			case <-guard.done():
			case <-ctx.Done():
				t.Fatal("replacement did not revoke its old writer")
			}
			select {
			case <-offers:
				t.Fatal("replacement was offered before old writer completion")
			default:
			}
			select {
			case <-replaced:
				t.Fatal("replacement receipt preceded its old writer barrier")
			default:
			}
			guard.complete()
			output = <-replaced
			require.NoError(t, <-replacementError)
			require.NotEqual(t, firstReceipt, output.Admissions[0].Generation)
			<-offers
			require.Len(t, accepted, 1)
			require.Empty(t, f.conn.written())
			original[3] = 2
			require.Equal(t, byte(0), value.Route.Attributes[3], "provider borrowed caller wire storage")
			output, err = f.api.WithdrawNLRIBatchResult(ctx, selector.ParseDefault(f.peer.addrString), batch, plugin.OperatorSender())
			require.NoError(t, err)
			require.Equal(t, uint32(1), output.Withdrawn)
			require.Empty(t, accepted)
			require.Empty(t, f.api.r.originReceipts.current)
			require.Empty(t, f.conn.written(), "loser withdrawal erased another origin's output")
		})
	}
}

func TestOriginIntakeCapturePreservesAllBuilderContributions(t *testing.T) {
	builder := attribute.NewBuilder().SetOrigin(0).SetLocalPref(147).SetMED(29).SetASPath([]uint32{70000, 80000}).SetAtomicAggregate()
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, Attrs: builder, NextHop: bgptypes.NewNextHopSelf(), OriginAS: 64512}
	value, err := captureOriginBuild(batch, nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, true))
	require.NoError(t, err)
	require.Equal(t, builder.Build(), value.attributes)
	require.Equal(t, bgpctx.APIContextID, value.attributeContext)
	require.Equal(t, uint32(64512), value.originAS)
	require.True(t, value.hasPathID)
	require.Zero(t, value.pathID)
}

func TestOriginIntakeMaintenanceMarkersStayOnExistingRails(t *testing.T) {
	base := bgptypes.NLRIBatch{}
	require.True(t, originBatchEligible(base))
	for _, change := range []func(*bgptypes.NLRIBatch){
		func(b *bgptypes.NLRIBatch) { b.SentOwnerMessage = 1 }, func(b *bgptypes.NLRIBatch) { b.InitialReplay = 1 },
		func(b *bgptypes.NLRIBatch) { b.SourcePolicyEpoch = 1 }, func(b *bgptypes.NLRIBatch) { b.RecoverySource = netip.MustParseAddr("192.0.2.1") },
		func(b *bgptypes.NLRIBatch) { b.RestoreConfigured = true }, func(b *bgptypes.NLRIBatch) { b.Stale = 1 }, func(b *bgptypes.NLRIBatch) { b.Replay = true },
	} {
		batch := base
		change(&batch)
		require.False(t, originBatchEligible(batch))
	}
}

func TestOriginIntakeNoProviderKeepsDirectOrigination(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, false)}, NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.9"))}
	output, err := f.api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(f.peer.addrString), batch, plugin.OperatorSender())
	require.NoError(t, err)
	require.Equal(t, uint32(1), output.Announced)
	require.Empty(t, output.Admissions)
	require.NotEmpty(t, f.conn.written())
	require.Nil(t, f.api.r.originReceipts, "direct origination does not create an origin domain")
}
