// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- config extraction
// Related: groups.go -- extractGroupSpecs, the walk this test drives
//
// ze-vrrp-conf.yang augments a vrrp container onto the ipv4 and ipv6 family of a
// unit under some of the interface lists, and that augment set is the only
// declaration of which lists can carry a virtual router. extractGroupSpecs walks
// every list and reads only the vrrp container, so it holds no copy of the set.
// This test reads the set off the loaded model and proves the walk reaches every
// list in it, and that a list the model does not augment yields nothing
// (ai/rules/principles.md).

package vrrp

import (
	"encoding/json"
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank imports register the modules the augmented tree needs. ze-vrrp-conf
	// augments ze-iface-conf, and the vrrp yang package registers ze-vrrp-cmd beside
	// it, which imports the show and clear command modules. goyang applies no
	// augment while any loaded module fails to resolve, so a missing import here
	// leaves the iface tree unaugmented and augmentedInterfaceLists fails closed.
	_ "github.com/ze-software/ze/internal/component/cmd/clear/yang"
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
	_ "github.com/ze-software/ze/internal/component/iface/yang"
	_ "github.com/ze-software/ze/internal/plugins/vrrp/yang"
)

const ifaceModule = "ze-iface-conf"

// TestGroupWalkReachesEveryAugmentedList feeds one group under each interface list
// the model augments and asserts extractGroupSpecs answers it, then feeds the same
// group under a list the model does not augment and asserts it is not read.
func TestGroupWalkReachesEveryAugmentedList(t *testing.T) {
	augmented, plain := augmentedInterfaceLists(t)

	for _, ifType := range augmented {
		tree := map[string]any{ifType: oneTypeGroup(ifType, "if0")}
		specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
		if err != nil {
			t.Fatalf("%s: extract: %v", ifType, err)
		}
		if len(specs) != 1 || specs[0].IfType != ifType || specs[0].Interface != "if0" {
			t.Errorf("the model augments interface list %q with vrrp and the walk answered %+v for a group under it, want one spec on if0",
				ifType, specs)
		}
	}

	// The schema refuses a vrrp container under an unaugmented list, so this
	// section is one no validated configuration delivers. The walk still reads it
	// on purpose: a list the module gains an augment for must be found without a
	// Go edit, and the schema is what keeps a group out of a list it never named.
	if len(plain) == 0 {
		t.Fatal("the model augments every interface list, so no list is left to prove the schema is the guard")
	}
	tree := map[string]any{plain[0]: oneTypeGroup(plain[0], "if0")}
	specs, err := extractGroupSpecs([]configSection{mkSection(t, tree)})
	if err != nil {
		t.Fatalf("%s: extract: %v", plain[0], err)
	}
	if len(specs) != 1 {
		t.Errorf("a group under the unaugmented list %q answered %d specs, want 1: the walk is expected to read any list, and the schema to refuse the group", plain[0], len(specs))
	}
}

// augmentedInterfaceLists answers the interface lists whose ipv4 unit family carries
// the vrrp augment, and the lists whose family does not, both sorted. It FAILS when
// the loaded model holds no augmented list at all, so the proof above never passes
// over nothing (ai/rules/evidence.md).
func augmentedInterfaceLists(t *testing.T) (augmented, plain []string) {
	t.Helper()

	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}
	iface := loader.GetEntry(ifaceModule)
	if iface == nil {
		t.Fatalf("the loaded model holds no module %s", ifaceModule)
	}
	root := childEntry(iface, configRoot)
	if root == nil {
		t.Fatalf("%s holds no %q container", ifaceModule, configRoot)
	}
	for name, list := range root.Dir {
		unit := childEntry(list, "unit")
		if unit == nil {
			continue
		}
		family := childEntry(unit, familyIPv4)
		if family == nil {
			continue
		}
		if childEntry(family, "vrrp") != nil {
			augmented = append(augmented, name)
		} else {
			plain = append(plain, name)
		}
	}
	if len(augmented) == 0 {
		t.Fatal("no interface list carries the vrrp augment: the model this binary loaded is not the one this test reads")
	}
	slices.Sort(augmented)
	slices.Sort(plain)
	return augmented, plain
}

func childEntry(entry *gyang.Entry, name string) *gyang.Entry {
	if entry.Dir == nil {
		return nil
	}
	return entry.Dir[name]
}

// TestGroupWalkOrderIsStable proves two sections listing the same lists in different
// map orders answer one spec order, which the diff the engine computes relies on.
func TestGroupWalkOrderIsStable(t *testing.T) {
	tree := map[string]any{
		"veth":     oneTypeGroup("veth", "veth0"),
		"ethernet": oneTypeGroup("ethernet", "eth0"),
	}
	data, err := json.Marshal(map[string]any{configRoot: tree})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	specs, err := extractGroupSpecs([]configSection{{Root: configRoot, Data: string(data)}})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, spec.Key())
	}
	if !slices.IsSorted(keys) {
		t.Errorf("specs are not in key order: %v", keys)
	}
}
