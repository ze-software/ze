package exabgp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/config"
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
// VALIDATES: ExaBGP process wrapped with zebgp exabgp plugin bridge.
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
	if !strings.Contains(runStr, "zebgp exabgp plugin") {
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

// TestMigrateFileBasedTests runs file-based migration tests.
// Each test directory in test/data/migrate/ contains:
//   - input.conf: ExaBGP config to migrate
//   - expected.conf: Expected ZeBGP output
//
// VALIDATES: File-based migration produces exact expected output.
// PREVENTS: Regression in migration output format.
func TestMigrateFileBasedTests(t *testing.T) {
	// Find test/data/migrate directory.
	testDataDir := findTestDataDir(t)
	if testDataDir == "" {
		t.Skip("test/data/migrate directory not found")
	}

	tests := []string{"simple", "graceful-restart", "route-refresh", "process"}

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

// findTestDataDir finds the test/data/migrate directory.
func findTestDataDir(t *testing.T) string {
	t.Helper()

	// Try relative paths from common locations
	paths := []string{
		"test/data/migrate",
		"../../test/data/migrate",
		"../../../test/data/migrate",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try from module root
	wd, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		testPath := filepath.Join(wd, "test/data/migrate")
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
		if !strings.Contains(got, "zebgp exabgp plugin") {
			t.Error("expected bridge wrapper in plugin run command")
		}
		if !strings.Contains(got, "-compat") {
			t.Error("expected '-compat' suffix in plugin name")
		}
	}

	// Log output for debugging.
	t.Logf("Migration output:\n%s", got)
}
