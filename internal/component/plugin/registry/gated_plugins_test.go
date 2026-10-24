// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin this build was compiled without
package registry

import (
	"slices"
	"testing"
)

// VALIDATES: the compiled table answers a gated plugin with its tags, nested
// gates included (AC-19), and answers a name no gate covers with no tag at all,
// so a caller can never print a guessed tag for it (AC-23).
//
// Method: the registry test binary links no plugin, so the answers come from
// the generated table alone, which is the situation of a binary built without
// the plugin. bgp-bmp is nested under ze_bgp; the other two names are a typo and
// an external plugin's name, which no gate covers.
func TestGatedPluginTagsDistinguishesGatedFromUnknown(t *testing.T) {
	tags, gated := GatedPluginTags("bgp-bmp")
	if !gated {
		t.Fatal("bgp-bmp answered as ungated")
	}
	if want := []string{"ze_bgp", "ze_bmp"}; !slices.Equal(tags, want) {
		t.Errorf("bgp-bmp tags = %v, want %v", tags, want)
	}

	for _, name := range []string{"bgp-bpm", "my-external-plugin"} {
		tags, gated := GatedPluginTags(name)
		if gated {
			t.Errorf("%s answered as gated", name)
		}
		if tags != nil {
			t.Errorf("%s answered tags %v, want none", name, tags)
		}
	}
}

// VALIDATES: the answer is the caller's own copy, so a caller that edits it
// cannot change what the next caller is told.
func TestGatedPluginTagsAnswersACopy(t *testing.T) {
	first, _ := GatedPluginTags("bgp-bmp")
	first[0] = "ze_edited"

	second, _ := GatedPluginTags("bgp-bmp")
	if second[0] != "ze_bgp" {
		t.Errorf("a caller's edit reached the table: second answer starts with %q", second[0])
	}
}

// TestGatedConfigRootRequirementsNamesTheSmallestTags proves the root lookup
// the parser uses for a compiled-out root. Method: `bgp` is owned by plugins
// needing ze_bgp and by bgp-bmp needing ze_bgp and ze_bmp, so the answer must be
// the single smallest set; `mrt` is owned only by mrt under ze_bgp; a typo is
// not gated and names nothing.
func TestGatedConfigRootRequirementsNamesTheSmallestTags(t *testing.T) {
	for _, root := range []string{"bgp", "mrt"} {
		requirements, gated := GatedConfigRootRequirements(root)
		if !gated {
			t.Fatalf("%s answered as ungated", root)
		}
		if len(requirements) != 1 {
			t.Fatalf("%s requirements = %v, want exactly one set", root, requirements)
		}
		if !slices.Equal(requirements[0], []string{"ze_bgp"}) {
			t.Errorf("%s requirements = %v, want [[ze_bgp]]", root, requirements)
		}
	}
	if requirements, _ := GatedConfigRootRequirements("exabgp"); len(requirements) != 1 || !slices.Equal(requirements[0], []string{"ze_exabgp"}) {
		t.Errorf("exabgp requirements = %v, want [[ze_exabgp]]", requirements)
	}
	if requirements, gated := GatedConfigRootRequirements("bgpp"); gated || requirements != nil {
		t.Errorf("typo bgpp answered gated=%v requirements=%v, want ungated and none", gated, requirements)
	}
}

// TestAppendSmallestRequirementKeepsOnlyMinimalSets proves the reduction in
// both orders, so the answer does not depend on which owner the table lists
// first.
func TestAppendSmallestRequirementKeepsOnlyMinimalSets(t *testing.T) {
	small, large := []string{"ze_bgp"}, []string{"ze_bgp", "ze_bmp"}
	for _, order := range [][][]string{{small, large}, {large, small}} {
		var got [][]string
		for _, tags := range order {
			got = appendSmallestRequirement(got, tags)
		}
		if len(got) != 1 || !slices.Equal(got[0], small) {
			t.Errorf("order %v reduced to %v, want [[ze_bgp]]", order, got)
		}
	}
	got := appendSmallestRequirement([][]string{{"ze_flowexport"}}, []string{"ze_trafficusage"})
	if len(got) != 2 {
		t.Errorf("two disjoint requirements reduced to %v, want both kept", got)
	}
}
