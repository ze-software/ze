package reactor

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"

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
	assert.Equal(t, int(message.MaxMessageLength(message.TypeUPDATE, true)), facts.maxMsgSize)
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
		Name:      "test-peer",
		GroupName: "test-group",
	}, facts.filterInfo)
}

func TestPrecomputeNextHop(t *testing.T) {
	tests := []struct {
		name     string
		settings *PeerSettings
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
			name: "self IPv6 with link-local",
			settings: &PeerSettings{
				NextHopMode:  NextHopSelf,
				LocalAddress: netip.MustParseAddr("2001:db8::1"),
				LinkLocal:    netip.MustParseAddr("fe80::1"),
			},
			wantMode: nhModeSelfV6LL,
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
			assert.Equal(t, tt.wantMode, facts.nhMode)

			var mods filterapi.ModAccumulator
			applyFactsNextHop(&facts, &mods)
			assert.Equal(t, tt.wantOps, mods.Len())
		})
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
