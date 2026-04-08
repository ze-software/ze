package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// findPeerByRemoteIP finds a peer by its remote > ip field in any group of the result tree.
// Returns the peer tree or nil if not found.
func findPeerByRemoteIP(t *testing.T, tree *config.Tree, addr string) *config.Tree {
	t.Helper()
	for _, groupEntry := range tree.GetListOrdered("group") {
		for _, peerEntry := range groupEntry.Value.GetListOrdered("peer") {
			if conn := peerEntry.Value.GetContainer("connection"); conn != nil {
				if remote := conn.GetContainer("remote"); remote != nil {
					if ip, ok := remote.Get("ip"); ok && ip == addr {
						return peerEntry.Value
					}
				}
			}
		}
	}
	return nil
}

// TestMigrateExtendedMessageDefault verifies extended-message is always added.
//
// VALIDATES: Migration always adds extended-message enable to capability block.
// PREVENTS: ExaBGP OPEN mismatch — ExaBGP always includes extended-message (code 6).
func TestMigrateExtendedMessageDefault(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Extended message should always be present in migrated configs.
	assert.Contains(t, output, "extended-message enable")
}

// TestMigrateHostnameToCapability verifies host-name/domain-name conversion.
//
// VALIDATES: ExaBGP host-name/domain-name at peer level converted to capability hostname block.
// PREVENTS: Hostname capability dropped during migration (test C failure).
func TestMigrateHostnameToCapability(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	host-name my-host
	domain-name example.com
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Hostname should be inside capability block, not at peer level.
	assert.Contains(t, output, "hostname {")
	assert.Contains(t, output, "host my-host")
	assert.Contains(t, output, "domain example.com")

	// Legacy fields should NOT appear at peer level.
	assert.NotContains(t, output, "host-name")
	assert.NotContains(t, output, "domain-name")
}

// TestMigrateLinkLocalNexthop verifies link-local-nexthop capability migration.
//
// VALIDATES: ExaBGP capability { link-local-nexthop; } preserved in migration.
// PREVENTS: Link-local-nexthop capability dropped during migration (test 3 failure).
func TestMigrateLinkLocalNexthop(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		link-local-nexthop
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// link-local-nexthop should be preserved now that the llnh plugin exists.
	assert.Contains(t, output, "link-local-nexthop enable")
	// extended-message should still be present.
	assert.Contains(t, output, "extended-message enable")
}

// TestMigrateASN4Disable verifies asn4 disable is preserved, not converted to enable.
//
// VALIDATES: ExaBGP "asn4 disable" stays "asn4 disable" after migration.
// PREVENTS: asn4 disable silently becoming asn4 enable (test Q failure).
func TestMigrateASN4Disable(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 65533
	peer-as 65533
	capability {
		asn4 disable
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	assert.Contains(t, output, "asn4 disable")
	assert.NotContains(t, output, "asn4 enable")
}

// TestMigrateSplitRoute verifies split directive is preserved during migration.
//
// VALIDATES: ExaBGP "route ... split /24" generates split field in update.
// PREVENTS: split directive dropped during migration (test U failure).
func TestMigrateSplitRoute(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 65533
	peer-as 65533
	static {
		route 172.10.0.0/22 next-hop 192.0.2.1 split /24
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	assert.Contains(t, output, "split /24")
}

// TestMigrateLinkLocal verifies local-link-local field is renamed to link-local during migration.
//
// VALIDATES: ExaBGP "local-link-local fe80::1" migrates to Ze "link-local fe80::1".
// PREVENTS: link-local address dropped during migration (test L failure).
func TestMigrateLinkLocal(t *testing.T) {
	input := `
neighbor ::1 {
	local-as 65533
	peer-as 65533
	local-link-local fe80::1
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	assert.Contains(t, output, "link-local fe80::1")
	assert.NotContains(t, output, "local-link-local")
}

// TestMigrateSimple verifies basic neighbor→peer conversion.
//
// VALIDATES: Simple ExaBGP config converts to ZeBGP peer syntax.
// PREVENTS: Basic migration regression.
func TestMigrateSimple(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1
	local-as 65001
	peer-as 65002
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Check peer exists inside a group
	peerTree := findPeerByRemoteIP(t, result.Tree, "10.0.0.1")
	assert.NotNil(t, peerTree, "expected peer 10.0.0.1 in a group")

	// Check neighbor removed
	neighbors := result.Tree.GetList("neighbor")
	assert.Empty(t, neighbors, "neighbors should be empty")
}

// TestMigrateWithGR verifies graceful-restart injects RIB plugin.
//
// VALIDATES: GR config injects RIB plugin and process binding.
// PREVENTS: Missing RIB for GR state storage (ZeBGP delegates RIB to plugins).
func TestMigrateWithGR(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1
	local-as 65001
	peer-as 65002
	capability {
		graceful-restart 120
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Check RIB plugin injected
	plugins := result.Tree.GetList("plugin")
	_, ok := plugins["bgp-rib"]
	assert.True(t, ok, "expected plugin bgp-rib to be injected for GR")

	// Check peer has RIB process binding (peer is inside a group)
	peerTree := findPeerByRemoteIP(t, result.Tree, "10.0.0.1")
	require.NotNil(t, peerTree, "expected peer 10.0.0.1 in a group")

	processes := peerTree.GetList("process")
	_, ok = processes["bgp-rib"]
	assert.True(t, ok, "expected process bgp-rib binding in peer")

	// Check RIB injected should be in result.RIBInjected
	assert.True(t, result.RIBInjected, "expected RIBInjected=true")
}

// TestMigrateWithGRBare verifies bare graceful-restart; converts to enable.
//
// VALIDATES: Bare graceful-restart; becomes graceful-restart enable; (not "true").
// PREVENTS: Parser "true" placeholder leaking to output.
func TestMigrateWithGRBare(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		graceful-restart
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Must contain "graceful-restart enable" not "graceful-restart true"
	assert.Contains(t, output, "graceful-restart enable")
	assert.NotContains(t, output, "graceful-restart true")
}

// TestMigrateWithRR verifies route-refresh injects RIB plugin.
//
// VALIDATES: Route-refresh config injects RIB plugin with refresh capability.
// PREVENTS: Missing RIB for refresh response (ZeBGP delegates RIB to plugins).
func TestMigrateWithRR(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1
	local-as 65001
	peer-as 65002
	capability {
		route-refresh
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Check RIB plugin injected
	plugins := result.Tree.GetList("plugin")
	_, ok := plugins["bgp-rib"]
	assert.True(t, ok, "expected plugin bgp-rib to be injected for route-refresh")

	// Check process binding includes refresh (peer is inside a group)
	peerTree := findPeerByRemoteIP(t, result.Tree, "10.0.0.1")
	require.NotNil(t, peerTree, "expected peer 10.0.0.1 in a group")
	processes := peerTree.GetList("process")
	ribProcess := processes["bgp-rib"]
	require.NotNil(t, ribProcess, "expected process bgp-rib binding")

	// Verify send leaf-list includes refresh
	sendValue, ok := ribProcess.Get("send")
	require.True(t, ok, "expected send leaf-list in rib process")
	assert.Contains(t, sendValue, "refresh")

	// Check capability uses enable syntax
	sessionBlock := peerTree.GetContainer("session")
	require.NotNil(t, sessionBlock, "expected session block")
	capBlock := sessionBlock.GetContainer("capability")
	require.NotNil(t, capBlock, "expected session > capability block")
	rrValue, ok := capBlock.Get("route-refresh")
	require.True(t, ok, "expected route-refresh key")
	assert.Equal(t, "enable", rrValue, "expected route-refresh enable")
}

// TestMigrateProcess verifies processes are collected for wrapper handling.
//
// VALIDATES: ExaBGP processes stored in MigrateResult.Processes (not as Ze plugins).
// PREVENTS: Creating bridge plugins with incompatible protocol.
func TestMigrateProcess(t *testing.T) {
	input := `
process my-plugin {
	run /path/to/plugin.py
	encoder json
}

neighbor 10.0.0.1 {
	router-id 1.1.1.1
	local-as 65001
	peer-as 65002
	api {
		processes [ my-plugin ]
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Processes should NOT be converted to plugins (protocol incompatible).
	plugins := result.Tree.GetList("plugin")
	for name := range plugins {
		assert.NotContains(t, name, "compat", "should not create bridge plugin")
	}

	// Processes should be stored in result for wrapper to handle.
	require.Len(t, result.Processes, 1, "expected 1 external process")
	assert.Equal(t, "my-plugin", result.Processes[0].Name)
	assert.Equal(t, "/path/to/plugin.py", result.Processes[0].RunCmd)
}

// TestMigrateL2VPNSupported verifies L2VPN/VPLS configs are now migrated successfully.
//
// VALIDATES: L2VPN VPLS routes are migrated (no longer unsupported).
// PREVENTS: Regression of L2VPN migration (test I).
func TestMigrateL2VPNSupported(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	l2vpn {
		vpls foo {
			endpoint 1
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate should succeed for L2VPN")

	output := SerializeTree(result.Tree)
	assert.Contains(t, output, "update {", "expected update block for L2VPN route")
}

// TestMigrateNil verifies nil input handling.
//
// VALIDATES: Nil input returns ErrNilTree.
// PREVENTS: Panic on nil tree.
func TestMigrateNil(t *testing.T) {
	result, err := MigrateFromExaBGP(nil)
	assert.Error(t, err, "expected error for nil tree")
	assert.Nil(t, result, "expected nil result for nil tree")
}

// TestMigrateFamilyConversion verifies family syntax conversion.
//
// VALIDATES: ExaBGP "ipv4 unicast" converts to ZeBGP "ipv4/unicast".
// PREVENTS: Wrong family syntax in migrated config.
func TestMigrateFamilyConversion(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	family {
		ipv4 unicast
		ipv6 unicast
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Serialize and check family syntax.
	output := SerializeTree(result.Tree)

	// Must have ZeBGP format (slash), not ExaBGP (space).
	assert.Contains(t, output, "ipv4/unicast")
	assert.Contains(t, output, "ipv6/unicast")
	assert.NotContains(t, output, "ipv4 unicast")
	assert.NotContains(t, output, "ipv6 unicast")
}

// TestMigrateTemplate verifies template inheritance expansion.
//
// VALIDATES: Template properties merged into neighbor via inherit.
// PREVENTS: Templates output separately (they should be expanded inline).
func TestMigrateTemplate(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65001
		hold-time 180
		family {
			ipv4 unicast
		}
		capability {
			route-refresh
		}
	}
}
neighbor 10.0.0.1 {
	inherit base
	peer-as 65002
	router-id 1.2.3.4
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Peer should have inherited local-as inside session > asn container.
	assert.Contains(t, output, "session {", "expected session container")
	assert.Contains(t, output, "asn {", "expected asn container")
	assert.Contains(t, output, "local 65001", "expected inherited local-as in session > asn")

	// Peer should have its own peer-as inside session > asn container.
	assert.Contains(t, output, "remote 65002", "expected peer-as in session > asn")

	// Template should NOT appear in output (expanded inline).
	assert.NotContains(t, output, "template", "template block should be expanded")
	assert.NotContains(t, output, "peer base", "template name should not appear")

	// Family should be inherited and converted.
	assert.Contains(t, output, "ipv4/unicast", "expected inherited family")

	// Capability should be inherited with enable.
	assert.Contains(t, output, "route-refresh enable", "expected inherited capability")
}

// TestMigrateStaticBlock verifies static block conversion to update blocks.
//
// VALIDATES: Static routes converted to update { attribute {} nlri {} }.
// PREVENTS: Static block dropped or output as-is (Ze rejects static).
func TestMigrateStaticBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	static {
		route 192.168.0.0/24 next-hop 10.0.0.1
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Static block should be converted to update block.
	assert.NotContains(t, output, "static {", "static block should be converted")
	assert.Contains(t, output, "update {", "expected update block")

	// Route should appear in nlri block.
	assert.Contains(t, output, "ipv4/unicast add 192.168.0.0/24", "expected nlri entry")

	// Next-hop should appear in attribute block.
	assert.Contains(t, output, "next-hop 10.0.0.1", "expected next-hop in attribute")
}

// TestMigrateStaticPathInformation verifies path-information is preserved.
//
// VALIDATES: path-information from static routes is migrated to attribute block.
// PREVENTS: ADD-PATH path-id being lost during migration.
func TestMigrateStaticPathInformation(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 1
	peer-as 1
	static {
		route 193.0.2.1 path-information 1.2.3.4 next-hop 10.0.0.1
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	// Debug: check what's parsed
	for _, nb := range tree.GetListOrdered("neighbor") {
		if static := nb.Value.GetContainer("static"); static != nil {
			for _, route := range static.GetListOrdered("route") {
				t.Logf("Route %s values: %v", route.Key, route.Value.Values())
				if pi, ok := route.Value.Get("path-information"); ok {
					t.Logf("  path-information = %s", pi)
				} else {
					t.Logf("  path-information NOT FOUND")
				}
			}
		}
	}

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)
	t.Logf("Migration output:\n%s", output)

	// path-information should appear in attribute block.
	assert.Contains(t, output, "path-information 1.2.3.4", "expected path-information in attribute block")
}

// TestMigrateAnnounceBlock verifies announce block conversion to update blocks.
//
// VALIDATES: Announce routes converted to update { attribute {} nlri {} }.
// PREVENTS: Announce block dropped or output as-is (Ze rejects announce).
func TestMigrateAnnounceBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	announce {
		ipv4 {
			unicast 10.0.0.0/24 next-hop 192.168.1.1 local-preference 100
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Announce block should be converted to update block.
	assert.NotContains(t, output, "announce {", "announce block should be converted")
	assert.Contains(t, output, "update {", "expected update block")

	// Route should appear in nlri block.
	assert.Contains(t, output, "ipv4/unicast add 10.0.0.0/24", "expected nlri entry")

	// Attributes should appear in attribute block.
	assert.Contains(t, output, "next-hop 192.168.1.1", "expected next-hop in attribute")
	assert.Contains(t, output, "local-preference 100", "expected local-preference in attribute")
}

// TestNeedsRIBPlugin verifies RIB requirement detection.
//
// VALIDATES: Detection of features requiring RIB plugin.
// PREVENTS: Missing RIB injection for GR/RR.
func TestNeedsRIBPlugin(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantRIB bool
	}{
		{
			name: "simple_no_rib",
			input: `neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
}`,
			wantRIB: false,
		},
		{
			name: "graceful_restart",
			input: `neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		graceful-restart
	}
}`,
			wantRIB: true,
		},
		{
			name: "route_refresh",
			input: `neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		route-refresh
	}
}`,
			wantRIB: true,
		},
		{
			name: "api_receive_update",
			input: `process foo {
	run /path
}
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	api {
		processes [ foo ]
		receive {
			update
		}
	}
}`,
			wantRIB: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := ParseExaBGPConfig(tt.input)
			require.NoError(t, err, "parse")

			got := NeedsRIBPlugin(tree)
			assert.Equal(t, tt.wantRIB, got, "NeedsRIBPlugin()")
		})
	}
}

// TestMigrateNexthopCapability verifies nexthop capability inference from block.
//
// VALIDATES: ExaBGP "nexthop { family afi; }" infers capability and copies block.
// PREVENTS: Missing nexthop capability in migrated config.
func TestMigrateNexthopCapability(t *testing.T) {
	// ExaBGP syntax: nexthop block maps families to next-hop AFI.
	// Presence of nexthop block implies Extended Next Hop capability.
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	nexthop {
		ipv4 unicast ipv6
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability with family syntax conversion.
	assert.Contains(t, output, "capability {", "expected capability block")
	assert.Contains(t, output, "nexthop {", "expected nexthop block")
	assert.Contains(t, output, "ipv4/unicast ipv6", "expected converted family syntax")
}

// TestMigrateNexthopExplicitAndBlock verifies both explicit capability and block together.
//
// VALIDATES: Explicit "capability { nexthop; }" + "nexthop { }" block works without duplication.
// PREVENTS: Duplicate capability entries when both syntaxes used.
func TestMigrateNexthopExplicitAndBlock(t *testing.T) {
	// ExaBGP allows both explicit capability AND nexthop block.
	// The capability should only appear once in output.
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		nexthop
	}
	nexthop {
		ipv4 unicast ipv6
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Nexthop block should appear exactly once (inside capability).
	assert.Equal(t, 1, strings.Count(output, "nexthop {"), "expected exactly 1 nexthop block")

	// Nexthop block content should be present.
	assert.Contains(t, output, "ipv4/unicast ipv6", "expected nexthop block content")
}

// TestMigrateNexthopBlock verifies nexthop block migration with multiple entries.
//
// VALIDATES: ExaBGP "ipv4 unicast ipv6" converts to ZeBGP "ipv4/unicast ipv6".
// PREVENTS: Migration failure for RFC 8950 nexthop AFI/SAFI configuration.
func TestMigrateNexthopBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	nexthop {
		ipv4 unicast ipv6
		ipv4 mpls-vpn ipv6
		ipv6 unicast ipv4
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability.
	assert.Contains(t, output, "capability {", "expected capability block")
	assert.Contains(t, output, "nexthop {", "expected nexthop block")

	// Check nexthop block syntax conversion.
	assert.Contains(t, output, "ipv4/unicast ipv6")
	assert.Contains(t, output, "ipv4/mpls-vpn ipv6")
	assert.Contains(t, output, "ipv6/unicast ipv4")

	// Should NOT have space-separated format.
	assert.NotContains(t, output, "ipv4 unicast ipv6", "should not contain ExaBGP format")
}

// TestMigrateNexthopExplicitCapabilityIgnored verifies explicit capability is ignored.
//
// VALIDATES: Explicit "capability { nexthop; }" is NOT migrated (useless without nexthop block).
// PREVENTS: Generating useless config that ZeBGP ignores anyway.
//
// Note: ZeBGP infers Extended Next Hop capability from nexthop { } block.
// An explicit capability declaration without a nexthop block has no effect.
func TestMigrateNexthopExplicitCapabilityIgnored(t *testing.T) {
	// Explicit capability, no nexthop block.
	// This is useless in ZeBGP - capability is inferred from nexthop block.
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		nexthop
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Explicit nexthop capability is NOT migrated - it's useless without nexthop block.
	assert.NotContains(t, output, "nexthop enable", "useless without nexthop block")

	// Should NOT have nexthop block (none in input).
	assert.NotContains(t, output, "nexthop {", "no nexthop block in input")

	// Should still have peer block with remote IP.
	assert.Contains(t, output, "peer peer-1", "expected peer block")
	assert.Contains(t, output, "ip 10.0.0.1", "expected remote ip")
}

// TestMigrateNexthopBothCapabilityAndBlock verifies behavior when both are present.
//
// VALIDATES: Nexthop block is copied, capability is inferred from block.
// PREVENTS: Duplicate or conflicting nexthop capability handling.
func TestMigrateNexthopBothCapabilityAndBlock(t *testing.T) {
	// Both explicit capability AND nexthop block.
	// Capability inferred from block (explicit one is redundant).
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		nexthop
	}
	nexthop {
		ipv4 unicast ipv6
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability (only once).
	assert.Equal(t, 1, strings.Count(output, "nexthop {"), "expected exactly 1 nexthop block")

	// Nexthop block content should be present.
	assert.Contains(t, output, "ipv4/unicast ipv6", "expected nexthop block content")
}

// TestMigrateNexthopBlockSAFINormalization verifies SAFI name normalization.
//
// VALIDATES: ExaBGP "nlri-mpls" and "labeled-unicast" convert to ZeBGP "mpls-label".
// PREVENTS: Migrated nexthop config not recognized by ZeBGP's parseNexthopFamilies.
func TestMigrateNexthopBlockSAFINormalization(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	nexthop {
		ipv4 nlri-mpls ipv6
		ipv4 labeled-unicast ipv6
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Both should be normalized to mpls-label.
	assert.Contains(t, output, "ipv4/mpls-label ipv6", "expected normalized SAFI")

	// Should NOT have ExaBGP SAFI names.
	assert.NotContains(t, output, "nlri-mpls", "ExaBGP name should be normalized")
	assert.NotContains(t, output, "labeled-unicast", "ExaBGP name should be normalized")
}

// TestMigrateTemplateWithNexthop verifies nexthop inheritance from templates.
//
// VALIDATES: Template nexthop blocks are inherited and converted correctly.
// PREVENTS: Nexthop capability lost during inheritance.
func TestMigrateTemplateWithNexthop(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65001
		nexthop {
			ipv4 unicast ipv6
		}
	}
}
neighbor 10.0.0.1 {
	inherit base
	peer-as 65002
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)

	// Template should NOT appear (expanded inline).
	assert.NotContains(t, output, "peer base", "template should be expanded")

	// Inherited local-as should be present inside session > asn container.
	assert.Contains(t, output, "session {", "expected session container")
	assert.Contains(t, output, "local 65001", "expected inherited local-as in session > asn")

	// Nexthop block should be inside session > capability.
	assert.Contains(t, output, "capability {", "expected capability block")
	assert.Contains(t, output, "nexthop {", "expected nexthop block")

	// Nexthop block should be converted.
	assert.Contains(t, output, "ipv4/unicast ipv6", "expected converted family syntax")
}

// TestMigrateFileBasedTests runs file-based migration tests.
// Each test directory in test/exabgp/ contains:
//   - input.conf: ExaBGP config to migrate
//   - expected.conf: Expected ZeBGP output
//
// VALIDATES: File-based migration produces exact expected output.
// PREVENTS: Regression in migration output format.
func TestMigrateFileBasedTests(t *testing.T) {
	// Find test/exabgp directory.
	testDataDir := findTestDataDir(t)
	if testDataDir == "" {
		t.Skip("test/exabgp directory not found")
	}

	tests := []string{"simple", "graceful-restart", "route-refresh", "process", "nexthop"}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			inputPath := filepath.Join(testDataDir, name, "input.conf")
			expectedPath := filepath.Join(testDataDir, name, "expected.conf")

			// Read input.
			inputData, err := os.ReadFile(inputPath) //nolint:gosec // Test data path.
			if err != nil {
				t.Skipf("input file not found: %v", err)
			}

			// Read expected output.
			expectedData, err := os.ReadFile(expectedPath) //nolint:gosec // Test data path.
			if err != nil {
				t.Skipf("expected file not found: %v", err)
			}

			// Parse input with ExaBGP schema.
			tree, err := ParseExaBGPConfig(string(inputData))
			require.NoError(t, err, "parse input")

			// Migrate.
			result, err := MigrateFromExaBGP(tree)
			require.NoError(t, err, "migrate")

			// Serialize result.
			gotOutput := SerializeTree(result.Tree)

			// Exact comparison against expected.conf.
			want := strings.TrimSpace(string(expectedData))
			got := strings.TrimSpace(gotOutput)

			assert.Equal(t, want, got, "migration output mismatch")

			// Also run structural validation for extra coverage.
			validateMigrationResult(t, name, gotOutput, result)
		})
	}
}

// findTestDataDir finds the test/exabgp directory.
func findTestDataDir(t *testing.T) string {
	t.Helper()

	// Try relative paths from common locations
	paths := []string{
		"test/exabgp",
		"../../test/exabgp",
		"../../../test/exabgp",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try from module root
	wd, _ := os.Getwd()
	for range 5 {
		testPath := filepath.Join(wd, "test", "exabgp")
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
		wd = filepath.Dir(wd)
	}

	return ""
}

// validateMigrationResult validates key transformations happened.
func validateMigrationResult(t *testing.T, testName, got string, result *MigrateResult) {
	t.Helper()

	switch testName {
	case "simple":
		// Should have peer with name, remote IP, and local/remote containers.
		assert.Contains(t, got, "peer peer-1")
		assert.Contains(t, got, "ip 10.0.0.1")

	case "graceful-restart":
		// Should have RIB plugin injected.
		assert.True(t, result.RIBInjected, "expected RIBInjected=true for graceful-restart")
		assert.Contains(t, got, "external bgp-rib")

	case "route-refresh":
		// Should have RIB plugin injected.
		assert.True(t, result.RIBInjected, "expected RIBInjected=true for route-refresh")
		// Should have route-refresh enable.
		assert.Contains(t, got, "route-refresh enable")

	case "process":
		// ExaBGP processes are stored in result.Processes for the wrapper to handle
		// (protocol incompatible — ExaBGP uses stdout text, Ze uses YANG RPC sockets).
		assert.Len(t, result.Processes, 1, "expected 1 external process")
		// No bridge plugins should be created in the migrated config.
		assert.NotContains(t, got, "-compat", "should not create bridge plugin in config")

	case "nexthop":
		// Should have nexthop block inside capability.
		assert.Contains(t, got, "capability {")
		assert.Contains(t, got, "nexthop {")
		// Should NOT have RIB injected (nexthop doesn't require state storage).
		assert.False(t, result.RIBInjected, "nexthop should not trigger RIB injection")
		// Should have nexthop block with converted syntax.
		assert.Contains(t, got, "ipv4/unicast ipv6")
		assert.NotContains(t, got, "ipv4 unicast ipv6", "should not contain ExaBGP format")
	}

	// Log output for debugging.
	t.Logf("Migration output:\n%s", got)
}

// TestTokenizeFlexValue verifies tokenization of flex value strings.
//
// VALIDATES: Brackets and parens are grouped as atomic tokens.
// PREVENTS: Splitting compound values (extended-community, bgp-prefix-sid-srv6) into parts.
func TestTokenizeFlexValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple_words",
			input: "shared-join rp 10.99.199.1 group 239.251.255.228",
			want:  []string{"shared-join", "rp", "10.99.199.1", "group", "239.251.255.228"},
		},
		{
			name:  "single_bracket_value",
			input: "extended-community [target:10:10]",
			want:  []string{"extended-community", "[target:10:10]"},
		},
		{
			name:  "multi_bracket_value",
			input: "extended-community [target:10:10 mup:10:10]",
			want:  []string{"extended-community", "[target:10:10 mup:10:10]"},
		},
		{
			name:  "paren_with_nested_bracket",
			input: "bgp-prefix-sid-srv6 (l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])",
			want:  []string{"bgp-prefix-sid-srv6", "(l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])"},
		},
		{
			name:  "full_mup_line",
			input: "mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 extended-community [target:10:10] bgp-prefix-sid-srv6 (l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])",
			want: []string{
				"mup-isd", "10.0.1.0/24", "rd", "100:100",
				"next-hop", "2001::1",
				"extended-community", "[target:10:10]",
				"bgp-prefix-sid-srv6", "(l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])",
			},
		},
		{
			name:  "full_mvpn_line",
			input: "shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [target:192.168.94.12:5]",
			want: []string{
				"shared-join", "rp", "10.99.199.1", "group", "239.251.255.228",
				"rd", "65000:99999", "source-as", "65000",
				"next-hop", "10.10.6.3",
				"extended-community", "[target:192.168.94.12:5]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizeFlexValue(tt.input)
			require.Len(t, got, len(tt.want), "tokenizeFlexValue(%q) token count", tt.input)
			for i := range got {
				assert.Equal(t, tt.want[i], got[i], "token[%d]", i)
			}
		})
	}
}

// TestSplitFlexAttrs verifies separation of path attributes from NLRI fields.
//
// VALIDATES: Known attribute keywords extracted with values; remaining tokens form NLRI.
// PREVENTS: Attribute keywords leaking into NLRI line or NLRI fields going into attributes.
func TestSplitFlexAttrs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAttrs map[string]string
		wantNLRI  []string
	}{
		{
			name:  "mvpn_shared_join",
			input: "shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [target:192.168.94.12:5]",
			wantAttrs: map[string]string{
				"next-hop":           "10.10.6.3",
				"extended-community": "target:192.168.94.12:5",
			},
			wantNLRI: []string{"shared-join", "rp", "10.99.199.1", "group", "239.251.255.228", "rd", "65000:99999", "source-as", "65000"},
		},
		{
			name:  "mup_isd_with_srv6",
			input: "mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 extended-community [target:10:10] bgp-prefix-sid-srv6 (l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])",
			wantAttrs: map[string]string{
				"next-hop":            "2001::1",
				"extended-community":  "target:10:10",
				"bgp-prefix-sid-srv6": "l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0]",
			},
			wantNLRI: []string{"mup-isd", "10.0.1.0/24", "rd", "100:100"},
		},
		{
			name:  "mup_t1st_with_source",
			input: "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 source 10.0.1.1 next-hop 10.0.0.2 extended-community [target:10:10]",
			wantAttrs: map[string]string{
				"next-hop":           "10.0.0.2",
				"extended-community": "target:10:10",
			},
			wantNLRI: []string{"mup-t1st", "192.168.0.2/32", "rd", "100:100", "teid", "12345", "qfi", "9", "endpoint", "10.0.0.1", "source", "10.0.1.1"},
		},
		{
			name:  "multi_extended_community",
			input: "mup-dsd 10.0.0.1 rd 100:100 next-hop 2001::2 extended-community [target:10:10 mup:10:10]",
			wantAttrs: map[string]string{
				"next-hop":           "2001::2",
				"extended-community": "[target:10:10 mup:10:10]",
			},
			wantNLRI: []string{"mup-dsd", "10.0.0.1", "rd", "100:100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAttrs, gotNLRI := splitFlexAttrs(tt.input)

			// Check attributes
			require.Len(t, gotAttrs, len(tt.wantAttrs), "attrs count")
			for k, want := range tt.wantAttrs {
				got, ok := gotAttrs[k]
				require.True(t, ok, "missing attr %q", k)
				assert.Equal(t, want, got, "attr[%q]", k)
			}

			// Check NLRI parts
			require.Len(t, gotNLRI, len(tt.wantNLRI), "nlri count")
			for i := range gotNLRI {
				assert.Equal(t, tt.wantNLRI[i], gotNLRI[i], "nlri[%d]", i)
			}
		})
	}
}

// TestConvertFlexToUpdate verifies full flex-to-update conversion.
//
// VALIDATES: Flex values produce correct update blocks with attribute and nlri sub-blocks.
// PREVENTS: Missing routes in migrated config for mcast-vpn and mup families.
func TestConvertFlexToUpdate(t *testing.T) {
	tests := []struct {
		name       string
		afi        string
		safi       string
		values     []string
		wantFamily string
		wantNHop   string
		wantNLRI   string
	}{
		{
			name: "mvpn_ipv4",
			afi:  "ipv4",
			safi: "mcast-vpn",
			values: []string{
				"shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 extended-community [target:192.168.94.12:5]",
			},
			// Migration translates ExaBGP "mcast-vpn" SAFI to Ze canonical "mvpn"
			// (registered by internal/component/bgp/plugins/nlri/mvpn).
			wantFamily: "ipv4/mvpn",
			wantNHop:   "10.10.6.3",
			wantNLRI:   "add shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000",
		},
		{
			name:       "mup_ipv4_isd",
			afi:        "ipv4",
			safi:       "mup",
			values:     []string{"mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 extended-community [target:10:10] bgp-prefix-sid-srv6 (l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0])"},
			wantFamily: "ipv4/mup",
			wantNHop:   "2001::1",
			wantNLRI:   "add mup-isd 10.0.1.0/24 rd 100:100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := config.NewTree()
			convertFlexToUpdate(tt.afi, tt.safi, tt.values, dst)

			updates := dst.GetListOrdered("update")
			require.Len(t, updates, len(tt.values), "update count")

			update := updates[0].Value

			// Check attribute block
			attr := update.GetContainer("attribute")
			require.NotNil(t, attr, "missing attribute block")
			nh, ok := attr.Get("next-hop")
			require.True(t, ok, "missing next-hop")
			assert.Equal(t, tt.wantNHop, nh, "next-hop")
			origin, ok := attr.Get("origin")
			require.True(t, ok, "missing origin")
			assert.Equal(t, "igp", origin, "origin")

			// Check nlri list entries — key=family, content=nlri-parts
			nlriEntries := update.GetListOrdered("nlri")
			require.NotEmpty(t, nlriEntries, "missing nlri block")
			entry := nlriEntries[0]
			assert.Equal(t, tt.wantFamily, entry.Key, "nlri family")
			content, _ := entry.Value.Get("content")
			assert.Equal(t, tt.wantNLRI, content, "nlri content")
		})
	}
}

// TestMigrationRefusesUnsupportedCap verifies migration rejects multi-session, operational, aigp.
//
// VALIDATES: AC-21, AC-22, AC-23 — migration errors on unsupported capabilities.
// PREVENTS: Silently migrating capabilities with no ze runtime implementation.
func TestMigrationRefusesUnsupportedCap(t *testing.T) {
	tests := []struct {
		name string
		cap  string
	}{
		{"multi-session", "multi-session"},
		{"operational", "operational"},
		{"aigp", "aigp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		` + tt.cap + `
	}
}
`
			tree, err := ParseExaBGPConfig(input)
			require.NoError(t, err, "parse")

			_, err = MigrateFromExaBGP(tree)
			require.Error(t, err, "expected error for unsupported capability %q", tt.cap)
			assert.Contains(t, err.Error(), "unsupported capability")
		})
	}
}

// TestMigrationSucceedsWithoutUnsupported verifies migration works when no unsupported caps present.
//
// VALIDATES: AC-27 — migration succeeds with only supported capabilities.
// PREVENTS: False positive rejections.
func TestMigrationSucceedsWithoutUnsupported(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
	capability {
		route-refresh
		extended-message
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)
	assert.Contains(t, output, "route-refresh enable")
}

// TestSanitizePeerName verifies that descriptions with spaces and special characters
// are converted to valid peer names (ASCII alphanumeric, hyphens, underscores).
func TestSanitizePeerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "router1", "router1"},
		{"spaces to hyphens", "a quagga test peer", "a-quagga-test-peer"},
		{"special chars", "m7i-4 router", "m7i-4-router"},
		{"multiple spaces", "router  with   gaps", "router-with-gaps"},
		{"leading trailing spaces", " leading ", "leading"},
		{"all special", "!@#$%", ""},
		{"mixed", "test (peer) #1", "test-peer-1"},
		{"underscores kept", "my_peer_name", "my_peer_name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePeerName(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestDerivePeerNameFromDescription verifies that migration uses sanitized description
// as peer name, falling back to peer-N when description is empty or all-special.
func TestDerivePeerNameFromDescription(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	description "a quagga test peer";
	router-id 10.0.0.2;
	local-address 127.0.0.1;
	local-as 65533;
	peer-as 65000;
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	output := SerializeTree(result.Tree)
	assert.Contains(t, output, "peer a-quagga-test-peer {", "description sanitized to peer name")
	assert.NotContains(t, output, "peer a quagga", "spaces must not appear in peer name")
}

// TestMigrateExaBGPSetsUpdateGroupsFalse verifies that migrated configs disable update groups.
//
// VALIDATES: ExaBGP migration injects environment { reactor { update-groups false; } }.
// PREVENTS: Migrated configs silently enabling cross-peer UPDATE grouping,
// which would change behavior vs ExaBGP (per-peer UPDATE building).
func TestMigrateExaBGPSetsUpdateGroupsFalse(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001
	peer-as 65002
}
`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err, "parse")

	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "migrate")

	// Verify the tree structure directly.
	env := result.Tree.GetContainer("environment")
	require.NotNil(t, env, "environment container must exist")

	reactor := env.GetContainer("reactor")
	require.NotNil(t, reactor, "reactor container must exist")

	val, ok := reactor.Get("update-groups")
	require.True(t, ok, "update-groups key must exist")
	assert.Equal(t, "false", val, "update-groups must be false for migrated ExaBGP configs")
}
