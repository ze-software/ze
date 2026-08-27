// The page is what a reader gets from the bare command, so it is asserted
// directly rather than through a subprocess's stdout.

package inventory

import (
	"strings"
	"testing"
)

// page is an inventory holding one of everything the rendering has a branch
// for: a covered RPC and an uncovered one, a plugin with capabilities and one
// without, and two code areas.
func page() Inventory {
	return Inventory{
		Generated: "2026-08-26 09:00 UTC",
		Plugins: []Plugin{
			{Name: "rib", Description: "the RIB", Families: []string{"ipv4 unicast"},
				Capabilities: []uint8{64, 69}, Dependencies: []string{"bgp"},
				RFCs: []string{"4271"}, HasYANG: true},
			{Name: "ntp", Description: "the clock"},
		},
		Families:      map[string]string{"ipv4 unicast": "rib", "ipv6 unicast": "rib"},
		FamilySupport: []Family{{Family: "ipv4 unicast", Plugin: "rib", Decode: true}},
		Capabilities:  map[string]string{"64": "gr", "69": "add-path"},
		YANGModules:   []YANGModule{{Name: "ze-rib.yang", Source: "plugin:rib"}},
		RPCsByModule:  map[string]int{"ze-rib.yang": 2},
		TotalRPCs:     2,
		RPCList: []RPC{
			{Name: "rib-show", Module: "ze-rib.yang", Covered: true},
			{Name: "rib-clear", Module: "ze-rib.yang"},
		},
		TestCounts:   map[string]int{"ui": 3},
		TotalTests:   3,
		PackageStats: []AreaStats{{Area: "internal/", Packages: 2, Files: 5, Lines: 100}, {Area: "cmd/", Packages: 1, Files: 1, Lines: 20}},
	}
}

// TestTextRendersEverySection pins the page a reader gets, section by section.
// The gate has printed this page since before it was a command, and a section
// that stopped rendering would pass every other test in this repository.
func TestTextRendersEverySection(t *testing.T) {
	text := page().Text()
	for _, want := range []string{
		"# Ze Inventory",
		"Generated: 2026-08-26 09:00 UTC",
		"## Summary",
		"| Plugins | 2 |",
		"| RPCs | 2 |",
		"| Address families | 2 |",
		"| Capability codes | 2 |",
		"| RPCs with .ci coverage | 1/2 |",
		"| .ci test files | 3 |",
		"| Go packages | 3 |",
		"| Go files | 6 |",
		"| Go lines | 120 |",
		"## Plugins (2)",
		"| rib | the RIB | ipv4 unicast | 64, 69 | bgp | 4271 | yes |",
		"| ntp | the clock |  | - |  |  | - |",
		"## Address Families (2)",
		"## Family Support Matrix",
		"| ipv4 unicast | rib | yes | - | - | - |",
		"## Capability Codes (2)",
		"| 64 | gr |",
		"## YANG Modules (1)",
		"| ze-rib.yang | plugin:rib |",
		"## RPCs by Module (2 total)",
		"## RPC Test Coverage (1/2 covered)",
		"### Uncovered RPCs (1)",
		"| rib-clear | ze-rib.yang |",
		"## Functional Tests (3 .ci files)",
		"| test/ui/ | 3 |",
		"## Go Packages",
		"| internal/ | 2 | 5 | 100 |",
		"| **total** | **3** | **6** | **120** |",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page is missing %q:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("the page does not end in a newline")
	}
}

// TestTextOmitsTheUncoveredSectionWhenNothingIsUncovered: the section is a list
// of work to do, and an empty list of work reads as a defect in the report.
func TestTextOmitsTheUncoveredSectionWhenNothingIsUncovered(t *testing.T) {
	inv := page()
	for i := range inv.RPCList {
		inv.RPCList[i].Covered = true
	}
	text := inv.Text()
	if strings.Contains(text, "### Uncovered RPCs") {
		t.Errorf("a fully covered inventory still printed the uncovered section:\n%s", text)
	}
	if !strings.Contains(text, "## RPC Test Coverage (2/2 covered)") {
		t.Errorf("the coverage heading did not report 2/2:\n%s", text)
	}
}

// TestTextSortsTheMapTables pins the order of every table built from a map. Go
// ranges a map in a different order on every run, so an unsorted table would
// make two runs of the gate disagree with each other.
func TestTextSortsTheMapTables(t *testing.T) {
	first := page().Text()
	for range 8 {
		if again := page().Text(); again != first {
			t.Fatal("two renderings of one inventory differ: a table built from a map is unsorted")
		}
	}
	families := strings.Index(first, "| ipv4 unicast | rib |")
	later := strings.Index(first, "| ipv6 unicast | rib |")
	if families < 0 || later < 0 || families > later {
		t.Error("the address-family table is not in name order")
	}
}

// TestTotalsSumEveryArea guards the one number the page states about itself.
func TestTotalsSumEveryArea(t *testing.T) {
	packages, files, lines := page().totals()
	if packages != 3 || files != 6 || lines != 120 {
		t.Errorf("totals answered %d, %d, %d; want 3, 6, 120", packages, files, lines)
	}
}
