// Design: (none -- build tool)
//
// Overview: inventory.go -- what produces this answer
//
// report.go holds what `le inventory` ANSWERS, apart from what produced it.
//
// The answer is one document with several tables in it: plugins, families,
// capabilities, YANG modules, RPCs, tests and package statistics. It renders
// ITSELF (Text), because the markdown page is what `make ze-inventory` has
// always printed. `| json` and `| yaml` render the same data unchanged.

package inventory

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Inventory is the whole answer of one run.
type Inventory struct {
	Generated     string            `json:"generated"`
	Plugins       []Plugin          `json:"plugins"`
	Families      map[string]string `json:"families"`
	FamilySupport []Family          `json:"family-support"`
	Capabilities  map[string]string `json:"capabilities"`
	YANGModules   []YANGModule      `json:"yang-modules"`
	RPCsByModule  map[string]int    `json:"rpcs-by-module"`
	TotalRPCs     int               `json:"total-rpcs"`
	RPCList       []RPC             `json:"rpc-list"`
	TestCounts    map[string]int    `json:"test-counts"`
	TotalTests    int               `json:"total-tests"`
	PackageStats  []AreaStats       `json:"package-stats"`
}

// Plugin is one registered plugin, as the plugin registry describes it.
type Plugin struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Families     []string `json:"families,omitempty"`
	Capabilities []uint8  `json:"capabilities,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	ConfigRoots  []string `json:"config-roots,omitempty"`
	RFCs         []string `json:"rfcs,omitempty"`
	Features     string   `json:"features,omitempty"`
	HasYANG      bool     `json:"has-yang"`
	HasDecoder   bool     `json:"has-decoder"`
	HasEncoder   bool     `json:"has-encoder"`
}

// YANGModule is one loaded module and where it came from: "infrastructure", or
// "plugin:<dir>" when the file sits under a plugin directory.
type YANGModule struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Family is one address family and what the plugin owning it can do with it.
type Family struct {
	Family      string `json:"family"`
	Plugin      string `json:"plugin"`
	Decode      bool   `json:"decode"`
	Encode      bool   `json:"encode"`
	RouteEncode bool   `json:"route-encode"`
	ConfigNLRI  bool   `json:"config-nlri"`
}

// RPC is one RPC declared by a .yang file on disk, and whether any .ci test
// names it.
type RPC struct {
	Name    string `json:"name"`
	Module  string `json:"module"`
	Covered bool   `json:"covered"`
}

// AreaStats counts the Go code under one top-level area.
type AreaStats struct {
	Area     string `json:"area"`
	Packages int    `json:"packages"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
}

// totals sums the per-area package, file and line counts.
func (inv Inventory) totals() (packages, files, lines int) {
	for _, stats := range inv.PackageStats {
		packages += stats.Packages
		files += stats.Files
		lines += stats.Lines
	}
	return packages, files, lines
}

// coveredRPCs counts the RPCs some .ci test names.
func (inv Inventory) coveredRPCs() int {
	covered := 0
	for _, rpc := range inv.RPCList {
		if rpc.Covered {
			covered++
		}
	}
	return covered
}

// Text renders the inventory as the markdown page `make ze-inventory` prints.
// It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it (internal/le/leroot, Prose). It carries no color: the page
// is pasted into documents, and the script printed none.
func (inv Inventory) Text() string {
	var tb textbuf.Buffer
	packages, files, lines := inv.totals()
	covered := inv.coveredRPCs()

	tb.Str("# Ze Inventory\n\n")
	tb.Str("Generated: ").Str(inv.Generated).Str("\n\n")

	tb.Str("## Summary\n\n")
	tb.Str("| Metric | Count |\n")
	tb.Str("|--------|-------|\n")
	summaryRow(&tb, "Plugins", len(inv.Plugins))
	summaryRow(&tb, "YANG modules", len(inv.YANGModules))
	summaryRow(&tb, "RPCs", inv.TotalRPCs)
	summaryRow(&tb, "Address families", len(inv.Families))
	summaryRow(&tb, "Capability codes", len(inv.Capabilities))
	tb.Str("| RPCs with .ci coverage | ").Int(int64(covered)).Byte('/').
		Int(int64(len(inv.RPCList))).Str(" |\n")
	summaryRow(&tb, ".ci test files", inv.TotalTests)
	summaryRow(&tb, "Go packages", packages)
	summaryRow(&tb, "Go files", files)
	summaryRow(&tb, "Go lines", lines)

	inv.writePlugins(&tb)
	inv.writeFamilies(&tb)
	inv.writeCapabilities(&tb)
	inv.writeYANG(&tb)
	inv.writeRPCs(&tb)
	inv.writeTests(&tb)
	inv.writePackages(&tb, packages, files, lines)

	return tb.String()
}

// summaryRow writes one `| name | count |` row of the summary table.
func summaryRow(tb *textbuf.Buffer, name string, count int) {
	tb.Str("| ").Str(name).Str(" | ").Int(int64(count)).Str(" |\n")
}

// writePlugins renders the plugin table.
func (inv Inventory) writePlugins(tb *textbuf.Buffer) {
	tb.Str("\n## Plugins (").Int(int64(len(inv.Plugins))).Str(")\n\n")
	tb.Str("| Name | Description | Families | Caps | Deps | RFCs | YANG |\n")
	tb.Str("|------|-------------|----------|------|------|------|------|\n")
	// Indexed rather than ranged by value: a Plugin is 176 bytes, and the
	// table is one row per registered plugin.
	for i := range inv.Plugins {
		plugin := &inv.Plugins[i]
		tb.Str("| ").Str(plugin.Name).Str(" | ").Str(plugin.Description).Str(" | ").
			Join(plugin.Families, ", ").Str(" | ")
		writeCapabilityCodes(tb, plugin.Capabilities)
		tb.Str(" | ").Join(plugin.Dependencies, ", ").Str(" | ").
			Join(plugin.RFCs, ", ").Str(" | ").Str(mark(plugin.HasYANG)).Str(" |\n")
	}
}

// writeFamilies renders the address-family table and the support matrix.
func (inv Inventory) writeFamilies(tb *textbuf.Buffer) {
	tb.Str("\n## Address Families (").Int(int64(len(inv.Families))).Str(")\n\n")
	tb.Str("| Family | Plugin |\n")
	tb.Str("|--------|--------|\n")
	for _, family := range sortedKeys(inv.Families) {
		tb.Str("| ").Str(family).Str(" | ").Str(inv.Families[family]).Str(" |\n")
	}

	tb.Str("\n## Family Support Matrix\n\n")
	tb.Str("| Family | Plugin | Decode | Encode | Route Build | Config NLRI |\n")
	tb.Str("|--------|--------|--------|--------|-------------|-------------|\n")
	for _, family := range inv.FamilySupport {
		tb.Str("| ").Str(family.Family).Str(" | ").Str(family.Plugin).Str(" | ").
			Str(mark(family.Decode)).Str(" | ").Str(mark(family.Encode)).Str(" | ").
			Str(mark(family.RouteEncode)).Str(" | ").Str(mark(family.ConfigNLRI)).Str(" |\n")
	}
}

// writeCapabilities renders the capability-code table.
func (inv Inventory) writeCapabilities(tb *textbuf.Buffer) {
	tb.Str("\n## Capability Codes (").Int(int64(len(inv.Capabilities))).Str(")\n\n")
	tb.Str("| Code | Plugin |\n")
	tb.Str("|------|--------|\n")
	for _, code := range sortedKeys(inv.Capabilities) {
		tb.Str("| ").Str(code).Str(" | ").Str(inv.Capabilities[code]).Str(" |\n")
	}
}

// writeYANG renders the YANG module table.
func (inv Inventory) writeYANG(tb *textbuf.Buffer) {
	tb.Str("\n## YANG Modules (").Int(int64(len(inv.YANGModules))).Str(")\n\n")
	tb.Str("| Module | Source |\n")
	tb.Str("|--------|--------|\n")
	for _, module := range inv.YANGModules {
		tb.Str("| ").Str(module.Name).Str(" | ").Str(module.Source).Str(" |\n")
	}
}

// writeRPCs renders the per-module counts and the uncovered list.
func (inv Inventory) writeRPCs(tb *textbuf.Buffer) {
	tb.Str("\n## RPCs by Module (").Int(int64(inv.TotalRPCs)).Str(" total)\n\n")
	tb.Str("| Module | RPCs |\n")
	tb.Str("|--------|------|\n")
	for _, module := range sortedCountKeys(inv.RPCsByModule) {
		tb.Str("| ").Str(module).Str(" | ").Int(int64(inv.RPCsByModule[module])).Str(" |\n")
	}

	covered := inv.coveredRPCs()
	uncovered := len(inv.RPCList) - covered
	tb.Str("\n## RPC Test Coverage (").Int(int64(covered)).Byte('/').
		Int(int64(len(inv.RPCList))).Str(" covered)\n\n")
	if uncovered == 0 {
		return
	}
	tb.Str("### Uncovered RPCs (").Int(int64(uncovered)).Str(")\n\n")
	tb.Str("| RPC | Module |\n")
	tb.Str("|-----|--------|\n")
	for _, rpc := range inv.RPCList {
		if !rpc.Covered {
			tb.Str("| ").Str(rpc.Name).Str(" | ").Str(rpc.Module).Str(" |\n")
		}
	}
}

// writeTests renders the .ci file counts per test directory.
func (inv Inventory) writeTests(tb *textbuf.Buffer) {
	tb.Str("\n## Functional Tests (").Int(int64(inv.TotalTests)).Str(" .ci files)\n\n")
	tb.Str("| Directory | Count |\n")
	tb.Str("|-----------|-------|\n")
	for _, dir := range sortedCountKeys(inv.TestCounts) {
		tb.Str("| test/").Str(dir).Str("/ | ").Int(int64(inv.TestCounts[dir])).Str(" |\n")
	}
}

// writePackages renders the per-area Go statistics and their total.
func (inv Inventory) writePackages(tb *textbuf.Buffer, packages, files, lines int) {
	tb.Str("\n## Go Packages\n\n")
	tb.Str("| Area | Packages | Files | Lines |\n")
	tb.Str("|------|----------|-------|-------|\n")
	for _, stats := range inv.PackageStats {
		tb.Str("| ").Str(stats.Area).Str(" | ").Int(int64(stats.Packages)).Str(" | ").
			Int(int64(stats.Files)).Str(" | ").Int(int64(stats.Lines)).Str(" |\n")
	}
	tb.Str("| **total** | **").Int(int64(packages)).Str("** | **").Int(int64(files)).
		Str("** | **").Int(int64(lines)).Str("** |\n")
}

// mark renders a boolean cell the way the page has always rendered one.
func mark(set bool) string {
	if set {
		return "yes"
	}
	return "-"
}

// writeCapabilityCodes renders a plugin's capability codes as a comma-separated
// cell, or "-" when it declares none.
func writeCapabilityCodes(tb *textbuf.Buffer, codes []uint8) {
	if len(codes) == 0 {
		tb.Byte('-')
		return
	}
	for i, code := range codes {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Uint8(code)
	}
}

// sortedKeys answers a string-keyed map's keys in order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedCountKeys answers an int-keyed map's keys in order.
func sortedCountKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
