// Design: docs/architecture/plugin/feature-gates.md -- every report section shares committed source.
package repoinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCollectUsesCommittedSourcesForEveryReportArea drives the public report
// over an authored entry carrying every family-support field, then dirties Go,
// schema and CI inputs. Exact counters preserve the report's existing sections.
func TestCollectUsesCommittedSourcesForEveryReportArea(t *testing.T) {
	root := committedInventoryFixture(t)
	for name, replacement := range map[string][2]string{
		"internal/component/plugin/registry/registry.go": {
			"Name, Description, YANG string; RunEngine func()",
			"Name, Description, YANG string; RunEngine func(); Families []string; CapabilityCodes []uint8; InProcessNLRIDecoder, InProcessNLRIEncoder, InProcessRouteEncoder, InProcessConfigNLRIBuilder func()",
		},
		"internal/thing/one.go": {
			`RunEngine: engine, YANG:`,
			`RunEngine: engine, Families: []string{"ipv4/unicast"}, CapabilityCodes: []uint8{64}, InProcessNLRIDecoder: engine, InProcessNLRIEncoder: engine, InProcessRouteEncoder: engine, InProcessConfigNLRIBuilder: engine, YANG:`,
		},
	} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(raw), replacement[0]) != 1 {
			t.Fatalf("fixture metadata marker %q is not unique", replacement[0])
		}
		write(t, root, name, strings.Replace(string(raw), replacement[0], replacement[1], 1))
	}
	commitInventoryFixture(t, root)
	write(t, root, "internal/thing/one.go", "package thing\n")
	write(t, root, "internal/plugins/thing/yang/thing.yang", "module thing {\n rpc dirty-only { }\n}\n")
	write(t, root, "test/ui/dirty.ci", "cmd=foreground:exec=ze dirty only\n")
	inv, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Plugins) != 1 || inv.Plugins[0].Name != "thing" {
		t.Errorf("report read host/dirty plugin population: %+v", inv.Plugins)
	}
	if inv.Families["ipv4/unicast"] != "thing" || inv.Capabilities["64"] != "thing" {
		t.Errorf("report lost committed family/capability maps: %v %v", inv.Families, inv.Capabilities)
	}
	if len(inv.FamilySupport) != 1 {
		t.Errorf("report family-support rows = %+v", inv.FamilySupport)
	} else {
		family := inv.FamilySupport[0]
		if family.Family != "ipv4/unicast" || family.Plugin != "thing" || !family.Decode || !family.Encode || !family.RouteEncode || !family.ConfigNLRI {
			t.Errorf("report dropped committed family-support fields: %+v", family)
		}
	}
	if inv.TotalRPCs != 3 || inv.RPCsByModule["thing.yang"] != 2 || inv.coveredRPCs() != 2 {
		t.Errorf("report read dirty RPC/coverage source: %+v", inv.RPCList)
	}
	if inv.TotalTests != 3 || inv.TestCounts["ui"] != 2 {
		t.Errorf("report read dirty CI source: %v", inv.TestCounts)
	}
	if packages, files, lines := inv.totals(); packages != 3 || files != 4 || lines != 11 {
		t.Errorf("report read dirty Go source: %d packages %d files %d lines", packages, files, lines)
	}
	found := false
	for _, module := range inv.YANGModules {
		if module.Name == "thing" {
			found = true
		}
	}
	if !found {
		t.Error("report dropped the committed plugin schema")
	}
}
