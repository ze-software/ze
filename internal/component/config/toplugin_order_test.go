package config

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// twoNeighboursOutOfAlphabeticalOrder is the discriminating fixture for every
// test in this file. The operator writes 10.0.0.2 first and 10.0.0.1 second, so
// a lowering that recovered the order by sorting the keys would return the
// opposite of the right answer rather than a coincidentally correct one.
//
// The nested static routes do the same at the second level: 192.0.2.0/24 is
// written before 10.0.0.0/8.
const twoNeighboursOutOfAlphabeticalOrder = `neighbor 10.0.0.2 {
 peer-as 65002;
 static {
  route 192.0.2.0/24 { next-hop 10.0.0.254; }
  route 10.0.0.0/8 { next-hop 10.0.0.253; }
 }
}
neighbor 10.0.0.1 {
 peer-as 65001;
}
`

func parseForOrder(t *testing.T, text string) *Tree {
	t.Helper()
	tree, err := NewParser(testSchema()).Parse(text)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree
}

func orderOf(t *testing.T, container map[string]any, listName string) []string {
	t.Helper()
	raw, ok := container[configorder.OrderKey(listName)]
	if !ok {
		t.Fatalf("%s carries no order; container holds %v", listName, loweredKeys(container))
	}
	order, ok := raw.([]string)
	if !ok {
		t.Fatalf("%s order is %T, want []string", listName, raw)
	}
	return order
}

func loweredKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func assertOrder(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order is %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

// keysWithReservedPrefix collects every path in a lowered map whose final
// component is a reserved order key. It walks lists and containers alike,
// because a leak at any depth is the defect.
func keysWithReservedPrefix(m map[string]any, prefix string, found *[]string) {
	for key, value := range m {
		path := AppendPath(prefix, key)
		if strings.HasPrefix(key, configorder.KeyPrefix) {
			*found = append(*found, path)
		}
		if child, ok := value.(map[string]any); ok {
			keysWithReservedPrefix(child, path, found)
		}
	}
}

// TestToPluginMapCarriesListOrder lowers a config whose two lists are both
// written out of alphabetical order, at the top level and nested inside a list
// entry.
//
// VALIDATES: the plugin-facing lowering emits the operator's order beside every
// multi-entry list, at every depth, and leaves the list itself a keyed map.
// PREVENTS: the whole defect class this file exists for. The Tree has always
// held the order in listOrder, and ToMap has never read it, so a prefix-list of
// two entries could not load and a firewall chain of two terms evaluated in Go
// map order.
func TestToPluginMapCarriesListOrder(t *testing.T) {
	lowered := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder).ToPluginMap()

	assertOrder(t, orderOf(t, lowered, "neighbor"), "10.0.0.2", "10.0.0.1")

	neighbors, ok := lowered["neighbor"].(map[string]any)
	if !ok {
		t.Fatalf("neighbor is %T, want a map keyed by the list key", lowered["neighbor"])
	}
	if len(neighbors) != 2 {
		t.Fatalf("got %d neighbors, want 2", len(neighbors))
	}

	entry, ok := neighbors["10.0.0.2"].(map[string]any)
	if !ok {
		t.Fatalf("neighbor 10.0.0.2 is %T, want a map", neighbors["10.0.0.2"])
	}
	static, ok := entry["static"].(map[string]any)
	if !ok {
		t.Fatalf("static is %T, want a container", entry["static"])
	}
	assertOrder(t, orderOf(t, static, "route"), "192.0.2.0/24", "10.0.0.0/8")
}

// TestToPluginMapLeavesSingleEntryListsAlone lowers a config holding one
// neighbor.
//
// VALIDATES: a single-entry list is delivered exactly as it is today, with no
// reserved key.
// PREVENTS: growing the payload of every plugin in the tree for a list whose
// order cannot be wrong. Most lists in a real config hold one entry.
func TestToPluginMapLeavesSingleEntryListsAlone(t *testing.T) {
	lowered := parseForOrder(t, "neighbor 10.0.0.1 {\n peer-as 65001;\n}\n").ToPluginMap()

	if _, ok := lowered[configorder.OrderKey("neighbor")]; ok {
		t.Errorf("a one-entry list carries an order key: %v", loweredKeys(lowered))
	}
	if _, ok := lowered["neighbor"].(map[string]any); !ok {
		t.Errorf("neighbor is %T, want a map keyed by the list key", lowered["neighbor"])
	}
}

// TestToMapIsUnchangedByTheOrderedLowering lowers the same config with ToMap
// and walks the result.
//
// VALIDATES: ToMap emits no reserved key at any depth, so its forty-odd
// consumers see exactly what they saw before.
// PREVENTS: the order leaking into gNMI Get and Subscribe, into
// `ze config show | json`, into the support bundle, and into
// ValidateTreeAllModules, which would refuse the whole config as an unknown
// node. That leak is why the order rides on a second lowering rather than in
// ToMap.
func TestToMapIsUnchangedByTheOrderedLowering(t *testing.T) {
	lowered := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder).ToMap()

	var found []string
	keysWithReservedPrefix(lowered, "", &found)
	if len(found) != 0 {
		t.Fatalf("ToMap leaked reserved keys: %v", found)
	}

	neighbors, ok := lowered["neighbor"].(map[string]any)
	if !ok {
		t.Fatalf("neighbor is %T, want a map keyed by the list key", lowered["neighbor"])
	}
	if len(neighbors) != 2 {
		t.Fatalf("got %d neighbors, want 2", len(neighbors))
	}
}

// TestPluginConfigSectionsCarryListOrderThroughJSON builds the config section a
// plugin receives and reads the entries back out of it.
//
// VALIDATES: the order survives ExtractConfigSubtree, json.Marshal and the
// plugin's own json.Unmarshal, and configorder.Entries returns the operator's
// order on the far side.
// PREVENTS: a lowering that is right in process and wrong on the wire. Go sorts
// object keys when it marshals a map, so the JSON text alone can never carry
// the order.
func TestPluginConfigSectionsCarryListOrderThroughJSON(t *testing.T) {
	tree := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder)

	sections, err := BuildPluginConfigSections(tree.ToPluginMap(), []string{"neighbor"})
	if err != nil {
		t.Fatalf("BuildPluginConfigSections: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}

	var delivered map[string]any
	if err := json.Unmarshal([]byte(sections[0].Data), &delivered); err != nil {
		t.Fatalf("unmarshal section: %v", err)
	}

	entries, err := configorder.Entries(delivered, "neighbor", "address")
	if err != nil {
		t.Fatalf("configorder.Entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Key != "10.0.0.2" || entries[1].Key != "10.0.0.1" {
		t.Errorf("delivered order is %q, %q, want 10.0.0.2 then 10.0.0.1", entries[0].Key, entries[1].Key)
	}
}

// TestDiffMapsSeesAListReorder lowers two configs that differ only in the order
// of one list's entries.
//
// VALIDATES: a reorder is a config change, so the reload delivers it.
// PREVENTS: an operator moving a prefix-list entry, committing, and watching
// nothing happen. Before the order was lowered the two maps were identical, so
// rootHasChanges answered no and the plugin was never reconfigured.
func TestDiffMapsSeesAListReorder(t *testing.T) {
	const swapped = `neighbor 10.0.0.1 {
 peer-as 65001;
}
neighbor 10.0.0.2 {
 peer-as 65002;
}
`
	const asWritten = `neighbor 10.0.0.2 {
 peer-as 65002;
}
neighbor 10.0.0.1 {
 peer-as 65001;
}
`

	before := parseForOrder(t, asWritten).ToPluginMap()
	after := parseForOrder(t, swapped).ToPluginMap()

	diff := DiffMaps(before, after)
	if len(diff.Changed) == 0 {
		t.Fatalf("a reorder produced no change: added %v, removed %v, changed %v", diff.Added, diff.Removed, diff.Changed)
	}

	if len(DiffMaps(parseForOrder(t, asWritten).ToMap(), parseForOrder(t, swapped).ToMap()).Changed) != 0 {
		t.Errorf("ToMap now reports a reorder, so it is carrying the order after all")
	}
}

// TestVerifyPluginConfigDeliversListOrder registers a verifier on the same root
// the configure path delivers, and compares the two payloads byte for byte.
//
// VALIDATES: the wiring row "operator config with a two-entry prefix-list,
// verified at commit". VerifyPluginConfig lowers with ToPluginMap, so a
// verifier reads the order the configure callback will read.
// PREVENTS: the two paths disagreeing. A verify that lowered with ToMap would
// hand a plugin a multi-entry list with no order, so the plugin refuses at
// commit a config the daemon then loads, or accepts one the daemon refuses.
func TestVerifyPluginConfigDeliversListOrder(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	var verified []rpc.ConfigSection
	if err := registry.Register(registry.Registration{
		Name:        "order-verify-test",
		Description: "captures the payload the verify path delivers",
		ConfigRoots: []string{"neighbor"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error {
			verified = sections
			return nil
		},
	}); err != nil {
		t.Fatalf("register verifier: %v", err)
	}

	tree := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder)
	if errs := VerifyPluginConfig(tree); len(errs) != 0 {
		t.Fatalf("verify returned %v", errs)
	}
	if len(verified) != 1 {
		t.Fatalf("verifier saw %d sections, want 1", len(verified))
	}

	configured, err := BuildPluginConfigSections(tree.ToPluginMap(), []string{"neighbor"})
	if err != nil {
		t.Fatalf("BuildPluginConfigSections: %v", err)
	}
	if len(configured) != 1 {
		t.Fatalf("configure path built %d sections, want 1", len(configured))
	}
	if verified[0].Data != configured[0].Data {
		t.Fatalf("verify delivers %s\nconfigure delivers %s", verified[0].Data, configured[0].Data)
	}

	var delivered map[string]any
	if err := json.Unmarshal([]byte(verified[0].Data), &delivered); err != nil {
		t.Fatalf("unmarshal section: %v", err)
	}
	entries, err := configorder.Entries(delivered, "neighbor", "address")
	if err != nil {
		t.Fatalf("configorder.Entries: %v", err)
	}
	if len(entries) != 2 || entries[0].Key != "10.0.0.2" || entries[1].Key != "10.0.0.1" {
		t.Fatalf("verify path delivered %d entries in the wrong order: %v", len(entries), entries)
	}
}

// TestToPluginMapEmitsNoOrderItDidNotRecord adds a list entry through the live
// map GetList hands out, which is the one way into t.lists that does not go
// through AddListEntry and so leaves listOrder short.
//
// VALIDATES: an order that does not account for every entry is not emitted at
// all, so configorder.Entries refuses the list and names it.
// PREVENTS: the lowering completing a short order by any rule of its own. A
// completed order is indistinguishable from one the operator wrote, so the
// fail-closed guarantee would end here and a first-match-wins list would be
// evaluated in an order nobody chose.
func TestToPluginMapEmitsNoOrderItDidNotRecord(t *testing.T) {
	tree := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder)
	tree.GetList("neighbor")["10.0.0.3"] = NewTree()

	lowered := tree.ToPluginMap()
	if order, ok := lowered[configorder.OrderKey("neighbor")]; ok {
		t.Fatalf("lowering emitted %v for a list whose order it does not hold", order)
	}
	if _, err := configorder.Entries(lowered, "neighbor", "address"); err == nil {
		t.Fatal("configorder.Entries accepted a three-entry list with no order")
	}

	// The nested list is untouched, so it keeps its order: a short order for one
	// list must not cost every other list its own.
	neighbors, ok := lowered["neighbor"].(map[string]any)
	if !ok {
		t.Fatalf("neighbor is %T, want a map keyed by the list key", lowered["neighbor"])
	}
	entry, ok := neighbors["10.0.0.2"].(map[string]any)
	if !ok {
		t.Fatalf("neighbor 10.0.0.2 is %T, want a map", neighbors["10.0.0.2"])
	}
	static, ok := entry["static"].(map[string]any)
	if !ok {
		t.Fatalf("static is %T, want a container", entry["static"])
	}
	assertOrder(t, orderOf(t, static, "route"), "192.0.2.0/24", "10.0.0.0/8")
}

// TestToPluginMapDoesNotResurrectADeletedListsOrder deletes a list and writes a
// new one under the same name, with its entries in the opposite order.
//
// VALIDATES: DeleteList clears the recorded order with the list, so the new
// list is lowered in the order the operator just wrote.
// PREVENTS: the deleted list's order surviving it. The stale order has the
// right length and names the right keys, so entryOrderLocked's own check
// cannot catch it: the two maps have to be cleared together.
func TestToPluginMapDoesNotResurrectADeletedListsOrder(t *testing.T) {
	tree := parseForOrder(t, twoNeighboursOutOfAlphabeticalOrder)
	assertOrder(t, orderOf(t, tree.ToPluginMap(), "neighbor"), "10.0.0.2", "10.0.0.1")

	tree.DeleteList("neighbor")
	tree.AddListEntry("neighbor", "10.0.0.1", NewTree())
	tree.AddListEntry("neighbor", "10.0.0.2", NewTree())

	assertOrder(t, orderOf(t, tree.ToPluginMap(), "neighbor"), "10.0.0.1", "10.0.0.2")
}

// TestToPluginMapFollowsAnEntryDeletedAndWrittenAgain deletes one entry of a
// list through the set-format parser, then writes it again. The operator's
// order is now the OTHER entry first, because the re-added entry was written
// last.
//
// VALIDATES: a whole-entry delete drops the key from the recorded order, so the
// key the operator wrote again is ordered where they wrote it.
// PREVENTS: the delete leaving the key in listOrder and the re-add appending it
// a second time. The duplicate order has the right length and names only keys
// the list holds, so entryOrderLocked's own checks pass it and the plugin is
// handed the DELETED entry's position with nothing said. Tree.GetListOrdered
// returns the re-added entry twice for the same reason, which is how the four
// serializers would print it twice.
func TestToPluginMapFollowsAnEntryDeletedAndWrittenAgain(t *testing.T) {
	tree, err := NewSetParser(testSchema()).Parse(
		"set neighbor 10.0.0.2 peer-as 65002\n" +
			"set neighbor 10.0.0.1 peer-as 65001\n" +
			"delete neighbor 10.0.0.2\n" +
			"set neighbor 10.0.0.2 peer-as 65002\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	assertOrder(t, orderOf(t, tree.ToPluginMap(), "neighbor"), "10.0.0.1", "10.0.0.2")

	ordered := tree.GetListOrdered("neighbor")
	if len(ordered) != 2 {
		t.Fatalf("GetListOrdered returned %d entries for a two-entry list", len(ordered))
	}
	if ordered[0].Key != "10.0.0.1" || ordered[1].Key != "10.0.0.2" {
		t.Fatalf("GetListOrdered returned %q then %q, want 10.0.0.1 then 10.0.0.2", ordered[0].Key, ordered[1].Key)
	}
}
