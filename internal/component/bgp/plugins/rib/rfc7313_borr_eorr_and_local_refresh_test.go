// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7313 received BoRR/EoRR and locally initiated route refresh tests
// Related: rib_refresh_rx.go -- handleBoRRForPeer, handleEoRRForPeer
// Related: rib_commands.go -- outboundResend, resendFamiliesBracketed
// Related: rib_selected_refresh.go -- selectedDispatch.requestLocalRefresh
// RFC 7313 Section 4 -- see rfc/short/rfc7313.md

package rib

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func nlriWire(pfx netip.Prefix) []byte {
	var buf [24]byte
	return append([]byte(nil), store.PrefixToNLRIInto(pfx, buf[:])...)
}

func buildIPv4AnnounceWireBody(pfx netip.Prefix, spec aggContributorSpec) []byte {
	attrs := buildContributorWireAttrs(spec)
	nlri := nlriWire(pfx)
	body := make([]byte, 2+2+len(attrs)+len(nlri))
	// Withdrawn Routes Length = 0
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)
	return body
}

// TestRFC7313BoRRMarksStaleAndEoRRPurgesRemainingStale proves RFC 7313 Section 4
// receiver procedures on both DirectBridge structured (handleRefreshStructured)
// and real JSON encoder/ParseEvent (EventKindRefresh / EventKindBoRR / EventKindEoRR) rails:
//   - Subtype-0 normal refresh does not mark Adj-RIB-In routes stale.
//   - Unsolicited EoRR without a prior BoRR does not purge even already-stale routes.
//   - BoRR marks every route of <AFI, SAFI> from that peer stale while leaving
//     other families of that peer and other peers' routes fresh.
//   - Re-advertising a prefix through the production receive path between BoRR
//     and EoRR clears its stale mark.
//   - EoRR immediately removes still-stale routes of that exact (peer, <AFI, SAFI>)
//     from Adj-RIB-In, withdraws or fails over their best paths in Loc-RIB and
//     BestChange, and preserves refreshed routes as well as simultaneously-stale
//     routes in sibling families and sibling peers.
//
// RFC requirement: RFC7313-4-4 positive -- receiving BoRR for IPv4 unicast from peer 1 marks all IPv4 unicast routes from peer 1 stale on both DirectBridge structured and JSON (JSONEncoder.Message -> ParseEvent) rails.
// RFC requirement: RFC7313-4-4 negative -- receiving BoRR for IPv4 unicast from peer 1 leaves peer 1's IPv6 unicast routes and peer 2's IPv4 unicast routes fresh (StaleLevelFresh), and receiving a normal subtype-0 refresh does not mark Adj-RIB-In routes stale.
// RFC requirement: RFC7313-4-5 positive -- receiving EoRR for IPv4 unicast after BoRR immediately removes still-stale IPv4 routes from peer 1's Adj-RIB-In, removes the sole-source route from Loc-RIB with BestChangeWithdraw, and fails over a dual-homed prefix to peer 2's backup path.
// RFC requirement: RFC7313-4-5 negative -- routes re-advertised through the production receive path between BoRR and EoRR (fresh), simultaneously-stale routes in peer 1's sibling family (IPv6) and peer 2's Adj-RIB-In survive peer 1's IPv4 EoRR intact; an unsolicited EoRR received without a prior BoRR does not remove even already-stale routes.
func TestRFC7313BoRRMarksStaleAndEoRRPurgesRemainingStale(t *testing.T) {
	encCtxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	for _, rail := range []string{"structured", "json"} {
		t.Run(rail, func(t *testing.T) {
			peer1 := netip.MustParseAddr("192.0.2.1")
			peer2 := netip.MustParseAddr("192.0.2.2")
			peer1V6 := netip.MustParseAddr("2001:db8::1")

			r, loc, cap := newAggregateTestRIB(t, nil, peer1, peer2, peer1V6)
			encoder := bgpformat.NewJSONEncoder("test-host")

			pKeep := netip.MustParsePrefix("198.51.100.0/24")   // re-advertised during refresh -> stays
			pDrop := netip.MustParsePrefix("198.51.101.0/24")   // not re-advertised, sole source -> purged & withdrawn
			pBackup := netip.MustParsePrefix("198.51.102.0/24") // not re-advertised, peer 2 has backup -> fails over to peer 2
			pV6 := netip.MustParsePrefix("2001:db8:100::/48")   // sibling family on peer 1 -> untouched by IPv4 EoRR even while stale
			pPeer2 := netip.MustParsePrefix("198.51.103.0/24")  // sibling peer -> untouched by peer 1 EoRR even while stale

			insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, pKeep, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1,
			})
			insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, pDrop, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1,
			})
			// Peer 1 wins pBackup (shorter AS_PATH [65010]); peer 2 holds backup ([65020, 65021]).
			insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, pBackup, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1,
			})
			insertSelectedRoute(t, r, 2, peer2, 65020, 65000, family.IPv4Unicast, pBackup, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65020, 65021}, nextHop: peer2,
			})
			insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv6Unicast, pV6, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1V6,
			})
			insertSelectedRoute(t, r, 2, peer2, 65020, 65000, family.IPv4Unicast, pPeer2, aggContributorSpec{
				origin: attribute.OriginIGP, asPath: []uint32{65020}, nextHop: peer2,
			})

			peerInfoFor := func(owner uint64, addr netip.Addr, peerAS uint32) *plugin.PeerInfo {
				return &plugin.PeerInfo{
					Address:      addr,
					AddressStr:   addr.String(),
					LocalAddress: netip.MustParseAddr("10.0.0.2"),
					LocalAS:      65000,
					PeerAS:       rpc.RemoteASOf(peerAS),
					PeerOwner:    owner,
				}
			}

			deliverRR := func(owner uint64, addr netip.Addr, peerAS uint32, fam family.Family, subtype byte) {
				raw := []byte{byte(fam.AFI >> 8), byte(fam.AFI), subtype, byte(fam.SAFI)}
				if rail == "structured" {
					r.dispatchStructured(&rpc.StructuredEvent{
						EventType:   rpc.EventKindRefresh,
						PeerAddress: addr.String(),
						PeerOwner:   owner,
						RawMessage:  &bgptypes.RawMessage{Type: msgtype.TypeROUTEREFRESH, RawBytes: raw},
					})
					return
				}
				encoded := encoder.Message(peerInfoFor(owner, addr, peerAS), bgptypes.RawMessage{
					Type:      msgtype.TypeROUTEREFRESH,
					Direction: rpc.DirectionReceived,
					RawBytes:  raw,
					MessageID: 100 + uint64(subtype),
				}, plugin.FormatParsed)
				require.NotEmpty(t, encoded)
				ev := mustParseEvent(t, encoded)
				r.dispatch(ev)
			}

			readvertiseKeep := func() {
				body := buildIPv4AnnounceWireBody(pKeep, aggContributorSpec{
					origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1,
				})
				wu := wireu.NewWireUpdate(body, encCtxID)
				if rail == "structured" {
					r.dispatchStructured(&rpc.StructuredEvent{
						EventType:   rpc.EventKindUpdate,
						Direction:   rpc.DirectionReceived,
						PeerAddress: peer1.String(),
						PeerOwner:   1,
						PeerAS:      rpc.RemoteASOf(65010),
						LocalAS:     65000,
						RawMessage: &bgptypes.RawMessage{
							Type:       msgtype.TypeUPDATE,
							Direction:  rpc.DirectionReceived,
							RawBytes:   body,
							WireUpdate: wu,
							MessageID:  201,
						},
					})
					return
				}
				rawJSON := string(bgpformat.AppendMessage(nil, peerInfoFor(1, peer1, 65010), bgptypes.RawMessage{
					Type:       msgtype.TypeUPDATE,
					Direction:  rpc.DirectionReceived,
					RawBytes:   body,
					WireUpdate: wu,
					MessageID:  201,
				}, bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull}))
				require.NotEmpty(t, rawJSON)
				ev := mustParseEvent(t, rawJSON)
				r.dispatch(ev)
			}

			// RFC7313-4-4 negative: normal subtype-0 route refresh does NOT mark Adj-RIB-In stale.
			deliverRR(1, peer1, 65010, family.IPv4Unicast, 0)
			for _, pfx := range []netip.Prefix{pKeep, pDrop, pBackup} {
				entry, found := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pfx))
				require.True(t, found)
				require.Equal(t, storage.StaleLevelFresh, entry.StaleLevel, "normal subtype-0 refresh must leave %s fresh", pfx)
			}

			// RFC7313-4-5 negative: unsolicited EoRR (no prior BoRR) must NOT purge even
			// already-stale routes (e.g. Graceful Restart stale).
			r.bgpPeers[1].MarkFamilyStale(family.IPv4Unicast, storage.StaleLevelFresh+1)
			deliverRR(1, peer1, 65010, family.IPv4Unicast, 2)
			require.Equal(t, 3, r.bgpPeers[1].FamilyLen(family.IPv4Unicast),
				"unsolicited EoRR without prior BoRR must not purge already-stale routes")
			// Reset stale back to fresh before exercising the real BoRR/EoRR sequence.
			r.bgpPeers[1].MarkFamilyStale(family.IPv4Unicast, storage.StaleLevelFresh)

			// RFC7313-4-4 positive: BoRR marks all IPv4 unicast routes from peer 1 stale.
			deliverRR(1, peer1, 65010, family.IPv4Unicast, 1)
			for _, pfx := range []netip.Prefix{pKeep, pDrop, pBackup} {
				entry, found := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pfx))
				require.True(t, found)
				require.Greater(t, entry.StaleLevel, storage.StaleLevelFresh, "peer 1 IPv4 route %s must be stale after BoRR", pfx)
			}

			// RFC7313-4-4 negative: peer 1 IPv6 unicast and peer 2 IPv4 unicast stay fresh after peer 1 IPv4 BoRR.
			v6Entry, found := r.bgpPeers[1].Lookup(family.IPv6Unicast, nlriWire(pV6))
			require.True(t, found)
			require.Equal(t, storage.StaleLevelFresh, v6Entry.StaleLevel, "other family on peer 1 must remain fresh")
			p2Entry, found := r.bgpPeers[2].Lookup(family.IPv4Unicast, nlriWire(pPeer2))
			require.True(t, found)
			require.Equal(t, storage.StaleLevelFresh, p2Entry.StaleLevel, "other peer's IPv4 route must remain fresh")

			// Now ALSO start BoRR on peer 1 IPv6 and peer 2 IPv4 so both sibling routes are
			// simultaneously STALE when peer 1 IPv4 EoRR arrives — proving EoRR scoping!
			deliverRR(1, peer1, 65010, family.IPv6Unicast, 1)
			deliverRR(2, peer2, 65020, family.IPv4Unicast, 1)
			v6Stale, found := r.bgpPeers[1].Lookup(family.IPv6Unicast, nlriWire(pV6))
			require.True(t, found)
			require.Greater(t, v6Stale.StaleLevel, storage.StaleLevelFresh)
			p2Stale, found := r.bgpPeers[2].Lookup(family.IPv4Unicast, nlriWire(pPeer2))
			require.True(t, found)
			require.Greater(t, p2Stale.StaleLevel, storage.StaleLevelFresh)

			// Peer 1 re-advertises pKeep through the real receive path between BoRR and EoRR -> stale mark cleared!
			readvertiseKeep()
			keptEntry, found := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pKeep))
			require.True(t, found)
			require.Equal(t, storage.StaleLevelFresh, keptEntry.StaleLevel, "re-advertised route must clear stale mark")

			// RFC7313-4-5 positive & negative: peer 1 IPv4 EoRR immediately removes pDrop and pBackup
			// from peer 1's Adj-RIB-In, withdraws pDrop from Loc-RIB + BestChange, fails pBackup
			// over to peer 2, and preserves pKeep AND simultaneously-stale pV6 and pPeer2.
			cap.reset()
			deliverRR(1, peer1, 65010, family.IPv4Unicast, 2)

			_, keepFound := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pKeep))
			require.True(t, keepFound, "re-advertised route must survive EoRR")
			_, dropFound := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pDrop))
			require.False(t, dropFound, "still-stale pDrop must be immediately removed on EoRR")
			_, backupP1Found := r.bgpPeers[1].Lookup(family.IPv4Unicast, nlriWire(pBackup))
			require.False(t, backupP1Found, "still-stale pBackup from peer 1 must be immediately removed on EoRR")

			v6After, v6Still := r.bgpPeers[1].Lookup(family.IPv6Unicast, nlriWire(pV6))
			require.True(t, v6Still, "simultaneously-stale sibling family must survive IPv4 EoRR")
			require.Greater(t, v6After.StaleLevel, storage.StaleLevelFresh)
			p2After, p2Still := r.bgpPeers[2].Lookup(family.IPv4Unicast, nlriWire(pPeer2))
			require.True(t, p2Still, "simultaneously-stale sibling peer must survive peer 1 EoRR")
			require.Greater(t, p2After.StaleLevel, storage.StaleLevelFresh)

			// Loc-RIB and BestChange reflect immediate removal of pDrop and failover of pBackup to peer 2.
			_, dropInLoc := loc.Lookup(family.IPv4Unicast, pDrop)
			require.False(t, dropInLoc, "purged sole-source prefix must be removed from Loc-RIB")
			bcDrop, hasBCDrop := cap.lastChangeFor(pDrop)
			require.True(t, hasBCDrop)
			require.Equal(t, ribevents.BestChangeWithdraw, bcDrop.Action)

			backupLoc, backupInLoc := loc.Lookup(family.IPv4Unicast, pBackup)
			require.True(t, backupInLoc, "pBackup must fail over to peer 2 in Loc-RIB")
			require.Equal(t, peer2, backupLoc.Paths[backupLoc.Best].NextHop)
		})
	}
}

// TestRFC7313LocallyInitiatedRouteRefreshSendsBoRRBeforeRoutes proves RFC 7313
// Section 4 locally initiated route refresh (`clear bgp rib out`) sends BoRR
// before re-advertising Adj-RIB-Out routes and EoRR after, on both legacy
// SENT-backed destinations and admitted ordinary destinations (without duplicate
// legacy dispatch), while skipping down peers, non-matching selectors, and empty
// families.
//
// RFC requirement: RFC7313-4-13 positive -- invoking locally initiated outbound refresh (`clear bgp rib out`) on an established peer emits BoRR before any re-advertised Adj-RIB-Out route and EoRR after, on both legacy (`resendFamiliesBracketed`) and ordinary (`selectedDispatch.requestLocalRefresh`) destinations, with no duplicate legacy refresh on an ordinary destination that also holds SENT entries in ribOut.
// RFC requirement: RFC7313-4-13 negative -- invoking `clear bgp rib out` for a down peer, non-matching selector, or family with no Adj-RIB-Out routes emits no BoRR, no route, and no EoRR on both legacy and ordinary destinations.
func TestRFC7313LocallyInitiatedRouteRefreshSendsBoRRBeforeRoutes(t *testing.T) {
	// Part 1: legacy SENT-backed peer via `clear bgp rib out`.
	r, sequence := rfc7313SequencedRIB(t)
	status, data, err := r.handleCommand("clear bgp rib out", "*", []string{rfc7313RefreshPeer, "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	resMap, ok := data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, resMap["peers"])
	require.Equal(t, 2, resMap["resent"])
	require.GreaterOrEqual(t, len(*sequence), 3)
	require.Equal(t, "marker request peer "+rfc7313RefreshPeer+" borr ipv4/unicast", (*sequence)[0],
		"locally initiated refresh must send BoRR before any re-advertised route")
	require.Equal(t, "marker request peer "+rfc7313RefreshPeer+" eorr ipv4/unicast", (*sequence)[len(*sequence)-1],
		"locally initiated refresh must send EoRR after all re-advertised routes")
	var decodedRoutes []refreshRouteIdentity
	for _, item := range (*sequence)[1 : len(*sequence)-1] {
		command, hasPrefix := strings.CutPrefix(item, "route ")
		require.True(t, hasPrefix, "only route commands belong between BoRR and EoRR: %q", item)
		if command == "update cursor done" {
			continue
		}
		decodedRoutes = append(decodedRoutes, consumedRefreshRoutes(t, consumeRefreshCommand(t, command))...)
	}
	require.ElementsMatch(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")},
	}, decodedRoutes, "re-advertised Adj-RIB-Out must carry both IPv4 prefixes and no other family")

	// Legacy Negative 1: non-matching selector emits no BoRR/routes/EoRR.
	*sequence = nil
	status, data, err = r.handleCommand("clear bgp rib out", "*", []string{"peer 192.0.2.254", "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.Empty(t, *sequence)

	// Legacy Negative 2: family with no Adj-RIB-Out routes emits no BoRR/EoRR.
	*sequence = nil
	status, data, err = r.handleCommand("clear bgp rib out", "*", []string{rfc7313RefreshPeer, "ipv4/multicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.Empty(t, *sequence)

	// Legacy Negative 3: down peer emits no BoRR/EoRR.
	r.peerMu.Lock()
	r.peerUp[1] = false
	r.peerMu.Unlock()
	*sequence = nil
	status, data, err = r.handleCommand("clear bgp rib out", "*", []string{rfc7313RefreshPeer, "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.Empty(t, *sequence)

	// Part 2: admitted ordinary destination via `clear bgp rib out` -> selectedDispatch.requestLocalRefresh.
	family.RegisterTestFamilies()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var mu sync.Mutex
	var events []string
	d, err := newSelectedDispatch(ctx,
		func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
			mu.Lock()
			for range input.Routes {
				events = append(events, "route")
			}
			mu.Unlock()
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		},
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		mu.Lock()
		switch input.Action {
		case rpc.SelectedRefreshBegin:
			events = append(events, "borr:"+input.Family)
		case rpc.SelectedRefreshEnd:
			events = append(events, "eorr:"+input.Family)
		}
		mu.Unlock()
		return selectedRefreshReceipt(input), nil
	}))
	defer func() { _ = d.stop(context.Background()) }()

	ordPeer := netip.MustParseAddr("192.0.2.55")
	ordRIB := newTestRIBManager(t)
	ordRIB.selectedDispatch = d
	var legacyCmds []string
	ordRIB.dispatchHook = func(cmd string) { legacyCmds = append(legacyCmds, cmd) }
	ordRIB.updateHook = func(cmd string, _ map[string]any) { legacyCmds = append(legacyCmds, cmd) }

	ordRIB.peerMu.Lock()
	ordRIB.peerUp[55] = true
	ordRIB.peerMeta[55] = &peerMetadata{
		Address:         ordPeer,
		Session:         101,
		refreshOwner:    55,
		refreshSession:  101,
		refreshOrdinary: true,
	}
	// Populate ribOut for this ordinary destination as handleSentStructured does in production;
	// outboundResend MUST NOT run a duplicate legacy resend alongside selectedDispatch.
	ordRIB.ribOut[55] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.55.0.0/24": {MsgID: 1, Family: family.IPv4Unicast, Prefix: "10.55.0.0/24", NextHop: "192.0.2.1"},
		},
	})
	ordRIB.peerMu.Unlock()

	route := selectedDispatchRoute(55, 0x5500)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationOwnerUp(ordPeer, 55, 101))
	require.NoError(t, d.barrier(ctx, ordPeer))

	mu.Lock()
	events = nil
	mu.Unlock()

	// Ordinary Positive: brackets BoRR -> route -> EoRR once via selectedDispatch, zero legacy duplicate!
	status, data, err = ordRIB.handleCommand("clear bgp rib out", "*", []string{ordPeer.String(), "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	resMap, ok = data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, resMap["peers"], "ordinary destination must be counted exactly once (no double legacy refresh)")
	require.Empty(t, legacyCmds, "ordinary destination must not trigger legacy bare-IP resend")

	require.NoError(t, d.barrier(ctx, ordPeer))

	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()
	require.GreaterOrEqual(t, len(got), 3)
	require.Equal(t, "borr:ipv4/unicast", got[0], "BoRR must precede re-advertised routes on locally initiated ordinary refresh")
	require.Equal(t, "eorr:ipv4/unicast", got[len(got)-1], "EoRR must follow re-advertised routes on locally initiated ordinary refresh")
	require.True(t, strings.HasPrefix(got[1], "route"), "Adj-RIB-Out route must appear between BoRR and EoRR")

	// Ordinary Negative 1: non-matching selector queues nothing.
	mu.Lock()
	events = nil
	mu.Unlock()
	status, data, err = ordRIB.handleCommand("clear bgp rib out", "*", []string{"peer 192.0.2.254", "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.NoError(t, d.barrier(ctx, ordPeer))
	mu.Lock()
	require.Empty(t, events)
	mu.Unlock()

	// Ordinary Negative 2: empty family with no Adj-RIB-Out routes queues nothing.
	status, data, err = ordRIB.handleCommand("clear bgp rib out", "*", []string{ordPeer.String(), "ipv4/multicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.NoError(t, d.barrier(ctx, ordPeer))
	mu.Lock()
	require.Empty(t, events)
	mu.Unlock()

	// Ordinary Negative 3: down peer queues nothing.
	ordRIB.peerMu.Lock()
	ordRIB.peerUp[55] = false
	ordRIB.peerMu.Unlock()
	status, data, err = ordRIB.handleCommand("clear bgp rib out", "*", []string{ordPeer.String(), "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, 0, data.(map[string]any)["peers"])
	require.NoError(t, d.barrier(ctx, ordPeer))
	mu.Lock()
	require.Empty(t, events)
	mu.Unlock()
}
