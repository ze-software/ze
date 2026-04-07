// Design: (none -- build tool)
//
// check-doc-drift compares documentation claims against the live plugin
// registry, filesystem counts, and structured doc tables. It reports
// any drift between what the code provides and what the docs claim.
//
// Usage: go run scripts/check-doc-drift.go [--strict]
// Called by: make ze-doc-drift, .claude/hooks/check-doc-drift.sh
//
// Exit codes:
//   0 = no drift
//   1 = drift detected (advisory)
//   2 = drift detected + --strict (blocking)
//
//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-doc-drift: %v\n", err)
		os.Exit(1)
	}

	strict := len(os.Args) > 1 && os.Args[1] == "--strict"

	issues := runChecks(root)

	if len(issues) == 0 {
		fmt.Println("No documentation drift detected.")
		return
	}

	fmt.Fprintf(os.Stderr, "\n\033[33m\033[1m  Documentation drift detected (%d issues)\033[0m\n\n", len(issues))
	for _, iss := range issues {
		fmt.Fprintf(os.Stderr, "  \033[31mx\033[0m %s:%d: %s\n", iss.File, iss.Line, iss.Message)
		if iss.Detail != "" {
			fmt.Fprintf(os.Stderr, "    \033[33m->\033[0m %s\n", iss.Detail)
		}
	}
	fmt.Fprintf(os.Stderr, "\n  Run: make ze-doc-drift\n\n")

	if strict {
		os.Exit(2)
	}
	os.Exit(1)
}

type issue struct {
	File    string
	Line    int
	Message string
	Detail  string
}

func runChecks(root string) []issue {
	var issues []issue

	pluginNames := registryPluginNames()
	familyNames := registryFamilyNames()

	ciTotal, ciByDir := countCITests(filepath.Join(root, "test"))
	interopCount := countInteropScenarios(filepath.Join(root, "test", "interop", "scenarios"))
	fuzzCount := countFuzzTargets(root)
	goTestCount := countGoTestFunctions(root)

	issues = append(issues, checkDesignMD(root, pluginNames, familyNames, ciTotal, ciByDir, interopCount, fuzzCount, goTestCount)...)
	issues = append(issues, checkComparisonMD(root, familyNames)...)

	return issues
}

func registryPluginNames() []string {
	var names []string
	for _, reg := range registry.All() {
		names = append(names, reg.Name)
	}
	sort.Strings(names)
	return names
}

func registryFamilyNames() []string {
	fam := registry.FamilyMap()
	names := make(map[string]bool)
	for name := range fam {
		names[name] = true
	}
	// Engine built-in families (not plugin-registered).
	for _, builtin := range []string{
		"ipv4/unicast", "ipv6/unicast",
		"ipv4/multicast", "ipv6/multicast",
	} {
		names[builtin] = true
	}
	var result []string
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func countCITests(testDir string) (int, map[string]int) {
	total := 0
	byDir := make(map[string]int)
	_ = filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".ci") {
			return nil
		}
		total++
		rel, _ := filepath.Rel(testDir, path)
		dir := strings.Split(rel, string(filepath.Separator))[0]
		byDir[dir]++
		return nil
	})
	return total, byDir
}

func countInteropScenarios(scenariosDir string) int {
	count := 0
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	return count
}

func countFuzzTargets(root string) int {
	count := 0
	re := regexp.MustCompile(`^func Fuzz`)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		if strings.Contains(path, "vendor") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if re.MatchString(scanner.Text()) {
				count++
			}
		}
		return nil
	})
	return count
}

func countGoTestFunctions(root string) int {
	count := 0
	re := regexp.MustCompile(`^func Test`)
	for _, area := range []string{"internal", "pkg", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, area), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if re.MatchString(scanner.Text()) {
					count++
				}
			}
			return nil
		})
	}
	return count
}

func checkDesignMD(root string, pluginNames, familyNames []string, ciTotal int, _ map[string]int, interopCount, fuzzCount, goTestCount int) []issue {
	path := filepath.Join(root, "docs", "DESIGN.md")
	lines, err := readLines(path)
	if err != nil {
		return nil
	}

	var issues []issue

	for i, line := range lines {
		lineNum := i + 1

		if m := extractCount(line, `(\d+) address families`); m > 0 {
			actual := len(familyNames)
			if m != actual {
				issues = append(issues, issue{
					File:    "docs/DESIGN.md",
					Line:    lineNum,
					Message: fmt.Sprintf("claims %d address families, registry has %d", m, actual),
					Detail:  fmt.Sprintf("registry: %s", strings.Join(familyNames, ", ")),
				})
			}
		}

		if m := extractApprox(line, `~([\d,]+).*functional test`); m > 0 {
			if !withinThreshold(m, ciTotal, 0.20) {
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: fmt.Sprintf("claims ~%d functional tests, actual is %d", m, ciTotal),
				})
			}
		}
		if m := extractApprox(line, `~([\d,]+).*[Gg]o test function`); m > 0 {
			if !withinThreshold(m, goTestCount, 0.20) {
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: fmt.Sprintf("claims ~%d Go test functions, actual is %d", m, goTestCount),
				})
			}
		}
		if m := extractApprox(line, `~([\d,]+).*[Ff]uzz target`); m > 0 {
			if !withinThreshold(m, fuzzCount, 0.30) {
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: fmt.Sprintf("claims ~%d fuzz targets, actual is %d", m, fuzzCount),
				})
			}
		}

		if m := extractCount(line, `(\d+) interop scenario`); m > 0 {
			if m != interopCount {
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: fmt.Sprintf("claims %d interop scenarios, actual is %d", m, interopCount),
				})
			}
		}
	}

	tablePlugins := extractTableColumn(lines, "Plugin", "Purpose", 0)
	for _, name := range pluginNames {
		found := false
		for _, tp := range tablePlugins {
			if strings.Trim(tp, "`") == name {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, issue{
				File:    "docs/DESIGN.md",
				Line:    0,
				Message: fmt.Sprintf("plugin %q registered but missing from Shipped Plugins table", name),
			})
		}
	}

	return issues
}

func checkComparisonMD(root string, familyNames []string) []issue {
	path := filepath.Join(root, "docs", "comparison.md")
	lines, err := readLines(path)
	if err != nil {
		return nil
	}

	var issues []issue
	familySet := make(map[string]bool)
	for _, f := range familyNames {
		familySet[f] = true
	}

	labelToFamily := map[string]string{
		"ipv4 unicast":             "ipv4/unicast",
		"ipv6 unicast":             "ipv6/unicast",
		"ipv4 multicast":           "ipv4/multicast",
		"ipv6 multicast":           "ipv6/multicast",
		"ipv4 labeled unicast":     "ipv4/mpls-label",
		"ipv6 labeled unicast":     "ipv6/mpls-label",
		"vpnv4 (rfc 4364)":         "ipv4/mpls-vpn",
		"vpnv4":                    "ipv4/mpls-vpn",
		"vpnv6":                    "ipv6/mpls-vpn",
		"l2vpn evpn (rfc 7432)":    "l2vpn/evpn",
		"l2vpn evpn":               "l2vpn/evpn",
		"l2vpn vpls":               "l2vpn/vpls",
		"ipv4 flowspec (rfc 8955)": "ipv4/flow",
		"ipv4 flowspec":            "ipv4/flow",
		"ipv6 flowspec":            "ipv6/flow",
		"vpn flowspec":             "ipv4/flow-vpn",
		"bgp-ls (rfc 7752)":        "bgp-ls/bgp-ls",
		"bgp-nlri-ls":              "bgp-ls/bgp-ls",
		"ipv4/ipv6 mup":            "ipv4/mup",
		"ipv4/ipv6 mvpn":           "ipv4/mvpn",
	}

	inFamilyTable := false
	for i, line := range lines {
		lineNum := i + 1
		if strings.Contains(line, "AFI/SAFI") && strings.Contains(line, "Ze") {
			inFamilyTable = true
			continue
		}
		if inFamilyTable && strings.HasPrefix(strings.TrimSpace(line), "|---") {
			continue
		}
		if inFamilyTable && !strings.HasPrefix(strings.TrimSpace(line), "|") {
			inFamilyTable = false
			continue
		}
		if !inFamilyTable {
			continue
		}

		cells := splitTableRow(line)
		if len(cells) < 3 {
			continue
		}

		label := strings.TrimSpace(strings.ToLower(cells[0]))
		zeClaim := strings.TrimSpace(strings.ToLower(cells[1]))

		regFamily, mapped := labelToFamily[label]
		if !mapped {
			continue
		}

		inRegistry := familySet[regFamily]

		switch {
		case zeClaim == "yes" && !inRegistry:
			issues = append(issues, issue{
				File: "docs/comparison.md", Line: lineNum,
				Message: fmt.Sprintf("claims Ze has %q but %q not in registry", cells[0], regFamily),
			})
		case zeClaim == "no" && inRegistry:
			issues = append(issues, issue{
				File: "docs/comparison.md", Line: lineNum,
				Message: fmt.Sprintf("claims Ze lacks %q but %q IS in registry", cells[0], regFamily),
				Detail:  "Change to Yes or Decode as appropriate",
			})
		}
	}

	return issues
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

func extractCount(line, pattern string) int {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	return n
}

func extractApprox(line, pattern string) int {
	return extractCount(line, pattern)
}

func withinThreshold(claimed, actual int, threshold float64) bool {
	if actual == 0 {
		return claimed == 0
	}
	diff := float64(claimed-actual) / float64(actual)
	if diff < 0 {
		diff = -diff
	}
	return diff <= threshold
}

func extractTableColumn(lines []string, header1, header2 string, colIdx int) []string {
	var values []string
	inTable := false
	pastSeparator := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inTable {
			if strings.Contains(trimmed, header1) && strings.Contains(trimmed, header2) && strings.HasPrefix(trimmed, "|") {
				inTable = true
			}
			continue
		}
		if !pastSeparator {
			if strings.Contains(trimmed, "---") {
				pastSeparator = true
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := splitTableRow(trimmed)
		if colIdx < len(cells) {
			values = append(values, strings.TrimSpace(cells[colIdx]))
		}
	}
	return values
}

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
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
