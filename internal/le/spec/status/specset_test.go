package specstatus

import (
	"reflect"
	"strings"
	"testing"

	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// TestOneGroupingFunctionAnswersEveryReader pins the naming rule GroupSets
// reads, and that the inventory's Set column and the closure detector's
// umbrella verdict take it from the same answer.
//
// Method: one plan tree holds every shape the rule decides. Collect answers the
// Set column, the closure population answers IsUmbrella, and both are compared
// with the expected sets and with GroupSets over the same inventory.
//
// VALIDATES: a prefix holding a digit (bcp194) is read; a lone numbered name
// (flow-export-3, knowledge-0-umbrella-deferred, iface-qinq-802) is no set; a
// token such as `1b` is no number, so rfcgate-1b stays out of rfcgate; an umbrella with no
// number (lifecycle) is the umbrella of its numbered children; a named
// `-0-umbrella` wins over a number-less file of the same prefix (vrf); two
// numbered children with no umbrella are a set; an umbrella alone is a set.
// PREVENTS: the two detectors this replaced disagreeing again. The file-name
// pattern took any numbered name for a set and could not read a digit in a
// prefix; the sibling test took spec-vrf.md, a child, for an umbrella.
func TestOneGroupingFunctionAnswersEveryReader(t *testing.T) {
	depends := strings.Replace(templateShapedSpec("ready", "2026-08-20"),
		"| Depends | - |", "| Depends | spec-bcp194-0-umbrella.md |", 1)
	root := planTree(t, map[string]string{
		"spec-bcp194-0-umbrella.md":                  templateShapedSpec("design", "2026-08-20"),
		"spec-bcp194-2-role-policy.md":               depends,
		"spec-bcp194-community-function4.md":         templateShapedSpec("ready", "2026-08-20"),
		"spec-flow-export-3-sampled-scale.md":        templateShapedSpec("ready", "2026-08-20"),
		"spec-knowledge-0-umbrella-deferred-x.md":    templateShapedSpec("in-progress", "2026-08-20"),
		"spec-lifecycle.md":                          templateShapedSpec("in-progress", "2026-08-20"),
		"spec-lifecycle-1-plugin-process.md":         templateShapedSpec("ready", "2026-08-20"),
		"spec-lifecycle-3-bgp-plugins.md":            templateShapedSpec("skeleton", "2026-08-20"),
		"spec-vrf.md":                                templateShapedSpec("in-progress", "2026-08-20"),
		"spec-vrf-0-umbrella.md":                     templateShapedSpec("design", "2026-08-20"),
		"spec-wire-edit-2-ci.md":                     templateShapedSpec("ready", "2026-08-20"),
		"spec-wire-edit-5-fanout.md":                 templateShapedSpec("ready", "2026-08-20"),
		"spec-perf-next-0-umbrella.md":               templateShapedSpec("blocked", "2026-08-20"),
		"spec-rfcgate-1b-x.md":                       templateShapedSpec("blocked", "2026-08-20"),
		"spec-rfcgate-2-a.md":                        templateShapedSpec("ready", "2026-08-20"),
		"spec-rfcgate-5-b.md":                        templateShapedSpec("ready", "2026-08-20"),
		"spec-iface-qinq-802-1ad.md":                 templateShapedSpec("ready", "2026-08-20"),
		"spec-standalone-with-no-number-anywhere.md": templateShapedSpec("ready", "2026-08-20"),
	})
	wantSet := map[string]string{
		"bcp194-0-umbrella":               "bcp194",
		"bcp194-2-role-policy":            "bcp194",
		"bcp194-community-function4":      NoSet,
		"flow-export-3-sampled-scale":     NoSet,
		"knowledge-0-umbrella-deferred-x": NoSet,
		"lifecycle":                       "lifecycle",
		"lifecycle-1-plugin-process":      "lifecycle",
		"lifecycle-3-bgp-plugins":         "lifecycle",
		"vrf":                             NoSet,
		"vrf-0-umbrella":                  "vrf",
		"wire-edit-2-ci":                  "wire-edit",
		"wire-edit-5-fanout":              "wire-edit",
		"perf-next-0-umbrella":            "perf-next",
		// `1b` is not a number, so this file stays out of the rfcgate set its
		// two numbered siblings form.
		"rfcgate-1b-x": NoSet,
		"rfcgate-2-a":  "rfcgate",
		"rfcgate-5-b":  "rfcgate",
		// 802 is a digit token, so the stem reads as child 802 of iface-qinq,
		// a lone numbered name and therefore no set.
		"iface-qinq-802-1ad":                 NoSet,
		"standalone-with-no-number-anywhere": NoSet,
	}
	wantUmbrella := map[string]bool{
		"bcp194-0-umbrella": true, "lifecycle": true, "vrf-0-umbrella": true, "perf-next-0-umbrella": true,
	}

	inventory, _ := collect(t, root, fixedNow)
	if len(inventory) != len(wantSet) {
		t.Fatalf("inventory = %d specs, want %d", len(inventory), len(wantSet))
	}
	for _, spec := range inventory {
		if spec.Set != wantSet[spec.Name] {
			t.Errorf("the Set column for %s = %q, want %q", spec.Name, spec.Set, wantSet[spec.Name])
		}
	}

	relatives, err := specpath.All(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := inspectClosurePopulation(root, relatives, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != len(wantSet) {
		t.Fatalf("closure population = %d specs, want %d", len(report), len(wantSet))
	}
	sets := GroupSets(inventory)
	for _, one := range report {
		if one.IsUmbrella != wantUmbrella[one.Stem] {
			t.Errorf("the closure detector's umbrella verdict for %s = %v, want %v", one.Stem, one.IsUmbrella, wantUmbrella[one.Stem])
		}
		if one.IsUmbrella != sets.IsUmbrella(one.Spec) {
			t.Errorf("%s: the closure detector says umbrella %v, the inventory's grouping says %v", one.Stem, one.IsUmbrella, sets.IsUmbrella(one.Spec))
		}
		if one.IsUmbrella {
			if wantSet[one.Stem] == NoSet {
				t.Errorf("%s is an umbrella to the closure detector and in no set in the Set column", one.Stem)
			}
		}
	}

	prefixes := make([]string, 0, len(sets))
	for _, set := range sets {
		prefixes = append(prefixes, set.Prefix)
	}
	if want := []string{"bcp194", "lifecycle", "perf-next", "rfcgate", "vrf", "wire-edit"}; !reflect.DeepEqual(prefixes, want) {
		t.Fatalf("sets = %v, want %v", prefixes, want)
	}
	rfcgate := memberSet(t, sets, "spec-rfcgate-2-a.md")
	rfcgateChildren := make([]string, 0, len(rfcgate.Open))
	for _, child := range rfcgate.Open {
		rfcgateChildren = append(rfcgateChildren, child.Path)
	}
	if want := []string{fixturePath("spec-rfcgate-2-a.md"), fixturePath("spec-rfcgate-5-b.md")}; !reflect.DeepEqual(rfcgateChildren, want) {
		t.Errorf("rfcgate open children = %v, want %v: 1b is not a number", rfcgateChildren, want)
	}
	bcp194 := memberSet(t, sets, "spec-bcp194-2-role-policy.md")
	if bcp194.Umbrella != fixturePath("spec-bcp194-0-umbrella.md") {
		t.Errorf("bcp194 umbrella = %q", bcp194.Umbrella)
	}
	wantChildren := []SetChild{{Number: 2, Path: fixturePath("spec-bcp194-2-role-policy.md"), Status: "ready", Depends: "spec-bcp194-0-umbrella.md"}}
	if !reflect.DeepEqual(bcp194.Open, wantChildren) {
		t.Errorf("bcp194 open children = %+v, want %+v", bcp194.Open, wantChildren)
	}
	lifecycle := memberSet(t, sets, "spec-lifecycle.md")
	numbers := make([]int, 0, len(lifecycle.Open))
	for _, child := range lifecycle.Open {
		numbers = append(numbers, child.Number)
	}
	if !reflect.DeepEqual(numbers, []int{1, 3}) {
		t.Errorf("lifecycle open children = %+v, want 1 and 3", lifecycle.Open)
	}
	if wireEdit := memberSet(t, sets, "spec-wire-edit-2-ci.md"); wireEdit.HasUmbrella() {
		t.Errorf("wire-edit has umbrella %q, want none", wireEdit.Umbrella)
	}
	if perfNext := memberSet(t, sets, "spec-perf-next-0-umbrella.md"); len(perfNext.Open) != 0 {
		t.Errorf("perf-next open children = %+v, want none", perfNext.Open)
	}
}

// fixturePath answers the inventory path planTree gives a bare file name.
func fixturePath(name string) string { return "plan/release/fixes/" + name }

// memberSet answers the set holding the fixture spec name, and fails the test
// when no set holds it.
func memberSet(t *testing.T, sets SpecSets, name string) SpecSet {
	t.Helper()
	set, member := sets.Of(fixturePath(name))
	if !member {
		t.Fatalf("%s is in no set", name)
	}
	return set
}
