package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

func aigpSelectionAttrs(nh [4]byte, metric *uint64, pref uint32, pathLength byte) []byte {
	attrs := makeAttrBytes(nh)
	attrs = append(attrs, 0x40, byte(attribute.AttrLocalPref), 4)
	attrs = binary.BigEndian.AppendUint32(attrs, pref)
	attrs = append(attrs, 0x40, byte(attribute.AttrASPath), 2+4*pathLength, 2, pathLength)
	for range int(pathLength) {
		attrs = binary.BigEndian.AppendUint32(attrs, 65001)
	}
	if metric != nil {
		attrs = append(attrs, 0x80, byte(attribute.AttrAIGP), 11)
		var value [11]byte
		attribute.WriteAIGPMetric(value[:], 0, *metric)
		attrs = append(attrs, value[:]...)
	}
	return attrs
}

// RFC requirement: RFC7311-4.1-1 positive -- the received metric plus next-hop cost chooses a longer AS_PATH before ordinary tiebreaking.
// RFC requirement: RFC7311-4.1-1 negative -- higher LOCAL_PREF still wins before AIGP, and a missing AIGP does not masquerade as metric zero.
func TestAIGPSelectsStoredRoutesBeforeASPath(t *testing.T) {
	r := newRIBManager(nil)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	peerA, peerB := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
	nhA, nhB := [4]byte{198, 51, 100, 1}, [4]byte{198, 51, 100, 2}
	for _, peer := range []netip.Addr{peerA, peerB} {
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		r.bgpPeers[peer] = storage.NewPeerRIB(peer.String())
		p := r.bgpPeers[peer]
		t.Cleanup(p.Release)
	}
	igpcost.Set(func(nh netip.Addr) igpcost.Distance {
		cost := uint64(10)
		if nh == netip.AddrFrom4(nhA) {
			cost = 100
		}
		return igpcost.Distance{Cost: cost, Resolved: true}
	})
	t.Cleanup(func() { igpcost.Set(nil) })
	prefix := ipv4Prefix(24, 10, 20, 0)
	metricA, metricB := uint64(5), uint64(20)
	r.bgpPeers[peerA].Insert(family.IPv4Unicast, aigpSelectionAttrs(nhA, &metricA, 100, 1), prefix)
	r.bgpPeers[peerB].Insert(family.IPv4Unicast, aigpSelectionAttrs(nhB, &metricB, 100, 3), prefix)
	winner, changed := r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
	require.True(t, changed)
	require.Equal(t, netip.AddrFrom4(nhB), winner.NextHop)
	require.Equal(t, uint64(20), winner.AIGP, "published metric is received value, not accumulated value")
	require.True(t, winner.AIGPPresent)
	replay := r.collectBestPaths()[family.IPv4Unicast]
	require.Len(t, replay, 1)
	require.Equal(t, uint64(20), replay[0].AIGP)

	r.bgpPeers[peerA].Insert(family.IPv4Unicast, aigpSelectionAttrs(nhA, nil, 101, 1), prefix)
	winner, changed = r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
	require.True(t, changed)
	require.Equal(t, netip.AddrFrom4(nhA), winner.NextHop, "LOCAL_PREF precedes AIGP")
	require.False(t, winner.AIGPPresent)

	r.bgpPeers[peerA].Insert(family.IPv4Unicast, aigpSelectionAttrs(nhA, nil, 100, 1), prefix)
	winner, changed = r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
	require.True(t, changed)
	require.Equal(t, netip.AddrFrom4(nhB), winner.NextHop, "missing AIGP is not a zero-cost path")
}

// RFC requirement: RFC7311-3.4.3-7 positive -- a changed next-hop distance reselects retained routes without a peer UPDATE.
// RFC requirement: RFC7311-3.4.3-7 negative -- an unchanged distance neither changes the winner nor replaces its received metric with the accumulated value.
func TestAIGPDistanceChangeReselectsRetainedRoutes(t *testing.T) {
	r := newRIBManager(nil)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	metric := uint64(10)
	costA := uint64(5)
	igpcost.Set(func(nh netip.Addr) igpcost.Distance {
		cost := uint64(20)
		if nh == netip.MustParseAddr("198.51.100.1") {
			cost = costA
		}
		return igpcost.Distance{Cost: cost, Resolved: true}
	})
	t.Cleanup(func() { igpcost.Set(nil) })
	wirePrefix := ipv4Prefix(24, 10, 20, 0)
	for i, text := range []string{"192.0.2.1", "192.0.2.2"} {
		peer := netip.MustParseAddr(text)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(text)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		routes.Insert(family.IPv4Unicast, aigpSelectionAttrs([4]byte{198, 51, 100, byte(i + 1)}, &metric, 100, 1), wirePrefix)
	}
	r.reselectAIGPRoutes()
	path, _, found := loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.20.0.1"))
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("198.51.100.1"), path.NextHop)
	costA = 30
	r.reselectAIGPRoutes()
	path, _, found = loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.20.0.1"))
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("198.51.100.2"), path.NextHop)
	require.True(t, path.AIGPPresent)
	require.Equal(t, metric, path.AIGP)
	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.False(t, changed)
}

// Multiprotocol next hops must participate in the same distance comparison as
// legacy NEXT_HOP. The selected event keeps the received metric for recursion.
func TestAIGPSelectionResolvesMultiprotocolNextHop(t *testing.T) {
	r := newRIBManager(nil)
	nextHops := []netip.Addr{
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("2001:db8::2"),
	}
	igpcost.Set(func(nextHop netip.Addr) igpcost.Distance {
		if nextHop == nextHops[0] {
			return igpcost.Distance{Cost: 100, Resolved: true}
		}
		if nextHop == nextHops[1] {
			return igpcost.Distance{Cost: 5, Resolved: true}
		}
		return igpcost.Distance{}
	})
	t.Cleanup(func() { igpcost.Set(nil) })
	raw := []byte{64, 0x20, 1, 0x0d, 0xb8, 0, 1, 0, 0}
	metric := uint64(10)
	for i, address := range []string{"192.0.2.1", "192.0.2.2"} {
		peer := netip.MustParseAddr(address)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(address)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		// A mixed UPDATE retains a legacy NEXT_HOP for its IPv4 routes.
		// It must not replace the IPv6 MP_REACH next hop during selection.
		attrs := aigpSelectionAttrs([4]byte{192, 0, 2, 254}, &metric, 100, 1)
		nextHop := nextHops[i].As16()
		mp := append([]byte{0, 2, 1, 16}, nextHop[:]...)
		mp = append(mp, 0)
		mp = append(mp, raw...)
		attrs = append(attrs, 0x80, 14, byte(len(mp)))
		attrs = append(attrs, mp...)
		routes.Insert(family.IPv6Unicast, attrs, raw)
	}
	change, changed := r.checkBestPathChange(family.IPv6Unicast, raw, false, nil)
	require.True(t, changed)
	require.Equal(t, nextHops[1], change.NextHop)
	require.True(t, change.AIGPPresent)
	require.Equal(t, metric, change.AIGP)
}
