package migration

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"github.com/stretchr/testify/require"
)

// legacyNeighborBlock is a common test fixture for old-style neighbor config.
const legacyNeighborBlock = `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`

// TestDetectLegacyNeighborAtRoot verifies detection of neighbor at root.
//
// VALIDATES: Config with "neighbor <IP>" needs migration.
//
// PREVENTS: Old configs being treated as current.
func TestDetectLegacyNeighborAtRoot(t *testing.T) {
	input := legacyNeighborBlock
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "neighbor at root needs migration")
}

// TestDetectLegacyPeerGlobAtRoot verifies detection of peer glob at root.
//
// VALIDATES: Config with "peer *" glob at root needs migration.
//
// PREVENTS: Root-level peer globs being treated as current.
func TestDetectLegacyPeerGlobAtRoot(t *testing.T) {
	input := `
peer * {
    hold-time 90;
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "peer glob at root needs migration")
}

// TestDetectLegacyTemplateNeighbor verifies detection of template.neighbor.
//
// VALIDATES: Config with template { neighbor <name> } needs migration.
//
// PREVENTS: Old template syntax being treated as current.
func TestDetectLegacyTemplateNeighbor(t *testing.T) {
	input := `
template {
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
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "template.neighbor needs migration")
}

// TestDetectCurrentPeerAtRoot verifies detection of peer IP at root.
//
// VALIDATES: Config with "peer <IP>" (not glob) does not need migration.
//
// PREVENTS: Current syntax being mistaken for old.
func TestDetectCurrentPeerAtRoot(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "peer IP at root does not need migration")
}

// TestDetectCurrentTemplateGroup verifies detection of template.group.
//
// VALIDATES: Config with template { group <name> } does not need migration.
//
// PREVENTS: Current template syntax being treated as old.
func TestDetectCurrentTemplateGroup(t *testing.T) {
	input := `
template {
    group ibgp {
        peer-as 65000;
    }
}
peer 192.0.2.1 {
    inherit ibgp;
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "template.group does not need migration")
}

// TestDetectCurrentTemplateMatch verifies detection of template.match.
//
// VALIDATES: Config with template { match * } does not need migration.
//
// PREVENTS: Match blocks being treated as old syntax.
func TestDetectCurrentTemplateMatch(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
    }
}
peer 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "template.match does not need migration")
}

// TestDetectMixedSyntax verifies mixed configs need migration.
//
// VALIDATES: Config with both current (template.match) AND old (neighbor) needs migration.
//
// PREVENTS: Partially migrated configs being treated as complete.
func TestDetectMixedSyntax(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
    }
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "mixed syntax needs migration")
}

// TestDetectEmptyConfig verifies empty config is treated as current.
//
// VALIDATES: Empty config returns false (no migration needed).
//
// PREVENTS: Empty configs causing errors.
func TestDetectEmptyConfig(t *testing.T) {
	tree := config.NewTree()
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "empty config does not need migration")
}

// TestNeedsMigrationNilTree verifies nil tree handling.
//
// VALIDATES: Nil tree returns false without panic.
//
// PREVENTS: Nil pointer dereference.
func TestNeedsMigrationNilTree(t *testing.T) {
	require.False(t, NeedsMigration(nil), "nil tree should not need migration")
}

// TestDetectCIDRPatternAtRoot verifies CIDR patterns at root need migration.
//
// VALIDATES: "peer 10.0.0.0/8 { }" at root needs migration to template.match.
//
// PREVENTS: CIDR patterns at root being treated as current.
func TestDetectCIDRPatternAtRoot(t *testing.T) {
	input := `
peer 10.0.0.0/8 {
    hold-time 90;
}
peer 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "CIDR pattern at root needs migration")
}

// TestDetectIPv6GlobPatternAtRoot verifies IPv6 glob patterns at root need migration.
//
// VALIDATES: "peer 2001:db8::* { }" at root needs migration to template.match.
//
// PREVENTS: IPv6 glob patterns at root being treated as current.
func TestDetectIPv6GlobPatternAtRoot(t *testing.T) {
	input := `
peer 2001:db8::* {
    hold-time 90;
}
peer 2001:db8::1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "IPv6 glob pattern at root needs migration")
}

// TestDetectPeerWithStatic verifies peer with static block needs migration.
//
// VALIDATES: "peer { static { } }" needs migration to announce.
//
// PREVENTS: Peer with deprecated static block being skipped.
func TestDetectPeerWithStatic(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "peer with static block needs migration")
}

// TestDetectTemplateGroupWithStatic verifies template.group with static needs migration.
//
// VALIDATES: "template { group { static { } } }" needs migration.
//
// PREVENTS: Template static blocks being skipped.
func TestDetectTemplateGroupWithStatic(t *testing.T) {
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
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "template.group with static needs migration")
}

// TestDetectPeerWithAnonymousAPI verifies peer with old-style anonymous api needs migration.
//
// VALIDATES: "peer { process { processes [...] } }" needs migration to named api.
//
// PREVENTS: Old API syntax being skipped during migration.
func TestDetectPeerWithAnonymousAPI(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    process {
        processes [ foo ];
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "peer with anonymous process block needs migration")
}

// TestDetectTemplateGroupWithAnonymousAPI verifies template.group with old process needs migration.
//
// VALIDATES: "template { group { process { processes [...] } } }" needs migration.
//
// PREVENTS: Template anonymous process blocks being skipped.
func TestDetectTemplateGroupWithAnonymousAPI(t *testing.T) {
	input := `
process collector {
    run ./collector.run;
}
template {
    group collectors {
        process {
            processes [ collector ];
        }
    }
}
peer 192.0.2.1 {
    inherit collectors;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "template.group with anonymous api needs migration")
}

// TestDetectPeerWithNamedAPI verifies peer with new-style named api does not need migration.
//
// VALIDATES: "peer { api foo { } }" does not need migration (already current).
//
// PREVENTS: Already-migrated configs being flagged for migration.
func TestDetectPeerWithNamedAPI(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    process foo {
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "peer with named process block does not need migration")
}

// TestDetectNamedAPIWithProcesses verifies named api with processes field needs migration.
//
// VALIDATES: "api speaking { processes [...] }" needs migration.
//
// PREVENTS: Named blocks with old syntax being skipped.
func TestDetectNamedAPIWithProcesses(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    process speaking {
        processes [ foo ];
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "named api with processes field needs migration")
}

// TestDetectNamedAPIWithFormatFlags verifies named api with format flags needs migration.
//
// VALIDATES: "api foo { receive { parsed; } }" needs migration.
//
// PREVENTS: Format flags in named blocks being skipped.
func TestDetectNamedAPIWithFormatFlags(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    process foo {
        receive {
            parsed;
            update;
        }
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "named api with format flags needs migration")
}

// TestDetectNamedAPIWithNewSyntax verifies named api with new syntax does not need migration.
//
// VALIDATES: "api foo { content { format parsed; } receive { update; } }" does not need migration.
//
// PREVENTS: Already-migrated configs being flagged for migration.
func TestDetectNamedAPIWithNewSyntax(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    process foo {
        content { format parsed; }
        receive { update; }
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "named api with new syntax does not need migration")
}

// parseWithBGPSchema is a helper that parses config using LegacyBGPSchema.
// Migration tests need to parse old syntax, so we use the legacy schema.
func parseWithBGPSchema(t *testing.T, input string) *config.Tree {
	t.Helper()
	p := config.NewParser(config.LegacyBGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	return tree
}
