package reactor

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func forwardSocketBarrier(t *testing.T, r *Reactor) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, r.fwdPool.Barrier(ctx))
}

// The destination remains held while the same retained path becomes valid,
// invalid and valid again. Content superseding may remove the first announce,
// but recovery must remain after the withdrawal, including its pool releases.
func TestForwardValidationOverflowRecoveryReachesSocket(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.destination.settings.RSClient = true
	f.destination.settings.NextHopMode = NextHopSelf
	f.destination.refreshForwardFacts()
	mux := newMixedBufMux()
	mux.setByteBudget(1 << 20)
	f.r.fwdPool.setOverflowMux(mux)
	f.destination.sendingInitialRoutes.Store(1)
	release := func() {
		f.destination.sendingInitialRoutes.Store(0)
		f.r.fwdPool.wakeOverflow(fwdKey{peerAddr: f.destination.Settings().PeerKey()})
	}
	t.Cleanup(release)
	eligible := true
	ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return eligible }, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	route := storedIPv4Route(f.source.Settings().Address.String())
	api := &reactorAPIAdapter{r: f.r}
	for _, verdict := range []bool{true, false, true} {
		eligible = verdict
		require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
	}
	require.Empty(t, f.conn.written(), "the held destination must not drain before recovery is queued")
	release()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2)
	withdraw, err := wire.ParseUpdateSections(bodies[0])
	require.NoError(t, err)
	require.Equal(t, []byte{24, 10, 0, 0}, withdraw.Withdrawn(bodies[0]))
	announce, err := wire.ParseUpdateSections(bodies[1])
	require.NoError(t, err)
	require.Equal(t, []byte{24, 10, 0, 0}, announce.NLRI(bodies[1]), "eligible path must be installed last")
	expectedAttrs := mustHex(t, route.AttrHex)
	_, _, nextHop, found := attribute.AttrFind(expectedAttrs, attribute.AttrNextHop)
	require.True(t, found)
	copy(nextHop, f.destination.Settings().LocalAddress.AsSlice())
	require.Equal(t, expectedAttrs, announce.Attrs(bodies[1]))
	require.Zero(t, f.r.recentUpdates.Len(), "superseded and delivered cache references must all release")
	_, used := mux.Stats()
	require.Zero(t, used, "superseded and delivered overflow handles must all return")
	require.Equal(t, peerPoolSize, f.r.fwdPool.outgoingPool(fwdKey{peerAddr: f.destination.Settings().PeerKey()}).available())
}

// Model the RS source worker lagging behind ingress: native forwarding decides
// each received UPDATE first, and cached work runs only after both have arrived.
func TestForwardAIGPLiveOrderReachesSocket(t *testing.T) {
	for _, change := range []string{"withdrawal", "replacement", "disable-then-withdraw", "enable-fast-path-then-withdraw"} {
		t.Run(change, func(t *testing.T) {
			ribevents.RegisterValidationLookup(nil, nil)
			f := newAIGPReplayFixture(t, nil)
			f.metric(7)
			first := f.receive(t, f.body(t, 100))
			update, ok := f.r.recentUpdates.Get(first)
			require.True(t, ok)
			if change != "enable-fast-path-then-withdraw" {
				_, delivered := reactorForwardRS(f.r, update, first, f.source.Settings().Address, f.source)
				require.Zero(t, delivered)
			}
			if change == "disable-then-withdraw" {
				settings := *f.source.Settings()
				settings.AIGPSession = new(false)
				f.source.settings = &settings
				f.source.refreshForwardFacts()
			}
			body := makeUpdateBody([]byte{24, 10, 20, 0}, nil, nil)
			if change == "replacement" {
				withMetric := f.body(t, 200)
				var err error
				body, err = stripAIGPBody(make([]byte, len(withMetric)), withMetric)
				require.NoError(t, err)
			}
			second := f.receive(t, body)
			update, ok = f.r.recentUpdates.Get(second)
			require.True(t, ok)
			_, delivered := reactorForwardRS(f.r, update, second, f.source.Settings().Address, f.source)
			f.forward(t, first)
			if delivered == 0 {
				f.forward(t, second)
			}
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			require.Len(t, bodies, 2)
			last := bodies[len(bodies)-1]
			sections, err := wire.ParseUpdateSections(last)
			require.NoError(t, err)
			if change == "replacement" {
				require.Equal(t, []byte{24, 10, 20, 0}, sections.NLRI(last))
				_, present := aigpReceivedMetric(t, last)
				require.False(t, present, "the older AIGP generation must not replace its attribute-free successor")
			} else {
				require.Equal(t, []byte{24, 10, 20, 0}, sections.Withdrawn(last), "the destination must finish withdrawn")
			}
		})
	}
}

func TestForwardAIGPLiveGenerationEndsWithSourceSession(t *testing.T) {
	for _, boundary := range []string{"reconnect", "replace-peer"} {
		for _, pending := range []string{"source-worker", "destination-worker"} {
			t.Run(boundary+"/"+pending, func(t *testing.T) {
				entered, release := make(chan struct{}, 1), make(chan struct{})
				f := newAIGPReplayFixture(t, func(items []fwdItem) {
					for _, item := range items {
						if item.sourceMessageID != 0 {
							entered <- struct{}{}
							<-release
							return
						}
					}
				})
				t.Cleanup(func() {
					select {
					case <-release:
					default:
						close(release)
					}
				})
				f.metric(7)
				id := f.receive(t, f.body(t, 100))
				if pending == "destination-worker" {
					f.forward(t, id)
					select {
					case <-entered:
					case <-time.After(2 * time.Second):
						t.Fatal("live announcement did not enter destination worker")
					}
				}
				old := f.source
				old.setState(PeerStateConnecting)
				f.r.notifyPeerClosed(old, "forwarding regression")
				if boundary == "reconnect" {
					old.setState(PeerStateEstablished)
				} else {
					old.Stop()
					replacement, _ := newAnnouncePeer(t, old.Settings().Address.String())
					settings := *old.Settings()
					replacement.settings = &settings
					replacement.recvCtxID = f.ctxID
					f.r.peers[old.Settings().PeerKey()] = replacement
					f.source = replacement
				}
				if pending == "source-worker" {
					sel, err := selector.Parse(f.destination.Settings().Address.String())
					require.NoError(t, err)
					err = (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder"))
					require.ErrorIs(t, err, errForwardNoSource)
				}
				// The fresh source must own the control route even when the
				// prior Peer was removed, not merely re-established.
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, f.body(t, 100), false))
				setup := aigpSocketBodies(t, f.conn)
				require.Len(t, setup, 1)
				f.forward(t, f.receive(t, makeUpdateBody([]byte{24, 10, 20, 0}, nil, nil)))
				close(release)
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)[len(setup):]
				require.Len(t, bodies, 1, "old session's live announcement must not reach the destination")
				sections, err := wire.ParseUpdateSections(bodies[0])
				require.NoError(t, err)
				require.Equal(t, []byte{24, 10, 20, 0}, sections.Withdrawn(bodies[0]))
			})
		}
	}
}

// RFC 7606 Section 5.1 permits received mixed fields. RFC 7311 Section 3.4.3
// adds the distance to the next hop governing each announced route, and metric
// replay starts with the received attribute again.
func TestForwardAIGPMixedNextHopsReachSocket(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(10)
	mpNextHop := netip.MustParseAddr("2001:db8:231::1")
	protocol := redistevents.RegisterProtocol("aigp-mixed-forward-test")
	setMPMetric := func(cost uint32) {
		locrib.Default().Insert(family.IPv6Unicast, netip.PrefixFrom(mpNextHop, 128), locrib.Path{Source: protocol, Metric: cost})
	}
	setMPMetric(20)
	t.Cleanup(func() { locrib.Default().Remove(family.IPv6Unicast, netip.PrefixFrom(mpNextHop, 128), protocol, 0) })
	// One session endpoint cannot supply both address families. Use genuine
	// IPv6 next-hop self for MP and an export rewrite to the speaker's IPv4
	// interface for legacy NLRI. Both are local for AIGP accumulation.
	legacySelf := f.destination.settings.LocalAddress
	mpSelf := netip.MustParseAddr("2001:db8:232::fe")
	f.destination.settings.LocalAddress = mpSelf
	f.destination.session.nextHopScope.Store(&receiveNextHopScope{
		local:     mpSelf,
		addresses: []netip.Prefix{netip.PrefixFrom(legacySelf, 24)},
	})
	legacyHop := legacySelf.As4()
	f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(
		func(_, _ filterapi.PeerFilterInfo, payload []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
			sections, err := wire.ParseUpdateSections(payload)
			require.NoError(t, err)
			if len(sections.NLRI(payload)) != 0 {
				mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, legacyHop[:])
			}
			return true
		})
	for _, peer := range []*Peer{f.source, f.destination} {
		peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}})
		peer.refreshForwardFacts()
	}
	base := f.body(t, 100)
	sections, err := wire.ParseUpdateSections(base)
	require.NoError(t, err)
	attrs := bytes.Clone(sections.Attrs(base))
	community := []byte{0xfd, 0xe8, 0, 42}
	attrs = append(attrs, 0xc0, 8, byte(len(community)))
	attrs = append(attrs, community...)
	mpAnnounce := []byte{48, 0x20, 1, 0x0d, 0xb8, 0x12, 0}
	mpWithdraw := []byte{48, 0x20, 1, 0x0d, 0xb8, 0x13, 0}
	var mp [128]byte
	n := writeMPReach(mp[:], 0, family.IPv6Unicast, mpNextHop.AsSlice(), mpAnnounce)
	attrs = append(attrs, mp[:n]...)
	attrs = append(attrs, 0x80, 15, byte(3+len(mpWithdraw)), 0, 2, 1)
	attrs = append(attrs, mpWithdraw...)
	// Both original withdrawals name advertisements from this same source.
	// RFC 4271 Section 4.3 and RFC 4760 Section 3.
	require.NoError(t, ownershipWriterForward(t, f.destination, f.source,
		ownershipRailBody(family.IPv4Unicast, false, []byte{24, 10, 21, 0}, 0), false))
	require.NoError(t, ownershipWriterForward(t, f.destination, f.source,
		ownershipRailBody(family.IPv6Unicast, false, mpWithdraw, 0), false))
	setup := aigpSocketBodies(t, f.conn)
	require.Len(t, setup, 2)
	body := makeUpdateBody([]byte{24, 10, 21, 0}, attrs, sections.NLRI(base))
	original := bytes.Clone(body)
	f.forward(t, f.receive(t, body))
	forwardSocketBarrier(t, f.r)
	initial := aigpSocketBodies(t, f.conn)[len(setup):]
	require.Len(t, initial, 4)
	check := func(bodies [][]byte, want map[family.Family]uint64) {
		t.Helper()
		got := make(map[family.Family]uint64)
		for _, body := range bodies {
			wu := wireu.NewWireUpdate(body, f.ctxID)
			nlri, err := wu.NLRI()
			require.NoError(t, err)
			reach, err := wu.MPReach()
			require.NoError(t, err)
			if len(nlri) == 0 && reach == nil {
				continue
			}
			fam := family.IPv4Unicast
			if reach != nil {
				fam = reach.Family()
				require.Equal(t, mpAnnounce, []byte(reach)[5+len(reach.NextHopBytes()):])
				require.Equal(t, mpSelf.AsSlice(), reach.NextHopBytes())
			} else {
				require.Equal(t, []byte{24, 10, 20, 0}, nlri)
			}
			metric, present := aigpReceivedMetric(t, body)
			require.True(t, present)
			got[fam] = metric
			parsed, err := wire.ParseUpdateSections(body)
			require.NoError(t, err)
			if reach == nil {
				_, _, nextHop, found := attribute.AttrFind(parsed.Attrs(body), attribute.AttrNextHop)
				require.True(t, found)
				require.Equal(t, legacySelf.AsSlice(), nextHop)
			}
			_, _, value, found := attribute.AttrFind(parsed.Attrs(body), attribute.AttrCommunity)
			require.True(t, found)
			require.Equal(t, community, value)
		}
		require.Equal(t, want, got)
	}
	check(initial, map[family.Family]uint64{family.IPv4Unicast: 110, family.IPv6Unicast: 120})
	_, legacyWithdraw := validationParts(t, initial)
	require.Equal(t, []byte{24, 10, 21, 0}, legacyWithdraw)
	var nativeWithdraw []byte
	for _, forwarded := range initial {
		unreach, err := wireu.NewWireUpdate(forwarded, f.ctxID).MPUnreach()
		require.NoError(t, err)
		if unreach != nil {
			require.Equal(t, family.IPv6Unicast, unreach.Family())
			nativeWithdraw = append(nativeWithdraw, unreach.WithdrawnBytes()...)
		}
	}
	require.Equal(t, mpWithdraw, nativeWithdraw)
	require.Equal(t, original, body)
	f.metric(30)
	setMPMetric(40)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	all := aigpSocketBodies(t, f.conn)[len(setup):]
	require.Len(t, all, 6)
	check(all[4:], map[family.Family]uint64{family.IPv4Unicast: 130, family.IPv6Unicast: 140})
}

func TestForwardFlowSpecOnlyGatePreservesUnicastSections(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		name := "denied"
		if allowed {
			name = "authorized"
		}
		t.Run(name, func(t *testing.T) {
			ribevents.RegisterValidationLookup(nil, nil)
			f := newAIGPReplayFixture(t, nil)
			f.destination.settings.NextHopMode = NextHopUnchanged
			f.destination.settings.RSClient = true
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			for _, peer := range []*Peer{f.source, f.destination} {
				peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{family.IPv4Unicast: true, fam: true}})
				peer.refreshForwardFacts()
			}
			rule := []byte{5, 1, 24, 10, 0, 0}
			ribevents.RegisterFlowSpecLookup(func(key ribevents.ValidationRoute, _ uint64) bool {
				return allowed && key.NLRI == string(rule)
			}, nil, nil, nil)
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			attrs := mustHex(t, storedIPv4Route(f.source.Settings().Address.String()).AttrHex)
			f.forward(t, f.receive(t, makeUpdateBody(nil, attrs, []byte{24, 10, 0, 0})))
			forwardSocketBarrier(t, f.r)
			actions := []byte{0xc0, 16, 8, 0x80, 6, 0, 0, 0, 0, 0, 0}
			attrs = append(attrs, actions...)
			legacyAttrs := bytes.Clone(attrs)
			var mp [64]byte
			n := writeMPReach(mp[:], 0, fam, nil, rule)
			attrs = append(attrs, mp[:n]...)
			body := makeUpdateBody([]byte{24, 10, 0, 0}, attrs, []byte{24, 10, 0, 1})
			f.forward(t, f.receive(t, body))
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			announced, withdrawn := validationParts(t, bodies[1:])
			require.Equal(t, []byte{24, 10, 0, 1}, announced)
			require.Equal(t, []byte{24, 10, 0, 0}, withdrawn, "mandatory FlowSpec authorization must not swallow the explicit unicast withdrawal")
			for _, body := range bodies[1:] {
				sections, err := wire.ParseUpdateSections(body)
				require.NoError(t, err)
				if len(sections.NLRI(body)) != 0 {
					require.Equal(t, legacyAttrs, sections.Attrs(body), "unicast sibling attributes must survive FlowSpec authorization")
				}
			}
			flowAnnounce, flowWithdraw := flowForwardParts(t, bodies[1:])
			if allowed {
				require.Equal(t, rule, flowAnnounce)
				require.Empty(t, flowWithdraw)
			} else {
				require.Empty(t, flowAnnounce)
				require.Equal(t, rule, flowWithdraw)
			}
		})
	}
}

// AFI/SAFI does not identify the governing next-hop field: IPv4 unicast can
// arrive in MP_REACH as well. Replay must compare the metric for that route's
// received next hop before deciding whether its advertisement changed.
func TestForwardAIGPMixedIPv4NextHopsReplay(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(10)
	mpNextHop := netip.MustParseAddr("198.18.230.1")
	protocol := redistevents.RegisterProtocol("aigp-mp-ipv4-forward-test")
	setMPMetric := func(cost uint32) {
		locrib.Default().Insert(family.IPv4Unicast, netip.PrefixFrom(mpNextHop, 32), locrib.Path{Source: protocol, Metric: cost})
	}
	setMPMetric(20)
	t.Cleanup(func() { locrib.Default().Remove(family.IPv4Unicast, netip.PrefixFrom(mpNextHop, 32), protocol, 0) })
	base := f.body(t, 100)
	sections, err := wire.ParseUpdateSections(base)
	require.NoError(t, err)
	mpAnnounce := []byte{24, 10, 22, 0}
	var mp [64]byte
	n := writeMPReach(mp[:], 0, family.IPv4Unicast, mpNextHop.AsSlice(), mpAnnounce)
	attrs := append(bytes.Clone(sections.Attrs(base)), mp[:n]...)
	f.forward(t, f.receive(t, makeUpdateBody(nil, attrs, sections.NLRI(base))))
	forwardSocketBarrier(t, f.r)
	check := func(bodies [][]byte, legacyMetric, mpMetric uint64) {
		t.Helper()
		got := make(map[string]uint64)
		for _, body := range bodies {
			wu := wireu.NewWireUpdate(body, f.ctxID)
			nlri, err := wu.NLRI()
			require.NoError(t, err)
			reach, err := wu.MPReach()
			require.NoError(t, err)
			if reach != nil {
				require.Equal(t, family.IPv4Unicast, reach.Family())
				require.Equal(t, f.destination.settings.LocalAddress.AsSlice(), reach.NextHopBytes(),
					"native IPv4 MP next-hop self must reach the socket as four bytes")
				nlri = []byte(reach)[5+len(reach.NextHopBytes()):]
			}
			metric, present := aigpReceivedMetric(t, body)
			require.True(t, present)
			got[string(nlri)] = metric
		}
		require.Equal(t, map[string]uint64{
			string([]byte{24, 10, 20, 0}): legacyMetric,
			string(mpAnnounce):            mpMetric,
		}, got)
	}
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 2)
	check(initial, 110, 120)
	// The new legacy distance equals the MP route's previous distance. Using
	// the legacy field for MP replay would incorrectly suppress that replay.
	f.metric(20)
	setMPMetric(30)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	all := aigpSocketBodies(t, f.conn)
	require.Len(t, all, 4)
	check(all[2:], 120, 130)
}

// Configured IPv4 rewrites must use native bytes only where the MP family
// permits them. Read the final socket on both rails, including SR Policy's
// independent next-hop AFI and IPv6 unicast's incompatible-self withdrawal.
func TestForwardConfiguredIPv4MPNextHopFamily(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, tc := range []struct {
			name    string
			family  family.Family
			hop     string
			nlri    string
			allowed bool
		}{
			{"ipv4-unicast", family.IPv4Unicast, "198.18.231.1", "180a0900", true},
			{"ipv6-unicast", family.IPv6Unicast, "2001:db8:1::9", "4020010db800070000", false},
			{"ipv6-sr-policy", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy},
				"2001:db8:1::9", "c0000000070000002a20010db8000000000000000000000001", true},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := rfc2545ReceiveFixture(t, false, true, true)
				f.destination.settings.NextHopMode = NextHopSelf
				for _, peer := range []*Peer{f.source, f.destination} {
					caps := []capability.Capability{
						&capability.ASN4{ASN: peer.settings.LocalAS},
						&capability.Multiprotocol{AFI: tc.family.AFI, SAFI: tc.family.SAFI},
					}
					neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
						LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
					})
					peer.session.negotiated = neg
					peer.negotiated.Store(NewNegotiatedCapabilities(neg))
					peer.refreshForwardFacts()
				}
				raw := mustHex(t, tc.nlri)
				var mp [128]byte
				n := writeMPReach(mp[:], 0, tc.family, netip.MustParseAddr(tc.hop).AsSlice(), raw)
				attrs := mixedAttrs(mp[:n])
				if tc.family.SAFI == family.SAFISRPolicy {
					tunnel := teSRPolicyValue(0, false)
					attrs = append(attrs, 0xd0, byte(attribute.AttrTunnelEncap), byte(len(tunnel)>>8), byte(len(tunnel)))
					attrs = append(attrs, tunnel...)
					attrs = append(attrs, 0xc0, byte(attribute.AttrExtCommunity), 8, 1, 2, 192, 0, 2, 2, 0, 0)
				}
				updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(attrs, nil))
				require.Len(t, updates, 1)
				update := updates[0]
				_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if tc.allowed {
					require.True(t, hasReach)
					require.False(t, hasUnreach)
					n = writeMPReach(mp[:], 0, tc.family, f.destination.settings.LocalAddress.AsSlice(), raw)
					_, _, want, found := attribute.AttrFind(mp[:n], attribute.AttrMPReachNLRI)
					require.True(t, found)
					require.Equal(t, want, reach)
				} else {
					require.False(t, hasReach)
					require.True(t, hasUnreach)
					require.Equal(t, mixedUnreachValue(uint16(tc.family.AFI), byte(tc.family.SAFI), raw), unreach)
				}
				require.Empty(t, update.NLRI)
				require.Empty(t, update.WithdrawnRoutes)
			})
		}
	}
}
