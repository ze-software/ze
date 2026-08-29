// VALIDATES: RFC 8669 Section 8 on EGRESS -- the BGP Prefix-SID attribute (code
// 40) reaches a peer in another AS only where the operator has declared that
// neighbor to be inside ze's SR domain, and it reaches an internal peer always.
// Asserted on the bytes each destination is sent, on the forward rail and on the
// two origination rails.
// PREVENTS: (a) Segment Routing label indices leaking out of the SR domain,
// which the section calls "undesired propagation" and which collides label
// indices between domains; (b) the opposite regression, an unconditional strip
// that would break the multi-AS SR domain Section 8 explicitly allows for and
// RFC 8670 deploys; (c) the strip reaching internal peers, which would make
// Segment Routing unusable inside the AS.
package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prefixSIDCodeByte = byte(attribute.AttrPrefixSID)

// prefixSIDValue is a Label-Index TLV (RFC 8669 Section 3.1): type 1, length 7,
// RESERVED 0, Flags 0, Label Index 100.
var prefixSIDValue = []byte{0x01, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64}

// prefixSIDBody is an UPDATE body carrying a Prefix-SID: ORIGIN igp, AS_PATH
// [65001], NEXT_HOP 10.0.0.254, PREFIX_SID, NLRI 192.0.2.0/24.
//
// The body advertises reachable NLRI on purpose. buildModifiedPayload refuses to
// rebuild a body that advertises none (advertiseGate), so a withdraw-only
// fixture would prove nothing about the suppression it is meant to drive.
func prefixSIDBody() []byte {
	return []byte{
		0, 0, // WithdrawnLen = 0
		0, 33, // TotalPathAttrLen = 33
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001] (4-byte)
		0x40, 3, 4, 10, 0, 0, 254, // NEXT_HOP 10.0.0.254
		0xC0, prefixSIDCodeByte, 10, // PREFIX_SID, optional transitive
		0x01, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64,
		24, 192, 0, 2, // NLRI 192.0.2.0/24
	}
}

// prefixSIDPeer builds an established RS-fast-path peer whose local AS is 65000.
func prefixSIDPeer(addr string, peerAS, routerID uint32, propagate, rsClient bool, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	p := NewPeer(&PeerSettings{
		Connection:             ConnectionBoth,
		Address:                netip.MustParseAddr(addr),
		LocalAS:                65000,
		PeerAS:                 peerAS,
		RouterID:               routerID,
		RSFastPath:             true,
		RSClient:               rsClient,
		PropagateSRv6PrefixSID: propagate,
	})
	p.state.Store(int32(PeerStateEstablished))
	p.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	p.sendCtx.Store(ctx)
	p.sendCtxID = ctxID
	p.refreshForwardFacts()
	return p
}

// prefixSIDForwardTo relays prefixSIDBody from an EBGP source (AS 65001) to one
// destination and returns the UPDATE body that destination is sent.
//
// The source is EXTERNAL so that neither RFC 4456's non-client rule nor the
// internal-to-internal prohibition can withhold the route: this test is about
// what the destination receives, and a destination that receives nothing would
// pass a presence assertion for the wrong reason.
func prefixSIDForwardTo(t *testing.T, peerAS uint32, propagate, rsClient bool) []byte {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	wu := wireu.NewWireUpdate(prefixSIDBody(), ctxID)
	wu.SetMessageID(300)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(300, 1)

	src := prefixSIDPeer("10.0.0.1", 65001, 0x01020301, false, false, ctx, ctxID)
	dst := prefixSIDPeer("10.0.0.2", peerAS, 0x01020302, propagate, rsClient, ctx, ctxID)

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{}, 1)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		done <- struct{}{}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		recentUpdates:   cache,
		attrModHandlers: attrModHandlersWithDefaults(),
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	reactorForwardRS(r, update, 300, netip.MustParseAddr("10.0.0.1"), src)

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, dispatched, 1)
		require.NotEmpty(t, dispatched[0].rawBodies)
		return dispatched[0].rawBodies[0]
	case <-time.After(2 * time.Second):
		t.Fatal("the destination was sent nothing, so no claim about its wire can be made")
		return nil
	}
}

// TestPrefixSIDEgressBoundary proves that the SR domain boundary, and not the AS
// boundary, decides whether a relayed Prefix-SID stays on the wire.
//
// Section 8 states the boundary as "a single SR/administrative domain that may
// include one or more ASes", so the AS numbers alone cannot answer it and the
// per-neighbor configuration is the answer. The four cases below are the whole
// decision: internal always, external only when declared, on the route-server
// rail as on the ordinary one.
//
// The route-server case is the one the previous code missed. Suppression lived
// in applyFactsNextHop, which returns at nhModeNone, and an RS client keeps the
// source next-hop (RFC 7947 Section 2.2.2), so nothing removed the attribute for
// exactly the peers most likely to be outside the SR domain.
//
// RFC requirement: RFC8669-8-1 positive -- an EBGP neighbor the operator has explicitly configured for propagation is sent the Prefix-SID attribute unchanged.
// RFC requirement: RFC8669-8-1 negative -- an EBGP neighbor with no such configuration is sent no Prefix-SID attribute, on the ordinary forward rail and on the route-server rail alike.
func TestPrefixSIDEgressBoundary(t *testing.T) {
	tests := []struct {
		name      string
		peerAS    uint32
		propagate bool
		rsClient  bool
		want      bool
	}{
		{name: "ibgp keeps it", peerAS: 65000, want: true},
		{name: "ebgp without configuration is stripped", peerAS: 65002, want: false},
		{name: "ebgp configured for propagation keeps it", peerAS: 65002, propagate: true, want: true},
		{name: "route-server client without configuration is stripped", peerAS: 65002, rsClient: true, want: false},
		{name: "route-server client configured for propagation keeps it", peerAS: 65002, rsClient: true, propagate: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := prefixSIDForwardTo(t, tt.peerAS, tt.propagate, tt.rsClient)
			attrs := decodeBodyAttrs(t, body)

			value, present := attrs[prefixSIDCodeByte]
			assert.Equal(t, tt.want, present, "presence of attribute 40 on this destination's wire")
			if tt.want {
				assert.Equal(t, prefixSIDValue, value, "a kept Prefix-SID is forwarded byte for byte")
			}

			// The removal is confined to code 40: the rest of the route still
			// arrives, so a passing absence assertion cannot be a dropped UPDATE.
			assert.Equal(t, []byte{0}, attrs[1], "ORIGIN survives")
			assert.Equal(t, []byte{10, 0, 0, 254}, attrs[3], "NEXT_HOP survives")
		})
	}
}

// TestPrefixSIDAllowedTo pins the rule itself, apart from any rail.
//
// It is the site every egress rail asks, so a change here changes all of them at
// once; that is the property it exists to hold.
func TestPrefixSIDAllowedTo(t *testing.T) {
	assert.True(t, prefixSIDAllowedTo(true, false), "an internal peer shares this AS, so Section 8 does not reach it")
	assert.True(t, prefixSIDAllowedTo(true, true), "configuration cannot make an internal peer refuse it")
	assert.False(t, prefixSIDAllowedTo(false, false), "an external peer is refused by default")
	assert.True(t, prefixSIDAllowedTo(false, true), "an external peer is permitted once explicitly configured")
}

// TestPrefixSIDSuppressIsRecordedOnce proves the RFC 8669 Section 8 suppression
// and the RFC 9252 Section 3.3 next-hop-change suppression do not both record an
// operation for the same destination.
//
// A duplicate would be harmless on the wire, because the handler folds every
// operation for a code, and expensive off it: the accumulator holds eight
// operations inline and spills to the heap past that, on the per-UPDATE
// per-destination path.
func TestPrefixSIDSuppressIsRecordedOnce(t *testing.T) {
	countCode40 := func(mods *filterapi.ModAccumulator) int {
		n := 0
		for _, op := range mods.Ops() {
			if op.Code == prefixSIDCodeByte {
				n++
			}
		}
		return n
	}

	t.Run("next-hop unchanged records exactly one", func(t *testing.T) {
		var facts peerForwardFacts
		facts.isEBGP = true
		precomputeNextHop(&PeerSettings{NextHopMode: NextHopUnchanged}, &facts)
		require.Equal(t, nhModeNone, facts.nhMode)

		var mods filterapi.ModAccumulator
		applyFactsNextHop(&facts, &mods)
		applyFactsPrefixSID(&facts, true, &mods)
		assert.Equal(t, 1, countCode40(&mods), "the Section 8 suppression is the only one")
	})

	t.Run("next-hop self still records exactly one", func(t *testing.T) {
		var facts peerForwardFacts
		facts.isEBGP = true
		precomputeNextHop(&PeerSettings{
			NextHopMode:  NextHopSelf,
			LocalAddress: netip.MustParseAddr("2001:db8::1"),
		}, &facts)
		require.NotEqual(t, nhModeNone, facts.nhMode)

		var mods filterapi.ModAccumulator
		applyFactsNextHop(&facts, &mods)
		applyFactsPrefixSID(&facts, true, &mods)
		assert.Equal(t, 1, countCode40(&mods), "the next-hop rail already removed it")
	})

	t.Run("a source without the attribute records none", func(t *testing.T) {
		var facts peerForwardFacts
		facts.isEBGP = true
		precomputeNextHop(&PeerSettings{NextHopMode: NextHopUnchanged}, &facts)

		var mods filterapi.ModAccumulator
		applyFactsPrefixSID(&facts, false, &mods)
		assert.Zero(t, countCode40(&mods), "nothing to remove, so no rebuild is forced")
	})

	t.Run("a filter that sets the attribute is overruled", func(t *testing.T) {
		var facts peerForwardFacts
		facts.isEBGP = true
		precomputeNextHop(&PeerSettings{NextHopMode: NextHopUnchanged}, &facts)

		var mods filterapi.ModAccumulator
		mods.Op(prefixSIDCodeByte, filterapi.AttrModSet, prefixSIDValue)
		applyFactsPrefixSID(&facts, false, &mods)

		last, suppress := filterapi.LastSetOrSuppress(mods.Ops())
		require.NotEqual(t, -1, last)
		assert.True(t, suppress, "Section 8 is not a policy a filter may override")
	})
}

// TestPrefixSIDOriginationBoundary proves the same boundary on the ORIGINATION
// rails, where the attribute comes from this router's own configuration rather
// than from a peer.
//
// Section 8 governs the attribute, not the route field that carried it, so all
// three ways a configured route can carry code 40 are covered: the modeled
// Prefix-SID of a labeled-unicast route, the same on a VPN route, and a raw
// attribute written under type code 40 (which config.parseRawAttributeInto
// leaves raw for every family, unicast included).
//
// RFC requirement: RFC8669-8-1 positive -- a locally originated route keeps its configured Prefix-SID toward an EBGP neighbor explicitly configured for propagation, and toward an internal peer.
// RFC requirement: RFC8669-8-1 negative -- the same route carries no Prefix-SID toward an EBGP neighbor with no such configuration, whether the attribute was configured as a Prefix-SID or written as a raw attribute.
func TestPrefixSIDOriginationBoundary(t *testing.T) {
	rawPrefixSID := RawAttribute{Code: prefixSIDCodeByte, Flags: 0xC0, Value: prefixSIDValue}
	nextHop := netip.MustParseAddr("10.0.0.254")

	t.Run("labeled unicast", func(t *testing.T) {
		route := &StaticRoute{
			Prefix:         netip.MustParsePrefix("192.0.2.0/24"),
			Labels:         []uint32{100},
			PrefixSIDBytes: prefixSIDValue,
			RawAttributes:  []RawAttribute{rawPrefixSID},
		}

		kept := toStaticRouteLabeledUnicastParams(route, nextHop, true)
		assert.Equal(t, prefixSIDValue, kept.PrefixSID, "inside the SR domain the configured SID is advertised")
		assert.Len(t, kept.RawAttributeBytes, 1, "and a raw one is left alone")

		stripped := toStaticRouteLabeledUnicastParams(route, nextHop, false)
		assert.Nil(t, stripped.PrefixSID, "outside it the configured SID is removed")
		assert.Empty(t, stripped.RawAttributeBytes, "and so is a raw attribute under the same code")
	})

	t.Run("vpn", func(t *testing.T) {
		route := &StaticRoute{
			Prefix:         netip.MustParsePrefix("192.0.2.0/24"),
			RD:             "65000:1",
			Labels:         []uint32{100},
			PrefixSIDBytes: prefixSIDValue,
		}

		assert.Equal(t, prefixSIDValue, toStaticRouteVPNParams(route, nextHop, true).PrefixSID)
		assert.Nil(t, toStaticRouteVPNParams(route, nextHop, false).PrefixSID)
	})

	t.Run("unicast raw attribute", func(t *testing.T) {
		route := &StaticRoute{
			Prefix:        netip.MustParsePrefix("192.0.2.0/24"),
			RawAttributes: []RawAttribute{rawPrefixSID},
		}

		assert.Len(t, toStaticRouteUnicastParams(route, nextHop, netip.Addr{}, nil, true).RawAttributeBytes, 1)
		assert.Empty(t, toStaticRouteUnicastParams(route, nextHop, netip.Addr{}, nil, false).RawAttributeBytes)
	})

	t.Run("plugin route raw attribute", func(t *testing.T) {
		wire := []byte{0xC0, prefixSIDCodeByte, byte(len(prefixSIDValue))}
		wire = append(wire, prefixSIDValue...)
		other := []byte{0x40, 1, 1, 0} // ORIGIN igp

		route := PluginRoute{RawAttrs: [][]byte{other, wire}}
		fam := family.IPv4Unicast

		assert.Len(t, toPluginParams(route, fam, true).RawAttrs, 2)
		assert.Equal(t, [][]byte{other}, toPluginParams(route, fam, false).RawAttrs,
			"only the Prefix-SID is removed; the plugin's other attributes still go out")
	})
}

// TestRawAttrsWithoutPrefixSID pins the filter's edges: it removes every
// Prefix-SID entry rather than the first, keeps everything else, and returns the
// caller's own slice when there is nothing to remove.
func TestRawAttrsWithoutPrefixSID(t *testing.T) {
	origin := []byte{0x40, 1, 1, 0}
	sid := []byte{0xC0, prefixSIDCodeByte, 1, 0}
	short := []byte{0xC0} // Too short to state a code.

	t.Run("no prefix-sid returns the same slice", func(t *testing.T) {
		in := [][]byte{origin, short}
		out := rawAttrsWithoutPrefixSID(in)
		assert.Equal(t, in, out)
		require.Len(t, out, 2, "a too-short entry is not judged here")
	})

	t.Run("every occurrence is removed", func(t *testing.T) {
		out := rawAttrsWithoutPrefixSID([][]byte{sid, origin, sid})
		assert.Equal(t, [][]byte{origin}, out)
	})

	t.Run("nil stays nil", func(t *testing.T) {
		assert.Nil(t, rawAttrsWithoutPrefixSID(nil))
	})
}
