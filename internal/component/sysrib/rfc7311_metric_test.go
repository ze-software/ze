package sysrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// RFC requirement: RFC7311-3.4.3-4 positive -- a terminal interior metric is added once, not once per forwarding gateway.
// RFC requirement: RFC7311-3.4.3-4 negative -- recursive BGP MED never replaces or contaminates the received AIGP distance.
// RFC requirement: RFC7311-3.4.3-7 positive -- changes to a recursive BGP route are visible to the next computation.
// RFC requirement: RFC7311-3.4.3-7 negative -- unchanged recursion gives the same distance; missing AIGP is explicit rather than a zero metric.
func TestAIGPRecursiveDistanceUsesReceivedMetrics(t *testing.T) {
	loc := locrib.NewRIB()
	first := netip.MustParsePrefix("203.0.113.0/24")
	second := netip.MustParsePrefix("198.51.100.0/24")
	terminal := netip.MustParsePrefix("192.0.2.0/24")
	firstPath := recursivePath(netip.MustParseAddr("198.51.100.1"), 9000)
	firstPath.IsBGP, firstPath.AIGPPresent, firstPath.AIGP = true, true, 100
	secondPath := recursivePath(netip.MustParseAddr("192.0.2.1"), 8000)
	secondPath.IsBGP, secondPath.AIGPPresent, secondPath.AIGP = true, true, 200
	interior := recursivePath(netip.MustParseAddr("10.0.0.1"), 30)
	interior.Interface, interior.OnLink = "eth0", true
	loc.Insert(ipv4Unicast, first, firstPath)
	loc.Insert(ipv4Unicast, second, secondPath)
	loc.Insert(ipv4Unicast, terminal, interior)
	resolver := newNHResolver(loc)
	lookup := func() (uint64, bool) {
		distance := resolver.IGPMetric(netip.MustParseAddr("203.0.113.1"))
		require.True(t, distance.Resolved)
		return distance.Cost, distance.MissingAIGP
	}
	cost, missing := lookup()
	require.Equal(t, uint64(330), cost)
	require.False(t, missing)
	cost, missing = lookup()
	require.Equal(t, uint64(330), cost)
	require.False(t, missing)

	secondPath.AIGP = 400
	loc.Insert(ipv4Unicast, second, secondPath)
	cost, missing = lookup()
	require.Equal(t, uint64(530), cost)
	require.False(t, missing)

	secondPath.AIGPPresent = false
	loc.Insert(ipv4Unicast, second, secondPath)
	_, missing = lookup()
	require.True(t, missing, "a recursive BGP route without AIGP forbids propagation")

	secondPath.AIGPPresent, secondPath.AIGP = true, ^uint64(0)-10
	loc.Insert(ipv4Unicast, second, secondPath)
	cost, missing = lookup()
	require.Equal(t, ^uint64(0), cost, "recursive accumulation must saturate")
	require.False(t, missing)
}

// RFC requirement: RFC7311-3.4.3-4 positive -- recursive static routes resolve through the next hop to its interior distance.
// RFC requirement: RFC7311-3.4.3-4 negative -- neither a recursive static route's preference metric nor an unresolved partial chain is a distance.
func TestAIGPRecursiveStaticDistance(t *testing.T) {
	loc := locrib.NewRIB()
	first := netip.MustParsePrefix("203.0.113.0/24")
	terminal := netip.MustParsePrefix("192.0.2.0/24")
	static := recursivePath(netip.MustParseAddr("192.0.2.1"), 9000)
	static.MetricRecursive = true
	loc.Insert(ipv4Unicast, first, static)
	loc.Insert(ipv4Unicast, terminal, connectedPath(30))
	resolver := newNHResolver(loc)
	target := netip.MustParseAddr("203.0.113.1")
	distance := resolver.IGPMetric(target)
	require.True(t, distance.Resolved)
	require.Equal(t, uint64(30), distance.Cost)

	static.NextHop = netip.MustParseAddr("198.51.100.1")
	loc.Insert(ipv4Unicast, first, static)
	distance = resolver.IGPMetric(target)
	require.False(t, distance.Resolved)
	require.Zero(t, distance.Cost)

	static.MetricRecursive = false
	static.Metric = 25
	static.Interface = "eth0"
	loc.Insert(ipv4Unicast, first, static)
	distance = resolver.IGPMetric(target)
	require.True(t, distance.Resolved)
	require.Equal(t, uint64(25), distance.Cost)
}

func TestAIGPDistanceDistinguishesConnectedZeroFromMissing(t *testing.T) {
	loc := locrib.NewRIB()
	loc.Insert(ipv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), connectedPath(0))
	resolver := newNHResolver(loc)
	connected := resolver.IGPMetric(netip.MustParseAddr("192.0.2.1"))
	require.True(t, connected.Resolved)
	require.Zero(t, connected.Cost)
	missing := resolver.IGPMetric(netip.MustParseAddr("198.51.100.1"))
	require.False(t, missing.Resolved)
}
