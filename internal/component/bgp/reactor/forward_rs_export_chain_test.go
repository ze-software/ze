// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection, the rail under test
// Related: forward_rs_chain.go -- the per-destination base the chain's rewrite becomes
//
// The export-chain half of the route-server fast path lives in its own file
// for the reason forward_rs_inactive_test.go gives: rfc7947_forward_rs_test.go
// carries RFC 7947 tags, and this test asserts a Ze rule, not an RFC 7947 one.

package reactor

import (
	"bytes"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// rsRejectChain answers an export policy whose one member rejects every
// route, compiled as bgp/config compiles it.
func rsRejectChain(t *testing.T) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "member", Type: libraryTypeReject})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(libraryTestExport, reg, []policychain.Member{{Name: "member"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "member"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

// forwardRSOnce runs one received UPDATE through reactorForwardRS from
// 10.0.0.1 to a destination at 10.0.0.2 holding export, and answers the
// skipped list and the bodies the destination was dispatched.
func forwardRSOnce(t *testing.T, export filterapi.Policy, payload []byte) ([]netip.AddrPort, [][]byte) {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(61)
	update := &ReceivedUpdate{WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("10.0.0.1"), ReceivedAt: time.Now()}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(61, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dst.settings.ExportPolicy = export
	dst.refreshForwardFacts()

	var bodies [][]byte
	var mu sync.Mutex
	done := make(chan struct{}, 1)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		for i := range items {
			for _, body := range items[i].rawBodies {
				bodies = append(bodies, bytes.Clone(body))
			}
		}
		mu.Unlock()
		done <- struct{}{}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	skipped, dispatched := reactorForwardRS(r, update, 61, netip.MustParseAddr("10.0.0.1"), src)
	if dispatched == 0 {
		return skipped, nil
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}
	mu.Lock()
	defer mu.Unlock()
	return skipped, bodies
}

// TestRSFastPathRunsTheExportChain holds the route-server fast path to spec
// A-13: a destination whose export chain is active stays on the fast path,
// which runs the chain itself, and the chain's verdict decides what the
// destination is sent.
//
// VALIDATES: a rejecting member turns the advertisement into the withdrawal
// of 10.0.0.0/24 and the destination is not skipped; the same member over a
// received End-of-RIB, the IPv4 empty UPDATE and the MP_UNREACH_NLRI form,
// sends the marker unchanged (RFC 4724 Section 2); a chain
// that cannot run (an external member with no filter engine), and a refusal
// that must be withdrawn one NLRI-bearing field at a time, each hand the
// destination to the plugin rail and the fast path sends nothing.
// PREVENTS: an active export member (bgp-role's inserted OTC egress stamp, or
// any operator filter) taking every destination off the fast path, so bgp-rs
// serves it and replaces a relayed End-of-RIB with its own; the fast path
// forwarding a route its chain refused; a refused End-of-RIB never reaching
// the client; the fast path forwarding past a chain that never ran; and the
// fast path asking the chain again about one section of an UPDATE it judged.
func TestRSFastPathRunsTheExportChain(t *testing.T) {
	received := libraryTestBody(100)
	// RFC 4271 Section 4.3: Withdrawn Routes Length 4, the /24, no attributes.
	withdrawal := []byte{0, 4, 24, 10, 0, 0, 0, 0}

	skipped, bodies := forwardRSOnce(t, rsRejectChain(t), received)
	if len(skipped) != 0 {
		t.Fatalf("rejecting chain: skipped = %v, want the destination kept on the fast path", skipped)
	}
	if len(bodies) != 1 || !bytes.Equal(bodies[0], withdrawal) {
		t.Fatalf("rejecting chain: bodies = %x, want the one withdrawal %x", bodies, withdrawal)
	}

	// RFC 4724 Section 2: the IPv4 unicast End-of-RIB, an UPDATE of minimum
	// length (no withdrawn routes, no attributes, no NLRI).
	endOfRIB := []byte{0, 0, 0, 0}
	skipped, bodies = forwardRSOnce(t, rsRejectChain(t), endOfRIB)
	if len(skipped) != 0 {
		t.Fatalf("rejecting chain, End-of-RIB: skipped = %v, want the destination kept on the fast path", skipped)
	}
	if len(bodies) != 1 || !bytes.Equal(bodies[0], endOfRIB) {
		t.Fatalf("rejecting chain, End-of-RIB: bodies = %x, want the marker %x unchanged", bodies, endOfRIB)
	}

	// RFC 4724 Section 2: "For any other address family, it is an UPDATE
	// message that contains only the MP_UNREACH_NLRI attribute [BGP-MP] with
	// no withdrawn routes for that <AFI, SAFI>." The IPv6 unicast marker:
	// withdrawn length 0, path attribute length 6, MP_UNREACH_NLRI (flags
	// 0x80, type 15, length 3) holding AFI 2, SAFI 1 and no withdrawn routes.
	// The rejecting verdict converts it to no withdrawal either, so the
	// client is sent it unchanged.
	ipv6EndOfRIB := []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}
	skipped, bodies = forwardRSOnce(t, rsRejectChain(t), ipv6EndOfRIB)
	if len(skipped) != 0 {
		t.Fatalf("rejecting chain, IPv6 End-of-RIB: skipped = %v, want the destination kept on the fast path", skipped)
	}
	if len(bodies) != 1 || !bytes.Equal(bodies[0], ipv6EndOfRIB) {
		t.Fatalf("rejecting chain, IPv6 End-of-RIB: bodies = %x, want the marker %x unchanged", bodies, ipv6EndOfRIB)
	}

	// A refusal of an UPDATE whose MP_REACH_NLRI and MP_UNREACH_NLRI name
	// two families is withdrawn one field at a time (withdrawalBySection).
	// The fast path's redo would ask the chain again about each section, so
	// the destination is handed to the plugin rail, which partitions the
	// judged output without invoking policy again.
	//
	// RFC 4760 Section 3 MP_REACH_NLRI value: AFI 2, SAFI 1, next-hop length
	// 16, 2001:db8::1, reserved 0, NLRI 2001:db8::/32.
	// RFC 4760 Section 4 MP_UNREACH_NLRI value: AFI 1, SAFI 1, 10.0.1.0/24.
	twoFamilies := []byte{0, 0, 0, 46,
		0x40, 1, 1, 0,
		0x40, 2, 0,
		0x80, 14, 26, 0, 2, 1, 16,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0, 32, 0x20, 0x01, 0x0d, 0xb8,
		0x80, 15, 7, 0, 1, 1, 24, 10, 0, 1,
	}
	skipped, bodies = forwardRSOnce(t, rsRejectChain(t), twoFamilies)
	if len(skipped) != 1 {
		t.Fatalf("rejecting chain, two families: skipped = %v, want the destination handed to the plugin rail", skipped)
	}
	if len(bodies) != 0 {
		t.Fatalf("rejecting chain, two families: bodies = %x, want nothing sent by the fast path", bodies)
	}

	cannotRun := mustPolicy(t, frefs("bgp-rs:test-filter"), routepolicy.FinalAccept)
	skipped, bodies = forwardRSOnce(t, cannotRun, received)
	if len(skipped) != 1 {
		t.Fatalf("chain that cannot run: skipped = %v, want the destination handed to the plugin rail", skipped)
	}
	if len(bodies) != 0 {
		t.Fatalf("chain that cannot run: bodies = %x, want nothing sent by the fast path", bodies)
	}
}
