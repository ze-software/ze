// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: export_answer_test.go -- exportAnswer, the export chain these tests answer
// Related: mapped_plain_next_hop_test.go -- the fixture shapes reused here

package reactor

import (
	"bytes"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExportRawNextHopRefused proves that a destination's export chain which
// answers a raw body whose MP_REACH_NLRI next hop the family does not admit
// sends that destination the withdrawal, never the announcement. Method: the
// chain answers the received mixed UPDATE with the same UPDATE whose native
// next hop is the invalid field itself (a 4-octet IPv4 address, or a mapped
// address paired with a link-local), on the cached rail and on the RS rail
// (rs-export: the fast path hands an export-chain destination back to the
// cached rail for a mixed UPDATE it withholds by section).
func TestExportRawNextHopRefused(t *testing.T) {
	for _, safi := range []family.SAFI{1, 2} {
		fam := family.Family{AFI: 2, SAFI: safi}
		for _, rail := range []string{"cached", "rs-export"} {
			for _, tc := range []struct{ name, wire string }{
				{"native-four-octets", "c0000209"},
				{"mapped-pair", mappedPlainHop + mappedPlainLinkLocal},
			} {
				t.Run(fam.String()+"/"+rail+"/"+tc.name, func(t *testing.T) {
					f := rfc2545ReceiveFixture(t, false, false, true)
					for _, peer := range []*Peer{f.source, f.destination} {
						mappedPlainNegotiate(peer, fam, family.IPv4Unicast)
					}
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					legacy := mustHex(t, "180a0900")
					attrsWith := func(hop string) []byte {
						reach := mappedPlainReach(t, fam, hop, mappedPlainNLRI)
						attrs := mixedAttrs(append([]byte{0x80, 14, byte(len(reach))}, reach...))
						attrs = append(attrs, 0x40, 3, 4)
						return append(attrs, f.source.Settings().Address.AsSlice()...)
					}
					raw := buildUpdatePayload(attrsWith(tc.wire), legacy)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}
					}, f.destination)

					updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(attrsWith(mappedPlainGlobal), legacy))
					native := 0
					for _, update := range updates {
						if len(update.NLRI) != 0 {
							continue
						}
						native++
						_, _, got, announced := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						_, _, withdrawn, withdrawal := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
						want := append([]byte{0, 2, byte(safi)}, mustHex(t, mappedPlainNLRI)...)
						if announced || !withdrawal || !bytes.Equal(withdrawn, want) {
							t.Errorf("reach=%x withdrawal=%x want withdrawal=%x", got, withdrawn, want)
						}
					}
					if native != 1 {
						t.Errorf("native decisions=%d want=1", native)
					}
				})
			}
		}
	}
}

// TestRSFastPathPluginChainRefusal proves how the RS fast path answers a
// destination whose export chain of one plugin filter refuses the route, so
// that the destination is sent the withdrawal by one rail and never by none.
// Method: a chain of probe:f at bgp export with a final accept, the filter
// reading prefix and answering reject, is decided here: the one withdrawal.
// The same chain with a final reject is handed back to the plugin rail
// (reactorForwardRS: "A peer is skipped when it carries a rejecting final"),
// whose forwardUpdateCore sends the withdrawal, and so is a chain whose
// filter cannot be asked: no verdict was reached.
func TestRSFastPathPluginChainRefusal(t *testing.T) {
	// RFC 4271 Section 4.3: ORIGIN IGP, empty AS_PATH, NEXT_HOP 10.0.0.1,
	// NLRI 10.0.0.0/24.
	received := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 10, 0, 0, 1, 24, 10, 0, 0}
	withdrawal := []byte{0, 4, 24, 10, 0, 0, 0, 0}

	rejects := &externalTransport{reads: []string{"prefix"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterReject}}
	skipped, bodies := forwardRSOnceVia(t, probeExportChain(t), rejects, received)
	if len(skipped) != 0 {
		t.Fatalf("member reject: skipped = %v, want the destination kept on the fast path", skipped)
	}
	if len(bodies) != 1 || !bytes.Equal(bodies[0], withdrawal) {
		t.Fatalf("member reject: bodies = %x, want the one withdrawal %x", bodies, withdrawal)
	}

	accepts := &externalTransport{reads: []string{"prefix"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterAccept}}
	skipped, bodies = forwardRSOnceVia(t, staleChainAt(t, "bgp export").policy, accepts, received)
	if len(skipped) != 1 || len(bodies) != 0 {
		t.Fatalf("final reject: skipped = %v bodies = %x, want the destination handed to the plugin rail", skipped, bodies)
	}

	gone := &externalTransport{reads: []string{"prefix"}, err: errors.New("the plugin has gone")}
	skipped, bodies = forwardRSOnceVia(t, probeExportChain(t), gone, received)
	if len(skipped) != 1 || len(bodies) != 0 {
		t.Fatalf("unreachable filter: skipped = %v bodies = %x, want the destination handed to the plugin rail", skipped, bodies)
	}
}

// forwardRSOnceVia is forwardRSOnce with transport answering the
// destination's plugin filters.
func forwardRSOnceVia(t *testing.T, export filterapi.Policy, transport filterTransport, payload []byte) ([]netip.AddrPort, [][]byte) {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(62)
	update := &ReceivedUpdate{WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("10.0.0.1"), ReceivedAt: time.Now()}
	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(62, 1)

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
		attrModHandlers:     attrModHandlersWithDefaults(),
		recentUpdates:       cache,
		peers:               map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src, dst.Settings().PeerKey(): dst},
		fwdPool:             testPool,
		filterTransportSeam: transport,
	}
	skipped, dispatched := reactorForwardRS(r, update, 62, netip.MustParseAddr("10.0.0.1"), src)
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
