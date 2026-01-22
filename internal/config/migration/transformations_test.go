//nolint:goconst // Test file uses inline strings for readability
package migration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTransformNeighborToPeer verifies neighbor→peer rename inside bgp {}.
//
// VALIDATES: "neighbor <IP>" becomes "bgp { peer <IP> }".
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

	// Verify neighbor is gone at root
	neighbors := result.Tree.GetList("neighbor")
	require.Empty(t, neighbors, "neighbor list should be empty after migration")

	// Verify peer is inside bgp {}
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peers := bgpContainer.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["192.0.2.1"]
	require.NotNil(t, peer)

	val, ok := peer.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	// Verify no longer needs migration
	require.False(t, NeedsMigration(result.Tree))
}

// TestMigratePeerGlobToMatch verifies peer glob→template.bgp.peer.
//
// VALIDATES: Root "peer *" becomes "template { bgp { peer * } }".
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

	// Verify peer is inside bgp {}
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peers := bgpContainer.GetList("peer")
	require.Len(t, peers, 1, "only non-glob peer should be in bgp")
	_, hasIP := peers["192.0.2.1"]
	require.True(t, hasIP)

	// Verify template.bgp.peer has the globs
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	tmplPeers := tmplBgp.GetList("peer")
	require.Len(t, tmplPeers, 2, "template.bgp should have 2 peer patterns")

	// Check * pattern
	matchAll := tmplPeers["*"]
	require.NotNil(t, matchAll)
	val, _ := matchAll.Get("hold-time")
	require.Equal(t, "90", val)

	// Check 192.168.*.* pattern
	matchSubnet := tmplPeers["192.168.*.*"]
	require.NotNil(t, matchSubnet)
	val, _ = matchSubnet.Get("hold-time")
	require.Equal(t, "60", val)
}

// TestMigrateTemplateNeighborToGroup verifies template.neighbor→template.bgp.peer.
//
// VALIDATES: "template { neighbor <name> }" becomes "template { bgp { peer * { inherit-name <name> } } }".
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

	// Verify template.group is gone (converted to template.bgp.peer)
	groups := tmpl.GetList("group")
	require.Empty(t, groups, "template.group should be empty after full migration")

	// Verify template.bgp.peer has entries with inherit-name
	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	// Groups become peer * with inherit-name
	patterns := tmplBgp.GetListOrdered("peer")
	require.Len(t, patterns, 2, "should have 2 patterns for ibgp and ebgp")

	// Check inherit-name values
	var inheritNames []string
	for _, p := range patterns {
		name, ok := p.Value.Get("inherit-name")
		if ok {
			inheritNames = append(inheritNames, name)
		}
	}
	require.Contains(t, inheritNames, "ibgp")
	require.Contains(t, inheritNames, "ebgp")
}

// TestMigratePreservesMatchOrder verifies template.bgp.peer blocks preserve config order.
//
// VALIDATES: Migration preserves order of peer globs in template.bgp.peer.
//
// PREVENTS: Pattern order being scrambled (important for precedence).
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

	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	// Get ordered peer patterns from template.bgp.peer
	patterns := tmplBgp.GetListOrdered("peer")
	require.Len(t, patterns, 3)

	// Verify order is preserved
	require.Equal(t, "*", patterns[0].Key)
	require.Equal(t, "10.*.*.*", patterns[1].Key)
	require.Equal(t, "192.168.*.*", patterns[2].Key)
}

// TestMigratePreservesPeerOrder verifies neighbor→peer preserves order.
//
// VALIDATES: Multiple neighbors become peers in same order inside bgp {}.
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

	// Get bgp container
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	// Get ordered peers inside bgp {}
	peers := bgpContainer.GetListOrdered("peer")
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

	// Should have same structure - peers inside bgp {}
	bgp1 := result1.Tree.GetContainer("bgp")
	bgp2 := result2.Tree.GetContainer("bgp")
	require.NotNil(t, bgp1)
	require.NotNil(t, bgp2)
	require.Equal(t, len(bgp1.GetList("peer")), len(bgp2.GetList("peer")))
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

// TestMigratePatternToTemplateBgpPeer verifies glob/CIDR patterns migrate correctly.
//
// VALIDATES: Patterns like "peer 10.0.0.0/8 {}" or "peer 2001:db8::* {}" become template.bgp.peer.
//
// PREVENTS: Patterns being lost during migration.
func TestMigratePatternToTemplateBgpPeer(t *testing.T) {
	tests := []struct {
		name         string
		patternKey   string
		patternInput string
		peerKey      string
		peerInput    string
	}{
		{
			name:         "CIDR_pattern",
			patternKey:   "10.0.0.0/8",
			patternInput: "peer 10.0.0.0/8 { hold-time 90; }",
			peerKey:      "192.0.2.1",
			peerInput:    "peer 192.0.2.1 { local-as 65000; }",
		},
		{
			name:         "IPv6_glob_pattern",
			patternKey:   "2001:db8::*",
			patternInput: "peer 2001:db8::* { hold-time 90; }",
			peerKey:      "2001:db8::1",
			peerInput:    "peer 2001:db8::1 { local-as 65000; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.patternInput + "\n" + tt.peerInput

			tree := parseWithBGPSchema(t, input)
			require.True(t, NeedsMigration(tree))

			result, err := Migrate(tree)
			require.NoError(t, err)

			// Pattern should move to template.bgp.peer
			tmpl := result.Tree.GetContainer("template")
			require.NotNil(t, tmpl)

			tmplBgp := tmpl.GetContainer("bgp")
			require.NotNil(t, tmplBgp, "template.bgp should exist")

			patterns := tmplBgp.GetList("peer")
			require.Len(t, patterns, 1)

			pattern := patterns[tt.patternKey]
			require.NotNil(t, pattern)
			val, _ := pattern.Get("hold-time")
			require.Equal(t, "90", val)

			// Non-pattern peer should be inside bgp {}
			bgpContainer := result.Tree.GetContainer("bgp")
			require.NotNil(t, bgpContainer)

			peers := bgpContainer.GetList("peer")
			require.Len(t, peers, 1)
			_, hasIP := peers[tt.peerKey]
			require.True(t, hasIP)
		})
	}
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

	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	// template.neighbor should be empty (converted)
	oldNeighbors := tmpl.GetList("neighbor")
	require.Empty(t, oldNeighbors)

	// template.group should be empty (converted to template.bgp.peer)
	groups := tmpl.GetList("group")
	require.Empty(t, groups, "template.group should be empty after full migration")

	// template.match should be empty (converted to template.bgp.peer)
	matches := tmpl.GetList("match")
	require.Empty(t, matches, "template.match should be empty after full migration")

	// template.bgp.peer should have both the match pattern and the group
	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	patterns := tmplBgp.GetListOrdered("peer")
	require.Len(t, patterns, 2, "should have 2 patterns (wildcard + ibgp group)")

	// neighbor should become peer inside bgp {}
	neighbors := result.Tree.GetList("neighbor")
	require.Empty(t, neighbors)

	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	peers := bgpContainer.GetList("peer")
	require.Len(t, peers, 1)
}

// TestMigrateStaticToAnnounce verifies static→announce extraction.
//
// VALIDATES: neighbor.static routes become bgp.peer.announce.<afi>.<safi>.
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

	// Should be peer inside bgp {} now (neighbor→bgp.peer)
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peers := bgpContainer.GetList("peer")
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
// VALIDATES: Peer with deprecated static block is still migrated into bgp {}.
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

	// Peer should be inside bgp {}
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peer := bgpContainer.GetList("peer")["192.0.2.1"]
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
// VALIDATES: template.neighbor.static becomes template.bgp.peer.announce.
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

	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)

	// template.neighbor should be empty
	oldNeighbors := tmpl.GetList("neighbor")
	require.Empty(t, oldNeighbors)

	// template.group should be empty (converted to template.bgp.peer)
	groups := tmpl.GetList("group")
	require.Empty(t, groups, "template.group should be empty after full migration")

	// template.bgp.peer should have the entry with inherit-name ibgp
	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	patterns := tmplBgp.GetListOrdered("peer")
	require.Len(t, patterns, 1, "should have 1 pattern")

	pattern := patterns[0].Value
	require.NotNil(t, pattern)

	// Check inherit-name
	inheritName, ok := pattern.Get("inherit-name")
	require.True(t, ok)
	require.Equal(t, "ibgp", inheritName)

	// static should be gone
	require.Nil(t, pattern.GetContainer("static"))

	// announce should exist with routes
	announce := pattern.GetContainer("announce")
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
	peerAs, ok := pattern.Get("peer-as")
	require.True(t, ok)
	require.Equal(t, "65000", peerAs)

	// Neighbor should be peer inside bgp {}
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peer := bgpContainer.GetList("peer")["192.0.2.1"]
	require.NotNil(t, peer)
}

// TestMigrateTemplateGroupStatic verifies template.group static migration.
//
// VALIDATES: template.group.static becomes template.bgp.peer.announce.
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

	// template.group should be empty (converted to template.bgp.peer)
	groups := tmpl.GetList("group")
	require.Empty(t, groups, "template.group should be empty after full migration")

	// template.bgp.peer should have the entry
	tmplBgp := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBgp, "template.bgp should exist")

	patterns := tmplBgp.GetListOrdered("peer")
	require.Len(t, patterns, 1, "should have 1 pattern")

	pattern := patterns[0].Value
	require.NotNil(t, pattern)

	// Check inherit-name
	inheritName, ok := pattern.Get("inherit-name")
	require.True(t, ok)
	require.Equal(t, "vpn-customers", inheritName)

	// static should be gone
	require.Nil(t, pattern.GetContainer("static"))

	// announce should exist
	announce := pattern.GetContainer("announce")
	require.NotNil(t, announce)

	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	require.Len(t, ipv4.GetList("unicast"), 1)

	// Peer should be inside bgp {}
	bgpContainer := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "bgp container should exist")

	peer := bgpContainer.GetList("peer")["192.0.2.1"]
	require.NotNil(t, peer)
}
