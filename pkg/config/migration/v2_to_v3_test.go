package migration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigrateV2ToV3NeighborToPeer verifies neighbor→peer rename.
//
// VALIDATES: "neighbor <IP>" becomes "peer <IP>".
//
// PREVENTS: Neighbor configs being lost during migration.
func TestMigrateV2ToV3NeighborToPeer(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)

	// Verify it's v2 before migration
	require.Equal(t, Version2, DetectVersion(tree))

	// Migrate
	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify neighbor is gone
	neighbors := result.GetList("neighbor")
	require.Empty(t, neighbors, "neighbor list should be empty after migration")

	// Verify peer exists with correct data
	peers := result.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["192.0.2.1"]
	require.NotNil(t, peer)

	val, ok := peer.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	// Verify it's now v3
	require.Equal(t, Version3, DetectVersion(result))
}

// TestMigrateV2ToV3PeerGlobToMatch verifies peer glob→template.match.
//
// VALIDATES: Root "peer *" becomes "template { match * }".
//
// PREVENTS: Glob patterns being lost during migration.
func TestMigrateV2ToV3PeerGlobToMatch(t *testing.T) {
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
	require.Equal(t, Version2, DetectVersion(tree))

	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// Verify root peer globs are gone
	peers := result.GetList("peer")
	require.Len(t, peers, 1, "only non-glob peer should remain")
	_, hasIP := peers["192.0.2.1"]
	require.True(t, hasIP)

	// Verify template.match has the globs
	tmpl := result.GetContainer("template")
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

// TestMigrateV2ToV3TemplateNeighborToGroup verifies template.neighbor→template.group.
//
// VALIDATES: "template { neighbor <name> }" becomes "template { group <name> }".
//
// PREVENTS: Named templates being lost during migration.
func TestMigrateV2ToV3TemplateNeighborToGroup(t *testing.T) {
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
	require.Equal(t, Version2, DetectVersion(tree))

	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// Verify template.neighbor is gone
	tmpl := result.GetContainer("template")
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

// TestMigrateV2ToV3PreservesOrder verifies match blocks preserve config order.
//
// VALIDATES: Migration preserves order of peer globs for match blocks.
//
// PREVENTS: Match order being scrambled (important for precedence).
func TestMigrateV2ToV3PreservesOrder(t *testing.T) {
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

	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	tmpl := result.GetContainer("template")
	require.NotNil(t, tmpl)

	// Get ordered matches
	matches := tmpl.GetListOrdered("match")
	require.Len(t, matches, 3)

	// Verify order is preserved
	require.Equal(t, "*", matches[0].Key)
	require.Equal(t, "10.*.*.*", matches[1].Key)
	require.Equal(t, "192.168.*.*", matches[2].Key)
}

// TestMigrateV2ToV3Idempotent verifies migration is idempotent.
//
// VALIDATES: Running migration twice produces same result.
//
// PREVENTS: Broken configs from repeated migration.
func TestMigrateV2ToV3Idempotent(t *testing.T) {
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
	result1, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// Second migration (on already-migrated config)
	result2, err := MigrateV2ToV3(result1)
	require.NoError(t, err)

	// Both should be v3
	require.Equal(t, Version3, DetectVersion(result1))
	require.Equal(t, Version3, DetectVersion(result2))

	// Should have same structure
	peers1 := result1.GetList("peer")
	peers2 := result2.GetList("peer")
	require.Equal(t, len(peers1), len(peers2))
}

// TestMigrateV2ToV3DoesNotMutateOriginal verifies original tree is unchanged.
//
// VALIDATES: Migration clones before modifying.
//
// PREVENTS: Original config corruption.
func TestMigrateV2ToV3DoesNotMutateOriginal(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)

	// Migrate
	_, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// Original should still have neighbor
	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 1, "original should be unchanged")
}

// TestMigrateV2ToV3CIDRPattern verifies CIDR patterns migrate correctly.
//
// VALIDATES: "peer 10.0.0.0/8 { }" becomes "template { match 10.0.0.0/8 { } }".
//
// PREVENTS: CIDR patterns being lost during migration.
func TestMigrateV2ToV3CIDRPattern(t *testing.T) {
	input := `
peer 10.0.0.0/8 {
    hold-time 90;
}
peer 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	require.Equal(t, Version2, DetectVersion(tree))

	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// CIDR pattern should move to template.match
	tmpl := result.GetContainer("template")
	require.NotNil(t, tmpl)

	matches := tmpl.GetList("match")
	require.Len(t, matches, 1)

	cidrMatch := matches["10.0.0.0/8"]
	require.NotNil(t, cidrMatch)
	val, _ := cidrMatch.Get("hold-time")
	require.Equal(t, "90", val)

	// Non-CIDR peer should remain
	peers := result.GetList("peer")
	require.Len(t, peers, 1)
	_, hasIP := peers["192.0.2.1"]
	require.True(t, hasIP)
}

// TestMigrateV2ToV3MixedConfig verifies partially-migrated configs work.
//
// VALIDATES: Config with both v3 and v2 syntax migrates correctly.
//
// PREVENTS: Mixed configs causing errors.
func TestMigrateV2ToV3MixedConfig(t *testing.T) {
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
	require.Equal(t, Version2, DetectVersion(tree))

	result, err := MigrateV2ToV3(tree)
	require.NoError(t, err)

	// template.neighbor should be renamed to template.group
	tmpl := result.GetContainer("template")
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
	neighbors := result.GetList("neighbor")
	require.Empty(t, neighbors)

	peers := result.GetList("peer")
	require.Len(t, peers, 1)
}
