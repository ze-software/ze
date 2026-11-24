// Design: docs/architecture/api/process-protocol.md -- exact accepted timer lineage.
// Related: origin_runtime_api_test.go -- actual typed/framed provider and wire fixture.
package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgpannounce "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/announce"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originRegistryAnnounce(t *testing.T, f *originRuntimeSDKFixture, batch bgptypes.NLRIBatch, owner plugin.SelectedOwner) (*rpc.UpdateRouteOutput, bgptypes.NLRIBatch) {
	t.Helper()
	result, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(f.writer.peer.addrString), batch, f.sender)
	require.NoError(t, err)
	require.True(t, result.Retained())
	require.Equal(t, uint32(1), result.Announced)
	require.Len(t, result.Admissions, 1)
	select {
	case <-f.offers:
	case <-f.ctx.Done():
		t.Fatal("real provider did not acknowledge the original")
	}
	retirement, err := plugin.CaptureOriginRetirement(f.sender, owner, result)
	require.NoError(t, err)
	batch.OriginRetirement = retirement
	return result, batch
}

// Arm the REAL Registry timer only after the prerequisite lifecycle state is
// reached. Registration is a pure accepted-value copy, so no sleep or race with
// provider Stop is needed. The companion announce-package test drives the real
// handler's capture and getOrInitRegistry callback; this fixture adds the real
// issuer/SDK/final wire and actual expiry scheduler to that same value path.
func expireOriginRegistryEntry(t *testing.T, f *originRuntimeSDKFixture, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	t.Helper()
	completed := make(chan error, 1)
	registry := bgpannounce.NewRegistry(func(sel *selector.Selector, original bgptypes.NLRIBatch, captured plugin.Sender) error {
		err := f.writer.api.WithdrawNLRIBatch(plugin.WithOriginRetirement(f.ctx, original.OriginRetirement), sel, original, captured)
		completed <- err
		return err
	})
	_, err := registry.Announce("origin", "expiry", selector.ParseDefault(f.writer.peer.addrString), batch, "registry-test", sender, time.Nanosecond)
	require.NoError(t, err)
	select {
	case err := <-completed:
		require.Zero(t, registry.Len(), "the real timer must remove its accepted entry")
		return err
	case <-f.ctx.Done():
		t.Fatal("real registry expiry failed to settle")
		return f.ctx.Err()
	}
}

// An operator has no Process token. Actual timer expiry while the provider is
// down must still remove only its accepted original before real restart replay.
// MUTATION: rejecting the genuine operator retirement context leaves expired local data replayable.
func TestOriginRuntimeOperatorRegistryExpiryCannotResurrect(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			first, tracked := originRegistryAnnounce(t, f, f.batch, plugin.SelectedOwner{})
			other := f.batch
			other.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
			_, _ = originRegistryAnnounce(t, f, other, plugin.SelectedOwner{})
			require.Len(t, updateBodies(t, f.writer.conn.written()), 2, "both originals must have useful wire output")
			provider := f.host.ProcessManager().GetProcess(f.name)
			require.NotNil(t, provider)
			provider.Stop()
			require.NoError(t, provider.WaitRuntimeCleanupContext(f.ctx))
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 2, 0, 0, 0)
			require.Error(t, expireOriginRegistryEntry(t, f, tracked, f.sender), "unavailable remote cleanup is not acknowledged")
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 0, 0, 0)
			beforeRestart := len(updateBodies(t, f.writer.conn.written()))
			response, err := f.host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: f.host}, "request plugin restart "+f.name)
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response)
			var surviving rpc.OriginIntakeInput
			select {
			case surviving = <-f.offers:
			case <-f.ctx.Done():
				t.Fatal("unexpired sibling did not replay")
			}
			require.Equal(t, []byte{24, 198, 18, 9}, surviving.Route.KeyNLRI)
			select {
			case replay := <-f.offers:
				t.Fatalf("expired operator original replayed: %+v", replay)
			default:
			}
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, beforeRestart+1)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[len(bodies)-1], false))
			fresh, _ := originRegistryAnnounce(t, f, f.batch, plugin.SelectedOwner{})
			require.Greater(t, fresh.Admissions[0].Generation, first.Admissions[0].Generation)
			beforeStale := len(f.writer.conn.written())
			require.NoError(t, expireOriginRegistryEntry(t, f, tracked, f.sender))
			require.Len(t, f.writer.conn.written(), beforeStale)
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 2, 2, 0, 0)
		})
	}
}

// A timed A and untagged B from the SAME Process are distinct accepted inputs.
// After producer exit/reoffer, A's timer is harmless while B's original timer
// still removes B across its changed provider receipt. An unrelated key survives.
// MUTATION: ignoring accepted lineage lets old A retire same-Process replacement B.
func TestOriginRuntimeRegistryTimerMatchesAcceptedLineage(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			producer, stop := originOwnerFixture(t, "origin-registry-source")
			peer := f.writer.peer
			peer.settings.Bus = testBusBlockForGrants(
				testPeerEndpointGrant{Name: f.name, Receive: []string{"state", "refresh", "update-sent"}, Send: []string{"update"}},
				testPeerEndpointGrant{Name: "origin-registry-source", Send: []string{"update"}},
			)
			f.publishGraph(t)
			peer.refreshForwardFacts()
			f.sender, f.batch.OriginOwner = producer.Sender(), producer
			first, timedA := originRegistryAnnounce(t, f, f.batch, producer)
			secondBatch := f.batch
			secondBatch.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
			second, timedB := originRegistryAnnounce(t, f, secondBatch, producer)
			require.Greater(t, second.Admissions[0].Generation, first.Admissions[0].Generation)
			other := f.batch
			other.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
			_, _ = originRegistryAnnounce(t, f, other, producer)
			stop()
			require.Error(t, producer.Current())
			require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, producer, true))
			for range 2 {
				select {
				case <-f.offers:
				case <-f.ctx.Done():
					t.Fatal("kept original did not complete its real reoffer")
				}
			}
			beforeStale := len(f.writer.conn.written())
			require.NoError(t, expireOriginRegistryEntry(t, f, timedA, f.sender))
			require.Len(t, f.writer.conn.written(), beforeStale, "A's old timer must neither withdraw nor resend B")
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 2, 2, 0, 0)
			require.NoError(t, expireOriginRegistryEntry(t, f, timedB, f.sender))
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 1, 0, 0)
			bodies := updateBodies(t, f.writer.conn.written())
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[len(bodies)-1], true))
			select {
			case replay := <-f.offers:
				t.Fatalf("retirement unexpectedly reannounced an original: %+v", replay)
			default:
			}
		})
	}
}

// TestOriginRuntimeParticipantExitRetiresBeforeConfiguredRestore verifies that
// EndOriginOwnerWithPolicy(ctx, owner, false) retires the exited participant's
// selected origin and restores the peer's configured static route while leaving
// an independent origin intact and replayable (E-2, E-6).
//
// VALIDATES: per-participant exit withdrawal restores configured static routes
// without disturbing another participant's surviving origin.
// PREVENTS: participant exit dropping a configured static route or replaying a
// retired participant's origin on provider restart.
func TestOriginRuntimeParticipantExitRetiresBeforeConfiguredRestore(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			addTestPeerBusGrant(f.writer.peer.settings, testPeerEndpointGrant{
				Name: "child",
				Send: []string{"update"},
			})
			deliveryPeers := []pluginserver.DeliveryPeer{
				testDeliveryPeer("192.0.2.79", "", testBusSourceLease(f.name)),
				testDeliveryPeer(f.writer.peer.addrString, "", f.writer.peer.settings.Bus),
			}
			require.NoError(t, f.host.UpdateDeliveryGraphWithEndpoints(
				bgpevents.Namespace,
				deliveryPeers,
				buildTestEndpointInputsForPeers(deliveryPeers, f.host, []pluginserver.BusParticipantInput{
					testBusEndpointAll(f.name),
					testBusEndpointAll("child"),
				}),
			))
			f.writer.peer.refreshForwardFacts()

			proc := f.host.ProcessManager().GetProcess(f.name)
			providerOwner, err := proc.SelectedOwner()
			require.NoError(t, err)
			childOwner, stopChild := originOwnerFixture(t, "child")

			peer := f.writer.peer
			configured := staticRouteAt("198.18.7.0/24", "192.0.2.252")
			peer.settings.StaticRoutes = []StaticRoute{configured}
			installed := peer.sendStaticRoutes(peer.session, peer.settings.StaticRoutes, false, 4096, false)
			require.Len(t, installed, 1)
			peer.staticWire = staticWireSet{session: peer.session, routes: installed}

			f.sender, f.batch.OriginOwner = childOwner.Sender(), childOwner
			f.announce(t)

			f.sender = providerOwner.Sender()
			other := f.batch
			other.OriginOwner = providerOwner
			other.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
			_, _ = originRegistryAnnounce(t, f, other, providerOwner)

			before := len(updateBodies(t, f.writer.conn.written()))
			stopChild()
			require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, childOwner, false))
			require.NoError(t, providerOwner.Current(), "the child exit must not stop the RIB provider")
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 1, 0, 0)
			bodies := updateBodies(t, f.writer.conn.written())
			require.Len(t, bodies, before+2, "selected removal then one configured give-back must settle")
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[before], true))
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[before+1], false))
			hop, present := bodyPathAttr(t, bodies[before+1], 3)
			require.True(t, present)
			require.Equal(t, []byte{192, 0, 2, 252}, hop)

			require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, providerOwner, true))
			require.NoError(t, originGenerationReady(f.ctx, f.host, proc))
			var surviving rpc.OriginIntakeInput
			select {
			case surviving = <-f.offers:
			case <-f.ctx.Done():
				t.Fatal("independent original did not remain replayable")
			}
			require.Equal(t, []byte{24, 198, 18, 9}, surviving.Route.KeyNLRI)
			select {
			case replay := <-f.offers:
				t.Fatalf("withdrawn child original replayed: %+v", replay)
			default:
			}
			afterReplay := updateBodies(t, f.writer.conn.written())
			for _, body := range afterReplay[len(bodies):] {
				require.NotContains(t, updatePrefixes(t, body, false), netip.MustParsePrefix("198.18.7.0/24"))
				require.NotContains(t, updatePrefixes(t, body, true), netip.MustParsePrefix("198.18.7.0/24"))
			}
		})
	}
}

// A replacement at the same destination is a different concrete Peer. Even
// when it now uses direct/no-RIB output, the old ordinary entry cannot acquire
// authority over its useful direct announcement by falling through retirement.
func TestOriginRuntimeRegistryDoesNotAdoptReplacementPeer(t *testing.T) {
	f := newOriginRuntimeSDKFixture(t, true)
	_, tracked := originRegistryAnnounce(t, f, f.batch, plugin.SelectedOwner{})
	original := f.writer.peer
	require.NoError(t, f.writer.api.r.originReceipts.revokePeer(f.ctx, original))
	replacement, wire := newGroupUpdatesPeer(t, original.addrString, "absent")
	replacement.settings.LocalAddress = original.settings.LocalAddress
	replacement.settings.ExportPolicy = original.settings.ExportPolicy
	replacement.reactor = f.writer.api.r
	replacement.refreshForwardFacts()
	replacement.session.egressRouteFilter = replacement.egressGate(replacement.session)
	require.NotEqual(t, original.sourceOwner, replacement.sourceOwner)
	f.writer.api.r.peers[replacement.Settings().PeerKey()] = replacement
	result, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(replacement.addrString), f.batch, f.sender)
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.Announced)
	before := updateBodies(t, wire.written())
	require.Len(t, before, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, before[0], false))
	require.NoError(t, expireOriginRegistryEntry(t, f, tracked, f.sender))
	require.Len(t, updateBodies(t, wire.written()), 1, "old ordinary timer must not withdraw the replacement's direct output")
}
