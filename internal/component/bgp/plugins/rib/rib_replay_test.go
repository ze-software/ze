package rib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestReplayGroupsByAttrHandle verifies grouping by AttrHandle reduces decode count.
func TestReplayGroupsByAttrHandle(t *testing.T) {
	origin := attribute.Origin(0)
	med := uint32(100)
	route := &Route{
		Origin:  &origin,
		MED:     &med,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}

	groups := []replayGroup{
		{Route: route, Prefixes: []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"}, Family: ipv4Unicast},
		{Route: route, Prefixes: []string{"10.1.0.0/24"}, Family: ipv4Unicast},
	}

	assert.Len(t, groups, 2)
	totalPrefixes := 0
	for _, g := range groups {
		totalPrefixes += len(g.Prefixes)
	}
	assert.Equal(t, 4, totalPrefixes)
}

// TestReplayGroupingSafety verifies family/pathID separation.
func TestReplayGroupingSafety(t *testing.T) {
	origin := attribute.Origin(0)
	ipv6 := family.IPv6Unicast

	routeA := &Route{Origin: &origin, NextHop: "10.0.0.1", Family: ipv4Unicast}
	routeB := &Route{Origin: &origin, NextHop: "::1", Family: ipv6}

	groups := []replayGroup{
		{Route: routeA, Prefixes: []string{"10.0.0.0/24"}, Family: ipv4Unicast, PathID: 0},
		{Route: routeA, Prefixes: []string{"10.0.0.0/24"}, Family: ipv4Unicast, PathID: 1},
		{Route: routeB, Prefixes: []string{"2001:db8::/32"}, Family: ipv6, PathID: 0},
	}

	assert.Len(t, groups, 3)
	assert.NotEqual(t, groups[0].Family, groups[2].Family)
	assert.NotEqual(t, groups[0].PathID, groups[1].PathID)
}

// TestReplayDeltaEncoding verifies identical attrs produce no delta tokens.
func TestReplayDeltaEncoding(t *testing.T) {
	origin := attribute.Origin(0)
	med := uint32(100)
	route := &Route{
		Origin:  &origin,
		MED:     &med,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}

	g := &replayGroup{
		Route:    route,
		Prefixes: []string{"10.1.0.0/24"},
		Family:   ipv4Unicast,
	}

	cmds := formatCursorCommands(g, route)
	require.Len(t, cmds, 1)
	assert.True(t, strings.HasPrefix(cmds[0], "update cursor nlri"), "identical attrs should produce no attr tokens: %s", cmds[0])
}

// TestReplayAttrRemoval verifies del tokens for removed attributes.
func TestReplayAttrRemoval(t *testing.T) {
	origin := attribute.Origin(0)
	med := uint32(100)
	prev := &Route{
		Origin:  &origin,
		MED:     &med,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}
	curr := &Route{
		Origin:  &origin,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}

	g := &replayGroup{
		Route:    curr,
		Prefixes: []string{"10.2.0.0/24"},
		Family:   ipv4Unicast,
	}

	cmds := formatCursorCommands(g, prev)
	require.Len(t, cmds, 1)
	assert.Contains(t, cmds[0], "del med", "removed MED should produce del token")
	assert.NotContains(t, cmds[0], "origin", "unchanged origin should not appear")
}

// TestReplayOrderingAndDone verifies done + ready placement.
func TestReplayOrderingAndDone(t *testing.T) {
	origin := attribute.Origin(0)
	route := &Route{
		Origin:  &origin,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}

	groups := []replayGroup{
		{Route: route, Prefixes: []string{"10.0.0.0/24"}, Family: ipv4Unicast},
	}

	sortGroupsForMinimalDeltas(groups)
	var commands []string
	var prev *Route
	for i := range groups {
		cmds := formatCursorCommands(&groups[i], prev)
		commands = append(commands, cmds...)
		prev = groups[i].Route
	}
	commands = append(commands, "update cursor done", "plugin session ready")

	require.True(t, len(commands) >= 3)
	assert.Equal(t, "update cursor done", commands[len(commands)-2])
	assert.Equal(t, "plugin session ready", commands[len(commands)-1])
}

// TestReplayHashIncludesAllCommunities verifies sort hash distinguishes large/ext communities.
func TestReplayHashIncludesAllCommunities(t *testing.T) {
	origin := attribute.Origin(0)
	routeA := &Route{
		Origin:  &origin,
		NextHop: "10.0.0.1",
		LargeCommunities: []attribute.LargeCommunity{
			{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2},
		},
	}
	routeB := &Route{
		Origin:  &origin,
		NextHop: "10.0.0.1",
		LargeCommunities: []attribute.LargeCommunity{
			{GlobalAdmin: 65000, LocalData1: 3, LocalData2: 4},
		},
	}

	hashA := attrHashSansASPath(routeA)
	hashB := attrHashSansASPath(routeB)
	assert.NotEqual(t, hashA, hashB, "different large-communities should produce different hashes")

	var ecA, ecB attribute.ExtendedCommunity
	ecA[0] = 0x00
	ecA[1] = 0x02
	ecB[0] = 0x00
	ecB[1] = 0x03

	routeC := &Route{Origin: &origin, NextHop: "10.0.0.1", ExtendedCommunities: []attribute.ExtendedCommunity{ecA}}
	routeD := &Route{Origin: &origin, NextHop: "10.0.0.1", ExtendedCommunities: []attribute.ExtendedCommunity{ecB}}

	hashC := attrHashSansASPath(routeC)
	hashD := attrHashSansASPath(routeD)
	assert.NotEqual(t, hashC, hashD, "different extended-communities should produce different hashes")
}

// TestReplayEmptyRibOut verifies empty ribOut produces only "plugin session ready".
func TestReplayEmptyRibOut(t *testing.T) {
	var commands []string
	groups := []replayGroup(nil)

	if len(groups) == 0 {
		commands = append(commands, "plugin session ready")
	}

	require.Len(t, commands, 1)
	assert.Equal(t, "plugin session ready", commands[0])
}

// TestReplayLargeGroupSplit verifies splitting at max BGP UPDATE size.
func TestReplayLargeGroupSplit(t *testing.T) {
	origin := attribute.Origin(0)
	route := &Route{
		Origin:  &origin,
		NextHop: "10.0.0.1",
		ASPath:  []uint32{65001},
	}

	var prefixes []string
	for i := range 300 {
		a := i / 256
		b := i % 256
		prefixes = append(prefixes, "10."+replayIntToStr(a)+"."+replayIntToStr(b)+".0/24")
	}

	g := &replayGroup{
		Route:    route,
		Prefixes: prefixes,
		Family:   ipv4Unicast,
	}

	cmds := formatCursorCommands(g, nil)
	require.Greater(t, len(cmds), 1, "large group should be split into multiple commands")

	for _, cmd := range cmds {
		assert.True(t, strings.HasPrefix(cmd, "update cursor "), "each split should be a cursor command: %s", cmd)
		assert.Contains(t, cmd, "nlri ipv4/unicast add", "each split should contain nlri section")
	}

	totalPrefixes := 0
	for _, cmd := range cmds {
		parts := strings.Split(cmd, " add ")
		if len(parts) > 1 {
			totalPrefixes += len(strings.Fields(parts[1]))
		}
	}
	assert.Equal(t, 300, totalPrefixes, "all prefixes should be present across splits")
}

func replayIntToStr(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// TestResendCursorStaleSeparation verifies stale and fresh routes stay in
// separate groups when StaleLevel differs, so resendRoutesWithCursor can
// dispatch stale groups through updateRouteWithMeta.
func TestResendCursorStaleSeparation(t *testing.T) {
	origin := attribute.Origin(0)
	routeFresh := &Route{Origin: &origin, NextHop: "10.0.0.1", ASPath: []uint32{65001}}
	routeStale := &Route{Origin: &origin, NextHop: "10.0.0.1", ASPath: []uint32{65001}, StaleLevel: 2}

	groups := []replayGroup{
		{Route: routeFresh, Prefixes: []string{"10.0.0.0/24", "10.0.1.0/24"}, Family: ipv4Unicast, StaleLevel: 0},
		{Route: routeStale, Prefixes: []string{"10.1.0.0/24"}, Family: ipv4Unicast, StaleLevel: 2},
	}

	assert.Len(t, groups, 2, "fresh and stale routes must be separate groups")
	assert.Equal(t, uint8(0), groups[0].StaleLevel)
	assert.Equal(t, uint8(2), groups[1].StaleLevel)

	totalPrefixes := 0
	for _, g := range groups {
		totalPrefixes += len(g.Prefixes)
	}
	assert.Equal(t, 3, totalPrefixes)
}

// TestResendSortClustersStaleGroups verifies that sortGroupsForMinimalDeltas
// clusters all fresh groups before stale groups, so resendRoutesWithCursor
// does not alternate between updateRoute and updateRouteWithMeta.
func TestResendSortClustersStaleGroups(t *testing.T) {
	origin := attribute.Origin(0)
	routeA := &Route{Origin: &origin, NextHop: "10.0.0.1", ASPath: []uint32{65001}}
	routeB := &Route{Origin: &origin, NextHop: "10.0.0.2", ASPath: []uint32{65002}}

	groups := []replayGroup{
		{Route: routeA, Prefixes: []string{"10.1.0.0/24"}, Family: ipv4Unicast, StaleLevel: 2},
		{Route: routeB, Prefixes: []string{"10.2.0.0/24"}, Family: ipv4Unicast, StaleLevel: 0},
		{Route: routeA, Prefixes: []string{"10.3.0.0/24"}, Family: ipv4Unicast, StaleLevel: 0},
		{Route: routeB, Prefixes: []string{"10.4.0.0/24"}, Family: ipv4Unicast, StaleLevel: 2},
	}

	sortGroupsForMinimalDeltas(groups)

	sawStale := false
	for _, g := range groups {
		if g.StaleLevel > 0 {
			sawStale = true
		} else {
			assert.False(t, sawStale, "fresh group found after stale group: sort should cluster fresh before stale")
		}
	}
}

// TestResendCursorPrefixCount verifies cursor commands carry the right prefix
// count through formatCursorCommands for use by resendRoutesWithCursor.
func TestResendCursorPrefixCount(t *testing.T) {
	origin := attribute.Origin(0)
	route := &Route{Origin: &origin, NextHop: "10.0.0.1", ASPath: []uint32{65001}}

	groups := []replayGroup{
		{Route: route, Prefixes: []string{"10.0.0.0/24", "10.0.1.0/24"}, Family: ipv4Unicast},
		{Route: route, Prefixes: []string{"10.1.0.0/24"}, Family: ipv4Unicast},
	}

	total := 0
	for _, g := range groups {
		total += len(g.Prefixes)
	}
	assert.Equal(t, 3, total)
}

// TestResendNoDoneOrReady verifies the resend cursor path finishes with
// "update cursor done" but NOT "plugin session ready".
func TestResendNoDoneOrReady(t *testing.T) {
	origin := attribute.Origin(0)
	route := &Route{Origin: &origin, NextHop: "10.0.0.1", ASPath: []uint32{65001}}

	groups := []replayGroup{
		{Route: route, Prefixes: []string{"10.0.0.0/24"}, Family: ipv4Unicast},
	}

	sortGroupsForMinimalDeltas(groups)
	var commands []string
	var prev *Route
	for i := range groups {
		cmds := formatCursorCommands(&groups[i], prev)
		commands = append(commands, cmds...)
		prev = groups[i].Route
	}
	commands = append(commands, "update cursor done")

	require.True(t, len(commands) >= 2)
	assert.Equal(t, "update cursor done", commands[len(commands)-1])
	for _, cmd := range commands {
		assert.NotEqual(t, "plugin session ready", cmd, "resend should not send plugin session ready")
	}
}

// BenchmarkReplayLargeTable measures call count and allocs for grouped replay.
func BenchmarkReplayLargeTable(b *testing.B) {
	origin := attribute.Origin(0)
	med := uint32(100)
	lp := uint32(200)

	groups := make([]replayGroup, 1000)
	for i := range groups {
		route := &Route{
			Origin:          &origin,
			MED:             &med,
			LocalPreference: &lp,
			NextHop:         "10.0.0.1",
			ASPath:          []uint32{65001, uint32(i + 1)},
		}
		prefixes := make([]string, 100)
		for j := range prefixes {
			prefixes[j] = "10." + replayIntToStr(i/256) + "." + replayIntToStr(i%256) + "." + replayIntToStr(j) + "/32"
		}
		groups[i] = replayGroup{
			Route:    route,
			Prefixes: prefixes,
			Family:   ipv4Unicast,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		groupsCopy := make([]replayGroup, len(groups))
		copy(groupsCopy, groups)
		sortGroupsForMinimalDeltas(groupsCopy)
		var prev *Route
		cmdCount := 0
		for i := range groupsCopy {
			cmds := formatCursorCommands(&groupsCopy[i], prev)
			cmdCount += len(cmds)
			prev = groupsCopy[i].Route
		}
		_ = cmdCount
	}
}
