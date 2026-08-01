package reactor

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// BenchmarkAPIOriginVsForward compares the per-route cost of the two origins now
// that they share a writer.
//
// The claim under test is that an API-originated route and a forwarded route with
// the same touched-attribute set take the same encoder, so their cost differs only
// by what they genuinely do differently -- not by which of three writers happened
// to run. Both sub-benchmarks build an UPDATE whose plan carries TWO contributed
// attributes over a base of three, and both run the plan-size-write walk in
// forward_build.go.
//
// It is a comparison, not a threshold: read the two lines together. A regression
// shows as one side moving while the other does not.
func BenchmarkAPIOriginVsForward(b *testing.B) {
	nextHop := netip.MustParseAddr("10.0.0.1")

	// The base both sides start from: ORIGIN, AS_PATH, COMMUNITY.
	baseBuilder := attribute.NewBuilder()
	baseBuilder.SetOrigin(0)
	baseBuilder.SetASPath([]uint32{65000, 65001})
	baseBuilder.AddCommunity(65000, 100)
	base := baseBuilder.Build()

	b.Run("api-origin", func(b *testing.B) {
		wn := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
		batch := bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{wn},
			NextHop: bgptypes.NewNextHopExplicit(nextHop),
			Wire:    attribute.NewAttributesWire(base, bgpctx.APIContextID),
		}
		adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
		attrBuf := make([]byte, message.MaxMsgLen)
		nlriBuf := make([]byte, message.MaxMsgLen)

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			// Two contributions over the three-attribute base: NEXT_HOP (3) and
			// LOCAL_PREF (5), the iBGP announce shape.
			if u := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, nextHop,
				true /*iBGP*/, false, true /*asn4*/, false, 65000); u == nil {
				b.Fatal("announce build failed")
			}
		}
	})

	b.Run("forward", func(b *testing.B) {
		payload := buildModTestPayload(base, []byte{24, 10, 0, 0})
		handlers := attrModHandlersWithDefaults()
		pp := newPeerPool(message.MaxMsgLen)
		var mods filterapi.ModAccumulator

		// Two touched attributes, matching the announce above: LOCAL_PREF (5) and
		// ORIGINATOR_ID (9). The values live outside the loop for the reason
		// TestModifyPathZeroAlloc gives: a literal inside would measure the fixture.
		newLocalPref := []byte{0, 0, 0, 200}
		originatorID := []byte{10, 0, 0, 1}

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			mods.Reset()
			mods.Op(5, filterapi.AttrModSet, newLocalPref)
			mods.Op(9, filterapi.AttrModSet, originatorID)
			out, idx, fail := buildModifiedPayload(payload, &mods, handlers, pp, nil)
			if fail.failed() || out == nil {
				b.Fatal("forward build failed")
			}
			if idx > 0 {
				pp.Return(idx)
			}
		}
	})
}

// BenchmarkAnnounceRails compares the two announce rails against each other, which
// is the property the rail-agreement tests assert on bytes and this one asserts on
// cost: neither rail should be the cheap one, because they are the same writer.
func BenchmarkAnnounceRails(b *testing.B) {
	nextHop := netip.MustParseAddr("10.0.0.1")
	builder := attribute.NewBuilder()
	builder.SetOrigin(0)
	builder.AddCommunity(65000, 100)
	packed := builder.Build()

	wn := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}

	b.Run("batch", func(b *testing.B) {
		batch := bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{wn},
			NextHop: bgptypes.NewNextHopExplicit(nextHop),
			Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
		}
		attrBuf := make([]byte, message.MaxMsgLen)
		nlriBuf := make([]byte, message.MaxMsgLen)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if u := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, nextHop,
				true, false, true, false, 65000); u == nil {
				b.Fatal("batch build failed")
			}
		}
	})

	b.Run("queued", func(b *testing.B) {
		aw := attribute.NewAttributesWire(packed, bgpctx.APIContextID)
		attrs, err := aw.All()
		if err != nil {
			b.Fatal(err)
		}
		asPath := adapter.buildBatchASPath(nil, 0, true, false, 65000)
		route := rib.NewRouteWithASPath(wn, nextHop, slices.Clone(attrs), asPath)
		attrBuf := make([]byte, message.MaxMsgLen)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if u := buildRIBRouteUpdate(attrBuf, route, 65000, true, true, false); u == nil {
				b.Fatal("queued build failed")
			}
		}
	})
}
