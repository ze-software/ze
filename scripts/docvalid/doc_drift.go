// Design: (none -- build tool)
//
// check-doc-drift compares documentation claims against the live plugin
// registry, Makefile gates, filesystem counts, and structured doc tables.
// It reports any drift between what the code provides and what the docs claim.
//
// Usage: go run scripts/check-doc-drift.go [--strict]
// Called by: make ze-doc-drift-check, .claude/hooks/check-doc-drift.sh
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
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func main() {
	strict := flag.Bool("strict", false, "exit 2 instead of 1 when drift is found")
	rootFlag := flag.String("root", "", "repository root to check")
	flag.Parse()

	root := *rootFlag
	if root == "" {
		var err error
		root, err = findModuleRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-doc-drift: %v\n", err)
			os.Exit(1)
		}
	}

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
	fmt.Fprintf(os.Stderr, "\n  Run: make ze-doc-drift-check\n\n")

	if *strict {
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
	releaseGateSuites := functionalGateSuites(root)

	issues = append(issues, checkDesignMD(root, pluginNames, familyNames, ciTotal, ciByDir, interopCount, fuzzCount, goTestCount)...)
	issues = append(issues, checkComparisonMD(root, familyNames)...)
	issues = append(issues, checkReadmeMD(root, ciTotal, interopCount, fuzzCount, goTestCount)...)
	issues = append(issues, checkFeaturesMD(root)...)
	issues = append(issues, checkFunctionalTestsMD(root, releaseGateSuites)...)
	issues = append(issues, checkMakefileHelp(root, releaseGateSuites)...)
	issues = append(issues, checkForbiddenDocClaims(root)...)
	issues = append(issues, unreadableFiles...)

	return issues
}

// unreadableFiles collects the files this tool could not read in full.
//
// Every count and every check here is drawn from a file scan, and a scan that
// stops early yields a low count and no finding. That is the shape that reports
// a document accurate because the check never reached the drift, so it is
// reported as drift of its own.
var unreadableFiles []issue

// noteUnreadable records a file whose scan stopped before the end.
func noteUnreadable(path string, err error) {
	var tb textbuf.Buffer
	unreadableFiles = append(unreadableFiles, issue{
		File:    path,
		Message: "read stopped early, so the checks over this file are incomplete",
		Detail:  tb.Err(err).String(),
	})
}

type forbiddenDocClaim struct {
	File    string
	Needle  string
	Message string
	Detail  string
}

var forbiddenDocClaims = []forbiddenDocClaim{
	{
		File:    "docs/architecture/api/text-parser.md",
		Needle:  "strings.Fields",
		Message: "stale text parser claim references strings.Fields",
		Detail:  "The route-server text parser uses textparse.NewScanner; update this doc to source-linked scanner wording.",
	},
	{
		File:    "docs/architecture/api/text-parser.md",
		Needle:  "All functions allocate via",
		Message: "stale text parser allocation summary",
		Detail:  "Describe scanner tokenization and the remaining result allocations separately.",
	},
	{
		File:    "docs/architecture/api/text-parser.md",
		Needle:  "No manual byte scanning or zero-allocation parsing exists",
		Message: "stale text parser allocation summary",
		Detail:  "Describe scanner tokenization and the remaining result allocations separately.",
	},
}

func checkForbiddenDocClaims(root string) []issue {
	var issues []issue
	for _, claim := range forbiddenDocClaims {
		lines, err := readLines(filepath.Join(root, claim.File))
		if err != nil {
			continue
		}
		for i, line := range lines {
			if !strings.Contains(line, claim.Needle) {
				continue
			}
			issues = append(issues, issue{
				File:    claim.File,
				Line:    i + 1,
				Message: claim.Message,
				Detail:  claim.Detail,
			})
		}
	}
	return issues
}

func functionalGateSuites(root string) []string {
	lines, err := readMakefileLines(root, "Makefile", make(map[string]bool))
	if err != nil {
		return nil
	}

	var suites []string
	seen := make(map[string]bool)
	inTarget := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "ze-functional-test:") {
			inTarget = true
			continue
		}
		if !inTarget {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			break
		}

		suite, ok := zeTestSuiteFromMakeLine(line)
		if !ok || seen[suite] {
			continue
		}
		seen[suite] = true
		suites = append(suites, suite)
	}
	return suites
}

func readMakefileLines(root, rel string, seen map[string]bool) ([]string, error) {
	rel = filepath.Clean(rel)
	if seen[rel] {
		return nil, nil
	}
	seen[rel] = true

	lines, err := readLines(filepath.Join(root, rel))
	if err != nil {
		return nil, err
	}

	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if strings.HasPrefix(line, "\t") || len(fields) < 2 || (fields[0] != "include" && fields[0] != "-include" && fields[0] != "sinclude") {
			out = append(out, line)
			continue
		}

		for _, inc := range fields[1:] {
			if strings.ContainsAny(inc, "$*?[{(") {
				out = append(out, line)
				continue
			}
			// GNU make resolves include paths relative to the directory make
			// was invoked from (repo root here), NOT relative to the including
			// makefile. A nested `include mk/test-fuzz-targets.mk` inside
			// mk/test-fuzz.mk therefore means <root>/mk/test-fuzz-targets.mk,
			// not <root>/mk/mk/test-fuzz-targets.mk. Joining with the including
			// file's dir mis-resolved it, made the hard include fail to read,
			// and aborted the whole Makefile parse -- which silently emptied
			// the derived ze-functional-test suite list ("could not derive
			// ze-functional-test suites from Makefile"). Resolve relative to
			// root, matching make.
			incRel := filepath.Clean(inc)
			incLines, err := readMakefileLines(root, incRel, seen)
			if err != nil {
				if fields[0] == "-include" || fields[0] == "sinclude" {
					continue
				}
				return nil, err
			}
			out = append(out, incLines...)
		}
	}
	return out, nil
}

func zeTestSuiteFromMakeLine(line string) (string, bool) {
	fields := strings.Fields(line)
	for i, field := range fields {
		// The run-suite lines invoke the test binary either literally
		// (bin/ze-test) or through the ZE_SUFFIX indirection variable
		// ($(ZE_TEST_RUN), which expands to bin/ze-test or the suffixed
		// path). Both name the same suite in the next field, so accept
		// either token. See mk/test-functional.mk ZE_SUFFIX block.
		if field != "bin/ze-test" && field != "$(ZE_TEST_RUN)" {
			continue
		}
		if i+1 >= len(fields) {
			return "", false
		}
		if fields[i+1] == "bgp" {
			if i+2 >= len(fields) {
				return "", false
			}
			return strings.TrimRight(fields[i+2], ";"), true
		}
		return strings.TrimRight(fields[i+1], ";"), true
	}
	return "", false
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
		// Skip dotfile-prefixed directories (e.g. .ruff_cache, a Python
		// linter's cache dir left behind by scenario check.py scripts) --
		// never a real scenario, but counted as one before this filter,
		// inflating the doc-claimed count by one per such cache dir.
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	return count
}

// countMatchingLines returns how many lines of path match re.
//
// A scan that stops early is recorded through noteUnreadable. It yields a LOW
// count, and a low count is the direction that agrees with a document claiming
// fewer tests than the tree holds.
func countMatchingLines(path string, re *regexp.Regexp) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if re.MatchString(scanner.Text()) {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		noteUnreadable(path, err)
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
		count += countMatchingLines(path, re)
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
			count += countMatchingLines(path, re)
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

		// A FLOOR claim ("100+ interop scenarios") is satisfied by any real count
		// at or above it; an exact claim ("106 interop scenarios") still has to
		// match. Prose pinning a number that grows on its own schedule costs a
		// doc edit and a red gate every time somebody adds a scenario -- this one
		// line was corrected twice in a single day, for no reader's benefit. The
		// floor keeps the guarantee that matters, which is that the page never
		// overclaims, and drops the churn. A bare number is still accepted and
		// still checked exactly, for counts an author does want pinned.
		if m, isFloor := extractFloorCount(line, `(\d+)(\+?) interop scenario`); m > 0 {
			var tb textbuf.Buffer
			switch {
			case isFloor && interopCount < m:
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: tb.Str("claims at least ").Int(int64(m)).
						Str(" interop scenarios, actual is ").Int(int64(interopCount)).String(),
				})
			case !isFloor && m != interopCount:
				issues = append(issues, issue{
					File: "docs/DESIGN.md", Line: lineNum,
					Message: tb.Str("claims ").Int(int64(m)).
						Str(" interop scenarios, actual is ").Int(int64(interopCount)).
						Str(" (write ").Int(int64(m)).
						Str("+ for a floor that needs no edit when a scenario is added)").String(),
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

func checkReadmeMD(root string, ciTotal, interopCount, fuzzCount, goTestCount int) []issue {
	path := filepath.Join(root, "README.md")
	lines, err := readLines(path)
	if err != nil {
		return nil
	}

	var issues []issue
	for i, line := range lines {
		lineNum := i + 1
		if iss, ok := checkReadmeCount(line, lineNum, `unit tests`, "unit tests", goTestCount); ok {
			issues = append(issues, iss)
		}
		if m := extractApprox(line, `(?i)roughly ([\d,]+).*functional tests`); m > 0 && !withinThreshold(m, ciTotal, 0.20) {
			issues = append(issues, issue{
				File: "README.md", Line: lineNum,
				Message: fmt.Sprintf("claims roughly %d functional tests, actual is %d", m, ciTotal),
			})
		}
		if iss, ok := checkReadmeCount(line, lineNum, `fuzz targets`, "fuzz targets", fuzzCount); ok {
			issues = append(issues, iss)
		}
		if iss, ok := checkReadmeCount(line, lineNum, `Docker-based interop scenarios`, "Docker-based interop scenarios", interopCount); ok {
			issues = append(issues, iss)
		}
	}
	return issues
}

// checkReadmeCount evaluates one `<number>[+] <unit>` headline test-count claim
// on a README line with modifier-aware semantics:
//   - `N+ <unit>` (at-least, soft): drift only when actual < N. This keeps the
//     R-1 anti-re-drift default -- headroom above the claimed floor is intended
//     and tolerated as the tree grows.
//   - bare `N <unit>` (exact): drift when actual != N in EITHER direction. This
//     catches bare over-claims AND undercounts that the previous
//     `([\d,]+)\+`-only regex could not see (e.g. a bare `57 fuzz targets` or a
//     bare `10,000 unit tests`).
//
// RE2 has no look-ahead, so the optional `+` is captured as its own group and
// inspected to tell bare from at-least apart.
func checkReadmeCount(line string, lineNum int, unit, label string, actual int) (issue, bool) {
	re := regexp.MustCompile(`([\d,]+)(\+?)\s+` + unit)
	m := re.FindStringSubmatch(line)
	if len(m) < 3 {
		return issue{}, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil || n <= 0 {
		return issue{}, false
	}
	if m[2] == "+" {
		if actual < n {
			var tb textbuf.Buffer
			tb.Str("claims ").Int(int64(n)).Str("+ ").Str(label).Str(", actual is ").Int(int64(actual))
			return issue{File: "README.md", Line: lineNum, Message: tb.String()}, true
		}
		return issue{}, false
	}
	if n != actual {
		var tb textbuf.Buffer
		tb.Str("claims ").Int(int64(n)).Byte(' ').Str(label).Str(" (bare exact count), actual is ").Int(int64(actual))
		return issue{File: "README.md", Line: lineNum, Message: tb.String()}, true
	}
	return issue{}, false
}

func checkFeaturesMD(root string) []issue {
	path := filepath.Join(root, "docs", "features.md")
	lines, err := readLines(path)
	if err != nil {
		return nil
	}

	allowed := map[string]bool{
		"supported":    true,
		"partial":      true,
		"experimental": true,
		"stub-backed":  true,
		"rejected":     true,
		"future":       true,
	}

	var issues []issue
	foundHeader := false
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| Feature |") {
			foundHeader = true
			if !strings.Contains(trimmed, "| Status |") || !strings.Contains(trimmed, "| Description |") {
				issues = append(issues, issue{
					File: "docs/features.md", Line: lineNum,
					Message: "feature inventory table must include Feature, Status, and Description columns",
				})
			}
			continue
		}
		if !foundHeader || !strings.HasPrefix(trimmed, "|") || strings.Contains(trimmed, "---") {
			continue
		}
		cells := splitTableRow(trimmed)
		if len(cells) < 3 {
			issues = append(issues, issue{
				File: "docs/features.md", Line: lineNum,
				Message: "feature inventory row must include status",
			})
			continue
		}
		status := strings.ToLower(strings.TrimSpace(cells[1]))
		if !allowed[status] {
			issues = append(issues, issue{
				File: "docs/features.md", Line: lineNum,
				Message: fmt.Sprintf("unknown feature status %q", cells[1]),
				Detail:  "valid statuses: supported, partial, experimental, stub-backed, rejected, future",
			})
		}
	}
	if !foundHeader {
		issues = append(issues, issue{
			File:    "docs/features.md",
			Line:    0,
			Message: "feature inventory table not found",
		})
	}
	return issues
}

func checkFunctionalTestsMD(root string, gateSuites []string) []issue {
	path := filepath.Join(root, "docs", "functional-tests.md")
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	if len(gateSuites) == 0 {
		return []issue{{
			File:    "docs/functional-tests.md",
			Line:    0,
			Message: "could not derive ze-functional-test suites from Makefile",
		}}
	}

	var issues []issue
	joined := strings.Join(lines, " ")
	re := regexp.MustCompile(`functional test target runs (\d+) suites: ([^.]+)\.`)
	m := re.FindStringSubmatch(joined)
	if len(m) < 3 {
		issues = append(issues, issue{
			File:    "docs/functional-tests.md",
			Line:    0,
			Message: "could not find release gate suite list",
			Detail:  "expected: functional test target runs N suites: a, b, c.",
		})
	} else {
		claimedCount, _ := strconv.Atoi(m[1])
		claimedSuites := splitSuiteList(m[2])
		if claimedCount != len(gateSuites) {
			issues = append(issues, issue{
				File: "docs/functional-tests.md", Line: lineNumberContaining(lines, m[0]),
				Message: fmt.Sprintf("claims %d release-gate suites, Makefile has %d", claimedCount, len(gateSuites)),
				Detail:  fmt.Sprintf("Makefile: %s", strings.Join(gateSuites, ", ")),
			})
		}
		if !sameStrings(claimedSuites, gateSuites) {
			issues = append(issues, issue{
				File: "docs/functional-tests.md", Line: lineNumberContaining(lines, m[0]),
				Message: "release-gate suite list does not match Makefile",
				Detail:  fmt.Sprintf("docs: %s; Makefile: %s", strings.Join(claimedSuites, ", "), strings.Join(gateSuites, ", ")),
			})
		}
	}

	manualSuites := extractTableColumn(lines, "Suite", "Runner", 0)
	gateSet := make(map[string]bool, len(gateSuites))
	for _, suite := range gateSuites {
		gateSet[strings.ToLower(suite)] = true
	}
	for _, suite := range manualSuites {
		name := strings.ToLower(strings.Trim(strings.TrimSpace(suite), "`"))
		if gateSet[name] {
			issues = append(issues, issue{
				File: "docs/functional-tests.md", Line: 0,
				Message: fmt.Sprintf("gated suite %q is listed as manual-only", suite),
			})
		}
	}
	return issues
}

func checkMakefileHelp(root string, gateSuites []string) []issue {
	path := filepath.Join(root, "Makefile")
	lines, err := readLines(path)
	if err != nil || len(gateSuites) == 0 {
		return nil
	}

	listRe := regexp.MustCompile(`ze-functional-test\s+- Run ze functional tests \(([^)]*)\)`)
	countRe := regexp.MustCompile(`ze-functional-test\s+All\s+(\d+)\s+gating suites`)
	for i, line := range lines {
		m := listRe.FindStringSubmatch(line)
		if len(m) < 2 {
			if count := countRe.FindStringSubmatch(line); len(count) == 2 {
				claimed, _ := strconv.Atoi(count[1])
				if claimed == len(gateSuites) {
					return nil
				}
				return []issue{{
					File:    "Makefile",
					Line:    i + 1,
					Message: fmt.Sprintf("ze-functional-test help claims %d suites, target has %d", claimed, len(gateSuites)),
					Detail:  fmt.Sprintf("target: %s", strings.Join(gateSuites, ", ")),
				}}
			}
			continue
		}
		claimed := splitSuiteList(m[1])
		if sameStrings(claimed, gateSuites) {
			return nil
		}
		return []issue{{
			File:    "Makefile",
			Line:    i + 1,
			Message: "ze-functional-test help suite list does not match target",
			Detail:  fmt.Sprintf("help: %s; target: %s", strings.Join(claimed, ", "), strings.Join(gateSuites, ", ")),
		}}
	}

	return []issue{{
		File:    "Makefile",
		Line:    0,
		Message: "ze-functional-test help line not found",
	}}
}

func splitSuiteList(raw string) []string {
	parts, count := stringsx.SplitCount(raw, ",")
	out := make([]string, 0, count)
	for _, part := range parts {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "and "))
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func lineNumberContaining(lines []string, fragment string) int {
	for i, line := range lines {
		if strings.Contains(line, fragment) {
			return i + 1
		}
	}
	return 0
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
	if err := scanner.Err(); err != nil {
		// Callers skip a file that fails to open, because an absent document is
		// not drift. A file that opened and then stopped is a different fact,
		// so it is recorded here rather than left to that skip.
		noteUnreadable(path, err)
		return nil, err
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

// extractFloorCount reads a count that may be written as a floor ("100+").
//
// The pattern must expose two groups: the digits, and an optional literal `+`.
// isFloor reports whether the `+` was present, so the caller can require
// actual >= claimed instead of equality. Returns 0 when the line does not match,
// which the callers already treat as "no claim on this line".
func extractFloorCount(line, pattern string) (n int, isFloor bool) {
	m := regexp.MustCompile(pattern).FindStringSubmatch(line)
	if len(m) < 3 {
		return 0, false
	}
	n, _ = strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	return n, m[2] == "+"
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
