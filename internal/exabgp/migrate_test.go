package exabgp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// TestMigrateSimple verifies basic neighbor→peer conversion.
//
// VALIDATES: Simple ExaBGP config converts to ZeBGP peer syntax.
// PREVENTS: Basic migration regression.
func TestMigrateSimple(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Check peer exists
	peers := result.Tree.GetList("peer")
	if _, ok := peers["10.0.0.1"]; !ok {
		t.Error("expected peer 10.0.0.1")
	}

	// Check neighbor removed
	neighbors := result.Tree.GetList("neighbor")
	if len(neighbors) != 0 {
		t.Errorf("neighbors should be empty, got: %v", neighbors)
	}
}

// TestMigrateWithGR verifies graceful-restart injects RIB plugin.
//
// VALIDATES: GR config injects RIB plugin and process binding.
// PREVENTS: Missing RIB for GR state storage (ZeBGP delegates RIB to plugins).
func TestMigrateWithGR(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	capability {
		graceful-restart 120;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Check RIB plugin injected
	plugins := result.Tree.GetList("plugin")
	if _, ok := plugins["rib"]; !ok {
		t.Error("expected plugin rib to be injected for GR")
	}

	// Check peer has RIB process binding
	peers := result.Tree.GetList("peer")
	peerTree, ok := peers["10.0.0.1"]
	if !ok {
		t.Fatal("expected peer 10.0.0.1")
	}

	processes := peerTree.GetList("process")
	if _, ok := processes["rib"]; !ok {
		t.Error("expected process rib binding in peer")
	}

	// Check RIB injected should be in result.RIBInjected
	if !result.RIBInjected {
		t.Error("expected RIBInjected=true")
	}
}

// TestMigrateWithGRBare verifies bare graceful-restart; converts to enable.
//
// VALIDATES: Bare graceful-restart; becomes graceful-restart enable; (not "true").
// PREVENTS: Parser "true" placeholder leaking to output.
func TestMigrateWithGRBare(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	capability {
		graceful-restart;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Must contain "graceful-restart enable;" not "graceful-restart true;"
	if !strings.Contains(output, "graceful-restart enable;") {
		t.Errorf("expected 'graceful-restart enable;' in output:\n%s", output)
	}
	if strings.Contains(output, "graceful-restart true;") {
		t.Errorf("should not contain 'graceful-restart true;' in output:\n%s", output)
	}
}

// TestMigrateWithRR verifies route-refresh injects RIB plugin.
//
// VALIDATES: Route-refresh config injects RIB plugin with refresh capability.
// PREVENTS: Missing RIB for refresh response (ZeBGP delegates RIB to plugins).
func TestMigrateWithRR(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	capability {
		route-refresh;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Check RIB plugin injected
	plugins := result.Tree.GetList("plugin")
	if _, ok := plugins["rib"]; !ok {
		t.Error("expected plugin rib to be injected for route-refresh")
	}

	// Check process binding includes refresh
	peers := result.Tree.GetList("peer")
	peerTree := peers["10.0.0.1"]
	processes := peerTree.GetList("process")
	ribProcess := processes["rib"]
	if ribProcess == nil {
		t.Fatal("expected process rib binding")
	}

	// Verify send block has refresh
	sendBlock := ribProcess.GetContainer("send")
	if sendBlock == nil {
		t.Fatal("expected send block in rib process")
	}

	// Check capability uses enable syntax
	capBlock := peerTree.GetContainer("capability")
	if capBlock == nil {
		t.Fatal("expected capability block")
	}
	rrValue, ok := capBlock.Get("route-refresh")
	if !ok || rrValue != "enable" {
		t.Errorf("expected route-refresh enable, got %v", rrValue)
	}
}

// TestMigrateProcess verifies process wrapping with bridge.
//
// VALIDATES: ExaBGP process wrapped with ze exabgp plugin bridge.
// PREVENTS: Direct ExaBGP plugin usage (needs JSON translation).
func TestMigrateProcess(t *testing.T) {
	// Uses actual ExaBGP 'api { processes [...] }' syntax.
	input := `
process my-plugin {
	run /path/to/plugin.py;
	encoder json;
}

neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ my-plugin ];
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Check plugin renamed and wrapped
	plugins := result.Tree.GetList("plugin")
	var foundPlugin *config.Tree
	for name, plugin := range plugins {
		if strings.Contains(name, "compat") {
			foundPlugin = plugin
			break
		}
	}
	if foundPlugin == nil {
		t.Fatal("expected compat plugin")
	}

	// Check run command wrapped with bridge
	runStr, ok := foundPlugin.Get("run")
	if !ok {
		t.Fatal("expected run in plugin")
	}
	if !strings.Contains(runStr, "ze exabgp plugin") {
		t.Errorf("expected run to use bridge, got: %s", runStr)
	}

	// Check process removed from top level
	processes := result.Tree.GetList("process")
	if len(processes) != 0 {
		t.Errorf("process should be removed from top level")
	}
}

// TestMigrateUnsupported verifies error on unsupported features.
//
// VALIDATES: Unsupported ExaBGP features return clear error.
// PREVENTS: Silent failure on incompatible configs.
func TestMigrateUnsupported(t *testing.T) {
	// L2VPN is complex and may require manual migration
	input := `
neighbor 10.0.0.1 {
	l2vpn {
		vpls foo {
			endpoint 1;
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)

	// Should either error or warn in result
	if err == nil && len(result.Warnings) == 0 {
		t.Error("expected error or warning for complex L2VPN config")
	}
}

// TestMigrateNil verifies nil input handling.
//
// VALIDATES: Nil input returns ErrNilTree.
// PREVENTS: Panic on nil tree.
func TestMigrateNil(t *testing.T) {
	result, err := MigrateFromExaBGP(nil)
	if err == nil {
		t.Error("expected error for nil tree")
	}
	if result != nil {
		t.Error("expected nil result for nil tree")
	}
}

// TestMigrateFamilyConversion verifies family syntax conversion.
//
// VALIDATES: ExaBGP "ipv4 unicast" converts to ZeBGP "ipv4/unicast".
// PREVENTS: Wrong family syntax in migrated config.
func TestMigrateFamilyConversion(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	family {
		ipv4 unicast;
		ipv6 unicast;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Serialize and check family syntax.
	output := SerializeTree(result.Tree)

	// Must have ZeBGP format (slash), not ExaBGP (space).
	if !strings.Contains(output, "ipv4/unicast") {
		t.Errorf("expected ipv4/unicast, got:\n%s", output)
	}
	if !strings.Contains(output, "ipv6/unicast") {
		t.Errorf("expected ipv6/unicast, got:\n%s", output)
	}
	if strings.Contains(output, "ipv4 unicast") {
		t.Errorf("should not contain 'ipv4 unicast' (ExaBGP format)")
	}
	if strings.Contains(output, "ipv6 unicast") {
		t.Errorf("should not contain 'ipv6 unicast' (ExaBGP format)")
	}
}

// TestMigrateTemplate verifies template block migration.
//
// VALIDATES: Template neighbors converted to peers with all transformations.
// PREVENTS: Templates output with ExaBGP syntax (neighbor, "ipv4 unicast").
func TestMigrateTemplate(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65001;
		hold-time 180;
		family {
			ipv4 unicast;
		}
		capability {
			route-refresh;
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Template should contain peer (not neighbor).
	if !strings.Contains(output, "peer base") {
		t.Errorf("expected 'peer base' in template, got:\n%s", output)
	}
	if strings.Contains(output, "neighbor base") {
		t.Errorf("should not contain 'neighbor base'")
	}

	// Family should be converted.
	if !strings.Contains(output, "ipv4/unicast") {
		t.Errorf("expected 'ipv4/unicast', got:\n%s", output)
	}

	// Capability should have enable.
	if !strings.Contains(output, "route-refresh enable") {
		t.Errorf("expected 'route-refresh enable', got:\n%s", output)
	}
}

// TestMigrateStaticBlock verifies static block serialization.
//
// VALIDATES: Static routes preserved in output.
// PREVENTS: Static block silently dropped.
func TestMigrateStaticBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	static {
		route 192.168.0.0/24 next-hop 10.0.0.1;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Static block should be present.
	if !strings.Contains(output, "static {") {
		t.Errorf("expected static block, got:\n%s", output)
	}

	// Route should be preserved (without "true" suffix).
	if !strings.Contains(output, "route 192.168.0.0/24 next-hop 10.0.0.1;") {
		t.Errorf("expected route entry, got:\n%s", output)
	}
	if strings.Contains(output, "true") {
		t.Errorf("should not contain 'true' placeholder")
	}
}

// TestMigrateAnnounceBlock verifies announce block serialization.
//
// VALIDATES: Announce routes preserved in output.
// PREVENTS: Announce block silently dropped.
func TestMigrateAnnounceBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	announce {
		ipv4 unicast 10.0.0.0/24 next-hop self;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Announce block should be present.
	if !strings.Contains(output, "announce {") {
		t.Errorf("expected announce block, got:\n%s", output)
	}

	// Announcement should be preserved.
	if !strings.Contains(output, "ipv4 unicast 10.0.0.0/24 next-hop self;") {
		t.Errorf("expected announcement entry, got:\n%s", output)
	}
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
	local-as 65001;
	peer-as 65002;
}`,
			wantRIB: false,
		},
		{
			name: "graceful_restart",
			input: `neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	capability {
		graceful-restart;
	}
}`,
			wantRIB: true,
		},
		{
			name: "route_refresh",
			input: `neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	capability {
		route-refresh;
	}
}`,
			wantRIB: true,
		},
		{
			name: "api_receive_update",
			input: `process foo {
	run /path;
}
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	api {
		processes [ foo ];
		receive {
			update;
		}
	}
}`,
			wantRIB: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := ParseExaBGPConfig(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			got := NeedsRIBPlugin(tree)
			if got != tt.wantRIB {
				t.Errorf("NeedsRIBPlugin() = %v, want %v", got, tt.wantRIB)
			}
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
	local-as 65001;
	peer-as 65002;
	nexthop {
		ipv4 unicast ipv6;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability with family syntax conversion.
	if !strings.Contains(output, "capability {") {
		t.Errorf("expected capability block in output:\n%s", output)
	}
	if !strings.Contains(output, "nexthop {") {
		t.Errorf("expected nexthop block in output:\n%s", output)
	}
	if !strings.Contains(output, "ipv4/unicast ipv6;") {
		t.Errorf("expected 'ipv4/unicast ipv6;' in nexthop block:\n%s", output)
	}
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
	local-as 65001;
	peer-as 65002;
	capability {
		nexthop;
	}
	nexthop {
		ipv4 unicast ipv6;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Nexthop block should appear exactly once (inside capability).
	count := strings.Count(output, "nexthop {")
	if count != 1 {
		t.Errorf("expected exactly 1 'nexthop {', got %d in:\n%s", count, output)
	}

	// Nexthop block content should be present.
	if !strings.Contains(output, "ipv4/unicast ipv6;") {
		t.Errorf("expected nexthop block content in output:\n%s", output)
	}
}

// TestMigrateNexthopBlock verifies nexthop block migration with multiple entries.
//
// VALIDATES: ExaBGP "ipv4 unicast ipv6" converts to ZeBGP "ipv4/unicast ipv6".
// PREVENTS: Migration failure for RFC 8950 nexthop AFI/SAFI configuration.
func TestMigrateNexthopBlock(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	nexthop {
		ipv4 unicast ipv6;
		ipv4 mpls-vpn ipv6;
		ipv6 unicast ipv4;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability.
	if !strings.Contains(output, "capability {") {
		t.Errorf("expected capability block in output:\n%s", output)
	}
	if !strings.Contains(output, "nexthop {") {
		t.Errorf("expected nexthop block in output:\n%s", output)
	}

	// Check nexthop block syntax conversion.
	if !strings.Contains(output, "ipv4/unicast ipv6;") {
		t.Errorf("expected 'ipv4/unicast ipv6;' in output:\n%s", output)
	}
	if !strings.Contains(output, "ipv4/mpls-vpn ipv6;") {
		t.Errorf("expected 'ipv4/mpls-vpn ipv6;' in output:\n%s", output)
	}
	if !strings.Contains(output, "ipv6/unicast ipv4;") {
		t.Errorf("expected 'ipv6/unicast ipv4;' in output:\n%s", output)
	}

	// Should NOT have space-separated format.
	if strings.Contains(output, "ipv4 unicast ipv6") {
		t.Errorf("should not contain ExaBGP format 'ipv4 unicast ipv6'")
	}
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
	local-as 65001;
	peer-as 65002;
	capability {
		nexthop;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Explicit nexthop capability is NOT migrated - it's useless without nexthop block.
	if strings.Contains(output, "nexthop enable") {
		t.Errorf("should not contain 'nexthop enable' (useless without nexthop block):\n%s", output)
	}

	// Should NOT have nexthop block (none in input).
	if strings.Contains(output, "nexthop {") {
		t.Errorf("should not have nexthop block:\n%s", output)
	}

	// Should still have peer block.
	if !strings.Contains(output, "peer 10.0.0.1") {
		t.Errorf("expected peer block:\n%s", output)
	}
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
	local-as 65001;
	peer-as 65002;
	capability {
		nexthop;
	}
	nexthop {
		ipv4 unicast ipv6;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Nexthop block should be inside capability (only once).
	count := strings.Count(output, "nexthop {")
	if count != 1 {
		t.Errorf("expected exactly 1 'nexthop {', got %d in:\n%s", count, output)
	}

	// Nexthop block content should be present.
	if !strings.Contains(output, "ipv4/unicast ipv6;") {
		t.Errorf("expected 'ipv4/unicast ipv6;' in output:\n%s", output)
	}
}

// TestMigrateNexthopBlockSAFINormalization verifies SAFI name normalization.
//
// VALIDATES: ExaBGP "nlri-mpls" and "labeled-unicast" convert to ZeBGP "mpls-label".
// PREVENTS: Migrated nexthop config not recognized by ZeBGP's parseNexthopFamilies.
func TestMigrateNexthopBlockSAFINormalization(t *testing.T) {
	input := `
neighbor 10.0.0.1 {
	local-as 65001;
	peer-as 65002;
	nexthop {
		ipv4 nlri-mpls ipv6;
		ipv4 labeled-unicast ipv6;
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Both should be normalized to mpls-label.
	if !strings.Contains(output, "ipv4/mpls-label ipv6;") {
		t.Errorf("expected 'ipv4/mpls-label ipv6;' in output:\n%s", output)
	}

	// Should NOT have ExaBGP SAFI names.
	if strings.Contains(output, "nlri-mpls") {
		t.Errorf("should not contain 'nlri-mpls' (ExaBGP name)")
	}
	if strings.Contains(output, "labeled-unicast") {
		t.Errorf("should not contain 'labeled-unicast' (ExaBGP name)")
	}
}

// TestMigrateTemplateWithNexthop verifies nexthop block in templates.
//
// VALIDATES: Template neighbor nexthop blocks are converted correctly.
// PREVENTS: Templates missing nexthop conversion.
func TestMigrateTemplateWithNexthop(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65001;
		nexthop {
			ipv4 unicast ipv6;
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	output := SerializeTree(result.Tree)

	// Template should have peer (not neighbor).
	if !strings.Contains(output, "peer base") {
		t.Errorf("expected 'peer base' in template, got:\n%s", output)
	}

	// Nexthop block should be inside capability.
	if !strings.Contains(output, "capability {") {
		t.Errorf("expected capability block in output:\n%s", output)
	}
	if !strings.Contains(output, "nexthop {") {
		t.Errorf("expected nexthop block in output:\n%s", output)
	}

	// Nexthop block should be converted.
	if !strings.Contains(output, "ipv4/unicast ipv6") {
		t.Errorf("expected 'ipv4/unicast ipv6', got:\n%s", output)
	}
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
			if err != nil {
				t.Fatalf("parse input: %v", err)
			}

			// Migrate.
			result, err := MigrateFromExaBGP(tree)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}

			// Serialize result.
			gotOutput := SerializeTree(result.Tree)

			// Exact comparison against expected.conf.
			want := strings.TrimSpace(string(expectedData))
			got := strings.TrimSpace(gotOutput)

			if got != want {
				t.Errorf("migration output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}

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
	for i := 0; i < 5; i++ {
		testPath := filepath.Join(wd, "test/exabgp")
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
		// Should have peer, not neighbor.
		if !strings.Contains(got, "peer 10.0.0.1") {
			t.Error("expected 'peer 10.0.0.1' in output")
		}

	case "graceful-restart":
		// Should have RIB plugin injected.
		if !result.RIBInjected {
			t.Error("expected RIBInjected=true for graceful-restart")
		}
		if !strings.Contains(got, "plugin rib") {
			t.Error("expected 'plugin rib' in output")
		}

	case "route-refresh":
		// Should have RIB plugin injected.
		if !result.RIBInjected {
			t.Error("expected RIBInjected=true for route-refresh")
		}
		// Should have route-refresh enable.
		if !strings.Contains(got, "route-refresh enable") {
			t.Error("expected 'route-refresh enable' in output")
		}

	case "process":
		// Should have compat plugin wrapped with bridge.
		if !strings.Contains(got, "ze exabgp plugin") {
			t.Error("expected bridge wrapper in plugin run command")
		}
		if !strings.Contains(got, "-compat") {
			t.Error("expected '-compat' suffix in plugin name")
		}

	case "nexthop":
		// Should have nexthop block inside capability.
		if !strings.Contains(got, "capability {") {
			t.Error("expected capability block in output")
		}
		if !strings.Contains(got, "nexthop {") {
			t.Error("expected nexthop block inside capability")
		}
		// Should NOT have RIB injected (nexthop doesn't require state storage).
		if result.RIBInjected {
			t.Error("nexthop should not trigger RIB injection")
		}
		// Should have nexthop block with converted syntax.
		if !strings.Contains(got, "ipv4/unicast ipv6") {
			t.Error("expected 'ipv4/unicast ipv6' in output")
		}
		if strings.Contains(got, "ipv4 unicast ipv6") {
			t.Error("should not contain ExaBGP format 'ipv4 unicast ipv6'")
		}
	}

	// Log output for debugging.
	t.Logf("Migration output:\n%s", got)
}
