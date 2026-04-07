// Design: (none — build tool)
//
// inventory generates a markdown inventory of Ze's plugins, YANG modules,
// RPCs, address families, capabilities, test files, and Go package stats.
//
// It imports the real plugin registry and YANG module list, so the output
// is always accurate — no regex parsing of source files for metadata.
//
// Usage: go run scripts/inventory.go [--json]
// Called by: make ze-inventory
//
//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}

	jsonMode := len(os.Args) > 1 && os.Args[1] == "--json"

	inv := collect(root)

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inv); err != nil {
			fatal(err)
		}
		return
	}

	printMarkdown(inv)
}

func fatal(err error) {
	fmt.Println("inventory:", err)
	os.Exit(1)
}

// Inventory holds all collected data.
type Inventory struct {
	Generated     string            `json:"generated"`
	Plugins       []PluginInfo      `json:"plugins"`
	Families      map[string]string `json:"families"`
	FamilySupport []FamilyInfo      `json:"family-support"`
	Capabilities  map[string]string `json:"capabilities"`
	YANGModules   []YANGInfo        `json:"yang-modules"`
	RPCsByModule  map[string]int    `json:"rpcs-by-module"`
	TotalRPCs     int               `json:"total-rpcs"`
	RPCList       []RPCInfo         `json:"rpc-list"`
	TestCounts    map[string]int    `json:"test-counts"`
	TotalTests    int               `json:"total-tests"`
	PackageStats  []AreaStats       `json:"package-stats"`
}

type PluginInfo struct {
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

type YANGInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type FamilyInfo struct {
	Family      string `json:"family"`
	Plugin      string `json:"plugin"`
	Decode      bool   `json:"decode"`
	Encode      bool   `json:"encode"`
	RouteEncode bool   `json:"route-encode"`
	ConfigNLRI  bool   `json:"config-nlri"`
}

type RPCInfo struct {
	Name    string `json:"name"`
	Module  string `json:"module"`
	Covered bool   `json:"covered"`
}

type AreaStats struct {
	Area     string `json:"area"`
	Packages int    `json:"packages"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
}

func collect(root string) Inventory {
	inv := Inventory{
		Generated: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}

	// Plugins from registry.
	for _, reg := range registry.All() {
		p := PluginInfo{
			Name:         reg.Name,
			Description:  reg.Description,
			Families:     reg.Families,
			Capabilities: reg.CapabilityCodes,
			Dependencies: reg.Dependencies,
			ConfigRoots:  reg.ConfigRoots,
			RFCs:         reg.RFCs,
			Features:     reg.Features,
			HasYANG:      reg.YANG != "",
			HasDecoder:   reg.InProcessNLRIDecoder != nil,
			HasEncoder:   reg.InProcessNLRIEncoder != nil,
		}
		inv.Plugins = append(inv.Plugins, p)
	}

	// Family map.
	inv.Families = registry.FamilyMap()

	// Capability map.
	capMap := registry.CapabilityMap()
	inv.Capabilities = make(map[string]string, len(capMap))
	for code, name := range capMap {
		inv.Capabilities[fmt.Sprintf("%d", code)] = name
	}

	// Family support matrix: per-family encode/decode/route-encode capabilities.
	inv.FamilySupport = collectFamilySupport()

	// YANG modules: use filesystem paths to determine source.
	yangPaths := discoverYANGPaths(root)
	for _, m := range yang.Modules() {
		source := "infrastructure"
		if path, ok := yangPaths[m.Name]; ok {
			if strings.Contains(path, "/plugins/") {
				// Extract plugin directory name from path.
				parts := strings.SplitAfter(path, "/plugins/")
				if len(parts) > 1 {
					pluginDir := strings.Split(parts[1], "/")[0]
					source = "plugin:" + pluginDir
				}
			}
		}
		inv.YANGModules = append(inv.YANGModules, YANGInfo{
			Name:   m.Name,
			Source: source,
		})
	}
	sort.Slice(inv.YANGModules, func(i, j int) bool {
		return inv.YANGModules[i].Name < inv.YANGModules[j].Name
	})

	// RPC counts and names from .yang files on disk.
	inv.RPCsByModule, inv.RPCList = extractRPCs(root)
	for _, count := range inv.RPCsByModule {
		inv.TotalRPCs += count
	}

	// RPC test coverage: search .ci files for RPC name references.
	ciContent := loadAllCIContent(filepath.Join(root, "test"))
	for i, rpc := range inv.RPCList {
		inv.RPCList[i].Covered = rpcHasCoverage(rpc.Name, ciContent)
	}

	// .ci test file counts.
	inv.TestCounts = countCITests(filepath.Join(root, "test"))
	for _, count := range inv.TestCounts {
		inv.TotalTests += count
	}

	// Go package stats.
	for _, area := range []string{"internal", "pkg", "cmd"} {
		stats := countGoStats(filepath.Join(root, area))
		stats.Area = area + "/"
		inv.PackageStats = append(inv.PackageStats, stats)
	}

	return inv
}

func discoverYANGPaths(root string) map[string]string {
	paths := make(map[string]string)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".yang") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		paths[d.Name()] = rel
		return nil
	})
	return paths
}

func collectFamilySupport() []FamilyInfo {
	var result []FamilyInfo
	seen := make(map[string]bool)

	for _, reg := range registry.All() {
		for _, fam := range reg.Families {
			if seen[fam] {
				continue
			}
			seen[fam] = true
			result = append(result, FamilyInfo{
				Family:      fam,
				Plugin:      reg.Name,
				Decode:      reg.InProcessNLRIDecoder != nil,
				Encode:      reg.InProcessNLRIEncoder != nil,
				RouteEncode: reg.InProcessRouteEncoder != nil,
				ConfigNLRI:  reg.InProcessConfigNLRIBuilder != nil,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Family < result[j].Family
	})
	return result
}

// extractRPCs returns per-module counts and a flat list of all RPC names.
func extractRPCs(root string) (map[string]int, []RPCInfo) {
	counts := make(map[string]int)
	var rpcs []RPCInfo

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".yang") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		module := d.Name()
		count := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "rpc ") {
				count++
				// Extract RPC name: "rpc foo-bar {" or "rpc foo { desc... }" -> "foo-bar"
				name := strings.TrimPrefix(line, "rpc ")
				if idx := strings.IndexAny(name, " {"); idx >= 0 {
					name = name[:idx]
				}
				rpcs = append(rpcs, RPCInfo{
					Name:   name,
					Module: module,
				})
			}
		}
		if count > 0 {
			counts[module] = count
		}
		return nil
	})

	sort.Slice(rpcs, func(i, j int) bool {
		if rpcs[i].Module != rpcs[j].Module {
			return rpcs[i].Module < rpcs[j].Module
		}
		return rpcs[i].Name < rpcs[j].Name
	})
	return counts, rpcs
}

// loadAllCIContent reads all .ci files under testDir into one string for searching.
func loadAllCIContent(testDir string) string {
	var buf strings.Builder
	_ = filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".ci") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		buf.Write(data)
		buf.WriteByte('\n')
		return nil
	})
	return buf.String()
}

// rpcHasCoverage checks if an RPC name appears in .ci test content.
// Searches for both hyphenated form (peer-list) and space form (peer list).
func rpcHasCoverage(rpcName, ciContent string) bool {
	// Direct match (e.g., "route-refresh" in .ci content).
	if strings.Contains(ciContent, rpcName) {
		return true
	}
	// Space-separated form (e.g., "peer-list" -> "peer list", "peer * list").
	spaced := strings.ReplaceAll(rpcName, "-", " ")
	if strings.Contains(ciContent, spaced) {
		return true
	}
	// Glob form: "peer-list" -> check for "peer * list" or "peer *" patterns.
	parts := strings.SplitN(spaced, " ", 2)
	if len(parts) == 2 {
		globForm := parts[0] + " * " + parts[1]
		if strings.Contains(ciContent, globForm) {
			return true
		}
	}
	return false
}

func countCITests(testDir string) map[string]int {
	counts := make(map[string]int)
	entries, err := os.ReadDir(testDir)
	if err != nil {
		return counts
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(testDir, entry.Name())
		count := 0
		_ = filepath.WalkDir(subdir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".ci") {
				count++
			}
			return nil
		})
		if count > 0 {
			counts[entry.Name()] = count
		}
	}
	return counts
}

func countGoStats(dir string) AreaStats {
	var stats AreaStats
	packages := make(map[string]bool)

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		pkg := filepath.Dir(path)
		if !packages[pkg] {
			packages[pkg] = true
			stats.Packages++
		}
		stats.Files++

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			stats.Lines++
		}
		return nil
	})

	return stats
}

func printMarkdown(inv Inventory) {
	fmt.Printf("# Ze Inventory\n\n")
	fmt.Printf("Generated: %s\n\n", inv.Generated)

	// Summary table.
	fmt.Printf("## Summary\n\n")
	fmt.Printf("| Metric | Count |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Plugins | %d |\n", len(inv.Plugins))
	fmt.Printf("| YANG modules | %d |\n", len(inv.YANGModules))
	fmt.Printf("| RPCs | %d |\n", inv.TotalRPCs)
	fmt.Printf("| Address families | %d |\n", len(inv.Families))
	fmt.Printf("| Capability codes | %d |\n", len(inv.Capabilities))
	rpcCov := 0
	for _, r := range inv.RPCList {
		if r.Covered {
			rpcCov++
		}
	}
	fmt.Printf("| RPCs with .ci coverage | %d/%d |\n", rpcCov, len(inv.RPCList))
	fmt.Printf("| .ci test files | %d |\n", inv.TotalTests)
	totalPkgs, totalFiles, totalLines := 0, 0, 0
	for _, s := range inv.PackageStats {
		totalPkgs += s.Packages
		totalFiles += s.Files
		totalLines += s.Lines
	}
	fmt.Printf("| Go packages | %d |\n", totalPkgs)
	fmt.Printf("| Go files | %d |\n", totalFiles)
	fmt.Printf("| Go lines | %d |\n", totalLines)

	// Plugins.
	fmt.Printf("\n## Plugins (%d)\n\n", len(inv.Plugins))
	fmt.Printf("| Name | Description | Families | Caps | Deps | RFCs | YANG |\n")
	fmt.Printf("|------|-------------|----------|------|------|------|------|\n")
	for _, p := range inv.Plugins {
		families := strings.Join(p.Families, ", ")
		caps := formatUint8Slice(p.Capabilities)
		deps := strings.Join(p.Dependencies, ", ")
		rfcs := strings.Join(p.RFCs, ", ")
		yangMark := "-"
		if p.HasYANG {
			yangMark = "yes"
		}
		fmt.Printf("| %s | %s | %s | %s | %s | %s | %s |\n",
			p.Name, p.Description, families, caps, deps, rfcs, yangMark)
	}

	// Address families.
	fmt.Printf("\n## Address Families (%d)\n\n", len(inv.Families))
	fmt.Printf("| Family | Plugin |\n")
	fmt.Printf("|--------|--------|\n")
	famKeys := sortedKeys(inv.Families)
	for _, f := range famKeys {
		fmt.Printf("| %s | %s |\n", f, inv.Families[f])
	}

	// Family support matrix.
	fmt.Printf("\n## Family Support Matrix\n\n")
	fmt.Printf("| Family | Plugin | Decode | Encode | Route Build | Config NLRI |\n")
	fmt.Printf("|--------|--------|--------|--------|-------------|-------------|\n")
	for _, f := range inv.FamilySupport {
		fmt.Printf("| %s | %s | %s | %s | %s | %s |\n",
			f.Family, f.Plugin, boolMark(f.Decode), boolMark(f.Encode),
			boolMark(f.RouteEncode), boolMark(f.ConfigNLRI))
	}

	// Capability codes.
	fmt.Printf("\n## Capability Codes (%d)\n\n", len(inv.Capabilities))
	fmt.Printf("| Code | Plugin |\n")
	fmt.Printf("|------|--------|\n")
	capKeys := sortedKeys(inv.Capabilities)
	for _, c := range capKeys {
		fmt.Printf("| %s | %s |\n", c, inv.Capabilities[c])
	}

	// YANG modules.
	fmt.Printf("\n## YANG Modules (%d)\n\n", len(inv.YANGModules))
	fmt.Printf("| Module | Source |\n")
	fmt.Printf("|--------|--------|\n")
	for _, m := range inv.YANGModules {
		fmt.Printf("| %s | %s |\n", m.Name, m.Source)
	}

	// RPCs by module.
	fmt.Printf("\n## RPCs by Module (%d total)\n\n", inv.TotalRPCs)
	fmt.Printf("| Module | RPCs |\n")
	fmt.Printf("|--------|------|\n")
	rpcKeys := sortedMapKeys(inv.RPCsByModule)
	for _, k := range rpcKeys {
		fmt.Printf("| %s | %d |\n", k, inv.RPCsByModule[k])
	}

	// RPC coverage.
	covered, uncovered := 0, 0
	for _, r := range inv.RPCList {
		if r.Covered {
			covered++
		} else {
			uncovered++
		}
	}
	fmt.Printf("\n## RPC Test Coverage (%d/%d covered)\n\n", covered, len(inv.RPCList))
	if uncovered > 0 {
		fmt.Printf("### Uncovered RPCs (%d)\n\n", uncovered)
		fmt.Printf("| RPC | Module |\n")
		fmt.Printf("|-----|--------|\n")
		for _, r := range inv.RPCList {
			if !r.Covered {
				fmt.Printf("| %s | %s |\n", r.Name, r.Module)
			}
		}
	}

	// .ci tests.
	fmt.Printf("\n## Functional Tests (%d .ci files)\n\n", inv.TotalTests)
	fmt.Printf("| Directory | Count |\n")
	fmt.Printf("|-----------|-------|\n")
	testKeys := sortedMapKeys(inv.TestCounts)
	for _, k := range testKeys {
		fmt.Printf("| test/%s/ | %d |\n", k, inv.TestCounts[k])
	}

	// Go package stats.
	fmt.Printf("\n## Go Packages\n\n")
	fmt.Printf("| Area | Packages | Files | Lines |\n")
	fmt.Printf("|------|----------|-------|-------|\n")
	for _, s := range inv.PackageStats {
		fmt.Printf("| %s | %d | %d | %d |\n", s.Area, s.Packages, s.Files, s.Lines)
	}
	fmt.Printf("| **total** | **%d** | **%d** | **%d** |\n", totalPkgs, totalFiles, totalLines)
}

func boolMark(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}

func formatUint8Slice(vals []uint8) string {
	if len(vals) == 0 {
		return "-"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ", ")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
