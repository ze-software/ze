// Design: docs/architecture/firewall/firewall-irr.md -- tests for firewall-irr's registered interface-binding shape builder
// Related: irr.go -- buildIfaceBindingShapeTables
// Related: register.go -- registers firewall-irr config table builder

package irr

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
)

// VALIDATES: AC-8 -- firewall-irr registers its config table builder
// "firewall-irr" from init(); for a candidate tree holding a firewall irr
// interface binding whose AS-SET has nothing stored in any IRR store, the
// builder still returns ze_irr_iface with its ingress chain, named set,
// MatchInputInterface and MatchInSet terms so a backend verifier (such as VPP)
// refuses the shape at commit/validate/boot, while buildIfaceTables on the
// apply path continues to emit nothing when no prefixes are stored.
// PREVENTS: an IRR interface binding slipping past VPP commit verification on
// a cold cache only to fail later at ApplyAll once prefixes arrive.
func TestIRRInterfaceBindingBuilderAnswersTheShape(t *testing.T) {
	builder, ok := firewall.ConfigTableBuilderFor(pluginName)
	if !ok {
		t.Fatalf("%s did not register its config table builder from init()", pluginName)
	}
	if err := firewall.RegisterConfigTableBuilder(pluginName, buildIfaceBindingShapeTables); err == nil {
		t.Fatalf("expected duplicate registration error for %q config table builder", pluginName)
	}

	t.Run("no bindings yields no tables", func(t *testing.T) {
		tree := config.NewTree()
		tree.GetOrCreateContainer(configRoot).Set("backend", "vpp")

		tables, err := builder(tree)
		if err != nil {
			t.Fatalf("builder without bindings: %v", err)
		}
		if len(tables) != 0 {
			t.Fatalf("expected 0 tables without bindings, got %d", len(tables))
		}
	})

	t.Run("binding with nothing stored yields ingress chain and named set shape", func(t *testing.T) {
		tree := config.NewTree()
		fwTree := tree.GetOrCreateContainer(configRoot)
		fwTree.Set("backend", "vpp")
		irrTree := fwTree.GetOrCreateContainer("irr")
		entry := config.NewTree()
		entry.Set("name", "eth1")
		entry.Set("source-as-set", "AS-CUSTOMER")
		irrTree.AddListEntry("interface", "eth1", entry)

		tables, err := builder(tree)
		if err != nil {
			t.Fatalf("builder with interface binding: %v", err)
		}
		if len(tables) != 1 {
			t.Fatalf("expected 1 shape table, got %d", len(tables))
		}

		tbl := tables[0]
		if tbl.Name != ifaceTableName || tbl.Family != firewall.FamilyInet {
			t.Fatalf("table = (%q, %v), want (%q, %v)", tbl.Name, tbl.Family, ifaceTableName, firewall.FamilyInet)
		}
		wantSetName, _ := setNames("AS-CUSTOMER")
		if len(tbl.Sets) != 1 || tbl.Sets[0].Name != wantSetName {
			t.Fatalf("sets = %+v, want 1 set named %q", tbl.Sets, wantSetName)
		}
		if len(tbl.Chains) != 1 || tbl.Chains[0].Name != ifaceChainName || !tbl.Chains[0].IsBase {
			t.Fatalf("chains = %+v, want 1 base chain named %q", tbl.Chains, ifaceChainName)
		}

		hasInSet := false
		hasInputIface := false
		for _, term := range tbl.Chains[0].Terms {
			for _, m := range term.Matches {
				if ms, ok := m.(firewall.MatchInSet); ok && ms.SetName == wantSetName {
					hasInSet = true
				}
				if mi, ok := m.(firewall.MatchInputInterface); ok && mi.Name == "eth1" {
					hasInputIface = true
				}
			}
		}
		if !hasInSet {
			t.Errorf("shape table chain %q carries no MatchInSet for %q", ifaceChainName, wantSetName)
		}
		if !hasInputIface {
			t.Errorf("shape table chain %q carries no MatchInputInterface for eth1", ifaceChainName)
		}

		// Confirm apply-path buildIfaceTables is unchanged and still emits nothing
		// for an empty prefix list so an interface is never blackholed.
		emptyLists := enforcedLists{}
		applyTables := buildIfaceTables(emptyLists, []ifaceBinding{{Interface: "eth1", ASSet: "AS-CUSTOMER"}})
		if len(applyTables) != 0 {
			t.Fatalf("apply-path buildIfaceTables with empty cache emitted %d tables, want 0", len(applyTables))
		}
	})
}
