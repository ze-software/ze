// Design: docs/architecture/plugin/plugin-system.md -- registration fields, config needs
// Related: optional_provider.go -- refuseAbsentOptionalProviders, the producer these tests drive

package config

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// isolateRegistry empties the plugin registry for one test and restores it
// after, so a test-only registration is the only one the check sees.
func isolateRegistry(t *testing.T) {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })
}

// registerNeedOwner registers a test-only plugin declaring one config need on
// path with value, met by any of providers.
func registerNeedOwner(t *testing.T, path []string, value string, providers ...string) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:                 "need-owner",
		Description:          "test owner of a config need",
		RunEngine:            func(net.Conn) int { return 0 },
		CLIHandler:           func([]string) int { return 0 },
		OptionalDependencies: providers,
		ConfigNeeds:          []registry.ConfigNeed{{Path: path, Value: value, Providers: providers}},
	})
	if err != nil {
		t.Fatalf("register need owner: %v", err)
	}
}

// registerProvider registers a test-only plugin under a provider's name, the
// way a build that compiled the provider in would.
func registerProvider(t *testing.T, name string) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "test provider",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register provider %s: %v", name, err)
	}
}

func configOf(tree map[string]any) func() map[string]any {
	return func() map[string]any { return tree }
}

// fixLine answers the line of message that starts with "  fix: ", so a test
// proves the fix sits on a line of its own.
func fixLine(t *testing.T, message string) string {
	t.Helper()
	for line := range strings.SplitSeq(message, "\n") {
		if strings.HasPrefix(line, "  fix: ") {
			return line
		}
	}
	t.Fatalf("no line starts with %q in:\n%s", "  fix: ", message)
	return ""
}

var enrichmentPath = []string{"flow-export", "enrichment", "bgp"}

func enrichmentConfig(value any) map[string]any {
	return map[string]any{"flow-export": map[string]any{"enrichment": map[string]any{"bgp": value}}}
}

// TestRefuseAbsentProviderNamesTagAndLeaf proves the AC-1 message: a held need
// whose single gated provider is absent is refused, naming the leaf as the
// operator types it, the provider, and the build tag on a fix line of its own.
// It also proves the need does not hold for an absent, other-valued or wrongly
// typed node, and that a registered provider meets it.
//
// VALIDATES: refuseAbsentOptionalProviders, appendHeldSettings, leafCarries, unmetNeedMessage.
// PREVENTS: a setting whose provider is compiled out passing with no tag named.
func TestRefuseAbsentProviderNamesTagAndLeaf(t *testing.T) {
	isolateRegistry(t)
	registerNeedOwner(t, enrichmentPath, "true", "bgp-rib")

	err := refuseAbsentOptionalProviders(configOf(enrichmentConfig("true")))
	if !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("held need with bgp-rib absent: err = %v, want ErrAbsentOptionalProvider", err)
	}
	message := err.Error()
	firstLine, _, _ := strings.Cut(message, "\n")
	wantProblem := `config: "flow-export enrichment bgp true" needs plugin bgp-rib, and this build does not include it`
	if firstLine != wantProblem {
		t.Fatalf("problem line = %q, want %q", firstLine, wantProblem)
	}
	if !strings.Contains(message, "need-owner cannot act on this setting without it") {
		t.Fatalf("message does not say why, naming the owner:\n%s", message)
	}
	wantFix := `  fix: rebuild ze with build tag ze_bgp added to its -tags list, or remove "flow-export enrichment bgp true" from the config`
	if got := fixLine(t, message); got != wantFix {
		t.Fatalf("fix line = %q, want %q", got, wantFix)
	}

	notHeld := map[string]map[string]any{
		"leaf false":            enrichmentConfig("false"),
		"leaf absent":           {"flow-export": map[string]any{"enrichment": map[string]any{}}},
		"no flow-export root":   {},
		"container where leaf":  enrichmentConfig(map[string]any{"true": "true"}),
		"leaf where container":  {"flow-export": "true"},
		"non-string leaf value": enrichmentConfig(true),
	}
	for name, tree := range notHeld {
		if err := refuseAbsentOptionalProviders(configOf(tree)); err != nil {
			t.Errorf("%s: the need does not hold, want nil, got %v", name, err)
		}
	}

	registerProvider(t, "bgp-rib")
	if err := refuseAbsentOptionalProviders(configOf(enrichmentConfig("true"))); err != nil {
		t.Fatalf("bgp-rib registered: the build meets the need, want nil, got %v", err)
	}
}

// TestRefuseAbsentProviderAnyOf proves AC-10 and AC-11 at unit level: an any-of
// need with no provider in the build names every provider as an alternative
// with its tag, and one provider registered, even unconfigured, meets it (R-8).
//
// VALIDATES: refusalOfUnmetNeed judges the build, never the config of a provider.
// PREVENTS: refusing detection because a compiled-in flow source is unconfigured.
func TestRefuseAbsentProviderAnyOf(t *testing.T) {
	path := []string{"ddos", "detection", "enable"}
	tree := map[string]any{"ddos": map[string]any{"detection": map[string]any{"enable": "true"}}}

	t.Run("neither source in the build", func(t *testing.T) {
		isolateRegistry(t)
		registerNeedOwner(t, path, "true", "flow-export", "traffic-usage")

		err := refuseAbsentOptionalProviders(configOf(tree))
		if !errors.Is(err, ErrAbsentOptionalProvider) {
			t.Fatalf("err = %v, want ErrAbsentOptionalProvider", err)
		}
		message := err.Error()
		if !strings.Contains(message, `needs one of the plugins flow-export or traffic-usage, and this build includes none of them`) {
			t.Fatalf("message does not name both providers as alternatives:\n%s", message)
		}
		wantFix := `  fix: rebuild ze with build tag ze_flowexport, or with build tag ze_trafficusage added to its -tags list, or remove "ddos detection enable true" from the config`
		if got := fixLine(t, message); got != wantFix {
			t.Fatalf("fix line = %q, want %q", got, wantFix)
		}
	})

	for _, present := range []string{"flow-export", "traffic-usage"} {
		t.Run(present+" in the build", func(t *testing.T) {
			isolateRegistry(t)
			registerNeedOwner(t, path, "true", "flow-export", "traffic-usage")
			registerProvider(t, present)

			// The config never configures the provider: the verdict is about
			// the build only.
			if err := refuseAbsentOptionalProviders(configOf(tree)); err != nil {
				t.Fatalf("%s registered: want nil, got %v", present, err)
			}
		})
	}
}

// TestRefuseAbsentProviderCompoundTag proves AC-19: a provider under a nested
// gate names every tag it needs, joined with "and".
//
// VALIDATES: writeBuildTags over a GatedPluginTags compound row.
// PREVENTS: a fix line naming ze_bmp alone for a plugin that also needs ze_bgp.
func TestRefuseAbsentProviderCompoundTag(t *testing.T) {
	isolateRegistry(t)
	path := []string{"monitoring", "bmp", "export"}
	registerNeedOwner(t, path, "true", "bgp-bmp")

	tree := map[string]any{"monitoring": map[string]any{"bmp": map[string]any{"export": "true"}}}
	err := refuseAbsentOptionalProviders(configOf(tree))
	if !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("err = %v, want ErrAbsentOptionalProvider", err)
	}
	if got := fixLine(t, err.Error()); !strings.Contains(got, "rebuild ze with build tags ze_bgp and ze_bmp added to its -tags list") {
		t.Fatalf("fix line does not name the compound requirement: %q", got)
	}
}

// TestRefuseAbsentProviderUnknownNamesNoTag proves a provider that no gate
// names and this build does not register is called unknown to every build,
// and that the fix offers no guessed tag.
func TestRefuseAbsentProviderUnknownNamesNoTag(t *testing.T) {
	isolateRegistry(t)
	registerNeedOwner(t, enrichmentPath, "true", "no-such-plugin")

	err := refuseAbsentOptionalProviders(configOf(enrichmentConfig("true")))
	if !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("err = %v, want ErrAbsentOptionalProvider", err)
	}
	message := err.Error()
	if !strings.Contains(message, "plugin no-such-plugin is not part of any ze build, so no build tag adds it") {
		t.Fatalf("message does not call the provider unknown:\n%s", message)
	}
	wantFix := `  fix: remove "flow-export enrichment bgp true" from the config`
	if got := fixLine(t, message); got != wantFix {
		t.Fatalf("fix line = %q, want %q", got, wantFix)
	}
}

// TestRefuseAbsentProviderLeafListMember proves a need on a leaf-list holds
// when the value is one of its members, and not when it is absent from them.
func TestRefuseAbsentProviderLeafListMember(t *testing.T) {
	isolateRegistry(t)
	registerNeedOwner(t, enrichmentPath, "true", "bgp-rib")

	if err := refuseAbsentOptionalProviders(configOf(enrichmentConfig([]string{"x", "true"}))); !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("value among leaf-list members: err = %v, want ErrAbsentOptionalProvider", err)
	}
	if err := refuseAbsentOptionalProviders(configOf(enrichmentConfig([]string{"x", "y"}))); err != nil {
		t.Fatalf("value not among leaf-list members: want nil, got %v", err)
	}
}

// TestRefuseAbsentProviderLowersOnlyForDeclaredNeeds proves the config is not
// lowered when no registration declares a need, and that a declared need with
// no config to judge refuses rather than passes (fail closed).
func TestRefuseAbsentProviderLowersOnlyForDeclaredNeeds(t *testing.T) {
	isolateRegistry(t)
	registerProvider(t, "no-needs")

	lowered := 0
	countingLower := func() map[string]any {
		lowered++
		return map[string]any{}
	}
	if err := refuseAbsentOptionalProviders(countingLower); err != nil {
		t.Fatalf("no declared need: want nil, got %v", err)
	}
	if lowered != 0 {
		t.Fatalf("no declared need: config lowered %d times, want 0", lowered)
	}

	registerNeedOwner(t, enrichmentPath, "true", "bgp-rib")
	if err := refuseAbsentOptionalProviders(configOf(nil)); !errors.Is(err, ErrConfigNeedUnevaluated) {
		t.Fatalf("declared need, no config: err = %v, want ErrConfigNeedUnevaluated", err)
	}
}

var domainSourcePath = []string{"bgp-ls-export", "domain", registry.AnyEntry, "source"}

// domainTree builds a `bgp-ls-export` root whose `domain` list holds one entry
// per name, each naming its source, and lowers it the way a plugin receives
// it, so the list shape under test is the one Tree.ToPluginMap emits, entry
// order key included once two entries exist.
func domainTree(sources map[string]string) map[string]any {
	tree := NewTree()
	root := tree.GetOrCreateContainer("bgp-ls-export")
	for name, source := range sources {
		entry := NewTree()
		entry.Set("source", source)
		root.AddListEntry("domain", name, entry)
	}
	return tree.ToPluginMap()
}

// TestRefuseAbsentProviderAnyListEntry proves AC-13 at unit level: a need
// whose path crosses a list holds when any entry carries the value at the rest
// of the path, and the refusal names the entry the operator wrote. Method:
// lower real Trees with zero, one and several matching `domain` entries, and
// malformed lowered shapes that must not hold.
//
// VALIDATES: appendHeldSettings crosses registry.AnyEntry over every entry,
// and refuseAbsentOptionalProviders refuses once per holding entry.
// PREVENTS: a BGP-LS export source naming a compiled-out IGP being accepted
// because its leaf sits inside a list.
func TestRefuseAbsentProviderAnyListEntry(t *testing.T) {
	isolateRegistry(t)
	registerNeedOwner(t, domainSourcePath, "isis", "isis")

	err := refuseAbsentOptionalProviders(configOf(domainTree(map[string]string{"core": "isis"})))
	if !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("one entry names isis: err = %v, want ErrAbsentOptionalProvider", err)
	}
	firstLine, _, _ := strings.Cut(err.Error(), "\n")
	wantProblem := `config: "bgp-ls-export domain core source isis" needs plugin isis, and this build does not include it`
	if firstLine != wantProblem {
		t.Fatalf("problem line = %q, want %q", firstLine, wantProblem)
	}

	err = refuseAbsentOptionalProviders(configOf(domainTree(map[string]string{"edge": "bgp-epe", "core": "isis", "west": "isis"})))
	if !errors.Is(err, ErrAbsentOptionalProvider) {
		t.Fatalf("two of three entries name isis: err = %v, want ErrAbsentOptionalProvider", err)
	}
	for _, entry := range []string{"core", "west"} {
		if !strings.Contains(err.Error(), `"bgp-ls-export domain `+entry+` source isis"`) {
			t.Errorf("refusal does not name entry %q:\n%s", entry, err)
		}
	}
	if strings.Contains(err.Error(), "edge") {
		t.Errorf("refusal names the entry that does not hold the need:\n%s", err)
	}

	notHeld := map[string]map[string]any{
		"no entry names isis":    domainTree(map[string]string{"edge": "bgp-epe", "west": "ospf"}),
		"no domain list":         domainTree(nil),
		"no export root":         {},
		"list is a leaf":         {"bgp-ls-export": map[string]any{"domain": "isis"}},
		"entry is a leaf":        {"bgp-ls-export": map[string]any{"domain": map[string]any{"core": "isis"}}},
		"source is a container":  {"bgp-ls-export": map[string]any{"domain": map[string]any{"core": map[string]any{"source": map[string]any{"isis": "isis"}}}}},
		"source not a string":    {"bgp-ls-export": map[string]any{"domain": map[string]any{"core": map[string]any{"source": 7}}}},
		"source beside the list": {"bgp-ls-export": map[string]any{"domain": map[string]any{"source": "isis"}}},
	}
	for name, tree := range notHeld {
		if err := refuseAbsentOptionalProviders(configOf(tree)); err != nil {
			t.Errorf("%s: the need does not hold, want nil, got %v", name, err)
		}
		if ConfigNeedHolds(tree)("need-owner", registry.ConfigNeed{Path: domainSourcePath, Value: "isis", Providers: []string{"isis"}}) {
			t.Errorf("%s: ConfigNeedHolds answered true", name)
		}
	}

	holds := ConfigNeedHolds(domainTree(map[string]string{"edge": "bgp-epe", "core": "isis"}))
	if !holds("need-owner", registry.ConfigNeed{Path: domainSourcePath, Value: "isis", Providers: []string{"isis"}}) {
		t.Error("ConfigNeedHolds: one of two entries names isis, want true, so the provider is pulled")
	}

	registerProvider(t, "isis")
	if err := refuseAbsentOptionalProviders(configOf(domainTree(map[string]string{"core": "isis"}))); err != nil {
		t.Fatalf("isis registered: the build meets the need, want nil, got %v", err)
	}
}
