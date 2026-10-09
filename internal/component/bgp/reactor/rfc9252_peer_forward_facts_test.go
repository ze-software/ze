package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardFactsNilBeforeEstablished(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr("10.0.0.1"),
		LocalAS: 65000,
		PeerAS:  65001,
	})
	assert.Nil(t, peer.forwardFacts())
}

func TestForwardFactsSetAfterRefresh(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.1"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020304,
	})
	peer.negotiated.Store(&NegotiatedCapabilities{ExtendedMessage: true})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), facts.addr)
	assert.Equal(t, uint32(65000), facts.localAS)
	assert.Equal(t, uint32(65001), facts.peerAS)
	assert.True(t, facts.isEBGP)
	assert.True(t, facts.extendedMsg)
	assert.Equal(t, int(message.MaxMessageLength(msgtype.TypeUPDATE, true)), facts.maxMsgSize)
}

func TestForwardFactsClearedOnTeardown(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr("10.0.0.1"),
		LocalAS: 65000,
		PeerAS:  65000,
	})
	peer.negotiated.Store(&NegotiatedCapabilities{})
	peer.refreshForwardFacts()
	require.NotNil(t, peer.forwardFacts())

	peer.clearEncodingContexts()
	assert.Nil(t, peer.forwardFacts())
}

func TestForwardFactsIBGP(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.1"),
		LocalAS:  65000,
		PeerAS:   65000,
		RouterID: 0xAABBCCDD,
	})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.False(t, facts.isEBGP)
	assert.Equal(t, uint32(0xAABBCCDD), facts.clusterID)
	assert.Equal(t, [4]byte{0xAA, 0xBB, 0xCC, 0xDD}, facts.clusterIDBytes)
}

func TestForwardFactsClusterIDExplicit(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:   netip.MustParseAddr("10.0.0.1"),
		LocalAS:   65000,
		PeerAS:    65000,
		RouterID:  0xAABBCCDD,
		ClusterID: 0x11223344,
	})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, uint32(0x11223344), facts.clusterID)
	assert.Equal(t, [4]byte{0x11, 0x22, 0x33, 0x44}, facts.clusterIDBytes)
}

// TestForwardFactsSecondaryAS drives the one field that decides how many ASNs
// the egress prepend carries toward a peer with a local-as override.
//
// RFC 7705 Section 3.3 gives its two options different rails, so they select
// different values here. "Replace Old AS" is outbound and MUST leave the
// globally configured ASN off this peer's AS_PATH, which is the zero. "No
// Prepend Inbound" is inbound and says nothing about what this peer receives,
// so the base two-ASN form survives it.
//
// VALIDATES: no-prepend and replace-as are read separately, with replace-as the
// only one that suppresses the globally configured ASN.
// PREVENTS: the collapse this table pinned until 2026-08-29, where either flag
// cleared secondaryAS and the two documented options put byte-identical
// AS_PATHs on the wire.
func TestForwardFactsSecondaryAS(t *testing.T) {
	tests := []struct {
		name        string
		globalLocal uint32
		localAS     uint32
		noPrepend   bool
		replaceAS   bool
		want        uint32
	}{
		{"no override", 0, 65000, false, false, 0},
		{"same AS", 65000, 65000, false, false, 0},
		{"dual-AS active", 65100, 65000, false, false, 65100},
		{"no-prepend keeps the global ASN", 65100, 65000, true, false, 65100},
		{"replace-as suppresses", 65100, 65000, false, true, 0},
		{"both: replace-as still suppresses", 65100, 65000, true, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer := NewPeer(&PeerSettings{
				Address:          netip.MustParseAddr("10.0.0.1"),
				LocalAS:          tt.localAS,
				GlobalLocalAS:    tt.globalLocal,
				PeerAS:           65001,
				LocalASNoPrepend: tt.noPrepend,
				LocalASReplaceAS: tt.replaceAS,
			})
			peer.refreshForwardFacts()
			assert.Equal(t, tt.want, peer.forwardFacts().secondaryAS)
		})
	}
}

func TestForwardFactsSendCtxID(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr("10.0.0.1"),
		LocalAS: 65000,
		PeerAS:  65001,
	})
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.negotiated.Store(&NegotiatedCapabilities{})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, ctxID, facts.sendCtxID)
	assert.True(t, facts.sendASN4)
}

// TestForwardFactsFilterInfo is the reactor-side wiring proof for
// plan/spec-fixit-local-asn-config-key.md AC-1: the forward-path PeerFilterInfo
// carries the effective per-peer local AS, so egress filters (role/OTC,
// gr/LLGR) read dest.LocalAS instead of re-parsing raw config JSON. Before the
// fix this field was omitted and defaulted to 0.
func TestForwardFactsFilterInfo(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:   netip.MustParseAddr("10.0.0.1"),
		LocalAS:   65000,
		PeerAS:    65001,
		Name:      "test-peer",
		GroupName: "test-group",
	})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, filterapi.PeerFilterInfo{
		Address:   netip.MustParseAddr("10.0.0.1"),
		PeerAS:    65001,
		LocalAS:   65000,
		Name:      "test-peer",
		GroupName: "test-group",
	}, facts.filterInfo)
}

// TestForwardFactsFilterInfoLocalASOverride proves the forward-path LocalAS is
// the EFFECTIVE per-peer value (session/asn/local override), not the router's
// global AS. This is why the chosen fix reads dest.LocalAS rather than a single
// captured global local-as: iBGP detection (RFC 9494 4.5.3) and OTC stamping
// (RFC 9234 R008) must honor a per-peer override.
func TestForwardFactsFilterInfoLocalASOverride(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:       netip.MustParseAddr("10.0.0.1"),
		LocalAS:       65010, // effective per-peer override
		GlobalLocalAS: 65000, // router global
		PeerAS:        65001,
	})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, uint32(65010), facts.filterInfo.LocalAS,
		"forward-path LocalAS must be the effective per-peer local AS, not the global")
}

func TestPrecomputeNextHop(t *testing.T) {
	tests := []struct {
		name     string
		settings *PeerSettings
		scope    *linkScope
		wantMode uint8
		wantOps  int
	}{
		{
			name:     "auto mode",
			settings: &PeerSettings{NextHopMode: NextHopAuto},
			wantMode: nhModeNone,
			wantOps:  0,
		},
		{
			name:     "unchanged mode",
			settings: &PeerSettings{NextHopMode: NextHopUnchanged},
			wantMode: nhModeNone,
			wantOps:  0,
		},
		{
			name: "self IPv4",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("192.168.1.1"),
			},
			wantMode: nhModeSelf4,
			wantOps:  2, // NEXT_HOP + MP_REACH NH; Service TLVs depend on effective identity.
		},
		{
			name: "self IPv6",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
			},
			wantMode: nhModeSelfV6,
			wantOps:  1, // MP_REACH NH.
		},
		{
			// RFC 2545 Section 3: both halves of the inclusion condition hold --
			// the local address (which IS the global next hop under next-hop-self)
			// and the peer both sit on a locally connected subnet.
			name: "self IPv6 with link-local, shared subnet",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
				LinkLocal:    netip.MustParseAddr("fe80::1"),
			},
			scope: &linkScope{
				connected:  []netip.Prefix{netip.MustParsePrefix("2001:db8::/64")},
				peer:       netip.MustParseAddr("2001:db8::2"),
				peerOnLink: true,
			},
			wantMode: nhModeSelfV6LL,
			wantOps:  1, // MP_REACH NH.
		},
		{
			// RFC 2545 Section 3 "in all other cases": the leaf is set, but the
			// peer shares no subnet with the speaker, so the 16-octet form goes
			// on the wire. This row is what stops the leaf alone deciding the form.
			name: "self IPv6 with link-local, peer off link",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
				LinkLocal:    netip.MustParseAddr("fe80::1"),
			},
			scope: &linkScope{
				connected:  []netip.Prefix{netip.MustParsePrefix("2001:db8::/64")},
				peer:       netip.MustParseAddr("2001:db8:ffff::2"),
				peerOnLink: false,
			},
			wantMode: nhModeSelfV6,
			wantOps:  1, // MP_REACH NH.
		},
		{
			// The interface table has not been read, so the condition is unproven
			// and the link-local is not appended.
			name: "self IPv6 with link-local, no link scope",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
				LinkLocal:    netip.MustParseAddr("fe80::1"),
			},
			wantMode: nhModeSelfV6,
			wantOps:  1, // MP_REACH NH.
		},
		{
			name: "explicit IPv4",
			settings: &PeerSettings{
				NextHopMode:    NextHopExplicit,
				NextHopAddress: netip.MustParseAddr("192.168.1.1"),
			},
			wantMode: nhModeExplicit4,
			wantOps:  2, // NEXT_HOP + MP_REACH NH.
		},
		{
			name: "explicit IPv6",
			settings: &PeerSettings{
				NextHopMode:    NextHopExplicit,
				NextHopAddress: netip.MustParseAddr("2001:db8::1"),
			},
			wantMode: nhModeExplicitV6,
			wantOps:  1, // MP_REACH NH.
		},
		{
			name:     "self no local address",
			settings: &PeerSettings{NextHopMode: NextHopSelf},
			wantMode: nhModeNone,
			wantOps:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var facts peerForwardFacts
			precomputeNextHop(tt.settings, &facts)
			applyLinkLocalNextHop(tt.settings, &facts, tt.scope)
			assert.Equal(t, tt.wantMode, facts.nhMode)

			var mods filterapi.ModAccumulator
			applyFactsNextHop(&facts, &mods, family.IPv6Unicast)
			assert.Equal(t, tt.wantOps, mods.Len())
		})
	}
}

// TestPrefixSIDPropagationNextHop receives a valid third-party IPv6 route and
// reads the recipient Session's framed output on both forwarding rails.
// RFC 9252 Section 2: "If the BGP next hop is unchanged during the advertisement,
// the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and
// Sub-Sub-TLV, SHOULD be propagated further." "In addition, all Reserved fields
// in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged."
// "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."
// RFC requirement: RFC9252-3.3-1 positive -- unchanged effective next hops retain Service TLVs, nonzero Reserved fields and unknown nested bytes in actual cached and RS Session output.
// RFC requirement: RFC9252-3.3-1 negative -- explicit A-to-A rewriting must not remove Service TLVs or Reserved/unknown nested bytes merely because configured rewriting is enabled.
// RFC requirement: RFC9252-3.3-2 positive -- next-hop-self changes the recipient's effective address and removes Service TLVs and unknown nested fields while preserving the route and non-Service TLVs.
// RFC requirement: RFC9252-3.3-2 negative -- unchanged and explicit-equal next hops retain the received Service TLVs rather than removing them.
func TestPrefixSIDPropagationNextHop(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, tc := range []rfc9252NextHopCase{
			{name: "unchanged", mode: NextHopUnchanged},
			{name: "explicit-equal", mode: NextHopExplicit},
			{name: "self-changed", mode: NextHopSelf, changed: true},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				// RFC 9252 Section 2: assert emitted route and attribute bytes,
				// not the accumulator operations that happen to implement them.
				rfc9252EffectiveNextHopCase(t, rail, "ipv6", tc)
			})
		}
	}
}

func TestPrecomputeSendCommunity(t *testing.T) {
	tests := []struct {
		name     string
		send     []string
		wantMask sendCommunityMask
		wantOps  int
	}{
		{"nil (send all)", nil, 0, 0},
		{"empty (send all)", []string{}, 0, 0},
		{"explicit all", []string{"all"}, 0, 0},
		{"none", []string{"none"}, scSuppressStandard | scSuppressExtended | scSuppressLarge, 3},
		{"standard only", []string{"standard"}, scSuppressExtended | scSuppressLarge, 2},
		{"extended only", []string{"extended"}, scSuppressStandard | scSuppressLarge, 2},
		{"large only", []string{"large"}, scSuppressStandard | scSuppressExtended, 2},
		{"standard+large", []string{"standard", "large"}, scSuppressExtended, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var facts peerForwardFacts
			precomputeSendCommunity(&PeerSettings{SendCommunity: tt.send}, &facts)
			assert.Equal(t, tt.wantMask, facts.scMask)

			var mods filterapi.ModAccumulator
			applyFactsSendCommunity(&facts, &mods)
			assert.Equal(t, tt.wantOps, mods.Len())

			var origMods filterapi.ModAccumulator
			applySendCommunityFilter(&PeerSettings{SendCommunity: tt.send}, &origMods)
			assert.Equal(t, origMods.Len(), mods.Len(), "op count must match original")
		})
	}
}

func TestForwardFactsDynamicPeerRefresh(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:   netip.MustParseAddr("10.0.0.1"),
		LocalAS:   65000,
		PeerAS:    0,
		IsDynamic: true,
	})
	peer.negotiated.Store(&NegotiatedCapabilities{})
	peer.refreshForwardFacts()

	facts := peer.forwardFacts()
	require.NotNil(t, facts)
	assert.Equal(t, uint32(0), facts.peerAS)
	assert.True(t, facts.isEBGP) // 65000 != 0

	peer.settings.PeerAS = 65000
	peer.refreshForwardFacts()

	facts = peer.forwardFacts()
	assert.Equal(t, uint32(65000), facts.peerAS)
	assert.False(t, facts.isEBGP)
}
