package reactor

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestAPIBatchRefusesUnusableIPv6NextHop checks the established API batch entry
// against exact delivered bytes, its refusal error and production send counters.
//
// RFC requirement: RFC2545-3-1 negative -- unusable IPv6-unicast global next hops
// are refused before the established API batch reaches the recipient.
// RFC requirement: RFC2545-3-1 positive -- a genuine IPv6 global is retained.
// RFC requirement: RFC2545-3-2 negative -- native IPv4 cannot produce a four-octet
// next-hop field for IPv6 unicast.
// RFC requirement: RFC2545-3-2 positive -- the global-only field is sixteen octets.
// MUTATION: bypass unicast admission in buildBatchAnnounceUpdate; invalid cases
// must still return the encoding refusal and leave zero bytes and sent updates.
func TestAPIBatchRefusesUnusableIPv6NextHop(t *testing.T) {
	testAPIOriginateIPv6NextHop(t, "batch", false)
}

// TestQueuedOriginateRefusesUnusableIPv6NextHop admits through the API while the
// initial-sync gate is closed, then drains through the actual Session writer.
//
// RFC requirement: RFC2545-3-1 negative -- unusable IPv6-unicast next hops leave
// only End-of-RIB, not an announcement, when queued initial synchronization drains.
// RFC requirement: RFC2545-3-1 positive -- a genuine IPv6 global is retained.
// RFC requirement: RFC2545-3-2 negative -- native IPv4 cannot produce a four-octet
// next-hop field for IPv6 unicast on the queued rail.
// RFC requirement: RFC2545-3-2 positive -- the global-only field is sixteen octets.
// MUTATION: bypass unicast admission in buildRIBRouteUpdate; invalid queued routes
// must still leave only End-of-RIB and no reachable-advertisement accounting.
func TestQueuedOriginateRefusesUnusableIPv6NextHop(t *testing.T) {
	testAPIOriginateIPv6NextHop(t, "queued", false)
}

// TestNamedCommitRefusesUnusableIPv6NextHop drives SendRoutes through the actual
// CommitService writer and checks its reported refusal alongside delivered bytes.
//
// RFC requirement: RFC2545-3-1 negative -- unusable IPv6-unicast next hops are
// refused by a named commit with no announced route counted or written.
// RFC requirement: RFC2545-3-1 positive -- a genuine IPv6 global is retained.
// RFC requirement: RFC2545-3-2 negative -- native IPv4 cannot produce a four-octet
// next-hop field for IPv6 unicast through a named commit.
// RFC requirement: RFC2545-3-2 positive -- the global-only field is sixteen octets.
// MUTATION: bypass unicast admission in CommitService.buildMPReachNLRI; malformed
// routes must still report AnnounceRefused and emit only the requested End-of-RIB.
func TestNamedCommitRefusesUnusableIPv6NextHop(t *testing.T) {
	testAPIOriginateIPv6NextHop(t, "commit", false)
}

// TestOriginatedIPv6NextHopAdmissionAfterExportPolicy drives ordinary origination
// through the configured export chain, whose replacement can invalidate an
// otherwise usable next hop after every builder check has already succeeded.
//
// RFC requirement: RFC2545-3-1 negative -- invalid export-policy next hops do not
// reach the socket or earn successful advertisement accounting.
// RFC requirement: RFC2545-3-1 positive -- global and negotiated link-local
// next-hop overrides retain their exact bytes.
// RFC requirement: RFC2545-3-2 negative -- a four-octet policy next hop cannot
// encode an IPv6-unicast announcement.
// RFC requirement: RFC2545-3-2 positive -- a usable sixteen-octet field survives.
// MUTATION: remove final ordinary-writer admission after the export override;
// invalid policy output must still be refused on each real producer rail.
func TestOriginatedIPv6NextHopAdmissionAfterExportPolicy(t *testing.T) {
	for _, rail := range []string{"batch", "queued", "commit"} {
		t.Run(rail, func(t *testing.T) {
			testAPIOriginateIPv6NextHop(t, rail, true)
		})
	}
	t.Run("forward-queued", testOriginatedPolicyForwardQueue)
}

// testAPIOriginateIPv6NextHop keeps the recipient distinct from every candidate
// and checks all three rails with the same literal UPDATE oracle.
func testAPIOriginateIPv6NextHop(t *testing.T, rail string, postPolicy bool) {
	t.Helper()
	type testCase struct {
		name        string
		nextHop     netip.Addr
		cap77       bool
		wireHop     string
		mixed       bool
		wireOnlyHop string
	}
	cases := []testCase{
		{name: "loopback", nextHop: netip.MustParseAddr("::1")},
		{name: "unspecified", nextHop: netip.MustParseAddr("::")},
		{name: "multicast", nextHop: netip.MustParseAddr("ff0e::1")},
		{name: "mapped", nextHop: netip.MustParseAddr("::ffff:192.0.2.1"), wireHop: "00000000000000000000ffffc0000201"},
		{name: "ipv4", nextHop: netip.MustParseAddr("192.0.2.1")},
		{name: "unset"},
		{name: "global", nextHop: netip.MustParseAddr("2001:db8:1::1"), wireHop: "20010db8000100000000000000000001"},
		{name: "cap77", nextHop: netip.MustParseAddr("fe80::1"), cap77: true, wireHop: "fe800000000000000000000000000001"},
	}
	if rail == "commit" || postPolicy {
		cases = append(cases, testCase{name: "link-local-unnegotiated", nextHop: netip.MustParseAddr("fe80::1")})
	}
	if postPolicy {
		cases = append(cases, testCase{name: "mixed-invalid-ipv6-valid-ipv4", nextHop: netip.MustParseAddr("::1"), mixed: true})
		cases = append(cases,
			testCase{name: "vpn-shaped-24-octet-unicast", nextHop: netip.MustParseAddr("2001:db8:1::1"),
				wireOnlyHop: "000000000000000020010db8000100000000000000000001"},
			testCase{name: "vpn-shaped-48-octet-unicast", nextHop: netip.MustParseAddr("2001:db8:1::1"),
				wireOnlyHop: "000000000000000020010db80001000000000000000000010000000000000000fe800000000000000000000000000001"},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			families := []family.Family{family.IPv6Unicast}
			if tc.mixed {
				families = append(families, family.IPv4Unicast)
			}
			initial, conn := newInitialSyncPeer(t, true, families...)
			initial.settings.Address = netip.MustParseAddr("2001:db8:2::2")
			r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000})
			if err := r.AddPeer(initial.settings); err != nil {
				t.Fatal(err)
			}
			r.mu.RLock()
			peer, found := r.findPeerByAddr(initial.settings.Address)
			r.mu.RUnlock()
			if !found {
				t.Fatal("configured recipient was not registered")
			}
			peer.state.Store(int32(PeerStateEstablished))
			peer.negotiated.Store(initial.negotiated.Load())
			peer.negotiated.Load().LinkLocalNextHop = tc.cap77
			peer.session = initial.session
			var caps []capability.Capability
			if tc.cap77 {
				caps = append(caps, &capability.LinkLocalNextHop{})
			}
			peer.session.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
			// AddPeer installed the production accounting callback. Keep it on
			// the actual Session rather than counting test-observed writes.
			peer.session.SetMessageCallback(peer.messageCallback)
			peer.refreshLinkScopeFrom(nil)
			if rail == "queued" {
				peer.sendingInitialRoutes.Store(1)
				peer.initialSyncEOROwed.Store(true)
			}
			api := &reactorAPIAdapter{r: r}
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv6Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0)},
				NextHop: bgptypes.NewNextHopExplicit(tc.nextHop),
			}
			sibling := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), 0)},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
			}
			laterIPv6 := rib.NewRouteWithASPath(
				nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:8::/64"), 0),
				netip.MustParseAddr("2001:db8:9::1"), nil, nil)
			policyCalls := 0
			if postPolicy {
				batch.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8:1::1"))
				// Literal wire construction deliberately permits bad address roles
				// that the normal builder now refuses before the policy boundary.
				hop := tc.nextHop.AsSlice()
				if tc.wireOnlyHop != "" {
					hop = mustHex(t, tc.wireOnlyHop)
				}
				mp := append([]byte{0, 2, 1, byte(len(hop))}, hop...)
				mp = append(mp, mustHex(t, "004020010db800070000")...)
				attrs := mustHex(t, "4001010040020602010000fde8")
				attrs = append(attrs, 0x80, 0x0e, byte(len(mp)))
				attrs = append(attrs, mp...)
				body := append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
				peer.settings.ExportFilters = []filterapi.FilterRef{{Name: "origination:replace-next-hop"}}
				peer.refreshForwardFacts()
				r.api = &pluginserver.Server{}
				var ipv6Policy bool
				r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					policyCalls++
					if !ipv6Policy {
						return PolicyResponse{Action: PolicyAccept}
					}
					return PolicyResponse{Action: PolicyModify, Raw: body}
				}
				peer.session.egressRouteFilter = func(body []byte) (bool, []byte) {
					ipv6Policy = payloadNextHop(body).mp == batch.NextHop.Addr
					return r.exportFilterForBody(peer, body)
				}
			}

			accepted := tc.wireHop != ""
			var err error
			if rail == "commit" {
				peer.sendCtx.Store(bgpctx.NewEncodingContext(
					&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
					&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionSend))
				routes := []*rib.Route{rib.NewRouteWithASPath(batch.NLRIs[0], batch.NextHop.Addr, nil, nil)}
				if tc.mixed {
					routes = append(routes, rib.NewRouteWithASPath(sibling.NLRIs[0], sibling.NextHop.Addr, nil, nil), laterIPv6)
					groups := rib.GroupByAttributesTwoLevel(routes)
					if len(groups) != 3 || !bytes.Equal(groups[1].NextHop, batch.NextHop.Addr.AsSlice()) ||
						!bytes.Equal(groups[2].NextHop, laterIPv6.NextHop().AsSlice()) {
						t.Fatal("mixed commit fixture must order the refused IPv6 group before its usable IPv6 sibling")
					}
				}
				// RFC 2545 Section 3: named commits must enforce the same
				// address role before their direct MP_REACH writer.
				result, commitErr := api.SendRoutes(selector.All(), routes, nil, true, plugin.OperatorSender())
				if commitErr != nil {
					t.Fatal(commitErr)
				}
				if len(result.Peers) != 1 {
					t.Fatalf("commit peer rows = %d, want 1", len(result.Peers))
				}
				announced := 0
				if accepted {
					announced = 1
					if len(result.Peers[0].Reasons) != 0 {
						t.Errorf("valid commit refusal reasons = %v", result.Peers[0].Reasons)
					}
				} else if reasons := result.Peers[0].Reasons; len(reasons) != 1 || reasons[0] != bgptypes.CommitReasonAnnounceRefused {
					t.Errorf("commit reasons = %v, want [%s]", reasons, bgptypes.CommitReasonAnnounceRefused)
				}
				wantEOR := 1
				if tc.mixed {
					announced++
					announced++ // The usable IPv6 group sorts AFTER the refused one.
					wantEOR++
				}
				if result.RoutesAnnounced != announced || result.UpdatesSent != announced+wantEOR || result.EORSent != wantEOR {
					t.Errorf("commit announced/updates/EOR = %d/%d/%d, want %d/%d/%d",
						result.RoutesAnnounced, result.UpdatesSent, result.EORSent, announced, announced+wantEOR, wantEOR)
				}
			} else {
				// RFC 2545 Section 3: enter through the operator batch API.
				err = api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
				if tc.mixed {
					if siblingErr := api.AnnounceNLRIBatch(t.Context(), selector.All(), sibling, plugin.OperatorSender()); siblingErr != nil {
						t.Fatalf("usable native IPv4 sibling refused after invalid IPv6: %v", siblingErr)
					}
				}
			}
			if rail == "queued" {
				if err != nil {
					t.Fatalf("queue admission failed: %v", err)
				}
				wantQueued := 1
				if tc.mixed {
					wantQueued++
				}
				if len(peer.opQueue) != wantQueued {
					t.Fatalf("queued operations = %d, want %d", len(peer.opQueue), wantQueued)
				}
				if len(conn.written()) != 0 {
					t.Fatal("queued announcement escaped before initial synchronization")
				}
				// RFC 2545 Section 3 and RFC 4724 Section 4: the actual drain
				// refuses the route while still completing End-of-RIB.
				peer.sendInitialRoutes()
				if len(peer.opQueue) != 0 {
					t.Errorf("queued operations = %d after drain, want 0", len(peer.opQueue))
				}
			} else if accepted {
				if err != nil {
					t.Errorf("valid API announcement failed: %v", err)
				}
			} else if rail == "batch" {
				wantErr := errAnnounceNextHopUnencodable
				if postPolicy {
					wantErr = message.ErrUnicastNextHopUnusable
				}
				if !errors.Is(err, wantErr) {
					t.Errorf("API refusal = %v, want %v", err, wantErr)
				}
			}

			var want []byte
			var updates, eors uint32
			if accepted {
				want = mustHex(t, "ffffffffffffffffffffffffffffffff0045020000002e4001010040020602010000fde8800e1e00020110"+tc.wireHop+"004020010db800070000")
				updates++
			}
			if tc.mixed {
				want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff002f02000000144001010040020602010000fde8400304c000020118c63364")...)
				updates++
				if rail == "commit" {
					want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff0045020000002e4001010040020602010000fde8800e1e0002011020010db8000900000000000000000001004020010db800080000")...)
					updates++
				}
			}
			announcementBytes := len(want)
			if rail != "batch" {
				want = append(want, eorWire(family.IPv6Unicast)...)
				updates++
				eors++
				if tc.mixed {
					want = append(want, eorWire(family.IPv4Unicast)...)
					updates++
					eors++
				}
			}
			got := conn.written()
			matches := bytes.Equal(got, want)
			if tc.mixed && rail != "batch" && !matches {
				// Both family-completion orders are legal; each marker must
				// still follow the usable announcements exactly once.
				alternate := append(bytes.Clone(want[:announcementBytes]), eorWire(family.IPv4Unicast)...)
				alternate = append(alternate, eorWire(family.IPv6Unicast)...)
				matches = bytes.Equal(got, alternate)
			}
			if !matches {
				t.Errorf("next hop %v: wire = %x, want %x", tc.nextHop, got, want)
			}
			if stats := peer.Stats(); stats.UpdatesSent != updates || stats.EORSent != eors {
				t.Errorf("sent updates/EOR = %d/%d, want %d/%d", stats.UpdatesSent, stats.EORSent, updates, eors)
			}
			if got := peer.hasAdvertised(); got != (accepted || tc.mixed) {
				t.Errorf("reachable advertisement recorded = %v, want %v", got, accepted || tc.mixed)
			}
			wantPolicyCalls := 1
			if tc.mixed {
				wantPolicyCalls++
				if rail == "commit" {
					wantPolicyCalls++
				}
			}
			if postPolicy && policyCalls != wantPolicyCalls {
				t.Errorf("export policy calls = %d, want %d", policyCalls, wantPolicyCalls)
			}
		})
	}
}

// testOriginatedPolicyForwardQueue uses the API's captured-owner maintenance
// rail, not a synthetic fwdItem. Both replacements are accepted into overflow
// while the worker is parked on the API-generated wake sentinel. Only then does
// the ordering hold open, so one stable overflow snapshot delivers both real
// items to the same worker batch, which must skip a refused first announcement
// and still write and flush its healthy native IPv4 sibling.
func testOriginatedPolicyForwardQueue(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		name := "global-control"
		if refuse {
			name = "refused-first"
		}
		t.Run(name, func(t *testing.T) {
			initial, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
			initial.settings.Address = netip.MustParseAddr("2001:db8:2::2")
			r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000})
			if err := r.AddPeer(initial.settings); err != nil {
				t.Fatal(err)
			}
			r.mu.RLock()
			peer, found := r.findPeerByAddr(initial.settings.Address)
			r.mu.RUnlock()
			if !found {
				t.Fatal("queued recipient was not registered")
			}
			peer.state.Store(int32(PeerStateEstablished))
			peer.negotiated.Store(initial.negotiated.Load())
			peer.session = initial.session
			peer.session.SetMessageCallback(peer.messageCallback)
			peer.refreshLinkScopeFrom(nil)
			api := &reactorAPIAdapter{r: r}
			first := bgptypes.NLRIBatch{
				Family:  family.IPv6Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0)},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8:1::1")),
			}
			later := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), 0)},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
			}
			for _, batch := range []*bgptypes.NLRIBatch{&first, &later} {
				if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), *batch, plugin.OperatorSender()); err != nil {
					t.Fatal(err)
				}
				batch.SentOwnerMessage = peer.session.sentReceipt.MessageID()
				if batch.SentOwnerMessage == 0 {
					t.Fatal("seed advertisement has no real ownership receipt")
				}
			}
			before := bytes.Clone(conn.written())
			beforeStats := peer.Stats()
			later.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.99"))
			hop := "20010db8000300000000000000000001"
			if refuse {
				hop = "00000000000000000000000000000001"
			}
			override := mustHex(t, "0000002e4001010040020602010000fde8800e1e00020110"+hop+"004020010db800070000")
			peer.settings.ExportFilters = []filterapi.FilterRef{{Name: "queued:replace-first-next-hop"}}
			peer.refreshForwardFacts()
			r.api = &pluginserver.Server{}
			var ipv6Policy bool
			r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				if ipv6Policy {
					return PolicyResponse{Action: PolicyModify, Raw: override}
				}
				return PolicyResponse{Action: PolicyAccept}
			}
			peer.session.egressRouteFilter = func(body []byte) (bool, []byte) {
				ipv6Policy = payloadNextHop(body).mpFamily == family.IPv6Unicast
				return r.exportFilterForBody(peer, body)
			}
			widths := make(chan int, 4)
			workerParked := make(chan struct{}, 1)
			workerGate := make(chan struct{})
			releaseWorker := sync.OnceFunc(func() { close(workerGate) })
			pool := newFwdPool(func(key fwdKey, items []fwdItem) {
				// dispatchOverflow itself queues the wake sentinel. Park that
				// callback before runWorker can start drainOverflow: the hold is
				// read per item in takeOverflowReleased, so flipping it during
				// that scan need not release both items in the same snapshot.
				select {
				case workerParked <- struct{}{}:
				default:
				}
				<-workerGate
				count := 0
				for i := range items {
					if items[i].peer != nil {
						count++
					}
				}
				if count != 0 {
					widths <- count
				}
				fwdBatchHandler(key, items)
			}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
			r.fwdPool = pool
			peer.sendingInitialRoutes.Store(1)
			t.Cleanup(func() {
				peer.sendingInitialRoutes.Store(0)
				// MUST release the worker before Stop waits for it, including
				// timeout/fatal exits while the ordering hold is still closed.
				releaseWorker()
				peer.wakeForwardOverflow()
				pool.Stop()
			})
			for _, batch := range []bgptypes.NLRIBatch{first, later} {
				// This API acknowledges queuing, not eventual policy admission.
				if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
					t.Fatalf("maintenance replacement was not queued: %v", err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			select {
			case <-workerParked:
			case <-ctx.Done():
				t.Fatal("API-generated wake sentinel never reached the worker fence")
			}
			if !bytes.Equal(conn.written(), before) {
				t.Fatal("queued replacement escaped its ordering hold")
			}
			peer.sendingInitialRoutes.Store(0)
			// No drain can observe the hold transition: both real API items
			// are queued and the worker is parked before its overflow scan.
			releaseWorker()
			peer.wakeForwardOverflow()
			if err := pool.Barrier(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case count := <-widths:
				if count != 2 {
					t.Fatalf("fixture processed %d real items in the first batch, want 2", count)
				}
			default:
				t.Fatal("queued replacements never reached the actual batch writer")
			}
			want := bytes.Clone(before)
			added := uint32(1)
			if !refuse {
				want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff004502")...)
				want = append(want, override...)
				added++
			}
			want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff002f02000000144001010040020602010000fde8400304c000026318c63364")...)
			if got := conn.written(); !bytes.Equal(got, want) {
				t.Errorf("queued writer wire = %x, want %x", got, want)
			}
			if stats := peer.Stats(); stats.UpdatesSent != beforeStats.UpdatesSent+added || stats.EORSent != beforeStats.EORSent {
				t.Errorf("queued writer updates/EOR = %d/%d, want %d/%d", stats.UpdatesSent, stats.EORSent, beforeStats.UpdatesSent+added, beforeStats.EORSent)
			}
			if peer.session.tearingDown.Load() || peer.session.writeFailed != nil {
				t.Fatal("route-scoped refusal retired the healthy session")
			}
		})
	}
}
