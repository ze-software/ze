package migration

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestDetectV2NeighborAtRoot verifies v2 detection for neighbor at root.
//
// VALIDATES: Config with "neighbor <IP>" is detected as v2.
//
// PREVENTS: v2 configs being treated as current.
func TestDetectV2NeighborAtRoot(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "neighbor at root should be v2")
}

// TestDetectV2PeerGlobAtRoot verifies v2 detection for peer glob at root.
//
// VALIDATES: Config with "peer *" glob at root is detected as v2.
//
// PREVENTS: Root-level peer globs being treated as v3.
func TestDetectV2PeerGlobAtRoot(t *testing.T) {
	input := `
peer * {
    hold-time 90;
}
neighbor 192.0.2.1 {
    local-as 65000;
}
`
	tree := parseWithBGPSchema(t, input)
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "peer glob at root should be v2")
}

// TestDetectV2TemplateNeighbor verifies v2 detection for template.neighbor.
//
// VALIDATES: Config with template { neighbor <name> } is detected as v2.
//
// PREVENTS: Old template syntax being treated as v3.
func TestDetectV2TemplateNeighbor(t *testing.T) {
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "template.neighbor should be v2")
}

// TestDetectV3PeerAtRoot verifies v3 detection for peer IP at root.
//
// VALIDATES: Config with "peer <IP>" (not glob) is detected as v3.
//
// PREVENTS: New syntax being mistaken for old.
func TestDetectV3PeerAtRoot(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)
	version := DetectVersion(tree)
	require.Equal(t, Version3, version, "peer IP at root should be v3")
}

// TestDetectV3TemplateGroup verifies v3 detection for template.group.
//
// VALIDATES: Config with template { group <name> } is detected as v3.
//
// PREVENTS: New template syntax being treated as old.
func TestDetectV3TemplateGroup(t *testing.T) {
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
	version := DetectVersion(tree)
	require.Equal(t, Version3, version, "template.group should be v3")
}

// TestDetectV3TemplateMatch verifies v3 detection for template.match.
//
// VALIDATES: Config with template { match * } is detected as v3.
//
// PREVENTS: Match blocks being treated as v2.
func TestDetectV3TemplateMatch(t *testing.T) {
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
	version := DetectVersion(tree)
	require.Equal(t, Version3, version, "template.match should be v3")
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "mixed v2/v3 should be detected as v2 (needs migration)")
}

// TestDetectEmptyConfigReturnsV3 verifies empty config is treated as current.
//
// VALIDATES: Empty config returns VersionCurrent (v3).
//
// PREVENTS: Empty configs causing errors.
func TestDetectEmptyConfigReturnsV3(t *testing.T) {
	tree := config.NewTree()
	version := DetectVersion(tree)
	require.Equal(t, Version3, version, "empty config should be v3 (current)")
}

// TestDetectNilTreeReturnsUnknown verifies nil tree handling.
//
// VALIDATES: Nil tree returns VersionUnknown without panic.
//
// PREVENTS: Nil pointer dereference.
func TestDetectNilTreeReturnsUnknown(t *testing.T) {
	version := DetectVersion(nil)
	require.Equal(t, VersionUnknown, version, "nil tree should be unknown")
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "CIDR pattern at root should be v2")
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "IPv6 glob pattern at root should be v2")
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "peer with static block should be v2")
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
	version := DetectVersion(tree)
	require.Equal(t, Version2, version, "template.group with static should be v2")
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
