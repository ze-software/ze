package all

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func assertSnapshot(t *testing.T, label string, got, expected []string) {
	t.Helper()
	if slices.Equal(got, expected) {
		return
	}
	have := make(map[string]bool, len(got))
	for _, s := range got {
		have[s] = true
	}
	want := make(map[string]bool, len(expected))
	for _, s := range expected {
		want[s] = true
		if !have[s] {
			t.Errorf("missing %s: %q", label, s)
		}
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected %s: %q (add to expected list if intentional)", label, s)
		}
	}
}

// the three registry snapshots below moved from hand-maintained
// `expected := []string{...}` literals to golden files in testdata/. The
// assertSnapshot comparison is unchanged; only the source of `expected` moved,
// and -update regenerates it from the live registry so it can never drift from
// all.go.
var updateSnapshot = flag.Bool("update", false,
	"rewrite plugin/all registry snapshots in testdata/ from the live registry")

// snapshot compares got to testdata/<name>.snapshot, or rewrites that golden
// file when -update is set. Regenerate after adding a plugin with:
//
//	go test -tags '<ze_core + features>' -update ./internal/component/plugin/all/
func snapshot(t *testing.T, name string, got []string) {
	t.Helper()
	path := filepath.Join("testdata", name+".snapshot")
	if *updateSnapshot {
		var b strings.Builder
		for _, s := range got {
			b.WriteString(s)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
			t.Fatalf("write snapshot %s: %v", path, err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot %s (regenerate: go test -tags '...' -update ./...): %v", path, err)
	}
	var expected []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if e := strings.TrimSpace(line); e != "" {
			expected = append(expected, e)
		}
	}
	assertSnapshot(t, name, got, expected)
}

// TestRegisteredPluginNames snapshots the full set of registered plugin names.
//
// VALIDATES: Every expected plugin is registered after init().
// PREVENTS: Silent removal of a plugin (deleted register.go, dropped import).
func TestRegisteredPluginNames(t *testing.T) {

	names := registry.Names()
	sort.Strings(names)

	// linux-only plugins (e.g. iface-dhcp) are excluded from the
	// cross-platform snapshot; TestPlatformPlugins covers them.
	platformOnly := map[string]bool{"iface-dhcp": true, "iface-ra": true}

	var filtered []string
	for _, n := range names {
		if !platformOnly[n] {
			filtered = append(filtered, n)
		}
	}

	snapshot(t, "plugins", filtered)
}

// TestFilterTypeMappings snapshots the registered filter types.
//
// VALIDATES: Every expected policy filter type is registered.
// PREVENTS: Silent removal of a policy feature (e.g. prefix-list filtering).
func TestFilterTypeMappings(t *testing.T) {
	expected := map[string]string{
		"as-path-length":    "bgp-filter-aspath-length",
		"as-path-list":      "bgp-filter-aspath",
		"community-match":   "bgp-filter-community-match",
		"family-filter":     "bgp-filter-family",
		"modify":            "bgp-filter-modify",
		"prefix-list":       "bgp-filter-prefix",
		"remove-private-as": "bgp-filter-remove-private-as",
	}

	fm := registry.FilterTypesMap()
	for ft, wantPlugin := range expected {
		if got := fm[ft]; got != wantPlugin {
			t.Errorf("FilterTypesMap[%q] = %q, want %q", ft, got, wantPlugin)
		}
	}

	for ft, plugin := range fm {
		if _, ok := expected[ft]; !ok {
			t.Errorf("unexpected filter type %q -> %q (add to expected map if intentional)", ft, plugin)
		}
	}
}

// TestRegisteredWireMethods snapshots the full set of RPC wire methods.
//
// VALIDATES: Every expected CLI/API command is registered after init().
// PREVENTS: Silent removal of a user-facing command (deleted handler, dropped register).
func TestRegisteredWireMethods(t *testing.T) {

	rpcs := pluginserver.AllBuiltinRPCs()
	var methods []string
	for _, r := range rpcs {
		methods = append(methods, r.WireMethod)
	}
	sort.Strings(methods)

	snapshot(t, "wire-methods", methods)
}

// TestYANGSchemaProviders snapshots the set of plugins that provide YANG schemas.
//
// VALIDATES: Every expected plugin provides a YANG schema.
// PREVENTS: Silent removal of configuration surface (deleted YANG field).
func TestYANGSchemaProviders(t *testing.T) {

	schemas := registry.YANGSchemas()
	var names []string
	for n := range schemas {
		names = append(names, n)
	}
	sort.Strings(names)

	snapshot(t, "yang-providers", names)
}

// TestGeneratedPluginImportsCurrent verifies that the generated blank-import
// file matches register.go discovery.
//
// CAVEAT -- shares the caching hole documented on TestYANGGlueCurrent
// (yang_glue_check_test.go): a register.go in a package this one does not yet
// import is not a build input here, so `go test` can serve a cached PASS after
// one appears, and the full-verify stage is ze-unit-test-cached. The uncached
// backstop is the ze-generated-files-check make stage, whose
// ze-plugin-imports-check prerequisite runs the same --check from a recipe in
// both stagesForMode branches. This test is the fast local signal, not the
// whole guard.
//
// VALIDATES: plugin/all generation is checked by the same generator that writes it.
// PREVENTS: Missing plugin registration when a register.go package is not imported.
func TestGeneratedPluginImportsCurrent(t *testing.T) {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "go", "run", "../../../../internal/le/plugin/imports/pluginimports.go", "--check")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin/all generated imports are stale: %v\n%s", err, out)
	}
}

// TestAllPluginsRegistered verifies that importing the all package populates
// the production plugin registry.
//
// VALIDATES: Production plugin aggregation registers plugins from plugin/all.
// VALIDATES: Production plugin aggregation excludes internal/test/plugins.
// PREVENTS: Shipping test scaffolding in cmd/ze.
func TestAllPluginsRegistered(t *testing.T) {
	names := registry.Names()
	if len(names) == 0 {
		t.Fatal("plugin/all registered no plugins")
	}

	for _, testOnly := range []string{"fakel2tp", "fakeredist"} {
		if slices.Contains(names, testOnly) {
			t.Errorf("test-only plugin %q registered in production plugin/all", testOnly)
		}
	}
}

// TestPlatformPlugins verifies platform-gated plugin registration.
//
// VALIDATES: Linux-only plugins register only on Linux builds.
// PREVENTS: Accidental non-Linux registration of Linux-only plugins.
func TestPlatformPlugins(t *testing.T) {
	names := registry.Names()
	for _, plugin := range []string{"iface-dhcp", "iface-ra"} {
		present := slices.Contains(names, plugin)
		if runtime.GOOS == "linux" {
			if !present {
				t.Errorf("linux-only plugin %q not registered on linux", plugin)
			}
			continue
		}
		if present {
			t.Errorf("linux-only plugin %q registered on %s", plugin, runtime.GOOS)
		}
	}
}

// TestAllPluginsHaveRunEngine verifies that every registered plugin has a RunEngine handler.
//
// VALIDATES: No plugin was registered without an engine handler.
// PREVENTS: Nil pointer dereference when starting plugin in engine mode.
func TestAllPluginsHaveRunEngine(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.RunEngine == nil {
			t.Errorf("plugin %q has nil RunEngine", reg.Name)
		}
	}
}

// TestAllPluginsHaveCLIHandler verifies that every registered plugin has a CLI handler.
//
// VALIDATES: No plugin was registered without a CLI handler.
// PREVENTS: Nil pointer dereference when dispatching CLI command.
func TestAllPluginsHaveCLIHandler(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.CLIHandler == nil {
			t.Errorf("plugin %q has nil CLIHandler", reg.Name)
		}
	}
}

// TestAllPluginsHaveDescription verifies that every registered plugin has a description.
//
// VALIDATES: Help text will have descriptions for all plugins.
// PREVENTS: Empty lines in `ze plugin help` output.
func TestAllPluginsHaveDescription(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.Description == "" {
			t.Errorf("plugin %q has empty Description", reg.Name)
		}
	}
}

// TestFamilyMappings verifies that expected families are mapped to plugins.
//
// VALIDATES: Family-to-plugin mapping works after init() registration.
// PREVENTS: Broken auto-discovery when a family plugin is configured.
func TestFamilyMappings(t *testing.T) {
	fm := registry.FamilyMap()

	expected := map[string]string{
		"ipv4/flow":         "bgp-nlri-flowspec",
		"ipv6/flow":         "bgp-nlri-flowspec",
		"ipv4/flow-vpn":     "bgp-nlri-flowspec",
		"ipv6/flow-vpn":     "bgp-nlri-flowspec",
		"l2vpn/evpn":        "bgp-nlri-evpn",
		"ipv4/mpls-vpn":     "bgp-nlri-vpn",
		"ipv6/mpls-vpn":     "bgp-nlri-vpn",
		"bgp-ls/bgp-ls":     "bgp-nlri-ls",
		"bgp-ls/bgp-ls-vpn": "bgp-nlri-ls",
	}

	for fam, wantPlugin := range expected {
		if got := fm[fam]; got != wantPlugin {
			t.Errorf("FamilyMap[%q] = %q, want %q", fam, got, wantPlugin)
		}
	}
}

// TestBgpRSDependsOnAdjRibIn verifies bgp-rs still declares its relationship
// with bgp-adj-rib-in after the spec-rs-fastpath-2-adjrib soft-dep refactor.
//
// VALIDATES: bgp-rs has OptionalDependencies containing "bgp-adj-rib-in".
// PREVENTS: accidental removal of the relationship -- which would let bgp-rs
// silently start without the replay-on-peer-up capability and without the
// soft-dep resolver pulling adj-rib-in in when it is registered.
func TestBgpRSDependsOnAdjRibIn(t *testing.T) {
	reg := registry.Lookup("bgp-rs")
	if reg == nil {
		t.Fatal("bgp-rs not registered")
		return
	}

	if slices.Contains(reg.Dependencies, "bgp-adj-rib-in") {
		t.Errorf("bgp-rs Dependencies=%v must NOT contain bgp-adj-rib-in (moved to OptionalDependencies by spec-rs-fastpath-2-adjrib)", reg.Dependencies)
	}
	if !slices.Contains(reg.OptionalDependencies, "bgp-adj-rib-in") {
		t.Errorf("bgp-rs OptionalDependencies=%v, want to contain bgp-adj-rib-in", reg.OptionalDependencies)
	}
}

// TestPolicyRoutesDependsOnFirewall verifies policy-routes startup is ordered
// after the firewall plugin that owns nftables apply/reconcile.
//
// VALIDATES: policy-routes has Dependencies containing "firewall".
// PREVENTS: policy-routes applying firewall tables before firewall startup.
func TestPolicyRoutesDependsOnFirewall(t *testing.T) {
	reg := registry.Lookup("policy-routes")
	if reg == nil {
		t.Fatal("policy-routes not registered")
		return
	}

	if !slices.Contains(reg.Dependencies, "firewall") {
		t.Errorf("policy-routes Dependencies=%v, want to contain firewall", reg.Dependencies)
	}
}

// TestCapabilityMappings verifies that capability codes are mapped to plugins.
//
// VALIDATES: Capability-to-plugin mapping works after init() registration.
// PREVENTS: Broken capability decode in OPEN message handling.
func TestCapabilityMappings(t *testing.T) {
	cm := registry.CapabilityMap()

	if cm[64] != "bgp-gr" {
		t.Errorf("CapabilityMap[64] = %q, want bgp-gr", cm[64])
	}
	if cm[73] != "bgp-hostname" {
		t.Errorf("CapabilityMap[73] = %q, want bgp-hostname", cm[73])
	}
	if cm[75] != "bgp-softver" {
		t.Errorf("CapabilityMap[75] = %q, want bgp-softver", cm[75])
	}
	if cm[77] != "bgp-llnh" {
		t.Errorf("CapabilityMap[77] = %q, want bgp-llnh", cm[77])
	}
	if cm[9] != "bgp-role" {
		t.Errorf("CapabilityMap[9] = %q, want bgp-role", cm[9])
	}
	if cm[2] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[2] = %q, want bgp-route-refresh", cm[2])
	}
	if cm[70] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[70] = %q, want bgp-route-refresh", cm[70])
	}
}
