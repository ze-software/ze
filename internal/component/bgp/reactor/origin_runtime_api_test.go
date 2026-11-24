// Design: docs/architecture/api/process-protocol.md -- actual SDK origin replay.
// Related: origin_intake_test.go -- real registered process startup harness.
package reactor

import (
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type originRuntimeSDKFixture struct {
	writer *selectedWriterFixture
	host   *pluginserver.Server
	sender plugin.Sender
	ctx    context.Context
	batch  bgptypes.NLRIBatch
	offers chan rpc.OriginIntakeInput
	name   string
}

func newOriginRuntimeSDKFixture(t *testing.T, typed bool) *originRuntimeSDKFixture {
	t.Helper()
	return newOriginRuntimeSDKFixtureWithIntake(t, typed, func() bool { return true })
}

// intake is sampled by each actual provider startup, before SDK registration.
func newOriginRuntimeSDKFixtureWithIntake(t *testing.T, typed bool, intake func() bool) *originRuntimeSDKFixture {
	t.Helper()
	return newOriginRuntimeSDKFixtureWithIntakeSession(t, typed, intake, func() uint64 { return 71 })
}

func newOriginRuntimeSDKFixtureWithIntakeSession(t *testing.T, typed bool, intake func() bool, destinationSession func() uint64) *originRuntimeSDKFixture {
	t.Helper()
	f := newSelectedWriterFixture(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	const name = "origin-runtime-wire-provider"
	declarations := []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
		SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}}
	sends := testAllBGPSendDecls()
	receives := testAllBGPReceiveDecls()
	offers := make(chan rpc.OriginIntakeInput, 4)
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 },
		Sends: sends, Receives: receives, Dissemination: declarations,
		RunEngine: func(conn net.Conn) int {
			if !typed {
				conn = struct{ net.Conn }{conn}
			}
			provider := sdk.NewWithConn(name, conn)
			var mu sync.Mutex
			var revision uint64
			current := make(map[string]uint64)
			forget := func() { mu.Lock(); clear(current); mu.Unlock() }
			provider.SetEventFormat("json")
			provider.OnEvent(func(line string) error {
				event, err := bgp.ParseEvent([]byte(line))
				if err != nil {
					return err
				}
				if event.GetPeerState() == "down" && event.Reason == rpc.ReasonPeerRemoved {
					forget()
				}
				return nil
			})
			provider.OnStructuredEvent(func(batch []any) error {
				for _, raw := range batch {
					event, ok := raw.(*rpc.StructuredEvent)
					if !ok {
						continue
					}
					if event.State == rpc.SessionStateDown && event.Reason == rpc.ReasonPeerRemoved {
						forget()
					}
				}
				return nil
			})
			provider.OnCurrentSelection(func(_ context.Context, in rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				out := &rpc.CurrentSelectionOutput{Destination: in.Destination, DestinationSession: in.DestinationSession, Cut: revision}
				for _, key := range in.Keys {
					result := rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent}
					if now := current[string(key.KeyNLRI)]; now != 0 {
						result.Presence = rpc.CurrentSelectionPresent
						result.CurrentRevision = now
					}
					out.Results = append(out.Results, result)
				}
				return out, nil
			})
			if intake() {
				provider.OnOriginIntake(func(work context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
					mu.Lock()
					revision++
					version := revision
					if in.Operation == rpc.OriginIntakeWithdraw {
						if current[string(in.Route.KeyNLRI)] == 0 {
							mu.Unlock()
							return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
						}
						delete(current, string(in.Route.KeyNLRI))
					} else {
						current[string(in.Route.KeyNLRI)] = version
					}
					mu.Unlock()
					route := rpc.SelectedRoute{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI, Revision: version, Kind: rpc.SelectedRouteAbsent}
					if in.Operation == rpc.OriginIntakeAnnounce {
						route.Kind = rpc.SelectedRouteOrigin
						route.OriginReceipt = in.OriginReceipt
						route.StaleLevel = rpc.SelectedFresh
						route.Route = &rpc.StoredRoute{Family: in.Route.Family, NLRIHex: hex.EncodeToString(in.Route.KeyNLRI), NLRIFraming: rpc.NLRIFramingPrefixOnly}
					}
					answer, err := provider.SelectRoutes(work, rpc.SelectRoutesInput{Destination: in.Destination, DestinationSession: destinationSession(), Routes: []rpc.SelectedRoute{route}})
					if err != nil {
						return nil, err
					}
					if in.Operation == rpc.OriginIntakeWithdraw {
						return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
					}
					out := &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI,
						Destination: in.Destination, PeerOwner: in.PeerOwner, Generation: in.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSelected}}}
					if answer.Results[0].Outcome == rpc.SelectedWritten {
						out.Announced = 1
					} else {
						out.Admissions[0].Disposition = rpc.RouteAdmissionSuppressedExport
					}
					offers <- rpc.CloneOriginIntake(in)
					return out, nil
				})
			}
			if err := provider.Run(ctx, &sdk.Registration{Sends: sends, Receives: receives, Dissemination: declarations}); err != nil && ctx.Err() == nil {
				t.Errorf("actual SDK provider: %v", err)
				return 1
			}
			return 0
		}}))
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Run: name, Internal: true}}}, f.api)
	require.NoError(t, err)
	destBus := testBusSelectedDest(name)
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", testBusSourceLease(name)),
		testDeliveryPeer(f.peer.addrString, "", destBus),
	})
	f.api.r.api = host
	f.api.r.originReceipts, err = newOriginReceiptStore(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.api.r.originReceipts.close(context.Background())) })
	startOriginIntakeTestHost(t, ctx, host)
	f.peer.settings.Bus = destBus
	configured, err := host.ResolveDisseminationOwnerForPeer(testDeliveryPeer(f.peer.addrString, "", destBus), SelectedDisseminationRole)
	require.NoError(t, err)
	f.peer.settings.DisseminationOwner = configured
	f.peer.refreshForwardFacts()
	f.peer.session.egressRouteFilter = f.peer.egressGate(f.peer.session)
	input := selectedOriginBuildInput()
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.7.0/24"), 0, false)},
		NextHop: input.nextHop, Wire: attribute.NewAttributesWire(input.attributes, input.attributeContext)}

	return &originRuntimeSDKFixture{writer: f, host: host, ctx: ctx, batch: batch, offers: offers, name: name, sender: plugin.OperatorSender()}
}

func (f *originRuntimeSDKFixture) announce(t *testing.T) rpc.OriginIntakeInput {
	t.Helper()
	result, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(f.writer.peer.addrString), f.batch, f.sender)
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.Announced)
	require.Len(t, result.Admissions, 1)
	select {
	case input := <-f.offers:
		return input
	case <-f.ctx.Done():
		t.Fatal("actual origin intake did not complete")
		return rpc.OriginIntakeInput{}
	}
}

func originRuntimeExpectReoffer(t *testing.T, f *originRuntimeSDKFixture, first rpc.OriginIntakeInput) {
	t.Helper()
	var second rpc.OriginIntakeInput
	select {
	case second = <-f.offers:
	case <-f.ctx.Done():
		t.Fatal("provider did not replay admitted original")
	}
	require.Greater(t, second.OriginReceipt, first.OriginReceipt)
	require.Equal(t, first.Route, second.Route)
	require.Equal(t, first.Source, second.Source)
	require.Equal(t, first.Producer, second.Producer)
	bodies := updateBodies(t, f.writer.conn.written())
	require.Len(t, bodies, 3)
	prefix := []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}
	require.Equal(t, prefix, updatePrefixes(t, bodies[0], false))
	require.Equal(t, prefix, updatePrefixes(t, bodies[1], true))
	require.Equal(t, prefix, updatePrefixes(t, bodies[2], false))
	path, present := bodyPathAttr(t, bodies[2], 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 65007}, aspathASNs(t, path, 4))
}

func TestOriginRuntimeActualSDKReplaysParkedOriginal(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			first := f.announce(t)
			proc := f.host.ProcessManager().GetProcess(f.name)
			require.NotNil(t, proc)
			owner, err := proc.SelectedOwner()
			require.NoError(t, err)
			require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, owner))
			require.NoError(t, f.writer.api.r.retireSelectedOwner(f.ctx, owner, nil))
			require.NoError(t, originGenerationReady(f.ctx, f.host, proc))
			originRuntimeExpectReoffer(t, f, first)
		})
	}
}

func TestOriginRuntimeActualSDKProcessReplacement(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			first := f.announce(t)
			old := f.host.ProcessManager().GetProcess(f.name)
			require.NotNil(t, old)
			oldOwner, err := old.SelectedOwner()
			require.NoError(t, err)
			response, err := f.host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: f.host}, "request plugin restart "+f.name)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response)
			replacement := f.host.ProcessManager().GetProcess(f.name)
			require.NotNil(t, replacement)
			require.NotSame(t, old, replacement)
			require.Error(t, oldOwner.Current())
			originRuntimeExpectReoffer(t, f, first)
		})
	}
}

func (f *originRuntimeSDKFixture) publishGraph(t *testing.T) *pluginserver.DeliveryGraph {
	t.Helper()
	peers := []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", testBusSourceLease(f.name)),
		deliveryPeerFromSettings(f.writer.peer.settingsSnapshot()),
	}
	endpoints := buildTestEndpointInputsForPeers(peers, f.host, nil)
	require.NoError(t, f.host.UpdateDeliveryGraphWithEndpoints("bgp", peers, endpoints))
	return f.host.DeliveryGraph()
}

func TestOriginRuntimeSourceSendGrantRemovalKeepsUsefulSibling(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			producerOwner, _ := originOwnerFixture(t, "origin-source-grant-test")
			peer := f.writer.peer
			peer.settings.Bus = testBusBlockForGrants(
				testPeerEndpointGrant{Name: f.name, Receive: []string{"state", "refresh", "update-sent"}, Send: []string{"update"}},
				testPeerEndpointGrant{Name: "origin-source-grant-test", Send: []string{"update"}},
			)
			beforeGraph := f.publishGraph(t)
			peer.refreshForwardFacts()
			f.sender = producerOwner.Sender()
			f.batch.OriginOwner = producerOwner
			original := f.announce(t)
			before := peer.settingsSnapshot()
			peer.settings.Bus = testBusSelectedDest(f.name)
			after := peer.settingsSnapshot()
			afterGraph := f.publishGraph(t)
			peer.refreshForwardFacts()
			require.NoError(t, peer.revokeRemovedSourceGrants(f.writer.api.r, before, after, beforeGraph, afterGraph))
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, 2)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[1], true))
			_, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(peer.addrString), f.batch, f.sender)
			require.Error(t, err, "the removed sender cannot mint a fresh contribution")
			// Same provider and destination remain useful for the unrelated operator.
			f.sender = plugin.OperatorSender()
			f.batch.OriginOwner = plugin.SelectedOwner{}
			f.batch.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
			useful := f.announce(t)
			require.Greater(t, useful.OriginReceipt, original.OriginReceipt)
			bodies = updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, 3)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[2], false))
		})
	}
}

func TestOriginRuntimeDestinationGrantRemovalUsesJoinedTerminalDown(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			f.announce(t)
			peer := f.writer.peer
			f.writer.api.r.bindEvents = bgpserver.NewEventDispatcher(f.host)
			before := peer.settingsSnapshot()
			beforeGraph := f.host.DeliveryGraph()
			peer.settings.Bus = testBusNone()
			after := peer.settingsSnapshot()
			peer.refreshForwardFacts()
			// Publish the removal before the barrier, as actual config reconciliation does.
			f.host.UpdateDeliveryGraph("bgp", nil)
			afterGraph := f.host.DeliveryGraph()
			require.NoError(t, peer.revokeRemovedOriginGrants(f.writer.api.r, before, after, beforeGraph, afterGraph))
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, 2)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[1], true))
			// The unavailable destination cannot become a direct-output fallback.
			_, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(peer.addrString), f.batch, plugin.OperatorSender())
			require.Error(t, err)
			healthy, wire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
			healthy.settings.ExportPolicy = peer.settings.ExportPolicy
			healthy.reactor = f.writer.api.r
			f.writer.api.r.mu.Lock()
			f.writer.api.r.peers[healthy.Settings().PeerKey()] = healthy
			f.writer.api.r.mu.Unlock()
			healthy.refreshForwardFacts()
			result, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(healthy.addrString), f.batch, plugin.OperatorSender())
			require.NoError(t, err)
			require.Equal(t, uint32(1), result.Announced)
			require.NotEmpty(t, wire.written())
		})
	}
}

func TestOriginRuntimeReadyRejectsSupersededProcessBeforeReissue(t *testing.T) {
	f := newOriginRuntimeSDKFixture(t, false)
	first := f.announce(t)
	original := f.host.ProcessManager().GetProcess(f.name)
	owner, err := original.SelectedOwner()
	require.NoError(t, err)
	require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, owner))
	manager := f.host.ProcessManager()
	manager.RemoveProcess(f.name)
	successor := process.NewProcess(original.Config())
	require.NoError(t, manager.AddProcess(f.name, successor))
	t.Cleanup(func() {
		original.Stop()
		join, cancel := context.WithTimeout(context.Background(), rpc.RequestCancelTimeout)
		defer cancel()
		require.NoError(t, original.Wait(join))
	})
	t.Cleanup(func() { manager.RemoveProcess(f.name); successor.Stop() })
	require.ErrorContains(t, originGenerationReady(f.ctx, f.host, original), "superseded")
	store := f.writer.api.r.originReceipts
	store.mu.Lock()
	next := store.next
	store.mu.Unlock()
	require.Equal(t, first.OriginReceipt, next, "stale ready must not mint a replacement receipt")
	require.Len(t, updateBodies(t, f.writer.conn.written()), 1)
}

func TestOriginRuntimeSameProviderReboundReoffersAfterJoinedDown(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			first := f.announce(t)
			peer := f.writer.peer
			f.writer.api.r.bindEvents = bgpserver.NewEventDispatcher(f.host)
			require.NoError(t, peer.parkReboundOrigins(f.writer.api.r, []string{f.name}))
			peer.unbindProcesses(f.writer.api.r, []string{f.name})
			peer.refreshForwardFacts()
			require.NoError(t, peer.replayPeerOrigins(f.writer.api.r))
			originRuntimeExpectReoffer(t, f, first)
		})
	}
}

func TestOriginRuntimeCompleteDestinationGrantChangeRequiresRestart(t *testing.T) {
	f := newOriginRuntimeSDKFixture(t, false)
	current := f.writer.peer.Settings()
	next := *current
	next.Bus = testBusBoth(f.name, []string{"state", "refresh", "update"}, []string{"update"})
	require.False(t, onlyReceiveOnlyBusDiffers(current.Bus, next.Bus), "ordinary destination's mandatory send grant makes its bus attachment restart-scoped")
	require.True(t, peerSettingsRestartRequired(current, &next, f.writer.peer.session))
}

func TestOriginRuntimeActualProviderExpiryAndRestartPreserveOnlySuccessor(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			original := f.host.ProcessManager().GetProcess(f.name)
			owner, err := original.SelectedOwner()
			require.NoError(t, err)
			f.sender = owner.Sender()
			f.batch.OriginOwner = owner
			first := f.announce(t)
			oldBatch := f.batch
			original.Stop()
			require.NoError(t, original.WaitRuntimeCleanupContext(f.ctx), "real ended-generation cleanup must park data and retire actual output")
			expired := plugin.WithOriginRetirement(f.ctx, originRuntimeIntakeRetirement(t, owner, first))
			_, err = f.writer.api.WithdrawNLRIBatchResult(expired, selector.ParseDefault(f.writer.peer.addrString), oldBatch, owner.Sender())
			require.Error(t, err, "unavailable captured provider is not a remote cleanup acknowledgment")
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
			response, err := f.host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: f.host}, "request plugin restart "+f.name)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response)
			select {
			case replay := <-f.offers:
				t.Fatalf("expired original replayed as receipt %d", replay.OriginReceipt)
			default:
			}
			require.Len(t, updateBodies(t, f.writer.conn.written()), 2, "actual provider restart must not resurrect expired output")
			replacement := f.host.ProcessManager().GetProcess(f.name)
			require.NotNil(t, replacement)
			require.NotSame(t, original, replacement)
			current, err := replacement.SelectedOwner()
			require.NoError(t, err)
			f.sender = current.Sender()
			f.batch.OriginOwner = current
			next := f.announce(t)
			require.Greater(t, next.OriginReceipt, first.OriginReceipt)
			result, err := f.writer.api.WithdrawNLRIBatchResult(expired, selector.ParseDefault(f.writer.peer.addrString), oldBatch, owner.Sender())
			require.NoError(t, err)
			require.Equal(t, uint32(1), result.Withdrawn)
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 1, 0, 0)
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, 3)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[2], false))
		})
	}
}

func TestOriginRuntimeTrustedRegistryRetiresParkedLiveOriginal(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			proc := f.host.ProcessManager().GetProcess(f.name)
			owner, err := proc.SelectedOwner()
			require.NoError(t, err)
			f.sender = owner.Sender()
			f.batch.OriginOwner = owner
			first := f.announce(t)
			// Park the already accepted original without replacing its live provider
			// (exercising the on-exit keep policy).
			// This is the exact retained record a registry callback must forget.
			_, err = f.writer.api.r.originReceipts.endOwnerWithPolicy(f.ctx, owner, true)
			require.NoError(t, err)
			expired := plugin.WithOriginRetirement(f.ctx, originRuntimeIntakeRetirement(t, owner, first))
			result, err := f.writer.api.WithdrawNLRIBatchResult(expired, selector.ParseDefault(f.writer.peer.addrString), f.batch, f.sender)
			require.NoError(t, err)
			require.Equal(t, uint32(1), result.Withdrawn)
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
			require.NoError(t, originGenerationReady(f.ctx, f.host, proc))
			select {
			case replay := <-f.offers:
				t.Fatalf("retired registry original replayed as %d", replay.OriginReceipt)
			default:
			}
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, 2)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[1], true))
		})
	}
}

func originRuntimeIntakeRetirement(t *testing.T, owner plugin.SelectedOwner, input rpc.OriginIntakeInput) *plugin.OriginRetirement {
	t.Helper()
	sender := owner.Sender()
	if !sender.IsSet() {
		sender = plugin.OperatorSender()
	}
	retirement, err := plugin.CaptureOriginRetirement(sender, owner, &rpc.UpdateRouteOutput{
		Admissions: []rpc.RouteAdmission{{Family: input.Route.Family, KeyNLRI: input.Route.KeyNLRI,
			Destination: input.Destination, PeerOwner: input.PeerOwner, Generation: input.OriginReceipt,
			Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSelected}},
	})
	require.NoError(t, err)
	return retirement
}
