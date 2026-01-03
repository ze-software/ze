package migration

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestDetectLegacyNeighborAtRoot verifies v2 detection for neighbor at root.
//
// VALIDATES: Config with "neighbor <IP>" is detected as v2.
//
// PREVENTS: v2 configs being treated as current.
func TestDetectLegacyNeighborAtRoot(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "neighbor at root should be v2")
}

// TestDetectLegacyPeerGlobAtRoot verifies v2 detection for peer glob at root.
//
// VALIDATES: Config with "peer *" glob at root is detected as v2.
//
// PREVENTS: Root-level peer globs being treated as v3.
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
	require.True(t, needsMigration, "peer glob at root should be v2")
}

// TestDetectLegacyTemplateNeighbor verifies v2 detection for template.neighbor.
//
// VALIDATES: Config with template { neighbor <name> } is detected as v2.
//
// PREVENTS: Old template syntax being treated as v3.
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
	require.True(t, needsMigration, "template.neighbor should be v2")
}

// TestDetectCurrentPeerAtRoot verifies v3 detection for peer IP at root.
//
// VALIDATES: Config with "peer <IP>" (not glob) is detected as v3.
//
// PREVENTS: New syntax being mistaken for old.
func TestDetectCurrentPeerAtRoot(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "peer IP at root should be v3")
}

// TestDetectCurrentTemplateGroup verifies v3 detection for template.group.
//
// VALIDATES: Config with template { group <name> } is detected as v3.
//
// PREVENTS: New template syntax being treated as old.
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
	require.False(t, needsMigration, "template.group should be v3")
}

// TestDetectCurrentTemplateMatch verifies v3 detection for template.match.
//
// VALIDATES: Config with template { match * } is detected as v3.
//
// PREVENTS: Match blocks being treated as v2.
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
	require.False(t, needsMigration, "template.match should be v3")
}

// TestDetectMixedV2V3ReturnsV2 verifies mixed configs are detected as v2.
//
// VALIDATES: Config with both v3 (template.match) AND v2 (neighbor) is v2.
//
// PREVENTS: Partially migrated configs being treated as complete.
func TestDetectMixedV2V3ReturnsV2(t *testing.T) {
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
	require.True(t, needsMigration, "mixed v2/v3 should be detected as v2 (needs migration)")
}

// TestDetectEmptyConfigReturnsV3 verifies empty config is treated as current.
//
// VALIDATES: Empty config returns false (no migration needed).
//
// PREVENTS: Empty configs causing errors.
func TestDetectEmptyConfigReturnsV3(t *testing.T) {
	tree := config.NewTree()
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "empty config should be v3 (current)")
}

// TestNeedsMigrationNilTree verifies nil tree handling.
//
// VALIDATES: Nil tree returns false without panic.
//
// PREVENTS: Nil pointer dereference.
func TestNeedsMigrationNilTree(t *testing.T) {
	require.False(t, NeedsMigration(nil), "nil tree should not need migration")
}

// TestDetectCIDRPatternAtRootIsV2 verifies CIDR patterns at root are v2.
//
// VALIDATES: "peer 10.0.0.0/8 { }" at root is v2 (needs migration to template.match).
//
// PREVENTS: CIDR patterns at root being treated as v3.
func TestDetectCIDRPatternAtRootIsV2(t *testing.T) {
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
	require.True(t, needsMigration, "CIDR pattern at root should be v2")
}

// TestDetectIPv6GlobPatternAtRootIsV2 verifies IPv6 glob patterns at root are v2.
//
// VALIDATES: "peer 2001:db8::* { }" at root is v2 (needs migration to template.match).
//
// PREVENTS: IPv6 glob patterns at root being treated as v3.
func TestDetectIPv6GlobPatternAtRootIsV2(t *testing.T) {
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
	require.True(t, needsMigration, "IPv6 glob pattern at root should be v2")
}

// TestDetectPeerWithStaticIsV2 verifies peer with static block is v2.
//
// VALIDATES: "peer { static { } }" is v2 (needs migration to announce).
//
// PREVENTS: v3-style peer with deprecated static block being skipped.
func TestDetectPeerWithStaticIsV2(t *testing.T) {
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
	require.True(t, needsMigration, "peer with static block should be v2")
}

// TestDetectTemplateGroupWithStaticIsV2 verifies template.group with static is v2.
//
// VALIDATES: "template { group { static { } } }" is v2.
//
// PREVENTS: Template static blocks being skipped.
func TestDetectTemplateGroupWithStaticIsV2(t *testing.T) {
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
	require.True(t, needsMigration, "template.group with static should be v2")
}

// TestDetectPeerWithAnonymousAPIIsV2 verifies peer with old-style anonymous api is v2.
//
// VALIDATES: "peer { api { processes [...] } }" is v2 (needs migration to named api).
//
// PREVENTS: Old API syntax being skipped during migration.
func TestDetectPeerWithAnonymousAPIIsV2(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api {
        processes [ foo ];
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "peer with anonymous api block should be v2")
}

// TestDetectTemplateGroupWithAnonymousAPIIsV2 verifies template.group with old api is v2.
//
// VALIDATES: "template { group { api { processes [...] } } }" is v2.
//
// PREVENTS: Template anonymous API blocks being skipped.
func TestDetectTemplateGroupWithAnonymousAPIIsV2(t *testing.T) {
	input := `
process collector {
    run ./collector.run;
}
template {
    group collectors {
        api {
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
	require.True(t, needsMigration, "template.group with anonymous api should be v2")
}

// TestDetectPeerWithNamedAPIIsV3 verifies peer with new-style named api is v3.
//
// VALIDATES: "peer { api foo { } }" is v3 (already migrated).
//
// PREVENTS: Already-migrated configs being flagged for migration.
func TestDetectPeerWithNamedAPIIsV3(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api foo {
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "peer with named api block should be v3")
}

// TestDetectNamedAPIWithProcessesIsV2 verifies named api with processes field is v2.
//
// VALIDATES: "api speaking { processes [...] }" is v2 (needs migration).
//
// PREVENTS: Named blocks with old syntax being skipped.
func TestDetectNamedAPIWithProcessesIsV2(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api speaking {
        processes [ foo ];
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "named api with processes field should be v2")
}

// TestDetectNamedAPIWithFormatFlagsIsV2 verifies named api with format flags is v2.
//
// VALIDATES: "api foo { receive { parsed; } }" is v2 (needs migration).
//
// PREVENTS: Format flags in named blocks being skipped.
func TestDetectNamedAPIWithFormatFlagsIsV2(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api foo {
        receive {
            parsed;
            update;
        }
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.True(t, needsMigration, "named api with format flags should be v2")
}

// TestDetectNamedAPIWithNewSyntaxIsV3 verifies named api with new syntax is v3.
//
// VALIDATES: "api foo { content { format parsed; } receive { update; } }" is v3.
//
// PREVENTS: Already-migrated configs being flagged for migration.
func TestDetectNamedAPIWithNewSyntaxIsV3(t *testing.T) {
	input := `
process foo {
    run ./foo.run;
}
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api foo {
        content { format parsed; }
        receive { update; }
    }
}
`
	tree := parseWithBGPSchema(t, input)
	needsMigration := NeedsMigration(tree)
	require.False(t, needsMigration, "named api with new syntax should be v3")
}

// parseWithBGPSchema is a helper that parses config using LegacyBGPSchema.
// Migration tests need to parse v2 syntax, so we use the legacy schema.
func parseWithBGPSchema(t *testing.T, input string) *config.Tree {
	t.Helper()
	p := config.NewParser(config.LegacyBGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	return tree
}
