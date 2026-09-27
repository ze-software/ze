package server

import (
	"encoding/json"
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: a plugin's registry ConfigReads join the roots the running server
// delivers, once, beside what the plugin declared over the SDK.
// PREVENTS: a read root reaching only `ze config validate` (plugin_verify.go)
// while the daemon's commit and SIGHUP deliver nothing for it, so a verifier
// that checks across roots judges an empty section.
func TestJoinConfigReadsAddsEachReadOnce(t *testing.T) {
	declared := []string{"static", "interface", "bfd"}
	reg := &plugin.PluginRegistration{WantsConfigRoots: declared}

	joinConfigReads(reg, []string{"bfd", "pki"})

	if want := []string{"static", "interface", "bfd", "pki"}; !slices.Equal(reg.WantsConfigRoots, want) {
		t.Fatalf("WantsConfigRoots = %v, want %v", reg.WantsConfigRoots, want)
	}
	if want := []string{"bfd", "pki"}; !slices.Equal(reg.ConfigReads, want) {
		t.Fatalf("ConfigReads = %v, want %v", reg.ConfigReads, want)
	}
	if !slices.Equal(declared, []string{"static", "interface", "bfd"}) {
		t.Fatalf("the SDK declaration was mutated: %v", declared)
	}
}

// VALIDATES: a plugin that reads another root receives every root it holds,
// whole, when any of them changed, and a change to the read root alone makes
// it a participant.
// PREVENTS: static and BGP checking a bfd-profile against a bfd section the
// reload never sent (a static or bgp edit refused with "config has no bfd
// section"), and a bfd-only edit that deletes a named profile reaching neither
// plugin, so the commit accepts it.
func TestReloadConfigSectionsDeliversReadersWhole(t *testing.T) {
	tree := map[string]any{
		"static": map[string]any{"table": "default"},
		"bfd":    map[string]any{"profile": map[string]any{"fast": map[string]any{}}},
		"bgp":    map[string]any{"router-id": "1.2.3.4"},
	}
	reader := &plugin.PluginRegistration{
		WantsConfigRoots: []string{"static", "interface", "bfd"},
		ConfigReads:      []string{"bfd"},
	}
	owner := &plugin.PluginRegistration{WantsConfigRoots: []string{"static", "interface"}}

	cases := []struct {
		name  string
		diff  *config.ConfigDiff
		reg   *plugin.PluginRegistration
		roots []string
	}{
		{"read root changed alone", changedAt("bfd/profile/fast/passive"), reader, []string{"static", "bfd"}},
		{"own root changed alone", changedAt("static/table"), reader, []string{"static", "bfd"}},
		{"unrelated root changed", changedAt("bgp/router-id"), reader, nil},
		{"no reads: changed roots only", changedAt("static/table"), owner, []string{"static"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := reloadConfigSections(tree, tc.diff, tc.reg)
			if err != nil {
				t.Fatal(err)
			}
			if got := sectionRoots(sections); !slices.Equal(got, tc.roots) {
				t.Fatalf("delivered roots = %v, want %v", got, tc.roots)
			}
			for _, s := range sections {
				var data map[string]any
				if err := json.Unmarshal([]byte(s.Data), &data); err != nil {
					t.Fatalf("%s: %v", s.Root, err)
				}
				if len(data) == 0 {
					t.Fatalf("%s delivered empty, want the whole current subtree", s.Root)
				}
			}
		})
	}
}

// VALIDATES: a root that is absent from the tree and did change arrives as an
// empty section (its deletion), while an absent root nothing changed is not
// delivered at all.
// PREVENTS: a reader told its own root was deleted because a read root changed.
func TestReloadConfigSectionsAbsentRoots(t *testing.T) {
	tree := map[string]any{"static": map[string]any{"table": "default"}}
	reader := &plugin.PluginRegistration{
		WantsConfigRoots: []string{"static", "bfd"},
		ConfigReads:      []string{"bfd"},
	}
	removed := &config.ConfigDiff{Removed: map[string]any{"bfd": map[string]any{}}}

	sections, err := reloadConfigSections(tree, removed, reader)
	if err != nil {
		t.Fatal(err)
	}
	if got := sectionRoots(sections); !slices.Equal(got, []string{"static", "bfd"}) {
		t.Fatalf("delivered roots = %v, want [static bfd]", got)
	}
	if sections[1].Data != "{}" {
		t.Fatalf("deleted bfd delivered %q, want {}", sections[1].Data)
	}

	sections, err = reloadConfigSections(map[string]any{"bfd": map[string]any{"x": 1}}, changedAt("bfd/x"), reader)
	if err != nil {
		t.Fatal(err)
	}
	if got := sectionRoots(sections); !slices.Equal(got, []string{"bfd"}) {
		t.Fatalf("absent unchanged static was delivered: %v", got)
	}
}

func changedAt(path string) *config.ConfigDiff {
	return &config.ConfigDiff{Changed: map[string]config.DiffPair{path: {Old: "a", New: "b"}}}
}

func sectionRoots(sections []rpc.ConfigSection) []string {
	var roots []string
	for _, s := range sections {
		roots = append(roots, s.Root)
	}
	return roots
}

// VALIDATES: the read roots joined at Stage 1 come from the registry row of the
// implementation a process runs, found through its use/run spelling, not from
// the operator's block name.
// PREVENTS: `plugin { internal rpki { use bgp-rpki } }` looking the registry up
// as "rpki", finding nothing, and bgp-rpki never receiving its pki section, so
// its TLS certificate lookup fails. Static and bgp under a renamed block lost
// their bfd section the same way.
func TestRegistryConfigReadsResolvesTheImplementation(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	const impl = "test-config-reader"
	if err := registry.Register(registry.Registration{
		Name:        impl,
		Description: "test reader",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		ConfigReads: []string{"pki"},
	}); err != nil {
		t.Fatal(err)
	}

	renamed := plugin.PluginConfig{Name: "renamed", Internal: true, Run: impl}
	if got := registryConfigReads(renamed); !slices.Equal(got, []string{"pki"}) {
		t.Fatalf("a block renamed from %s answered %v, want [pki]", impl, got)
	}
	autoloaded := plugin.PluginConfig{Name: impl}
	if got := registryConfigReads(autoloaded); !slices.Equal(got, []string{"pki"}) {
		t.Fatalf("an auto-loaded %s answered %v, want [pki]", impl, got)
	}
	external := plugin.PluginConfig{Name: "renamed", Run: "/usr/bin/some-plugin"}
	if got := registryConfigReads(external); got != nil {
		t.Fatalf("an external plugin answered %v, want nil", got)
	}
	// An `external test-config-reader { run "/opt/x" }` block runs a program
	// that shares the compiled-in row's name. The row is not this block's, so
	// the program is not handed the roots the compiled-in plugin reads.
	collision := plugin.PluginConfig{Name: impl, Run: "/opt/x"}
	if got := registryConfigReads(collision); got != nil {
		t.Fatalf("an external block named %s answered %v, want nil", impl, got)
	}
}

// VALIDATES: the roots a removed plugin's recovery looks for in the committed
// tree come from the registry row of the implementation the process runs, and
// an external block that shares a compiled-in row's name owns none of them.
// PREVENTS: restoreRemovedPlugin keying registry.ConfigRootsMap by the block
// name, so `external bgp-rpki { run "/opt/x" }` was restored as if it were the
// compiled-in bgp-rpki, and a renamed internal block found no roots at all.
func TestRegistryConfigRootsResolvesTheImplementation(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	const impl = "test-config-owner"
	if err := registry.Register(registry.Registration{
		Name:        impl,
		Description: "test owner",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		ConfigRoots: []string{"owned"},
	}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		cfg  plugin.PluginConfig
		want []string
	}{
		{"renamed internal block", plugin.PluginConfig{Name: "renamed", Internal: true, Run: impl}, []string{"owned"}},
		{"auto-loaded", plugin.PluginConfig{Name: impl, Internal: true}, []string{"owned"}},
		{"external program", plugin.PluginConfig{Name: "renamed", Run: "/usr/bin/some-plugin"}, nil},
		{"external block sharing the row's name", plugin.PluginConfig{Name: impl, Run: "/opt/x"}, nil},
	} {
		if got := registryConfigRoots(tc.cfg); !slices.Equal(got, tc.want) {
			t.Errorf("%s: answered %v, want %v", tc.name, got, tc.want)
		}
	}
}

// VALIDATES: a transaction participant built from a registration with read
// roots owns only the roots it declared itself, and lists the read roots as
// WantsConfig, which the orchestrator delivers and does not count for
// operation coverage.
// PREVENTS: bgp's participant owning root bfd, so a commit that edits a peer
// and a bfd profile together aborted with ErrParticipantRootUncovered.
func TestBuildTxInputsSplitsOwnedFromReadRoots(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp"})
	reg := &plugin.PluginRegistration{WantsConfigRoots: []string{"bgp"}}
	joinConfigReads(reg, []string{"bfd"})
	proc.SetRegistration(reg)

	participants, _, _, err := buildTxInputs([]affectedPlugin{{proc: proc}}, &config.ConfigDiff{})
	if err != nil {
		t.Fatal(err)
	}
	if len(participants) != 1 {
		t.Fatalf("participants = %d, want 1", len(participants))
	}
	if got := participants[0].ConfigRoots; !slices.Equal(got, []string{"bgp"}) {
		t.Fatalf("ConfigRoots = %v, want [bgp]", got)
	}
	if got := participants[0].WantsConfig; !slices.Equal(got, []string{"bfd"}) {
		t.Fatalf("WantsConfig = %v, want [bfd]", got)
	}
}
