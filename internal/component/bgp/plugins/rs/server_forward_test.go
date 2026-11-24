// Design: docs/architecture/core-design.md -- route server client-set scoping.
// Related: server_forward.go -- selectForwardTargets and fastPathOwedTarget.
// Related: server_handlers.go -- handleState client-set classification.

package rs

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteServerServesItsClientSet proves RFC 7947 client-set scoping for
// bgp-rs over both structured and native text transports.
//
// VALIDATES: AC-19 and S13 — every connected rs-client except the source is a
// forwarding target (c1 -> c2, c3); a connected non-rs-client (i1) is never a
// forwarding target, never triggers peer-up replay, and its UPDATEs are released
// without taking a source lease or forwarding to rs-clients; bgp-rs declares
// rs-client=true claim scope.
// PREVENTS: bgp-rs forwarding routes to or from non-rs-client peers (such as
// iBGP peers served by bgp-rr on the same box) or double-delivering routes.
func TestRouteServerServesItsClientSet(t *testing.T) {
	require.Equal(t,
		[]rpc.FilterSessionFact{{Fact: "rs-client", Values: []string{"true"}}},
		registry.ClaimSessionScopeFor(rsPluginName),
	)

	type peerSpec struct {
		addr     string
		owner    uint64
		conn     uint64
		session  uint64
		rsClient bool
		isIBGP   bool
	}
	c1Spec := peerSpec{addr: "192.0.2.1", owner: 701, conn: 41, session: 141, rsClient: true}
	c2Spec := peerSpec{addr: "192.0.2.2", owner: 702, conn: 42, session: 142, rsClient: true}
	c3Spec := peerSpec{addr: "192.0.2.3", owner: 703, conn: 43, session: 143, rsClient: true}
	i1Spec := peerSpec{addr: "192.0.2.10", owner: 710, conn: 50, session: 150, rsClient: false, isIBGP: true}

	for _, transport := range []struct {
		name string
		up   func(rs *routeServer, spec peerSpec, reason string, initialReplay uint64)
	}{
		{
			name: "structured",
			up: func(rs *routeServer, spec peerSpec, reason string, initialReplay uint64) {
				ev := parseStructuredState(&rpc.StructuredEvent{
					EventType:          rpc.EventKindState,
					PeerAddress:        spec.addr,
					PeerOwner:          spec.owner,
					ConnectionID:       spec.conn,
					DestinationSession: spec.session,
					SessionGeneration:  spec.conn + 1000,
					InitialReplay:      initialReplay,
					State:              rpc.SessionStateUp,
					Reason:             reason,
					RSClient:           spec.rsClient,
					IsIBGP:             spec.isIBGP,
				})
				require.NotNil(t, ev)
				rs.handleState(ev)
			},
		},
		{
			name: "native-text",
			up: func(rs *routeServer, spec peerSpec, reason string, initialReplay uint64) {
				var tb textbuf.Buffer
				tb.Str("peer ").Str(spec.addr).Str(" remote as 65001 state up")
				tb.Str(" peer-owner ").Uint(spec.owner)
				tb.Str(" connection-id ").Uint(spec.conn)
				tb.Str(" destination-session ").Uint(spec.session)
				tb.Str(" session-generation ").Uint(spec.conn + 1000)
				if reason != "" {
					tb.Str(" reason ").Str(reason)
				}
				if initialReplay != 0 {
					tb.Str(" initial-replay ").Uint(initialReplay)
				}
				if spec.rsClient {
					tb.Str(" rs-client true")
				}
				if spec.isIBGP {
					tb.Str(" is-ibgp true")
				}
				tb.Byte('\n')
				rs.dispatchText(tb.String())
			},
		},
	} {
		t.Run(transport.name, func(t *testing.T) {
			rs := newTestRouteServer(t)

			var mu sync.Mutex
			var leasedOwners []uint64
			var forwardedTo [][]string
			var releasedIDs []uint64
			rs.sourceLeaseHook = func(_ context.Context, input rpc.SourceLeaseInput) error {
				mu.Lock()
				leasedOwners = append(leasedOwners, input.PeerOwner)
				mu.Unlock()
				return nil
			}
			rs.forwardCachedHook = func(_ []uint64, destinations []string) {
				mu.Lock()
				forwardedTo = append(forwardedTo, slices.Clone(destinations))
				mu.Unlock()
			}
			rs.releaseCachedHook = func(ids []uint64) {
				mu.Lock()
				releasedIDs = append(releasedIDs, ids...)
				mu.Unlock()
			}

			for _, spec := range []peerSpec{c1Spec, c2Spec, c3Spec} {
				transport.up(rs, spec, rpc.ReasonAttached, 0)
			}
			// Bring up non-RS-client i1 with a non-zero InitialReplay and no ReasonAttached:
			// it must NOT spawn replayForPeer and must NOT become an RS client target.
			transport.up(rs, i1Spec, "", 900)

			c1 := rsFixtureKey(c1Spec.addr, c1Spec.owner, c1Spec.conn)
			i1 := rsFixtureKey(i1Spec.addr, i1Spec.owner, i1Spec.conn)
			families := map[family.Family]bool{family.IPv4Unicast: true}

			rs.mu.RLock()
			i1Peer := rs.peers[i1]
			require.NotNil(t, i1Peer)
			require.False(t, i1Peer.RSClient, "non-RS-client must have RSClient=false")
			require.False(t, i1Peer.updateTarget(), "non-RS-client must not be an update target")
			require.Nil(t, i1Peer.replayDone, "non-RS-client UP must not start peer-up replay")

			// Positive: c1 targets every other RS client (c2, c3), not c1 and not i1.
			c1Targets := rs.selectForwardTargets(nil, c1, 10, families)
			// Negative 1: fastPathOwedTarget rejects non-RS-client i1.
			i1FastTarget := rs.fastPathOwedTarget(rsFixtureTarget(i1Spec.addr, i1Spec.owner, i1Spec.session), c1, 10, families)
			// Negative 2: i1 as source selects no RS clients.
			i1Targets := rs.selectForwardTargets(nil, i1, 10, families)
			rs.mu.RUnlock()

			// Wait for the lightweight non-RS-client session-ready goroutine to finish,
			// then verify no replay RPC (query-routes / cache-forward) ran for i1.
			for _, ch := range rs.replays.snapshot() {
				<-ch
			}
			require.Empty(t, rs.replays.snapshot(), "non-RS-client UP must leave no active replay goroutines")
			mu.Lock()
			require.Empty(t, forwardedTo, "non-RS-client UP must not replay routes")
			mu.Unlock()
			require.Equal(t, []string{c2Spec.addr, c3Spec.addr}, rsFixtureAddresses(c1Targets))
			require.False(t, i1FastTarget, "non-RS-client must not be a fast-path handback target")
			require.Empty(t, i1Targets, "non-RS-client source must not select any RS client targets")

			// Negative 2 (end-to-end text dispatch): UPDATE from i1 releases cache without
			// acquiring a source lease, recording withdrawals, or forwarding to RS clients.
			i1Update := "peer " + i1Spec.addr + " remote as 65001 received update 50" +
				" peer-owner " + strconv.FormatUint(i1Spec.owner, 10) +
				" connection-id " + strconv.FormatUint(i1Spec.conn, 10) +
				" source-receipt 501 source-policy-epoch 5" +
				" origin igp next-hop " + i1Spec.addr + " nlri ipv4/unicast add prefix 10.10.0.0/24\n"
			rs.dispatchText(i1Update)
			flushWorkers(t, rs)

			mu.Lock()
			require.Empty(t, leasedOwners, "non-RS-client UPDATE must not acquire a source lease")
			require.Empty(t, forwardedTo, "non-RS-client UPDATE must not forward to RS clients")
			require.Equal(t, []uint64{50}, releasedIDs, "non-RS-client UPDATE must release its cache entry")
			mu.Unlock()

			rs.withdrawalMu.Lock()
			require.Empty(t, rs.withdrawals[i1Spec.addr], "non-RS-client UPDATE must not enter withdrawal inventory")
			rs.withdrawalMu.Unlock()

			// Positive control (end-to-end text dispatch): UPDATE from RS client c1 forwards to c2 and c3 only.
			c1Update := "peer " + c1Spec.addr + " remote as 65001 received update 51" +
				" peer-owner " + strconv.FormatUint(c1Spec.owner, 10) +
				" connection-id " + strconv.FormatUint(c1Spec.conn, 10) +
				" source-receipt 502 source-policy-epoch 5" +
				" origin igp next-hop " + c1Spec.addr + " nlri ipv4/unicast add prefix 10.1.0.0/24\n"
			rs.dispatchText(c1Update)
			flushWorkers(t, rs)

			mu.Lock()
			require.Equal(t, []uint64{c1Spec.owner}, leasedOwners)
			require.Equal(t, [][]string{{c2Spec.addr, c3Spec.addr}}, forwardedTo)
			mu.Unlock()
		})
	}
}
