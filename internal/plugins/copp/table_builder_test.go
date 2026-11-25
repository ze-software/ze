// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- tests for CoPP's registered firewall config table builder
// Related: register_table_builder.go -- registers buildCoppTablesFromTree

package copp

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
)

// VALIDATES: AC-8 -- copp registers its config table builder "copp" with the
// firewall component from init(), and the registered builder returns the exact
// firewall.Table that translatePolicy / applyCoppPolicy installs for the same
// candidate configuration (and returns nil when control-plane-protection is
// absent).
// PREVENTS: copp's commit-time table shape drifting from applyCoppPolicy, or
// firewall-producer-tables missing copp at boot/commit/validate.
func TestCoppRegistersItsTableBuilder(t *testing.T) {
	builder, ok := firewall.ConfigTableBuilderFor("copp")
	if !ok {
		t.Fatal("copp did not register its config table builder from init()")
	}
	if err := firewall.RegisterConfigTableBuilder("copp", buildCoppTablesFromTree); err == nil {
		t.Fatal("expected duplicate registration error for \"copp\" config table builder")
	}

	t.Run("absent control-plane-protection returns no tables", func(t *testing.T) {
		tree := config.NewTree()
		tree.GetOrCreateContainer("bgp").AddListEntry("peer", "edge", peerTree("192.0.2.1"))

		tables, err := builder(tree)
		if err != nil {
			t.Fatalf("builder on tree without copp: %v", err)
		}
		if len(tables) != 0 {
			t.Fatalf("expected 0 tables when copp is absent, got %d", len(tables))
		}
	})

	t.Run("candidate tree produces exact table translatePolicy installs", func(t *testing.T) {
		tree := config.NewTree()
		cppBGP := tree.GetOrCreateContainer(configRoot).GetOrCreateContainer("bgp")
		cppBGP.Set("rate", "100/second")
		cppBGP.Set("burst", "20")
		cppBGP.Set("over-limit-policy", "drop")
		tree.GetOrCreateContainer(bgpConfigRoot).AddListEntry("peer", "edge", peerTree("192.0.2.1"))

		got, err := builder(tree)
		if err != nil {
			t.Fatalf("builder on candidate tree: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("builder returned %d tables, want 1", len(got))
		}

		// Parse the exact same sections OnConfigure/OnConfigVerify receive and compare.
		sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRoot, bgpConfigRoot})
		if err != nil {
			t.Fatalf("BuildPluginConfigSections: %v", err)
		}
		var coppData string
		for _, s := range sections {
			if s.Root == configRoot {
				coppData = s.Data
			}
		}
		policy, found, err := parseCoppSections(sections, coppData)
		if err != nil || !found {
			t.Fatalf("parseCoppSections: found=%v err=%v", found, err)
		}
		want := translatePolicy(policy)
		if !reflect.DeepEqual(got[0], want) {
			t.Fatalf("builder table differs from translatePolicy:\ngot:  %+v\nwant: %+v", got[0], want)
		}
	})
}
