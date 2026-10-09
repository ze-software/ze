// Design: docs/architecture/module-tiers.md -- ownership and registration edge fixtures.
package archtier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestPluginOwnershipEdges changes one real import at a time and checks the
// diagnostic as well as the verdict. Comments and strings are not dependencies.
func TestPluginOwnershipEdges(t *testing.T) {
	const target = "internal/component/bgp/plugins/nlri/ls"
	const named = "package use\nimport codec \"example.com/m/" + target + "\"\nvar Value = codec.Value\n"
	cases := []struct {
		name   string
		file   string
		source string
		bad    bool
	}{
		{"alias", "internal/component/bgp/reactor/use.go", named, true},
		{"platform", "internal/component/bgp/reactor/use_windows.go", "//go:build windows\n\n" + named, true},
		{"dot", "internal/component/bgp/reactor/use.go", "package use\nimport . \"example.com/m/" + target + "\"\nvar V = Value\n", true},
		{"grouped-raw", "internal/component/bgp/reactor/use.go", "package use\nimport (\ncodec `example.com/m/" + target + "`\n)\nvar Value = codec.Value\n", true},
		{"blank-outside-composition", "internal/component/bgp/reactor/use.go", blankImport("use", target), true},
		{"core-helper", "internal/core/helper/use.go", named, true},
		{"public-helper", "pkg/helper/use.go", named, true},
		{"misleading-dispatch", "cmd/ze/dispatch_fake.go", strings.Replace(named, "package use", "package main", 1), true},
		{"nested-command", "cmd/ze/hub/dispatch.go", blankImport("hub", target), true},
		{"fake-all", "internal/component/other/all/all.go", blankImport("all", target), true},
		{"functional-all", "internal/component/plugin/all/all_extra.go", strings.Replace(named, "package use", "package all", 1), true},
		{"executable-all", "internal/component/plugin/all/all_extra.go", blankImport("all", target) + "func init() {}\n", true},
		{"composition", "internal/component/plugin/all/all_extra.go", blankImport("all", target), false},
		{"dispatch-composition", "cmd/ze/new_personality.go", blankImport("main", target), false},
		{"within-owner", target + "/helper/use.go", named, false},
		{"schema-dependency", "internal/component/bgp/plugins/nlri/flowspec/yang/register.go", blankImport("yang", target+"/yang"), false},
		{"schema-into-implementation", "internal/component/bgp/plugins/nlri/flowspec/yang/register.go", blankImport("yang", target), true},
		{"named-schema-dependency", "internal/component/bgp/plugins/nlri/flowspec/yang/register.go", strings.Replace(named, "/ls\"", "/ls/yang\"", 1), true},
		{"sibling-nlri", "internal/component/bgp/plugins/nlri/flowspec/use.go", named, true},
		{"test", "internal/component/bgp/reactor/use_test.go", named, false},
		{"tool", "internal/le/probe/use.go", named, false},
		{"harness", "internal/test/probe/use.go", named, false},
		{"comments-and-strings", "internal/component/bgp/reactor/use.go", "package use\n// import \"example.com/m/" + target + "\"\nconst Text = \"example.com/m/" + target + "\"\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := ownershipFixture()
			files[tc.file] = tc.source
			files["internal/component/bgp/plugins/nlri/flowspec/register.go"] = "package flowspec\n"
			graph, err := collectSourceEdges(fixtureTree(t, files), fixtureModule)
			if err != nil {
				t.Fatal(err)
			}
			result := pluginOwnershipGate(fixtureModule, graph)
			if (result.Code != 0) != tc.bad {
				t.Fatalf("code %d, bad=%v: %s", result.Code, tc.bad, result.Diagnosis)
			}
			if tc.bad && !strings.Contains(result.Diagnosis, tc.file+" imports "+target) {
				t.Fatalf("missing offending edge: %s", result.Diagnosis)
			}
		})
	}
}

// TestPluginOwnershipRoots proves host contracts remain shared while standalone
// plugin roots and registered host descendants remain independently owned.
func TestPluginOwnershipRoots(t *testing.T) {
	roots := PluginDirs()
	declared := []string{
		"internal/component/bfd", "internal/component/bfd/driver",
		"internal/component/bgp/plugins/rib", "internal/component/bgp/plugins/rib/cli",
		"internal/component/bgp/plugins/nlri/ls", "internal/component/bgp/plugins/nlri/flowspec",
	}
	cases := map[string]string{
		"internal/component/bfd":                       "",
		"internal/component/bfd/api":                   "",
		"internal/component/bfd/packet":                "",
		"internal/component/bfd/driver/impl":           "internal/component/bfd/driver",
		"internal/component/iface":                     "",
		"internal/component/bgp/reactor/filter":        "internal/component/bgp/reactor/filter",
		"internal/component/bgp/plugins/rib/pool":      "internal/component/bgp/plugins/rib",
		"internal/component/bgp/plugins/rib/cli":       "internal/component/bgp/plugins/rib",
		"internal/component/bgp/plugins/nlri/ls":       "internal/component/bgp/plugins/nlri/ls",
		"internal/component/bgp/plugins/nlri/flowspec": "internal/component/bgp/plugins/nlri/flowspec",
		"internal/plugins/unregistered/helper":         "internal/plugins/unregistered",
	}
	for imported, want := range cases {
		if got := pluginOwner(imported, roots, declared); got != want {
			t.Errorf("owner(%s) = %q, want %q", imported, got, want)
		}
	}
	// A future nested namespace gets its own owner, never its enclosing owner.
	roots = append(roots, "internal/plugins/outer/plugins")
	declared = append(declared, "internal/plugins/outer")
	if got := pluginOwner("internal/plugins/outer/plugins/inner/helper", roots, declared); got != "internal/plugins/outer/plugins/inner" {
		t.Fatalf("nested owner = %q", got)
	}
}

// TestRegisteredTierCheckEnforcesPluginOwnership dispatches through the actual
// le registration, with all other gates clean. A helper edge makes it red and
// removing that edge makes it green again, without a baseline or source rebuild.
func TestRegisteredTierCheckEnforcesPluginOwnership(t *testing.T) {
	root := fixtureTree(t, ownershipFixture())
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	check := func(want int) {
		t.Helper()
		if got := leroot.Dispatch("le", []string{"arch", "tier", "check"}); got != want {
			t.Fatalf("registered check = %d, want %d", got, want)
		}
	}
	check(0)
	const helper = "internal/component/bgp/helper/use.go"
	if err := writeFixture(root, map[string]string{
		helper:              blankImport("helper", "internal/component/bgp/plugins/nlri/ls"),
		"cmd/ze/wrapper.go": blankImport("main", "internal/component/bgp/helper"),
	}); err != nil {
		t.Fatal(err)
	}
	check(2)
	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range report.Checks {
		if result.Name == "plugin-ownership" {
			if result.Code != 2 || !strings.Contains(result.Diagnosis, helper) {
				t.Fatalf("ownership result = %+v", result)
			}
			continue
		}
		if result.Code != 0 {
			t.Fatalf("unrelated gate failed: %+v", result)
		}
	}
	if err := os.Remove(filepath.Join(root, helper)); err != nil {
		t.Fatal(err)
	}
	check(0)
}

// TestMalformedSourceStopsRegisteredTierCheck uses an inactive platform file
// with a broken body; ImportsOnly parsing would miss it.
func TestMalformedSourceStopsRegisteredTierCheck(t *testing.T) {
	files := ownershipFixture()
	const broken = "internal/component/bgp/helper/use_windows.go"
	files[broken] = "//go:build windows\n\npackage helper\nfunc broken( {\n"
	root := fixtureTree(t, files)
	if _, err := Check(root); err == nil {
		t.Fatal("malformed production source passed")
	} else if !strings.Contains(err.Error(), "use_windows.go") {
		t.Fatalf("error omitted source path: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if code := leroot.Dispatch("le", []string{"arch", "tier", "check"}); code != 2 {
		t.Fatalf("malformed registered check = %d, want 2", code)
	}
}

// TestStandalonePluginAndHostChildEdges exercises the graph, not just owner
// lookup, for a standalone plugin root and an independently registered child.
func TestStandalonePluginAndHostChildEdges(t *testing.T) {
	for _, target := range []string{
		"internal/component/bgp/reactor/filter",
		"internal/component/bfd/driver",
		"internal/plugins/diag/cmd",
	} {
		t.Run(target, func(t *testing.T) {
			files := ownershipFixture()
			files[target+"/register.go"] = "package implementation\n"
			const importer = "internal/component/bfd/use.go"
			files[importer] = blankImport("bfd", target)
			graph, err := collectSourceEdges(fixtureTree(t, files), fixtureModule)
			if err != nil {
				t.Fatal(err)
			}
			result := pluginOwnershipGate(fixtureModule, graph)
			if result.Code != 2 || !strings.Contains(result.Diagnosis, importer+" imports "+target) {
				t.Fatalf("missing ownership violation: %+v", result)
			}
		})
	}
}

// TestUnreadableSourceStopsRegisteredTierCheck uses a dangling source symlink
// so the failure is deterministic even when tests run with root privileges.
func TestUnreadableSourceStopsRegisteredTierCheck(t *testing.T) {
	root := fixtureTree(t, ownershipFixture())
	unreadable := filepath.Join(root, "cmd", "ze", "unreadable.go")
	if err := os.Symlink(filepath.Join(root, "absent.go"), unreadable); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root); err == nil {
		t.Fatal("unreadable source passed")
	}
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if code := leroot.Dispatch("le", []string{"arch", "tier", "check"}); code != 2 {
		t.Fatalf("unreadable registered check = %d, want 2", code)
	}
}

// TestOwnershipDiagnosticsAreSorted fixes the order even though the graph is a
// map, and proves registration exemptions do not propagate through helpers.
func TestOwnershipDiagnosticsAreSorted(t *testing.T) {
	files := ownershipFixture()
	const target = "internal/component/bgp/plugins/nlri/ls"
	files["pkg/z/use.go"] = blankImport("z", target)
	files["pkg/a/use.go"] = blankImport("a", target)
	files["cmd/ze/wrapper.go"] = blankImport("main", "pkg/z")
	graph, err := collectSourceEdges(fixtureTree(t, files), fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	result := pluginOwnershipGate(fixtureModule, graph)
	first := strings.Index(result.Diagnosis, "pkg/a/use.go imports "+target)
	second := strings.Index(result.Diagnosis, "pkg/z/use.go imports "+target)
	if result.Code != 2 || first < 0 || second <= first {
		t.Fatalf("expected both offending edges in path order: %+v", result)
	}
}
