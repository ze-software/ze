// Design: docs/architecture/config/syntax.md -- YANG schema default application
// Related: library_chains.go -- parseAttributeBases, the reader of bgp/defaults/attribute

package bgpconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestAigpDefaultLeafDoesNotExist covers AC-6. There is no aigp default and
// there must not be one: RFC 7311 Section 4.1 removes a route with no AIGP TLV
// from consideration rather than scoring it, so no number stands in for the
// absence, and Section 3.4.1 forbids ze to create the attribute outside the
// AIGP administrative domain. The chain's bases (parseAttributeBases) read the
// two leaves this container declares and no third.
//
// The reason is asserted at the container, because that is where somebody about
// to add the leaf would read it. It is read out of the container's
// description, the long text the ? box prints, not out of the one-line
// summary, which command.MaxSummaryChars bounds at 96 characters.
func TestAigpDefaultLeafDoesNotExist(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Lookup(attributeBasesPath + "/aigp"); err == nil {
		t.Fatal("an aigp default leaf must not exist")
	}
	node, err := schema.Lookup(attributeBasesPath)
	if err != nil {
		t.Fatal(err)
	}
	container, ok := node.(*config.ContainerNode)
	if !ok {
		t.Fatalf("%s is %T, want a container", attributeBasesPath, node)
	}
	children := slices.Sorted(slices.Values(container.Children()))
	if !slices.Equal(children, []string{"local-preference", "med"}) {
		t.Fatalf("children = %v, want the two attributes that have a base and no third", children)
	}
	help := containerHelp(t, attributeBasesPath)
	if !strings.Contains(help, "RFC 7311") || !strings.Contains(help, "aigp") {
		t.Fatalf("help = %q, want the reason for the absence (RFC 7311, aigp) where somebody would add the leaf", help)
	}
}

// TestAttributeDefaultLeafRangeHoldsAtTheBoundaries covers the spec's numeric
// boundary table for both leaves. ValidateLeafValue is the check the config file
// parse path runs (parser_list.go), so the refusal an operator meets is the one
// asserted here.
//
// 0 is VALID for both, and it is med's declared default, so the range cannot be
// written as 1..4294967295 the way an increment value is.
func TestAttributeDefaultLeafRangeHoldsAtTheBoundaries(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"med", "local-preference"} {
		t.Run(name, func(t *testing.T) {
			node, err := schema.Lookup(attributeBasesPath + "/" + name)
			if err != nil {
				t.Fatal(err)
			}
			leaf, ok := node.(*config.LeafNode)
			if !ok {
				t.Fatalf("%s is %T, want a leaf", name, node)
			}
			if len(leaf.Ranges) == 0 {
				t.Fatal("the leaf states its bound rather than leaving it to the type")
			}
			for _, valid := range []string{"0", "4294967295"} {
				if err := config.ValidateLeafValue(leaf, nil, valid); err != nil {
					t.Fatalf("%s refused: %v", valid, err)
				}
			}
			for _, invalid := range []string{"4294967296", "-1"} {
				if err := config.ValidateLeafValue(leaf, nil, invalid); err == nil {
					t.Fatalf("%s accepted", invalid)
				}
			}
		})
	}
}

// containerHelp answers the long explanation the YANG container at path
// declares in its description statement, read straight off the YANG entry as
// the ? box does (entryDescription, internal/component/cli/completer.go). A
// module the binary never registered would turn the walk into an empty string,
// so each step of the walk is required rather than skipped.
func containerHelp(t *testing.T, path string) string {
	t.Helper()
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	entry := loader.GetEntry("ze-bgp-conf")
	if entry == nil {
		t.Fatal("ze-bgp-conf is not registered in this test binary")
	}
	for name := range strings.SplitSeq(path, "/") {
		if entry.Dir == nil {
			t.Fatalf("%s has no children, so %q cannot be reached", entry.Name, name)
		}
		entry = entry.Dir[name]
		if entry == nil {
			t.Fatalf("the YANG tree declares no %q under %s", name, path)
		}
	}
	return entry.Description
}
