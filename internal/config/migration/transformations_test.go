//nolint:goconst // Test file uses inline strings for readability
package migration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTransformNeighborToPeer verifies neighbor→peer rename.
//
// VALIDATES: "neighbor <IP>" becomes "peer <IP>".
//
// PREVENTS: Neighbor configs being lost during migration.
func TestTransformNeighborToPeer(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)

	// Verify needs migration before
	require.True(t, NeedsMigration(tree))

	// Migrate
	result, err := Migrate(tree)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify neighbor is gone
	neighbors := result.Tree.GetList("neighbor")
	require.Empty(t, neighbors, "neighbor list should be empty after migration")

	// Verify peer exists with correct data
	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["192.0.2.1"]
	require.NotNil(t, peer)

	val, ok := peer.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	// Verify no longer needs migration
	require.False(t, NeedsMigration(result.Tree))
}

// TestMigratePeerGlobToMatch verifies peer glob→template.match.
//
// VALIDATES: Root "peer *" becomes "template { match * }".
//
// PREVENTS: Glob patterns being lost during migration.
func TestMigratePeerGlobToMatch(t *testing.T) {
	input := `
peer * {
    hold-time 90;
}
peer 192.168.*.* {
    hold-time 60;
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// Verify root peer globs are gone
	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1, "only non-glob peer should remain")
	_, hasIP := peers["192.0.2.1"]
	require.True(t, hasIP)

	// Verify template.match has the globs
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	matches := tmpl.GetList("match")
	require.Len(t, matches, 2)

	// Check * pattern
	matchAll := matches["*"]
	require.NotNil(t, matchAll)
	val, _ := matchAll.Get("hold-time")
	require.Equal(t, "90", val)

	// Check 192.168.*.* pattern
	matchSubnet := matches["192.168.*.*"]
	require.NotNil(t, matchSubnet)
	val, _ = matchSubnet.Get("hold-time")
	require.Equal(t, "60", val)
}

// TestMigrateTemplateNeighborToGroup verifies template.neighbor→template.group.
//
// VALIDATES: "template { neighbor <name> }" becomes "template { group <name> }".
//
// PREVENTS: Named templates being lost during migration.
func TestMigrateTemplateNeighborToGroup(t *testing.T) {
	input := `
template {
    neighbor ibgp {
        peer-as 65000;
    }
    neighbor ebgp {
        peer-as 65001;
    }
}
neighbor 192.0.2.1 {
    inherit ibgp;
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// Verify template.neighbor is gone
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	oldNeighbors := tmpl.GetList("neighbor")
	require.Empty(t, oldNeighbors, "template.neighbor should be empty")

	// Verify template.group has the templates
	groups := tmpl.GetList("group")
	require.Len(t, groups, 2)

	ibgp := groups["ibgp"]
	require.NotNil(t, ibgp)
	val, _ := ibgp.Get("peer-as")
	require.Equal(t, "65000", val)

	ebgp := groups["ebgp"]
	require.NotNil(t, ebgp)
	val, _ = ebgp.Get("peer-as")
	require.Equal(t, "65001", val)
}

// TestMigratePreservesMatchOrder verifies match blocks preserve config order.
//
// VALIDATES: Migration preserves order of peer globs for match blocks.
//
// PREVENTS: Match order being scrambled (important for precedence).
func TestMigratePreservesMatchOrder(t *testing.T) {
	input := `
peer * {
    hold-time 90;
}
peer 10.*.*.* {
    hold-time 80;
}
peer 192.168.*.* {
    hold-time 60;
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := Migrate(tree)
	require.NoError(t, err)

	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	// Get ordered matches
	matches := tmpl.GetListOrdered("match")
	require.Len(t, matches, 3)

	// Verify order is preserved
	require.Equal(t, "*", matches[0].Key)
	require.Equal(t, "10.*.*.*", matches[1].Key)
	require.Equal(t, "192.168.*.*", matches[2].Key)
}

// TestMigratePreservesPeerOrder verifies neighbor→peer preserves order.
//
// VALIDATES: Multiple neighbors become peers in same order.
//
// PREVENTS: Peer order being scrambled.
func TestMigratePreservesPeerOrder(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
}
neighbor 10.0.0.1 {
    local-as 65001;
}
neighbor 172.16.0.1 {
    local-as 65002;
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := Migrate(tree)
	require.NoError(t, err)

	// Get ordered peers
	peers := result.Tree.GetListOrdered("peer")
	require.Len(t, peers, 3)

	// Verify order is preserved
	require.Equal(t, "192.0.2.1", peers[0].Key)
	require.Equal(t, "10.0.0.1", peers[1].Key)
	require.Equal(t, "172.16.0.1", peers[2].Key)
}

// TestMigrateIdempotent verifies migration is idempotent.
//
// VALIDATES: Running migration twice produces same result.
//
// PREVENTS: Broken configs from repeated migration.
func TestMigrateIdempotent(t *testing.T) {
	input := `
peer * {
    hold-time 90;
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)

	// First migration
	result1, err := Migrate(tree)
	require.NoError(t, err)

	// Second migration (on already-migrated config)
	result2, err := Migrate(result1.Tree)
	require.NoError(t, err)

	// Both should be current syntax
	require.False(t, NeedsMigration(result1.Tree))
	require.False(t, NeedsMigration(result2.Tree))

	// Should have same structure
	peers1 := result1.Tree.GetList("peer")
	peers2 := result2.Tree.GetList("peer")
	require.Equal(t, len(peers1), len(peers2))
}

// TestMigrateDoesNotMutateOriginal verifies original tree is unchanged.
//
// VALIDATES: Migration clones before modifying.
//
// PREVENTS: Original config corruption.
func TestMigrateDoesNotMutateOriginal(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)

	// Migrate
	_, err := Migrate(tree)
	require.NoError(t, err)

	// Original should still have neighbor
	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 1, "original should be unchanged")
}

// TestMigrateNilTree verifies nil tree handling.
//
// VALIDATES: Nil tree returns ErrNilTree without panic.
//
// PREVENTS: Nil pointer dereference.
func TestMigrateNilTreeV2ToV3(t *testing.T) {
	result, err := Migrate(nil)
	require.ErrorIs(t, err, ErrNilTree)
	require.Nil(t, result, "nil input should return nil result")
}

// TestMigrateCIDRPattern verifies CIDR patterns migrate correctly.
//
// VALIDATES: "peer 10.0.0.0/8 { }" becomes "template { match 10.0.0.0/8 { } }".
//
// PREVENTS: CIDR patterns being lost during migration.
func TestMigrateCIDRPattern(t *testing.T) {
	input := `
peer 10.0.0.0/8 {
    hold-time 90;
}
peer 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// CIDR pattern should move to template.match
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	matches := tmpl.GetList("match")
	require.Len(t, matches, 1)

	cidrMatch := matches["10.0.0.0/8"]
	require.NotNil(t, cidrMatch)
	val, _ := cidrMatch.Get("hold-time")
	require.Equal(t, "90", val)

	// Non-CIDR peer should remain
	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1)
	_, hasIP := peers["192.0.2.1"]
	require.True(t, hasIP)
}

// TestMigrateIPv6GlobPattern verifies IPv6 glob patterns migrate correctly.
//
// VALIDATES: "peer 2001:db8::* { }" becomes "template { match 2001:db8::* { } }".
//
// PREVENTS: IPv6 glob patterns being lost during migration.
func TestMigrateIPv6GlobPattern(t *testing.T) {
	input := `
peer 2001:db8::* {
    hold-time 90;
}
peer 2001:db8::1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// IPv6 glob pattern should move to template.match
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	matches := tmpl.GetList("match")
	require.Len(t, matches, 1)

	ipv6Match := matches["2001:db8::*"]
	require.NotNil(t, ipv6Match)
	val, _ := ipv6Match.Get("hold-time")
	require.Equal(t, "90", val)

	// Non-glob IPv6 peer should remain
	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1)
	_, hasIP := peers["2001:db8::1"]
	require.True(t, hasIP)
}

// TestMigrateMixedConfig verifies partially-migrated configs work.
//
// VALIDATES: Config with both current and old syntax migrates correctly.
//
// PREVENTS: Mixed configs causing errors.
func TestMigrateMixedConfig(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
    }
    neighbor ibgp {
        peer-as 65000;
    }
}
neighbor 192.0.2.1 {
    inherit ibgp;
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// template.neighbor should be renamed to template.group
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	oldNeighbors := tmpl.GetList("neighbor")
	require.Empty(t, oldNeighbors)

	groups := tmpl.GetList("group")
	require.Len(t, groups, 1)
	_, hasIbgp := groups["ibgp"]
	require.True(t, hasIbgp)

	// Existing match should be preserved
	matches := tmpl.GetList("match")
	require.Len(t, matches, 1)
	_, hasWildcard := matches["*"]
	require.True(t, hasWildcard)

	// neighbor should become peer
	neighbors := result.Tree.GetList("neighbor")
	require.Empty(t, neighbors)

	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1)
}

// TestMigrateStaticToAnnounce verifies static→announce extraction.
//
// VALIDATES: neighbor.static routes become peer.announce.<afi>.<safi>.
//
// PREVENTS: Static routes being lost during migration.
func TestMigrateStaticToAnnounce(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 next-hop self;
        route 2001:db8::/32 next-hop self;
        route 224.0.0.0/4 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// Should be peer now (neighbor→peer)
	peers := result.Tree.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["192.0.2.1"]
	require.NotNil(t, peer)

	// static should be gone
	require.Nil(t, peer.GetContainer("static"))

	// announce should exist with routes
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)

	// IPv4 unicast
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	ipv4Unicast := ipv4.GetList("unicast")
	require.Len(t, ipv4Unicast, 1)
	require.NotNil(t, ipv4Unicast["10.0.0.0/8"])

	// IPv4 multicast
	ipv4Mcast := ipv4.GetList("multicast")
	require.Len(t, ipv4Mcast, 1)
	require.NotNil(t, ipv4Mcast["224.0.0.0/4"])

	// IPv6 unicast
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	ipv6Unicast := ipv6.GetList("unicast")
	require.Len(t, ipv6Unicast, 1)
	require.NotNil(t, ipv6Unicast["2001:db8::/32"])
}

// TestMigratePeerWithStatic verifies peer+static is migrated.
//
// VALIDATES: Peer with deprecated static block is still migrated.
//
// PREVENTS: Configs using peer (not neighbor) with static being skipped.
func TestMigratePeerWithStatic(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	// Should detect as needing migration because of static block
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	peer := result.Tree.GetList("peer")["192.0.2.1"]
	require.NotNil(t, peer)

	// static should be gone
	require.Nil(t, peer.GetContainer("static"))

	// announce should exist
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)

	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	require.Len(t, ipv4.GetList("unicast"), 1)
}

// TestMigrateTemplateNeighborWithStatic verifies template.neighbor with static migration.
//
// VALIDATES: template.neighbor.static becomes template.group.announce.
//
// PREVENTS: Static routes in template.neighbor being lost during rename.
func TestMigrateTemplateNeighborWithStatic(t *testing.T) {
	input := `
template {
    neighbor ibgp {
        peer-as 65000;
        static {
            route 10.0.0.0/8 next-hop self;
            route 2001:db8::/32 next-hop self;
        }
    }
}
neighbor 192.0.2.1 {
    inherit ibgp;
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	// template.neighbor should become template.group
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	oldNeighbors := tmpl.GetList("neighbor")
	require.Empty(t, oldNeighbors)

	groups := tmpl.GetList("group")
	require.Len(t, groups, 1)

	group := groups["ibgp"]
	require.NotNil(t, group)

	// static should be gone
	require.Nil(t, group.GetContainer("static"))

	// announce should exist with routes
	announce := group.GetContainer("announce")
	require.NotNil(t, announce)

	// IPv4
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	require.Len(t, ipv4.GetList("unicast"), 1)

	// IPv6
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	require.Len(t, ipv6.GetList("unicast"), 1)

	// peer-as should still be there
	peerAs, ok := group.Get("peer-as")
	require.True(t, ok)
	require.Equal(t, "65000", peerAs)
}

// TestMigrateTemplateGroupStatic verifies template.group static migration.
//
// VALIDATES: template.group.static becomes template.group.announce.
//
// PREVENTS: Template static routes being skipped.
func TestMigrateTemplateGroupStatic(t *testing.T) {
	input := `
template {
    group vpn-customers {
        static {
            route 10.0.0.0/8 next-hop self;
        }
    }
}
peer 192.0.2.1 {
    inherit vpn-customers;
}
`
	tree := parseWithBGPSchema(t, input)

	// Should detect as needing migration because of static in template.group
	require.True(t, NeedsMigration(tree))

	result, err := Migrate(tree)
	require.NoError(t, err)

	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	group := tmpl.GetList("group")["vpn-customers"]
	require.NotNil(t, group)

	// static should be gone
	require.Nil(t, group.GetContainer("static"))

	// announce should exist
	announce := group.GetContainer("announce")
	require.NotNil(t, announce)

	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	require.Len(t, ipv4.GetList("unicast"), 1)
}
