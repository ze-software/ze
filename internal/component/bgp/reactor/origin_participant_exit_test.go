// Design: docs/architecture/api/process-protocol.md -- per-participant route ownership on exit (E-2, AC-41, S38).
// Related: origin_runtime.go -- selected-RIB originReceiptStore exit cleanup and EndOriginOwnerWithPolicy.
// Related: origin_direct.go -- direct-path per-participant route ownership table.

package reactor

import (
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type multiProducerRIBFixture struct {
	writer *selectedWriterFixture
	host   *pluginserver.Server
	ctx    context.Context
	name   string
}

// newMultiProducerRIBFixture builds a real deciding-provider fixture that
// stores OriginIntake candidates keyed by (keyNLRI, producer), re-elects a
// surviving producer on withdraw, and emits SelectedRouteAbsent when the last
// producer withdraws — matching bgp-rib's candidate table semantics.
func newMultiProducerRIBFixture(t *testing.T, providerName string) *multiProducerRIBFixture {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	f := newSelectedWriterFixture(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)

	declarations := []rpc.DisseminationDecl{{
		Namespace:          "bgp",
		Role:               SelectedDisseminationRole,
		SourceReceive:      []string{"update-received", "state"},
		DestinationReceive: []string{"state", "refresh", "update-sent"},
		DestinationSend:    []string{"update"},
	}}
	sends := []rpc.MessageSendDecl{{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate}}
	receives := []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRefresh},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate, Tags: []rpc.MessageTagDecl{{
			Name:   "direction",
			Values: []string{"received", "sent"},
		}}},
	}

	require.NoError(t, registry.Register(registry.Registration{
		Name:          providerName,
		CLIHandler:    func([]string) int { return 0 },
		Dissemination: declarations,
		Sends:         sends,
		Receives:      receives,
		RunEngine: func(conn net.Conn) int {
			provider := sdk.NewWithConn(providerName, conn)
			var mu sync.Mutex
			var revision uint64
			type candidate struct {
				producer string
				receipt  uint64
				family   string
				keyNLRI  []byte
			}
			// candidates[keyHex][producerName] -> candidate
			candidates := make(map[string]map[string]candidate)
			// activeRev[keyHex] -> current selected revision
			activeRev := make(map[string]uint64)

			pickBest := func(byProducer map[string]candidate) (candidate, bool) {
				if len(byProducer) == 0 {
					return candidate{}, false
				}
				names := make([]string, 0, len(byProducer))
				for p := range byProducer {
					names = append(names, p)
				}
				slices.Sort(names)
				return byProducer[names[0]], true
			}

			provider.OnCurrentSelection(func(_ context.Context, in rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				out := &rpc.CurrentSelectionOutput{
					Destination:        in.Destination,
					DestinationSession: in.DestinationSession,
					Cut:                revision,
				}
				for _, key := range in.Keys {
					result := rpc.CurrentSelectionResult{
						Family:   key.Family,
						KeyNLRI:  key.KeyNLRI,
						Revision: key.Revision,
						Presence: rpc.CurrentSelectionAbsent,
					}
					if rev := activeRev[string(key.KeyNLRI)]; rev != 0 {
						result.Presence = rpc.CurrentSelectionPresent
						result.CurrentRevision = rev
					}
					out.Results = append(out.Results, result)
				}
				return out, nil
			})

			provider.OnOriginIntake(func(work context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
				keyStr := string(in.Route.KeyNLRI)
				prodName := in.Producer.Name
				if prodName == "" {
					prodName = "operator"
				}

				mu.Lock()
				revision++
				ver := revision
				if in.Operation == rpc.OriginIntakeWithdraw {
					byProd := candidates[keyStr]
					if _, exists := byProd[prodName]; !exists {
						mu.Unlock()
						return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
					}
					delete(byProd, prodName)
					if len(byProd) == 0 {
						delete(candidates, keyStr)
						delete(activeRev, keyStr)
						mu.Unlock()
						route := rpc.SelectedRoute{
							Family:   in.Route.Family,
							KeyNLRI:  in.Route.KeyNLRI,
							Revision: ver,
							Kind:     rpc.SelectedRouteAbsent,
						}
						if _, err := provider.SelectRoutes(work, rpc.SelectRoutesInput{
							Destination:        in.Destination,
							DestinationSession: 71,
							Routes:             []rpc.SelectedRoute{route},
						}); err != nil {
							return nil, err
						}
						return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
					}
					winner, _ := pickBest(byProd)
					activeRev[keyStr] = ver
					mu.Unlock()
					route := rpc.SelectedRoute{
						Family:        winner.family,
						KeyNLRI:       winner.keyNLRI,
						Revision:      ver,
						Kind:          rpc.SelectedRouteOrigin,
						OriginReceipt: winner.receipt,
						StaleLevel:    rpc.SelectedFresh,
						Route: &rpc.StoredRoute{
							Family:      winner.family,
							NLRIHex:     hex.EncodeToString(winner.keyNLRI),
							NLRIFraming: rpc.NLRIFramingPrefixOnly,
						},
					}
					if _, err := provider.SelectRoutes(work, rpc.SelectRoutesInput{
						Destination:        in.Destination,
						DestinationSession: 71,
						Routes:             []rpc.SelectedRoute{route},
					}); err != nil {
						return nil, err
					}
					return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
				}

				byProd := candidates[keyStr]
				if byProd == nil {
					byProd = make(map[string]candidate)
					candidates[keyStr] = byProd
				}
				byProd[prodName] = candidate{
					producer: prodName,
					receipt:  in.OriginReceipt,
					family:   in.Route.Family,
					keyNLRI:  slices.Clone(in.Route.KeyNLRI),
				}
				winner, _ := pickBest(byProd)
				activeRev[keyStr] = ver
				mu.Unlock()

				route := rpc.SelectedRoute{
					Family:        winner.family,
					KeyNLRI:       winner.keyNLRI,
					Revision:      ver,
					Kind:          rpc.SelectedRouteOrigin,
					OriginReceipt: winner.receipt,
					StaleLevel:    rpc.SelectedFresh,
					Route: &rpc.StoredRoute{
						Family:      winner.family,
						NLRIHex:     hex.EncodeToString(winner.keyNLRI),
						NLRIFraming: rpc.NLRIFramingPrefixOnly,
					},
				}
				answer, err := provider.SelectRoutes(work, rpc.SelectRoutesInput{
					Destination:        in.Destination,
					DestinationSession: 71,
					Routes:             []rpc.SelectedRoute{route},
				})
				if err != nil {
					return nil, err
				}
				out := &rpc.UpdateRouteOutput{
					Admissions: []rpc.RouteAdmission{{
						Family:      in.Route.Family,
						KeyNLRI:     in.Route.KeyNLRI,
						Destination: in.Destination,
						PeerOwner:   in.PeerOwner,
						Generation:  in.OriginReceipt,
						Outcome:     rpc.RouteAdmissionRetained,
						Disposition: rpc.RouteAdmissionSelected,
					}},
				}
				if answer.Results[0].Outcome == rpc.SelectedWritten {
					out.Announced = 1
				} else {
					out.Admissions[0].Disposition = rpc.RouteAdmissionSuppressedExport
				}
				return out, nil
			})

			if err := provider.Run(ctx, &sdk.Registration{
				Dissemination: declarations,
				Sends:         sends,
				Receives:      receives,
			}); err != nil && ctx.Err() == nil {
				t.Errorf("multi-producer RIB provider: %v", err)
				return 1
			}
			return 0
		},
	}))

	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins: []plugin.PluginConfig{{Name: providerName, Run: providerName, Internal: true}},
	}, f.api)
	require.NoError(t, err)
	sourceBus := testBusBlockForGrants(testPeerEndpointGrant{
		Name:    providerName,
		Receive: []string{"state", "update-received"},
	})
	destBus := testBusBlockForGrants(testPeerEndpointGrant{
		Name:    providerName,
		Receive: []string{"state", "refresh", "update-sent"},
		Send:    []string{"update"},
	})
	deliveryPeers := []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", sourceBus),
		testDeliveryPeer(f.peer.addrString, "", destBus),
	}
	f.peer.settings.Bus = destBus
	f.api.r.api = host
	f.api.r.originReceipts, err = newOriginReceiptStore(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.api.r.originReceipts.close(context.Background())) })
	startOriginIntakeTestHost(t, ctx, host)

	require.NoError(t, host.UpdateDeliveryGraphWithEndpoints(
		bgpevents.Namespace,
		deliveryPeers,
		buildTestEndpointInputsForPeers(deliveryPeers, host, []pluginserver.BusParticipantInput{testBusEndpointAll(providerName)}),
	))
	configured, err := host.ResolveDisseminationOwnerForPeer(
		testDeliveryPeer(f.peer.addrString, "", destBus),
		SelectedDisseminationRole,
	)
	require.NoError(t, err)
	f.peer.settings.DisseminationOwner = configured
	f.peer.refreshForwardFacts()
	f.peer.session.egressRouteFilter = f.peer.egressGate(f.peer.session)

	return &multiProducerRIBFixture{writer: f, host: host, ctx: ctx, name: providerName}
}

func (f *multiProducerRIBFixture) grantProducer(name string) {
	addTestPeerBusGrant(f.writer.peer.settings, testPeerEndpointGrant{
		Name: name,
		Send: []string{"update"},
	})
	sourceBus := testBusBlockForGrants(testPeerEndpointGrant{
		Name:    f.name,
		Receive: []string{"state", "update-received"},
	})
	deliveryPeers := []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", sourceBus),
		testDeliveryPeer(f.writer.peer.addrString, "", f.writer.peer.settings.Bus),
	}
	_ = f.host.UpdateDeliveryGraphWithEndpoints(
		bgpevents.Namespace,
		deliveryPeers,
		buildTestEndpointInputsForPeers(deliveryPeers, f.host, []pluginserver.BusParticipantInput{
			testBusEndpointAll(f.name),
			testBusEndpointAll(name),
		}),
	)
	f.writer.peer.refreshForwardFacts()
}

func participantBatch(prefixStr, nextHopStr string, owner plugin.SelectedOwner) bgptypes.NLRIBatch {
	input := selectedOriginBuildInput()
	return bgptypes.NLRIBatch{
		OriginOwner: owner,
		Family:      family.IPv4Unicast,
		NLRIs:       []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefixStr), 0, false)},
		NextHop:     bgptypes.NewNextHopExplicit(netip.MustParseAddr(nextHopStr)),
		Wire:        attribute.NewAttributesWire(input.attributes, input.attributeContext),
	}
}

// VALIDATES: S38 / AC-41 on the selected-RIB (bgp-rib originReceiptStore) path:
// (1) when participants A and B both announce route R, A exiting under default
// withdraw policy leaves B's contribution advertised on the peer;
// (2) when the sole remaining participant exits under default withdraw policy,
// R is withdrawn from the peer;
// (3) when a sole participant exits under on-exit keep policy, R stays
// advertised to the peer.
// PREVENTS: exited participants leaking routes on the bgp-rib path, withdrawing
// another participant's surviving route, or dropping routes under on-exit keep.
func TestOriginParticipantExitSelectedRIBPath(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.7.0/24")

	t.Run("two-participants-first-exits-survivor-remains-then-second-exits-withdraws", func(t *testing.T) {
		f := newMultiProducerRIBFixture(t, "multi-rib-two")
		f.grantProducer("script-a")
		f.grantProducer("script-b")

		ownerA, stopA := originOwnerFixture(t, "script-a")
		ownerB, stopB := originOwnerFixture(t, "script-b")

		_, err := f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", ownerA), ownerA.Sender(),
		)
		require.NoError(t, err)

		_, err = f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.20", ownerB), ownerB.Sender(),
		)
		require.NoError(t, err)
		originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 2, 2, 0, 0)

		// Participant A exits with default withdraw policy: B survives and
		// becomes the advertised winner; NO withdrawal is sent on the wire.
		beforeAExit := len(updateBodies(t, f.writer.conn.written()))
		stopA()
		require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, ownerA))
		originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 1, 0, 0)

		bodiesAfterA := updateBodies(t, f.writer.conn.written())
		for _, body := range bodiesAfterA[beforeAExit:] {
			require.Empty(t, updatePrefixes(t, body, true), "surviving participant B must keep route advertised without withdrawal")
		}
		lastAfterA := bodiesAfterA[len(bodiesAfterA)-1]
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, lastAfterA, false))
		hop, ok := bodyPathAttr(t, lastAfterA, 3)
		require.True(t, ok)
		require.Equal(t, []byte{192, 0, 2, 20}, hop, "peer must now carry surviving participant B's next-hop")

		// Participant B (now sole producer) exits with default withdraw policy:
		// route R is withdrawn from the peer.
		stopB()
		require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, ownerB))
		originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)

		bodiesAfterB := updateBodies(t, f.writer.conn.written())
		lastAfterB := bodiesAfterB[len(bodiesAfterB)-1]
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, lastAfterB, true), "sole remaining participant exit must withdraw route")
	})

	t.Run("sole-participant-default-withdraw-and-on-exit-keep", func(t *testing.T) {
		f := newMultiProducerRIBFixture(t, "multi-rib-policy")
		f.grantProducer("script-withdraw")
		f.grantProducer("script-keep")

		ownerWithdraw, stopWithdraw := originOwnerFixture(t, "script-withdraw")
		ownerKeep, stopKeep := originOwnerFixture(t, "script-keep")

		_, err := f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", ownerWithdraw), ownerWithdraw.Sender(),
		)
		require.NoError(t, err)

		_, err = f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch("198.18.8.0/24", "192.0.2.11", ownerKeep), ownerKeep.Sender(),
		)
		require.NoError(t, err)

		// Default withdraw removes 198.18.7.0/24 from the peer.
		stopWithdraw()
		require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, ownerWithdraw, false))
		bodies := updateBodies(t, f.writer.conn.written())
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodies[len(bodies)-1], true))

		// on-exit keep retains 198.18.8.0/24 on the peer without withdrawal.
		beforeKeepExit := len(bodies)
		stopKeep()
		require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, ownerKeep, true))
		originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 1, 1, 0, 0)
		bodiesAfterKeep := updateBodies(t, f.writer.conn.written())
		for _, body := range bodiesAfterKeep[beforeKeepExit:] {
			require.Empty(t, updatePrefixes(t, body, true), "on-exit keep must not withdraw the route")
		}
	})
}

func newDirectOriginPeerFixture(t *testing.T, producers ...string) (*reactorAPIAdapter, *Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, "192.0.2.50", "false")
	r := &Reactor{
		config: &Config{LocalAS: 65000, RouterID: 0x0a000001},
		peers:  map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer},
	}
	// Leave r.originReceipts == nil explicitly to prove direct-path ownership
	// works without originReceiptStore / bgp-rib.
	peer.reactor = r
	grants := make([]testPeerEndpointGrant, 0, len(producers))
	extra := make([]pluginserver.BusParticipantInput, 0, len(producers))
	for _, name := range producers {
		grants = append(grants, testPeerEndpointGrant{
			Name: name,
			Send: []string{"update"},
		})
		extra = append(extra, testBusEndpointAll(name))
	}
	setTestPeerBus(t, peer, testBusBlockForGrants(grants...), extra...)
	peer.refreshForwardFacts()
	return &reactorAPIAdapter{r: r}, peer, conn
}

// VALIDATES: S38 / AC-41 on the direct (no-bgp-rib) path:
// (1) when participants A and B both announce route R to a direct peer, A
// exiting under default withdraw policy re-announces B's surviving route
// without withdrawing R, and B exiting then withdraws R;
// (2) sole participant exit under default withdraw policy withdraws R (or
// restores a configured static route if one exists for the prefix);
// (3) sole participant exit under on-exit keep policy keeps R advertised;
// (4) peer removal clears direct-path entries with no leak;
// (5) sourceGenerationCleanup reads proc.Config().OnExit == plugin.OnExitKeep.
// PREVENTS: direct peers leaking routes on participant exit, withdrawing a
// surviving participant's route, or dropping routes under on-exit keep.
func TestOriginParticipantExitDirectPath(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.7.0/24")

	t.Run("two-participants-b-then-a-a-exits-b-reannounced-then-b-exits-withdraws", func(t *testing.T) {
		api, peer, conn := newDirectOriginPeerFixture(t, "script-a", "script-b")
		ownerA, stopA := originOwnerFixture(t, "script-a")
		ownerB, stopB := originOwnerFixture(t, "script-b")

		// B announces first (next-hop .20), then A overwrites on wire (next-hop .10).
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.20", ownerB), ownerB.Sender(),
		))
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", ownerA), ownerA.Sender(),
		))
		beforeAExit := len(updateBodies(t, conn.written()))
		require.Equal(t, 2, beforeAExit)

		// A exits (default withdraw): B's route (.20) is re-announced; NO withdrawal!
		stopA()
		require.NoError(t, api.EndOriginOwner(t.Context(), ownerA))
		bodiesAfterA := updateBodies(t, conn.written())
		require.Len(t, bodiesAfterA, beforeAExit+1)
		require.Empty(t, updatePrefixes(t, bodiesAfterA[beforeAExit], true))
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodiesAfterA[beforeAExit], false))
		hop, ok := bodyPathAttr(t, bodiesAfterA[beforeAExit], 3)
		require.True(t, ok)
		require.Equal(t, []byte{192, 0, 2, 20}, hop, "surviving participant B's next-hop must be re-announced")

		// B exits (now sole contributor): R is withdrawn!
		stopB()
		require.NoError(t, api.EndOriginOwner(t.Context(), ownerB))
		bodiesAfterB := updateBodies(t, conn.written())
		require.Len(t, bodiesAfterB, beforeAExit+2)
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodiesAfterB[len(bodiesAfterB)-1], true))

		api.r.directOrigins.mu.Lock()
		require.Empty(t, api.r.directOrigins.entries)
		require.Empty(t, api.r.directOrigins.byOwner)
		require.Empty(t, api.r.directOrigins.byPeer)
		api.r.directOrigins.mu.Unlock()
	})

	t.Run("sole-participant-withdraws-on-default-exit", func(t *testing.T) {
		api, peer, conn := newDirectOriginPeerFixture(t, "script-sole")
		owner, stop := originOwnerFixture(t, "script-sole")

		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", owner), owner.Sender(),
		))
		require.Len(t, updateBodies(t, conn.written()), 1)

		stop()
		require.NoError(t, api.EndOriginOwner(t.Context(), owner))
		bodies := updateBodies(t, conn.written())
		require.Len(t, bodies, 2)
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodies[1], true))
	})

	t.Run("sole-participant-keeps-route-on-exit-keep", func(t *testing.T) {
		api, peer, conn := newDirectOriginPeerFixture(t, "script-keep")
		owner, stop := originOwnerFixture(t, "script-keep")

		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", owner), owner.Sender(),
		))
		require.Len(t, updateBodies(t, conn.written()), 1)

		stop()
		require.NoError(t, api.EndOriginOwnerWithPolicy(t.Context(), owner, true))
		require.Len(t, updateBodies(t, conn.written()), 1, "on-exit keep must not send a withdrawal")
	})

	t.Run("sole-participant-exit-restores-configured-static-route", func(t *testing.T) {
		api, peer, conn := newDirectOriginPeerFixture(t, "script-override")
		configured := staticRouteAt("198.18.7.0/24", "192.0.2.252")
		peer.settings.StaticRoutes = []StaticRoute{configured}
		installed := peer.sendStaticRoutes(peer.session, peer.settings.StaticRoutes, false, 4096, false)
		require.Len(t, installed, 1)
		peer.staticWire = staticWireSet{session: peer.session, routes: installed}

		owner, stop := originOwnerFixture(t, "script-override")
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", owner), owner.Sender(),
		))
		beforeExit := len(updateBodies(t, conn.written()))

		stop()
		require.NoError(t, api.EndOriginOwner(t.Context(), owner))
		bodies := updateBodies(t, conn.written())
		require.Len(t, bodies, beforeExit+1)
		last := bodies[len(bodies)-1]
		require.Empty(t, updatePrefixes(t, last, true), "configured fallback replaces withdrawal")
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, last, false))
		hop, ok := bodyPathAttr(t, last, 3)
		require.True(t, ok)
		require.Equal(t, []byte{192, 0, 2, 252}, hop, "configured static next-hop must be restored")
	})

	t.Run("peer-removal-clears-direct-origins", func(t *testing.T) {
		api, peer, _ := newDirectOriginPeerFixture(t, "script-leak")
		owner, _ := originOwnerFixture(t, "script-leak")

		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", owner), owner.Sender(),
		))
		api.r.completeOriginPeerRemoval(peer)

		api.r.directOrigins.mu.Lock()
		require.Empty(t, api.r.directOrigins.entries)
		require.Empty(t, api.r.directOrigins.byOwner)
		require.Empty(t, api.r.directOrigins.byPeer)
		api.r.directOrigins.mu.Unlock()
	})

	t.Run("sourceGenerationCleanup-threads-on-exit-policy", func(t *testing.T) {
		snap := registry.Snapshot()
		t.Cleanup(func() { registry.Restore(snap) })
		for _, regName := range []string{"ep-withdraw", "ep-keep"} {
			name := regName
			require.NoError(t, registry.Register(registry.Registration{
				Name:       name,
				CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					<-t.Context().Done()
					_ = conn.Close()
					return 0
				},
			}))
		}

		api, peer, conn := newDirectOriginPeerFixture(t, "ep-withdraw", "ep-keep")
		srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, api)
		require.NoError(t, err)

		procWithdraw := process.NewProcess(plugin.PluginConfig{Name: "ep-withdraw", Internal: true})
		require.NoError(t, procWithdraw.StartWithContext(t.Context()))
		require.NoError(t, procWithdraw.InitConns())
		procWithdraw.StartDelivery(t.Context())
		t.Cleanup(func() { procWithdraw.Stop() })
		ownerWithdraw, err := procWithdraw.SelectedOwner()
		require.NoError(t, err)

		procKeep := process.NewProcess(plugin.PluginConfig{Name: "ep-keep", Internal: true, OnExit: plugin.OnExitKeep})
		require.NoError(t, procKeep.StartWithContext(t.Context()))
		require.NoError(t, procKeep.InitConns())
		procKeep.StartDelivery(t.Context())
		t.Cleanup(func() { procKeep.Stop() })
		ownerKeep, err := procKeep.SelectedOwner()
		require.NoError(t, err)

		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.7.0/24", "192.0.2.10", ownerWithdraw), ownerWithdraw.Sender(),
		))
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch("198.18.8.0/24", "192.0.2.11", ownerKeep), ownerKeep.Sender(),
		))
		require.Len(t, updateBodies(t, conn.written()), 2)

		// Stopping ep-withdraw triggers withdrawal via sourceGenerationCleanup.
		procWithdraw.Stop()
		sourceGenerationCleanup(srv, procWithdraw)
		bodies := updateBodies(t, conn.written())
		require.Len(t, bodies, 3)
		require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodies[2], true))

		// Stopping ep-keep retains 198.18.8.0/24 via sourceGenerationCleanup.
		procKeep.Stop()
		sourceGenerationCleanup(srv, procKeep)
		require.Len(t, updateBodies(t, conn.written()), 3, "sourceGenerationCleanup must honor OnExitKeep")
	})
}

// TestParticipantRoutesWithdrawnWhenItGoes proves the S38 / AC-41 route
// ownership contract on participant exit across both selected-RIB (bgp-rib)
// and direct-peer paths:
//  1. When participants a and b both announce 203.0.113.0/24 and a exits under
//     the default withdraw policy, the peer still holds 203.0.113.0/24 from b
//     with no withdrawal sent.
//  2. When participant a alone announces 198.51.100.0/24 and exits under the
//     default withdraw policy, 198.51.100.0/24 is withdrawn from the peer.
//  3. When participant a alone announces 198.51.100.0/24 and exits with
//     on-exit keep enabled, 198.51.100.0/24 stays advertised with no withdrawal.
//
// VALIDATES: S38, AC-41 (per-participant route withdrawal and on-exit keep).
// PREVENTS: Leaked routes on participant exit, withdrawing a surviving
// participant's shared prefix, or dropping routes under on-exit keep.
func TestParticipantRoutesWithdrawnWhenItGoes(t *testing.T) {
	const (
		sharedPrefixStr = "203.0.113.0/24"
		solePrefixStr   = "198.51.100.0/24"
		nextHopA        = "192.0.2.10"
		nextHopB        = "192.0.2.20"
		epA             = "endpoint-a"
		epB             = "endpoint-b"
		epAKeep         = "endpoint-a-keep"
	)
	sharedPrefix := netip.MustParsePrefix(sharedPrefixStr)
	solePrefix := netip.MustParsePrefix(solePrefixStr)

	t.Run("selected-rib-path", func(t *testing.T) {
		f := newMultiProducerRIBFixture(t, "s38-rib-provider")
		f.grantProducer(epA)
		f.grantProducer(epB)
		f.grantProducer(epAKeep)

		ownerA, stopA := originOwnerFixture(t, epA)
		ownerB, stopB := originOwnerFixture(t, epB)
		ownerKeep, stopKeep := originOwnerFixture(t, epAKeep)

		// a and b both announce 203.0.113.0/24; a also alone announces 198.51.100.0/24.
		_, err := f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch(sharedPrefixStr, nextHopA, ownerA), ownerA.Sender(),
		)
		require.NoError(t, err)
		_, err = f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch(sharedPrefixStr, nextHopB, ownerB), ownerB.Sender(),
		)
		require.NoError(t, err)
		_, err = f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch(solePrefixStr, nextHopA, ownerA), ownerA.Sender(),
		)
		require.NoError(t, err)

		// Stopping a withdraws a's sole route 198.51.100.0/24 while 203.0.113.0/24
		// remains advertised from surviving participant b.
		beforeAExit := len(updateBodies(t, f.writer.conn.written()))
		stopA()
		require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, ownerA))

		bodiesAfterA := updateBodies(t, f.writer.conn.written())[beforeAExit:]
		var withdrawn []netip.Prefix
		var reannouncedB bool
		for _, body := range bodiesAfterA {
			withdrawn = append(withdrawn, updatePrefixes(t, body, true)...)
			if slices.Equal(updatePrefixes(t, body, false), []netip.Prefix{sharedPrefix}) {
				if hop, ok := bodyPathAttr(t, body, 3); ok && slices.Equal(hop, []byte{192, 0, 2, 20}) {
					reannouncedB = true
				}
			}
		}
		require.Equal(t, []netip.Prefix{solePrefix}, withdrawn, "a's sole prefix must be withdrawn and shared prefix must not")
		require.True(t, reannouncedB, "surviving participant b must hold shared prefix on the peer")

		// Stopping b now withdraws 203.0.113.0/24.
		beforeBExit := len(updateBodies(t, f.writer.conn.written()))
		stopB()
		require.NoError(t, f.writer.api.EndOriginOwner(f.ctx, ownerB))
		bodiesAfterB := updateBodies(t, f.writer.conn.written())[beforeBExit:]
		require.NotEmpty(t, bodiesAfterB)
		require.Equal(t, []netip.Prefix{sharedPrefix}, updatePrefixes(t, bodiesAfterB[len(bodiesAfterB)-1], true))

		// With on-exit keep, stopping endpoint-a-keep leaves 198.51.100.0/24 advertised.
		_, err = f.writer.api.AnnounceNLRIBatchResult(
			f.ctx, selector.ParseDefault(f.writer.peer.addrString),
			participantBatch(solePrefixStr, nextHopA, ownerKeep), ownerKeep.Sender(),
		)
		require.NoError(t, err)
		beforeKeepExit := len(updateBodies(t, f.writer.conn.written()))
		stopKeep()
		require.NoError(t, f.writer.api.EndOriginOwnerWithPolicy(f.ctx, ownerKeep, true))
		for _, body := range updateBodies(t, f.writer.conn.written())[beforeKeepExit:] {
			require.Empty(t, updatePrefixes(t, body, true), "on-exit keep must not withdraw the route")
		}
	})

	t.Run("direct-peer-path", func(t *testing.T) {
		api, peer, conn := newDirectOriginPeerFixture(t, epA, epB, epAKeep)
		ownerA, stopA := originOwnerFixture(t, epA)
		ownerB, stopB := originOwnerFixture(t, epB)
		ownerKeep, stopKeep := originOwnerFixture(t, epAKeep)

		// b announces shared prefix first, then a announces shared prefix and sole prefix.
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch(sharedPrefixStr, nextHopB, ownerB), ownerB.Sender(),
		))
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch(sharedPrefixStr, nextHopA, ownerA), ownerA.Sender(),
		))
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch(solePrefixStr, nextHopA, ownerA), ownerA.Sender(),
		))

		beforeAExit := len(updateBodies(t, conn.written()))
		stopA()
		require.NoError(t, api.EndOriginOwner(t.Context(), ownerA))

		bodiesAfterA := updateBodies(t, conn.written())[beforeAExit:]
		var withdrawn []netip.Prefix
		var reannouncedB bool
		for _, body := range bodiesAfterA {
			withdrawn = append(withdrawn, updatePrefixes(t, body, true)...)
			if slices.Equal(updatePrefixes(t, body, false), []netip.Prefix{sharedPrefix}) {
				if hop, ok := bodyPathAttr(t, body, 3); ok && slices.Equal(hop, []byte{192, 0, 2, 20}) {
					reannouncedB = true
				}
			}
		}
		require.Equal(t, []netip.Prefix{solePrefix}, withdrawn, "a's sole prefix must be withdrawn and shared prefix must not")
		require.True(t, reannouncedB, "surviving participant b must be re-announced for shared prefix")

		beforeBExit := len(updateBodies(t, conn.written()))
		stopB()
		require.NoError(t, api.EndOriginOwner(t.Context(), ownerB))
		bodiesAfterB := updateBodies(t, conn.written())[beforeBExit:]
		require.NotEmpty(t, bodiesAfterB)
		require.Equal(t, []netip.Prefix{sharedPrefix}, updatePrefixes(t, bodiesAfterB[len(bodiesAfterB)-1], true))

		// With on-exit keep, sole prefix stays advertised without withdrawal.
		require.NoError(t, api.AnnounceNLRIBatch(
			t.Context(), selector.ParseDefault(peer.addrString),
			participantBatch(solePrefixStr, nextHopA, ownerKeep), ownerKeep.Sender(),
		))
		beforeKeepExit := len(updateBodies(t, conn.written()))
		stopKeep()
		require.NoError(t, api.EndOriginOwnerWithPolicy(t.Context(), ownerKeep, true))
		require.Len(t, updateBodies(t, conn.written()), beforeKeepExit, "on-exit keep must not send a withdrawal")
	})
}

// TestCommandContextOriginIdentityWithdrawnOnProcessExit verifies the end-to-end
// daemon wiring from CommandContext.OriginContext() (used by `send bgp ... update
// text` without a pre-populated batch.OriginOwner and with nil Wire attributes)
// through a *plugin.Coordinator-backed Server and sourceGenerationCleanup on
// process exit (S38).
//
// VALIDATES: CommandContext.OriginContext -> AnnounceNLRIBatchResult ->
// Coordinator.FullReactor -> sourceGenerationCleanup -> EndOriginOwnerWithPolicy
// for both default withdraw and on-exit keep.
// PREVENTS: routes dispatched via CommandContext losing process incarnation
// ownership or failing to withdraw when the Server wraps its reactor in a Coordinator.
func TestCommandContextOriginIdentityWithdrawnOnProcessExit(t *testing.T) {
	f := newMultiProducerRIBFixture(t, "s38-cmdctx-rib")
	for _, epName := range []string{"a", "b", "a-keep"} {
		name := epName
		f.grantProducer(name)
		require.NoError(t, registry.Register(registry.Registration{
			Name:       name,
			CLIHandler: func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				<-t.Context().Done()
				_ = conn.Close()
				return 0
			},
		}))
	}

	f.writer.api.SetConfigTree(map[string]any{})
	coord := plugin.NewCoordinator(map[string]any{})
	require.NoError(t, coord.AttachReactor(f.writer.api))
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coord)
	require.NoError(t, err)

	startProc := func(name, onExit string) *process.Process {
		t.Helper()
		proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true, OnExit: onExit})
		require.NoError(t, proc.StartWithContext(t.Context()))
		require.NoError(t, proc.InitConns())
		proc.StartDelivery(t.Context())
		t.Cleanup(func() { proc.Stop() })
		return proc
	}

	announceViaCommandContext := func(proc *process.Process, prefixStr, nextHopStr string) {
		t.Helper()
		cmdCtx := &pluginserver.CommandContext{
			Server:         srv,
			Process:        proc,
			RequestContext: f.ctx,
			Sender:         plugin.ProcessSender(proc.Name()),
		}
		originCtx := cmdCtx.OriginContext()
		p := netip.MustParsePrefix(prefixStr)
		// Match `send bgp <peer> update text nhop <ip> nlri add <prefix>`:
		// OriginOwner is intentionally left unset on the batch so reactor reads
		// it from originCtx, and Wire is nil (no explicit path attributes).
		batch := bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, p, 0, false)},
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(nextHopStr)),
		}
		_, err := f.writer.api.AnnounceNLRIBatchResult(originCtx, selector.ParseDefault(f.writer.peer.addrString), batch, cmdCtx.Sender)
		require.NoError(t, err)
	}

	procA := startProc("a", "")
	procB := startProc("b", "")
	procKeep := startProc("a-keep", plugin.OnExitKeep)

	sharedPrefix := netip.MustParsePrefix("203.0.113.0/24")
	solePrefix := netip.MustParsePrefix("198.51.100.0/24")

	// Exact S38 Run 1 sequence: a announces 203.0.113.0/24 and 198.51.100.0/24,
	// then b announces 203.0.113.0/24 with a distinct next-hop.
	announceViaCommandContext(procA, "203.0.113.0/24", "192.0.2.10")
	announceViaCommandContext(procA, "198.51.100.0/24", "192.0.2.10")
	announceViaCommandContext(procB, "203.0.113.0/24", "192.0.2.20")

	// Exit a through sourceGenerationCleanup on the Coordinator-backed Server:
	// two wire UPDATEs follow -- b's re-announcement of 203.0.113.0/24 with
	// next-hop 192.0.2.20 and the withdrawal of 198.51.100.0/24.
	beforeExit := len(updateBodies(t, f.writer.conn.written()))
	procA.Stop()
	sourceGenerationCleanup(srv, procA)

	bodiesAfterA := updateBodies(t, f.writer.conn.written())[beforeExit:]
	require.Len(t, bodiesAfterA, 2, "exiting a produces one failover re-announce for b and one withdrawal for a's sole prefix")
	var withdrawn []netip.Prefix
	var reannouncedB bool
	for _, body := range bodiesAfterA {
		withdrawn = append(withdrawn, updatePrefixes(t, body, true)...)
		if slices.Equal(updatePrefixes(t, body, false), []netip.Prefix{sharedPrefix}) {
			if hop, ok := bodyPathAttr(t, body, 3); ok && slices.Equal(hop, []byte{192, 0, 2, 20}) {
				reannouncedB = true
			}
		}
	}
	require.Equal(t, []netip.Prefix{solePrefix}, withdrawn)
	require.True(t, reannouncedB)

	// S38 Run 2: on-exit keep retains 198.51.100.0/24 across sourceGenerationCleanup.
	announceViaCommandContext(procKeep, "198.51.100.0/24", "192.0.2.10")
	beforeKeepExit := len(updateBodies(t, f.writer.conn.written()))
	procKeep.Stop()
	sourceGenerationCleanup(srv, procKeep)
	for _, body := range updateBodies(t, f.writer.conn.written())[beforeKeepExit:] {
		require.Empty(t, updatePrefixes(t, body, true), "on-exit keep must not withdraw originated route")
	}
}
