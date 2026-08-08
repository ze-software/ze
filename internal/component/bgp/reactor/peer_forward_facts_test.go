package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

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
		{"no-prepend suppresses", 65100, 65000, true, false, 0},
		{"replace-as suppresses", 65100, 65000, false, true, 0},
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
			wantOps:  3, // NEXT_HOP + MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
		},
		{
			name: "self IPv6",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
			},
			wantMode: nhModeSelfV6,
			wantOps:  2, // MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
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
				peerOnLink: true,
			},
			wantMode: nhModeSelfV6LL,
			wantOps:  2, // MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
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
				peerOnLink: false,
			},
			wantMode: nhModeSelfV6,
			wantOps:  2, // MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
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
			wantOps:  2, // MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
		},
		{
			name: "explicit IPv4",
			settings: &PeerSettings{
				NextHopMode:    NextHopExplicit,
				NextHopAddress: netip.MustParseAddr("192.168.1.1"),
			},
			wantMode: nhModeExplicit4,
			wantOps:  3, // NEXT_HOP + MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
		},
		{
			name: "explicit IPv6",
			settings: &PeerSettings{
				NextHopMode:    NextHopExplicit,
				NextHopAddress: netip.MustParseAddr("2001:db8::1"),
			},
			wantMode: nhModeExplicitV6,
			wantOps:  2, // MP_REACH NH + PrefixSID suppress (RFC 9252 S3.3)
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
			applyFactsNextHop(&facts, &mods)
			assert.Equal(t, tt.wantOps, mods.Len())
		})
	}
}

// TestPrefixSIDPropagationNextHop verifies the RFC 9252 Section 3.3 propagation
// rules for the BGP Prefix-SID attribute (code 40) on the egress next-hop path.
// Ze originates no local SRv6 SID, so when the next-hop changes it removes the
// entire attribute (which necessarily removes every unrecognized sub-TLV and
// sub-sub-TLV it carried), and when the next-hop is unchanged it emits no op for
// code 40 so the received bytes -- including all Reserved fields -- are forwarded
// verbatim.
//
// RFC requirement: RFC9252-3.3-1 positive -- with next-hop unchanged no op touches the Prefix-SID attribute, so all of its Reserved fields propagate unchanged.
// RFC requirement: RFC9252-3.3-2 negative -- with next-hop unchanged the Prefix-SID attribute is not removed (no suppress op is emitted).
// RFC requirement: RFC9252-3.3-2 positive -- when the next-hop changes the whole Prefix-SID attribute is suppressed, removing every unrecognized sub-TLV and sub-sub-TLV a fortiori.
// RFC requirement: RFC9252-3.3-1 negative -- when the next-hop changes the Prefix-SID attribute is not preserved (it is suppressed).
func TestPrefixSIDPropagationNextHop(t *testing.T) {
	const prefixSIDCode = 40

	countPrefixSIDOps := func(mods *filterapi.ModAccumulator) (suppress, touched int) {
		for _, op := range mods.Ops() {
			if op.Code != prefixSIDCode {
				continue
			}
			touched++
			if op.Action == filterapi.AttrModSuppress {
				suppress++
			}
		}
		return suppress, touched
	}

	t.Run("next-hop unchanged preserves Prefix-SID", func(t *testing.T) {
		var facts peerForwardFacts
		precomputeNextHop(&PeerSettings{NextHopMode: NextHopUnchanged}, &facts)
		var mods filterapi.ModAccumulator
		applyFactsNextHop(&facts, &mods)

		suppress, touched := countPrefixSIDOps(&mods)
		assert.Zero(t, touched, "next-hop unchanged must not touch the Prefix-SID attribute")
		assert.Zero(t, suppress, "next-hop unchanged must not suppress the Prefix-SID attribute")
	})

	t.Run("next-hop changed removes Prefix-SID", func(t *testing.T) {
		var facts peerForwardFacts
		precomputeNextHop(&PeerSettings{
			NextHopMode:  NextHopSelf,
			LocalAddress: netip.MustParseAddr("2001:db8::1"),
		}, &facts)
		var mods filterapi.ModAccumulator
		applyFactsNextHop(&facts, &mods)

		suppress, _ := countPrefixSIDOps(&mods)
		assert.Equal(t, 1, suppress, "next-hop self must suppress the Prefix-SID attribute exactly once")
	})
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
