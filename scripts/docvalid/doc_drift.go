// Design: (none -- build tool)
//
// check-doc-drift compares documentation claims against the live plugin
// registry, Makefile gates, filesystem counts, and structured doc tables, and
// reports every claim the tree disagrees with.
//
// Usage: CGO_ENABLED=0 go run scripts/docvalid/doc_drift.go [--strict]
// Called by: make ze-doc-drift-check. Exit 0 means no drift. Exit 1 means drift,
// and exit 2 means drift with --strict. No hook calls it. The old header named
// .claude/hooks/check-doc-drift.sh, but that file does not exist.
//
//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func main() {
	strict := flag.Bool("strict", false, "exit 2 instead of 1 when drift is found")
	rootFlag := flag.String("root", "", "repository root to check")
	writeGenerated := flag.Bool("write-generated", false,
		"rewrite the generated documentation this tool checks, instead of checking it")
	commandCatalogPath := flag.String("command-catalog", "",
		"read the live command catalog from this JSON file instead of generating it")
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

	if *writeGenerated {
		if err := writeGeneratedDocs(root); err != nil {
			fmt.Fprintln(os.Stderr, "check-doc-drift:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", pipeOperatorReferencePath)
		return
	}

	issues := runChecks(root, *commandCatalogPath)

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

func runChecks(root, commandCatalogPath string) []issue {
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
	issues = append(issues, checkPipeOperatorReference(root, commandCatalogPath)...)
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

// leFunctionalSuite is one row of `le functional --list --json`: a suite, and
// whether the gating run runs it.
type leFunctionalSuite struct {
	Name   string `json:"name"`
	Gating bool   `json:"gating"`
}

// functionalGateSuites returns the suites `make ze-functional-test` runs, by
// ASKING `le` for its own table.
//
// This walked the Makefile until the suites, their budgets and the run logic
// moved to scripts/le/application/functional.py. It followed $(MAKE)
// delegation and read `$(ZE_TEST_RUN) <suite>` out of the recipe, and neither
// survives a recipe that delegates to a program: the derivation goes empty,
// and an empty derivation here is reported as drift in the DOCUMENT, which
// sends a reader to edit prose that was never wrong.
//
// An empty answer is returned on any failure, and BOTH callers turn it into a
// loud "could not derive" finding rather than a silent pass. That is load
// bearing rather than tidy. The population used to come from parsing a file in
// the tree, which cannot fail halfway. It now comes from running a program,
// which can be missing, wedged or broken. A caller that read empty as "no
// suites to check" would disable itself on exactly that failure, and say
// nothing.
func functionalGateSuites(root string) []string {
	// Bounded: this runs inside ze-doc-verify, and a wedged `le` would
	// otherwise hold that gate open with no output and no deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := osexec.CommandContext(ctx, filepath.Join(root, "le"),
		"functional", "--list", "--json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var rows []leFunctionalSuite
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil
	}
	var suites []string
	for _, row := range rows {
		if row.Gating {
			suites = append(suites, row.Name)
		}
	}
	return suites
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
	// Same order and the same reason as checkMakefileHelp: the read decides
	// whether there is anything here to check, and only then does an empty
	// population mean the derivation failed.
	if len(gateSuites) == 0 {
		return []issue{{
			File:    "docs/functional-tests.md",
			Line:    0,
			Message: "could not derive ze-functional-test suites from `le functional --list --json`",
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
	if err != nil {
		return nil
	}
	// Emptiness is judged AFTER the read and separately from it. The read
	// answers "is there anything here to check", and a fixture root with no
	// Makefile must stay silent. Once the file exists, an empty population
	// means the derivation FAILED, and reading that as "no suites to check"
	// would disable this check on exactly the failure it must report.
	if len(gateSuites) == 0 {
		return []issue{{
			File:    "Makefile",
			Line:    0,
			Message: "could not derive ze-functional-test suites from `le functional --list --json`",
		}}
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

// pipeOperatorReferencePath is the generated operator table every documentation
// page links to instead of listing the operators itself.
const pipeOperatorReferencePath = "docs/features/pipe-operators.generated.md"

// checkPipeOperatorReference fails when either the global operator table or a
// generated per-command surface disagrees with its live catalog.
func checkPipeOperatorReference(root, commandCatalogPath string) []issue {
	issues := checkGlobalPipeOperatorReference(root)

	return append(issues, checkPublishedCommandSurfaces(root, commandCatalogPath)...)
}
func checkGlobalPipeOperatorReference(root string) []issue {
	featuresDir := filepath.Join(root, "docs", "features")
	info, err := os.Stat(featuresDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []issue{{
			File:    pipeOperatorReferencePath,
			Message: "could not inspect the generated pipe operator reference",
			Detail:  err.Error(),
		}}
	}
	if !info.IsDir() {
		return nil
	}

	path := filepath.Join(root, pipeOperatorReferencePath)
	published, err := os.ReadFile(path) //nolint:gosec // repository-relative documentation path
	if err != nil {
		return []issue{{
			File:    pipeOperatorReferencePath,
			Message: "the generated pipe operator reference is missing",
			Detail:  "run `make ze-docs-pipe-operators-update`",
		}}
	}
	if string(published) == command.RenderOperatorReference() {
		return nil
	}
	return []issue{{
		File:    pipeOperatorReferencePath,
		Message: "the published pipe operator table and the operator catalog disagree",
		Detail: "the catalog in internal/component/command/pipe_catalog.go is the source; " +
			"run `make ze-docs-pipe-operators-update`",
	}}
}

type publishedCommandArg struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
}

type publishedCommandPipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TakesArg    bool   `json:"takes-arg,omitempty"`
}

type publishedCommandOperator struct {
	Name        string `json:"name"`
	Class       string `json:"class"`
	Available   string `json:"available"`
	LocalOnly   bool   `json:"local-only,omitempty"`
	Description string `json:"description"`
}

type publishedCommandAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expansion   string `json:"expansion"`
}

type publishedCommand struct {
	Path          string                     `json:"path"`
	Description   string                     `json:"description,omitempty"`
	Mode          string                     `json:"mode"`
	WireMethod    string                     `json:"wire-method,omitempty"`
	Backend       []string                   `json:"backend,omitempty"`
	TaskSupport   string                     `json:"task-support,omitempty"`
	Args          []publishedCommandArg      `json:"args,omitempty"`
	Pipes         []publishedCommandPipe     `json:"pipes,omitempty"`
	Operators     []publishedCommandOperator `json:"operators,omitempty"`
	AnswerShape   string                     `json:"answer-shape,omitempty"`
	AddressFields []string                   `json:"address-fields,omitempty"`
	Aliases       []publishedCommandAlias    `json:"pipe-aliases,omitempty"`
	Syntax        string                     `json:"syntax,omitempty"`
	Subcommands   []string                   `json:"subcommands,omitempty"`
}

const commandCatalogGenerationTimeout = 2 * time.Minute

var commandSurfaceRendererFiles = []string{
	"models.py",
	"page_registry.py",
	"render-cli-catalog.py",
	"render-command-equivalents.py",
	"render-llms-txt.py",
	"sitefacts.py",
	"sitelib.py",
	"sitepaths.py",
	"zebinary.py",
}

func checkPublishedCommandSurfaces(root, commandCatalogPath string) []issue {
	if commandCatalogPath == "" {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return []issue{{
				File:    commandSurfacePath(root, filepath.Join(root, "go.mod")),
				Message: "could not determine whether the root owns command surfaces",
				Detail:  err.Error(),
			}}
		}
	}
	websiteCandidate := filepath.Join(filepath.Dir(root), "gh-pages", "data", "cli-commands.json")
	wikiCandidate := filepath.Join(filepath.Dir(root), "wiki", "command-catalog.md")
	if commandCatalogPath != "" {
		websiteCandidate = filepath.Join(root, "website", "data", "cli-commands.json")
		wikiCandidate = filepath.Join(root, "wiki", "command-catalog.md")
	}

	websitePaths, websitePathErr := existingPaths(websiteCandidate)
	if websitePathErr != nil {
		return []issue{commandSurfaceReadIssue("website command catalog", websitePathErr)}
	}
	if len(websitePaths) == 0 {
		hasPublishedWebsite, err := publishedWebsiteRootExists(
			filepath.Dir(filepath.Dir(websiteCandidate)), commandCatalogPath != "",
		)
		if err != nil {
			return []issue{commandSurfaceReadIssue("website command surfaces", err)}
		}
		if hasPublishedWebsite {
			return []issue{{
				File:    commandSurfacePath(root, websiteCandidate),
				Message: "the published website command catalog is missing",
				Detail:  "regenerate the website command surfaces before running ze-doc-verify",
			}}
		}
	}
	wikiPaths, wikiPathErr := existingPaths(wikiCandidate)
	if wikiPathErr != nil {
		return []issue{commandSurfaceReadIssue("wiki command catalog", wikiPathErr)}
	}

	liveRaw, live, err := loadLiveCommandCatalog(root, commandCatalogPath)
	if err != nil {
		return []issue{{
			File:    "cmd/ze/help_command.go",
			Message: "could not generate or parse the live per-command catalog",
			Detail:  err.Error(),
		}}
	}

	publicWebsiteRoot := ""
	if len(websitePaths) != 0 {
		publicWebsiteRoot = filepath.Dir(filepath.Dir(websitePaths[0]))
	}
	expectedRoot, err := renderExpectedCommandSurfaces(
		root, commandCatalogPath, publicWebsiteRoot, liveRaw, len(live),
	)
	if err != nil {
		return []issue{{
			File:    "website/tools",
			Message: "could not generate the expected per-command surfaces",
			Detail:  err.Error(),
		}}
	}

	issues := validateGeneratedCommandSurfaces(root, expectedRoot, live)
	if publicWebsiteRoot != "" {
		issues = append(issues,
			compareWebsiteCommandCatalog(root, websitePaths[0], live)...)
		issues = append(issues,
			compareRenderedCommandSurfaces(root, publicWebsiteRoot, expectedRoot, live)...)
	}
	for _, path := range wikiPaths {
		issues = append(issues, compareWikiCommandCatalog(root, path, liveRaw)...)
	}
	if err := os.RemoveAll(expectedRoot); err != nil {
		issues = append(issues, issue{
			File:    commandSurfacePath(root, expectedRoot),
			Message: "could not remove the temporary command surfaces",
			Detail:  err.Error(),
		})
	}
	return issues
}

func renderExpectedCommandSurfaces(
	root, commandCatalogPath, publicWebsiteRoot string,
	liveRaw []byte,
	commandCount int,
) (string, error) {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return "", fmt.Errorf("locate command renderers: %w", err)
	}
	tmpParent := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return "", fmt.Errorf("create command render temporary parent %s: %w", tmpParent, err)
	}
	outputRoot, err := os.MkdirTemp(tmpParent, "docvalid-command-surfaces-")
	if err != nil {
		return "", fmt.Errorf("create command render temporary root: %w", err)
	}
	if err := prepareCommandRenderer(
		root, moduleRoot, commandCatalogPath, publicWebsiteRoot, outputRoot, liveRaw, commandCount,
	); err != nil {
		if removeErr := os.RemoveAll(outputRoot); removeErr != nil {
			return "", fmt.Errorf("%w; remove failed renderer output %s: %v",
				err, outputRoot, removeErr)
		}
		return "", err
	}
	for _, name := range []string{
		"render-cli-catalog.py",
		"render-command-equivalents.py",
		"render-llms-txt.py",
	} {
		if err := runCommandSurfaceRenderer(moduleRoot, outputRoot, name); err != nil {
			if removeErr := os.RemoveAll(outputRoot); removeErr != nil {
				return "", fmt.Errorf("%w; remove failed renderer output %s: %v",
					err, outputRoot, removeErr)
			}
			return "", err
		}
	}
	return outputRoot, nil
}

func prepareCommandRenderer(
	root, moduleRoot, commandCatalogPath, publicWebsiteRoot, outputRoot string,
	liveRaw []byte,
	commandCount int,
) error {
	toolsOutput := filepath.Join(outputRoot, "tools")
	if err := os.MkdirAll(toolsOutput, 0o755); err != nil {
		return fmt.Errorf("create temporary renderer tools directory: %w", err)
	}
	for _, name := range commandSurfaceRendererFiles {
		source := filepath.Join(moduleRoot, "website", "tools", name)
		override := filepath.Join(root, "website", "tools", name)
		if commandCatalogPath != "" {
			exists, err := optionalCommandSurfacePath(override)
			if err != nil {
				return fmt.Errorf("inspect command renderer override %s: %w", override, err)
			}
			if exists {
				source = override
			}
		}
		if err := copyCommandSurfaceFile(source, filepath.Join(toolsOutput, name)); err != nil {
			return fmt.Errorf("copy command renderer %s: %w", name, err)
		}
	}

	dataSource := filepath.Join(moduleRoot, "website", "data")
	if publicWebsiteRoot != "" {
		if commandCatalogPath == "" {
			dataSource = filepath.Join(publicWebsiteRoot, "data")
		}
	}
	dataOutput := filepath.Join(outputRoot, "data")
	if err := copyCommandSurfaceTree(dataSource, dataOutput, false); err != nil {
		return fmt.Errorf("copy command renderer data from %s: %w", dataSource, err)
	}
	if commandCatalogPath != "" {
		if err := writeCommandSurfaceFixtureData(dataOutput, commandCount); err != nil {
			return err
		}
	}
	if publicWebsiteRoot == "" {
		if err := writeMissingCommandSurfaceData(dataOutput, commandCount); err != nil {
			return err
		}
	}
	if err := os.WriteFile(
		filepath.Join(dataOutput, "cli-commands.json"), liveRaw, 0o644,
	); err != nil {
		return fmt.Errorf("write live command catalog for renderers: %w", err)
	}

	useCasesSource := filepath.Join(moduleRoot, "website", "use-cases")
	if publicWebsiteRoot != "" {
		publishedUseCases := filepath.Join(publicWebsiteRoot, "use-cases")
		exists, err := optionalCommandSurfacePath(publishedUseCases)
		if err != nil {
			return fmt.Errorf("inspect published use-case sources %s: %w", publishedUseCases, err)
		}
		if exists {
			useCasesSource = publishedUseCases
		}
	}
	if err := copyCommandSurfaceTree(
		useCasesSource, filepath.Join(outputRoot, "use-cases"), true,
	); err != nil {
		return fmt.Errorf("copy command renderer use-case sources: %w", err)
	}
	return nil
}

func writeCommandSurfaceFixtureData(dataRoot string, commandCount int) error {
	mapping := `{
  "schema-version": 1,
  "summary": "Docvalid renderer fixture.",
  "vendors": {
    "fixture": {
      "label": "Fixture",
      "short-label": "Fixture",
      "rooting-model": "fixture-rooted",
      "documentation": []
    }
  },
  "entries": []
}
`
	if err := os.WriteFile(
		filepath.Join(dataRoot, "command-equivalents.json"), []byte(mapping), 0o644,
	); err != nil {
		return fmt.Errorf("write command-equivalents renderer fixture: %w", err)
	}
	return writeMissingCommandSurfaceData(dataRoot, commandCount)
}

func writeMissingCommandSurfaceData(dataRoot string, commandCount int) error {
	var facts textbuf.Buffer
	facts.Str(`{
  "features": {"core_experimental": 0, "planned": 0},
  "tests": {"unit_display": "0", "fuzz_display": "0", "e2e_display": "0"},
  "interop": {"scenarios": 0, "target_display": "0"},
  "cli_commands": `).Int(int64(commandCount)).Str(`,
  "config_sections": 0,
  "dependencies": 0,
  "changes": 0,
  "blog_articles": 0,
  "generated_at": "docvalid fixture"
}
`)
	for name, content := range map[string]string{
		"plugin-registry.json":  "[]\n",
		"site-facts.json":       facts.String(),
		"yang-config-tree.json": "{}\n",
	} {
		path := filepath.Join(dataRoot, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s renderer fixture: %w", name, err)
		}
	}
	return nil
}

func copyCommandSurfaceTree(source, target string, markdownOnly bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if markdownOnly {
			if filepath.Ext(path) != ".md" {
				return nil
			}
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyCommandSurfaceFile(path, targetPath)
	})
}

func copyCommandSurfaceFile(source, target string) error {
	data, err := os.ReadFile(source) //nolint:gosec // repository renderer or generated public artifact
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644) //nolint:gosec // isolated temporary renderer output
}

func runCommandSurfaceRenderer(moduleRoot, outputRoot, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	path := filepath.Join(outputRoot, "tools", name)
	cmd := osexec.CommandContext(ctx, "python3", path)
	cmd.Dir = outputRoot
	var envValue textbuf.Buffer
	mainRepoEnv := envValue.Str("ZE_MAIN_REPO=").Str(moduleRoot).String()
	envValue.Reset()
	repoRootEnv := envValue.Str("ZE_REPO_ROOT=").Str(moduleRoot).String()
	envValue.Reset()
	siteOutputEnv := envValue.Str("ZE_SITE_OUTPUT=").Str(outputRoot).String()
	cmd.Env = append(os.Environ(),
		"PYTHONDONTWRITEBYTECODE=1",
		"ZE_CLI_CATALOG_USE_CACHE=1",
		mainRepoEnv,
		repoRootEnv,
		siteOutputEnv,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run canonical command renderer %s: %w: %s",
			name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func optionalCommandSurfacePath(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func publishedWebsiteRootExists(path string, fixture bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect published website root %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("published website root %s is not a directory", path)
	}
	if !fixture {
		return true, nil
	}
	for _, relative := range []string{"data", "reference", "llms.txt"} {
		candidate := filepath.Join(path, relative)
		exists, err := optionalCommandSurfacePath(candidate)
		if err != nil {
			return false, fmt.Errorf("inspect published website surface %s: %w", candidate, err)
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func existingPaths(paths ...string) ([]string, error) {
	var existing []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return nil, fmt.Errorf("%s is a directory", path)
			}
			existing = append(existing, path)
			continue
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		parent, parentErr := os.Stat(filepath.Dir(path))
		if parentErr == nil {
			if !parent.IsDir() {
				return nil, fmt.Errorf("%s is not a directory", filepath.Dir(path))
			}
			return nil, fmt.Errorf("%s is missing", path)
		}
		if !os.IsNotExist(parentErr) {
			return nil, fmt.Errorf("inspect %s: %w", filepath.Dir(path), parentErr)
		}
	}
	return existing, nil
}

func commandSurfaceReadIssue(surface string, err error) issue {
	return issue{
		File:    surface,
		Message: "could not read the published per-command surface",
		Detail:  err.Error(),
	}
}

func loadLiveCommandCatalog(root, commandCatalogPath string) ([]byte, []publishedCommand, error) {
	if commandCatalogPath != "" {
		data, err := os.ReadFile(commandCatalogPath) //nolint:gosec // caller-selected fixture or repository artifact
		if err != nil {
			return nil, nil, fmt.Errorf("read command catalog %s: %w", commandCatalogPath, err)
		}
		commands, err := parseCommandCatalog(commandCatalogPath, data)
		return data, commands, err
	}

	tags, err := shippedCommandCatalogTags(root)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	args := []string{"run", "-tags", strings.Join(tags, ","), "./cmd/ze", "help", "command", "--json"}
	cmd := osexec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("generate `ze help command --json`: %w: %s",
			err, strings.TrimSpace(stderr.String()))
	}
	commands, err := parseCommandCatalog("ze help command --json", data)
	return data, commands, err
}

func shippedCommandCatalogTags(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt for command generation: %w", err)
	}
	tags := []string{"ze_core"}
	seen := map[string]bool{"ze_core": true}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(fields[0], "#") {
			continue
		}
		if !seen[fields[0]] {
			seen[fields[0]] = true
			tags = append(tags, fields[0])
		}
	}
	return tags, nil
}

func parseCommandCatalog(source string, data []byte) ([]publishedCommand, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var commands []publishedCommand
	if err := decoder.Decode(&commands); err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse %s: content follows the command array", source)
		}
		return nil, fmt.Errorf("parse %s after command array: %w", source, err)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("parse %s: command array is empty", source)
	}
	seen := make(map[string]bool, len(commands))
	for _, entry := range commands {
		if err := validatePublishedCommand(source, entry, seen); err != nil {
			return nil, err
		}
		seen[entry.Path] = true
	}
	return commands, nil
}

func validatePublishedCommand(source string, entry publishedCommand, seen map[string]bool) error {
	if entry.Path == "" {
		return fmt.Errorf("parse %s: command has an empty path", source)
	}
	if seen[entry.Path] {
		return fmt.Errorf("parse %s: command path %q appears twice", source, entry.Path)
	}
	if entry.Mode == "" {
		return fmt.Errorf("parse %s: command %q has no mode", source, entry.Path)
	}
	switch entry.AnswerShape {
	case "", "doc", "map", "tab":
	default:
		return fmt.Errorf("parse %s: command %q has unknown answer shape %q",
			source, entry.Path, entry.AnswerShape)
	}
	for _, arg := range entry.Args {
		if arg.Name == "" {
			return fmt.Errorf("parse %s: command %q has an argument without a name", source, entry.Path)
		}
		if arg.Type == "" {
			return fmt.Errorf("parse %s: command %q argument %q has no kind", source, entry.Path, arg.Name)
		}
	}
	for _, pipe := range entry.Pipes {
		if pipe.Name == "" {
			return fmt.Errorf("parse %s: command %q has a filter without a name", source, entry.Path)
		}
		if pipe.Description == "" {
			return fmt.Errorf("parse %s: command %q filter %q has no description", source, entry.Path, pipe.Name)
		}
	}
	for _, field := range entry.AddressFields {
		if field == "" {
			return fmt.Errorf("parse %s: command %q has an empty address field", source, entry.Path)
		}
	}
	for _, op := range entry.Operators {
		if op.Name == "" {
			return fmt.Errorf("parse %s: command %q has an operator without a name", source, entry.Path)
		}
		if op.Class == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no class", source, entry.Path, op.Name)
		}
		if op.Description == "" {
			return fmt.Errorf("parse %s: command %q operator %q has no description", source, entry.Path, op.Name)
		}
		switch op.Available {
		case "always", "with-rows", "when-streaming":
		default:
			return fmt.Errorf("parse %s: command %q operator %q has unknown availability %q",
				source, entry.Path, op.Name, op.Available)
		}
	}
	for _, alias := range entry.Aliases {
		if alias.Name == "" {
			return fmt.Errorf("parse %s: command %q has an alias without a name", source, entry.Path)
		}
		if alias.Description == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no description", source, entry.Path, alias.Name)
		}
		if alias.Expansion == "" {
			return fmt.Errorf("parse %s: command %q alias %q has no expansion", source, entry.Path, alias.Name)
		}
	}
	return nil
}

func compareWebsiteCommandCatalog(root, path string, live []publishedCommand) []issue {
	publishedRaw, err := os.ReadFile(path) //nolint:gosec // generated sibling checkout artifact
	if err != nil {
		return []issue{commandSurfaceReadIssue(commandSurfacePath(root, path), err)}
	}
	published, err := parseCommandCatalog(commandSurfacePath(root, path), publishedRaw)
	if err != nil {
		return []issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not parse the published website command catalog",
			Detail:  err.Error(),
		}}
	}
	for i := range published {
		// The website derives display syntax from the canonical description.
		// It is not part of `ze help command --json`.
		published[i].Syntax = ""
	}
	liveJSON, err := json.Marshal(live)
	if err != nil {
		return []issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not encode the live website command catalog",
			Detail:  err.Error(),
		}}
	}
	publishedJSON, err := json.Marshal(published)
	if err != nil {
		return []issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not encode the published website command catalog",
			Detail:  err.Error(),
		}}
	}
	if bytes.Equal(liveJSON, publishedJSON) {
		return nil
	}
	return []issue{{
		File:    commandSurfacePath(root, path),
		Message: "the published website command catalog and the live command catalog disagree",
		Detail: "regenerate the website CLI surface; every command's operators, qualifiers, aliases, " +
			"filters, shape, address fields, descriptions, and argument kinds must match",
	}}
}

func compareWikiCommandCatalog(root, path string, liveRaw []byte) []issue {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return []issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not locate the wiki command generator",
			Detail:  err.Error(),
		}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandCatalogGenerationTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "python3",
		filepath.Join(moduleRoot, "scripts", "dev", "gen_wiki_commands.py"))
	cmd.Dir = moduleRoot
	cmd.Stdin = bytes.NewReader(liveRaw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	want, err := cmd.Output()
	if err != nil {
		var detail textbuf.Buffer
		return []issue{{
			File:    commandSurfacePath(root, path),
			Message: "could not generate the expected wiki command catalog",
			Detail: detail.Err(err).Str(": ").
				Str(strings.TrimSpace(stderr.String())).String(),
		}}
	}
	published, err := os.ReadFile(path) //nolint:gosec // generated sibling checkout artifact
	if err != nil {
		return []issue{commandSurfaceReadIssue(commandSurfacePath(root, path), err)}
	}
	if bytes.Equal(published, want) {
		return nil
	}
	return []issue{{
		File:    commandSurfacePath(root, path),
		Message: "the published wiki command catalog and the live command catalog disagree",
		Detail:  "run `make ze-wiki-commands-update`; the wiki must preserve every per-command contract field",
	}}
}

var commandSurfaceSlugSeparator = regexp.MustCompile(`[^a-z0-9]+`)

func validateGeneratedCommandSurfaces(
	root, expectedRoot string,
	live []publishedCommand,
) []issue {
	primaryHTMLPath := filepath.Join(expectedRoot, "reference", "cli", "index.html")
	primaryMarkdownPath := filepath.Join(expectedRoot, "reference", "cli", "index.md")
	llmsPath := filepath.Join(expectedRoot, "llms.txt")
	equivalentHTMLPath := filepath.Join(
		expectedRoot, "reference", "command-equivalents", "index.html",
	)
	equivalentMarkdownPath := filepath.Join(
		expectedRoot, "reference", "command-equivalents", "index.md",
	)
	primaryHTML, err := os.ReadFile(primaryHTMLPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []issue{generatedCommandSurfaceReadIssue(root, primaryHTMLPath, err)}
	}
	primaryMarkdown, err := os.ReadFile(primaryMarkdownPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []issue{generatedCommandSurfaceReadIssue(root, primaryMarkdownPath, err)}
	}
	llms, err := os.ReadFile(llmsPath) //nolint:gosec // isolated renderer output
	if err != nil {
		return []issue{generatedCommandSurfaceReadIssue(root, llmsPath, err)}
	}
	equivalentHTML, err := os.ReadFile(equivalentHTMLPath) //nolint:gosec // rendered command surface
	if err != nil {
		return []issue{generatedCommandSurfaceReadIssue(root, equivalentHTMLPath, err)}
	}
	equivalentMarkdown, err := os.ReadFile(equivalentMarkdownPath) //nolint:gosec // rendered command surface
	if err != nil {
		return []issue{generatedCommandSurfaceReadIssue(root, equivalentMarkdownPath, err)}
	}

	var issues []issue
	for _, command := range live {
		slug := commandSurfaceSlug(command.Path)
		var equivalentHTMLMarker textbuf.Buffer
		if !strings.Contains(string(equivalentHTML),
			equivalentHTMLMarker.Str(`id="cmd-eq-`).Str(slug).Byte('"').String()) {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, equivalentHTMLPath), command.Path,
				"command-equivalent HTML index row",
			))
		}
		var equivalentMarkdownMarker textbuf.Buffer
		if !strings.Contains(string(equivalentMarkdown),
			equivalentMarkdownMarker.Str("](").Str(slug).Str("/)").String()) {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, equivalentMarkdownPath), command.Path,
				"command-equivalent Markdown index row",
			))
		}
		primaryRow, ok := commandSurfaceHTMLRow(string(primaryHTML), slug)
		if !ok {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, primaryHTMLPath), command.Path,
				"primary CLI HTML command row",
			))
		} else {
			issues = append(issues, validatePrimaryCommandContract(
				commandSurfacePath(root, primaryHTMLPath), primaryRow, command,
			)...)
		}

		primaryMarkdownRow, ok := commandSurfaceMarkdownRow(
			string(primaryMarkdown), command.Path,
		)
		if !ok {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, primaryMarkdownPath), command.Path,
				"primary CLI Markdown command row",
			))
		} else {
			issues = append(issues, validatePrimaryMarkdownContract(
				commandSurfacePath(root, primaryMarkdownPath),
				primaryMarkdownRow,
				command,
			)...)
		}

		detailHTMLPath := filepath.Join(
			expectedRoot, "reference", "command-equivalents", slug, "index.html",
		)
		detailMarkdownPath := filepath.Join(
			expectedRoot, "reference", "command-equivalents", slug, "index.md",
		)
		detailHTML, detailHTMLErr := os.ReadFile(detailHTMLPath) //nolint:gosec // isolated renderer output
		if detailHTMLErr != nil {
			issues = append(issues, generatedCommandSurfaceReadIssue(
				root, detailHTMLPath, detailHTMLErr,
			))
		} else {
			issues = append(issues, validateEquivalentCommandContract(
				commandSurfacePath(root, detailHTMLPath), string(detailHTML), command,
			)...)
		}
		detailMarkdown, detailMarkdownErr := os.ReadFile(detailMarkdownPath) //nolint:gosec // isolated renderer output
		if detailMarkdownErr != nil {
			issues = append(issues, generatedCommandSurfaceReadIssue(
				root, detailMarkdownPath, detailMarkdownErr,
			))
		} else {
			issues = append(issues, validateEquivalentMarkdownContract(
				commandSurfacePath(root, detailMarkdownPath),
				string(detailMarkdown),
				command,
			)...)
		}

		meta, ok := llmsCommandMetadata(string(llms), command.Path)
		if !ok {
			issues = append(issues, generatedCommandContractIssue(
				commandSurfacePath(root, llmsPath), command.Path, "llms.txt command row",
			))
			continue
		}
		issues = append(issues,
			validateLLMSCommandContract(commandSurfacePath(root, llmsPath), meta, command)...)
	}
	return issues
}

func generatedCommandSurfaceReadIssue(root, path string, err error) issue {
	return issue{
		File:    commandSurfacePath(root, path),
		Message: "the canonical command renderer did not produce a required surface",
		Detail:  err.Error(),
	}
}

func generatedCommandContractIssue(path, command, dimension string) issue {
	return issue{
		File:    path,
		Message: "the generated per-command surface dropped part of the live command contract",
		Detail:  missingCommandDimension(command, dimension),
	}
}

func missingCommandDimension(command, dimension string) string {
	var detail textbuf.Buffer
	return detail.Str("command ").Quoted(command).Str(" is missing ").Str(dimension).String()
}

func availabilityCommandDimension(availability, name string) string {
	var dimension textbuf.Buffer
	return dimension.Str(availability).Str(" availability for operator ").
		Quoted(name).String()
}

func namedCommandDimension(kind, name string) string {
	var dimension textbuf.Buffer
	return dimension.Str(kind).Byte(' ').Quoted(name).String()
}

func commandSurfaceSlug(path string) string {
	slug := commandSurfaceSlugSeparator.ReplaceAllString(strings.ToLower(path), "-")
	return strings.Trim(slug, "-")
}

func commandSurfaceHTMLRow(content, slug string) (string, bool) {
	var marker textbuf.Buffer
	startMarker := marker.Str(`<tr id="cmd-`).Str(slug).Str(`">`).String()
	start := strings.Index(content, startMarker)
	if start == -1 {
		return "", false
	}
	remaining := content[start:]
	end := strings.Index(remaining, "</tr>")
	if end == -1 {
		return "", false
	}
	return remaining[:end+len("</tr>")], true
}

func validatePrimaryCommandContract(
	path, row string,
	command publishedCommand,
) []issue {
	var issues []issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("<span>Answer shape</span><code>").
			Str(html.EscapeString(command.AnswerShape)).Str("</code>").String()
		if !strings.Contains(row, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	if len(command.AddressFields) != 0 {
		marker.Reset()
		want := marker.Str("<span>Address fields</span><code>").
			Str(html.EscapeString(strings.Join(command.AddressFields, " · "))).
			Str("</code>").String()
		if !strings.Contains(row, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "address fields",
			))
		}
	}
	for _, operator := range command.Operators {
		label := commandAvailabilityLabel(operator.Available)
		if !commandHTMLGroupContains(row, label, operator.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path,
				availabilityCommandDimension(operator.Available, operator.Name),
			))
		}
		if operator.LocalOnly {
			if !commandHTMLGroupContains(row, "Local process only", operator.Name) {
				issues = append(issues, generatedCommandContractIssue(
					path, command.Path,
					namedCommandDimension("local-only surface qualifier for operator", operator.Name),
				))
			}
		}
	}
	return issues
}

func commandSurfaceMarkdownRow(content, path string) (string, bool) {
	var marker textbuf.Buffer
	prefix := marker.Str("| `").Str(commandMarkdownValue(path)).Str("` |").String()
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line, true
		}
	}
	return "", false
}

func commandMarkdownValue(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func validatePrimaryMarkdownContract(
	path, row string,
	command publishedCommand,
) []issue {
	var issues []issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("Answer shape: `").
			Str(commandMarkdownValue(command.AnswerShape)).Byte('`').String()
		if !strings.Contains(row, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	for _, field := range command.AddressFields {
		if !commandMarkdownGroupContains(row, "Address fields", field) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("address field", field),
			))
		}
	}
	for _, operator := range command.Operators {
		label := commandAvailabilityLabel(operator.Available)
		if !commandMarkdownGroupContains(row, label, operator.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path,
				availabilityCommandDimension(operator.Available, operator.Name),
			))
		}
		if operator.LocalOnly {
			if !commandMarkdownGroupContains(row, "Local process only", operator.Name) {
				issues = append(issues, generatedCommandContractIssue(
					path, command.Path,
					namedCommandDimension("local-only surface qualifier for operator", operator.Name),
				))
			}
		}
	}
	return issues
}

func commandMarkdownGroupContains(content, label, name string) bool {
	var marker textbuf.Buffer
	startMarker := marker.Str(label).Str(": ").String()
	start := strings.Index(content, startMarker)
	if start == -1 {
		return false
	}
	values := content[start+len(startMarker):]
	end := len(values)
	if lineBreak := strings.Index(values, "<br>"); lineBreak != -1 {
		end = lineBreak
	}
	if cellEnd := strings.Index(values, " |"); cellEnd != -1 {
		end = min(end, cellEnd)
	}
	values = values[:end]
	for _, value := range strings.Split(values, ", ") {
		if strings.Trim(value, "`") == commandMarkdownValue(name) {
			return true
		}
	}
	return false
}

func commandAvailabilityLabel(availability string) string {
	switch availability {
	case "always":
		return "Always"
	case "with-rows":
		return "With rows"
	case "when-streaming":
		return "While streaming"
	default:
		return availability
	}
}

func commandHTMLGroupContains(content, label, name string) bool {
	var marker textbuf.Buffer
	startMarker := marker.Str("<span>").Str(html.EscapeString(label)).
		Str("</span><code>").String()
	start := strings.Index(content, startMarker)
	if start == -1 {
		return false
	}
	values := content[start+len(startMarker):]
	end := strings.Index(values, "</code>")
	if end == -1 {
		return false
	}
	for _, value := range strings.Split(values[:end], " · ") {
		if html.UnescapeString(value) == name {
			return true
		}
	}
	return false
}

func validateEquivalentCommandContract(
	path, content string,
	command publishedCommand,
) []issue {
	var issues []issue
	var marker textbuf.Buffer
	if command.AnswerShape != "" {
		want := marker.Str("<dt>Answer shape</dt><dd>").
			Str(html.EscapeString(command.AnswerShape)).Str("</dd>").String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	if len(command.AddressFields) != 0 {
		marker.Reset()
		want := marker.Str("<dt>Address fields</dt><dd>").
			Str(html.EscapeString(strings.Join(command.AddressFields, ", "))).
			Str("</dd>").String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "address fields",
			))
		}
	}
	for _, operator := range command.Operators {
		label := equivalentAvailabilityLabel(operator.Available, command.AnswerShape != "")
		if !equivalentHTMLGroupContains(content, label, operator.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path,
				availabilityCommandDimension(operator.Available, operator.Name),
			))
		}
		if operator.LocalOnly {
			if !equivalentHTMLGroupContains(
				content, "Pipes, local process only", operator.Name,
			) {
				issues = append(issues, generatedCommandContractIssue(
					path, command.Path,
					namedCommandDimension("local-only surface qualifier for operator", operator.Name),
				))
			}
		}
	}
	for _, filter := range command.Pipes {
		marker.Reset().Str("<code>").Str(html.EscapeString(filter.Name))
		if filter.TakesArg {
			marker.Str(" &lt;value&gt;")
		}
		want := marker.Str("</code>: ").
			Str(html.EscapeString(filter.Description)).String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("command filter", filter.Name),
			))
		}
	}
	for _, alias := range command.Aliases {
		marker.Reset()
		want := marker.Str("<code>").Str(html.EscapeString(alias.Name)).
			Str("</code>: ").Str(html.EscapeString(alias.Description)).
			Str(" (<code>").Str(html.EscapeString(alias.Expansion)).
			Str("</code>)").String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("pipe alias", alias.Name),
			))
		}
	}
	return issues
}
func validateEquivalentMarkdownContract(
	path, content string,
	command publishedCommand,
) []issue {
	var issues []issue
	var marker textbuf.Buffer
	registryPath := marker.Str("- Registry path: `").
		Str(commandMarkdownValue(command.Path)).Byte('`').String()
	if !strings.Contains(content, registryPath) {
		issues = append(issues, generatedCommandContractIssue(
			path, command.Path, "registry path",
		))
	}
	if command.AnswerShape != "" {
		marker.Reset()
		want := marker.Str("- Answer shape: ").
			Str(commandMarkdownValue(command.AnswerShape)).String()
		if !strings.Contains(content, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	for _, field := range command.AddressFields {
		if !equivalentMarkdownGroupContains(content, "Address fields", field) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("address field", field),
			))
		}
	}
	for _, operator := range command.Operators {
		label := equivalentMarkdownAvailabilityLabel(operator.Available)
		if !equivalentMarkdownGroupContains(content, label, operator.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path,
				availabilityCommandDimension(operator.Available, operator.Name),
			))
		}
		if operator.LocalOnly {
			if !equivalentMarkdownGroupContains(
				content, "Pipes, local process only", operator.Name,
			) {
				issues = append(issues, generatedCommandContractIssue(
					path, command.Path,
					namedCommandDimension("local-only surface qualifier for operator", operator.Name),
				))
			}
		}
	}
	for _, filter := range command.Pipes {
		marker.Reset().Byte('`').Str(commandMarkdownValue(filter.Name))
		if filter.TakesArg {
			marker.Str(" <value>")
		}
		marker.Byte('`')
		if filter.Description != "" {
			marker.Str(": ").Str(commandMarkdownValue(filter.Description))
		}
		want := marker.String()
		if !strings.Contains(
			commandMarkdownLine(content, "Command pipes"), want,
		) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("command filter", filter.Name),
			))
		}
	}
	for _, alias := range command.Aliases {
		marker.Reset().Byte('`').Str(commandMarkdownValue(alias.Name)).Byte('`')
		if alias.Description != "" {
			marker.Str(": ").Str(commandMarkdownValue(alias.Description))
		}
		if alias.Expansion != "" {
			marker.Str(" (`").Str(commandMarkdownValue(alias.Expansion)).Str("`)")
		}
		want := marker.String()
		if !strings.Contains(
			commandMarkdownLine(content, "Pipe aliases"), want,
		) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("pipe alias", alias.Name),
			))
		}
	}
	return issues
}

func equivalentMarkdownAvailabilityLabel(availability string) string {
	switch availability {
	case "always":
		return "Pipes, always"
	case "with-rows":
		return "Pipes, on rows"
	case "when-streaming":
		return "Pipes, while streaming"
	default:
		return availability
	}
}

func equivalentMarkdownGroupContains(content, label, want string) bool {
	line := commandMarkdownLine(content, label)
	if line == "" {
		return false
	}
	var marker textbuf.Buffer
	values := strings.TrimPrefix(line,
		marker.Str("- ").Str(label).Str(": ").String())
	for _, value := range strings.Split(values, ", ") {
		if value == commandMarkdownValue(want) {
			return true
		}
	}
	return false
}

func commandMarkdownLine(content, label string) string {
	var marker textbuf.Buffer
	prefix := marker.Str("- ").Str(label).Str(": ").String()
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func equivalentAvailabilityLabel(availability string, declaredShape bool) string {
	switch availability {
	case "always":
		return "Pipes, always"
	case "with-rows":
		if declaredShape {
			return "Pipes, on its rows"
		}
		return "Pipes, when the answer has rows"
	case "when-streaming":
		return "Pipes, while streaming"
	default:
		return availability
	}
}

func equivalentHTMLGroupContains(content, label, name string) bool {
	var marker textbuf.Buffer
	startMarker := marker.Str("<dt>").Str(html.EscapeString(label)).
		Str("</dt><dd>").String()
	start := strings.Index(content, startMarker)
	if start == -1 {
		return false
	}
	values := content[start+len(startMarker):]
	end := strings.Index(values, "</dd>")
	if end == -1 {
		return false
	}
	for _, value := range strings.Split(values[:end], ", ") {
		if html.UnescapeString(value) == name {
			return true
		}
	}
	return false
}

func llmsCommandMetadata(content, path string) (string, bool) {
	var marker textbuf.Buffer
	prefix := marker.Str("- `").Str(path).Str("` (").String()
	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		remaining := line[len(prefix):]
		end := strings.Index(remaining, "): ")
		if end == -1 {
			return "", false
		}
		return remaining[:end], true
	}
	return "", false
}

func validateLLMSCommandContract(
	path, meta string,
	command publishedCommand,
) []issue {
	var issues []issue
	var marker textbuf.Buffer
	for _, operator := range command.Operators {
		if !commandMetaPipeGroupContains(meta, operator.Available, operator.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path,
				availabilityCommandDimension(operator.Available, operator.Name),
			))
		}
		if operator.LocalOnly {
			if !commandMetaPipeGroupContains(meta, "local-only", operator.Name) {
				issues = append(issues, generatedCommandContractIssue(
					path, command.Path,
					namedCommandDimension("local-only surface qualifier for operator", operator.Name),
				))
			}
		}
	}
	if command.AnswerShape != "" {
		if commandMetaValue(meta, "shape") != command.AnswerShape {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, "answer shape",
			))
		}
	}
	for _, field := range command.AddressFields {
		if !commandMetaListContains(meta, "address-fields", field) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("address field", field),
			))
		}
	}
	for _, filter := range command.Pipes {
		if !commandMetaListContains(meta, "filters", filter.Name) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("command filter", filter.Name),
			))
		}
	}
	aliases := commandMetaValue(meta, "aliases")
	for _, alias := range command.Aliases {
		marker.Reset()
		want := marker.Str(alias.Name).Byte('=').Str(alias.Expansion).String()
		if !strings.Contains(aliases, want) {
			issues = append(issues, generatedCommandContractIssue(
				path, command.Path, namedCommandDimension("pipe alias", alias.Name),
			))
		}
	}
	return issues
}

func commandMetaPipeGroupContains(meta, availability, name string) bool {
	pipes := commandMetaValue(meta, "pipes")
	for group := range strings.SplitSeq(pipes, ", ") {
		label, values, ok := strings.Cut(group, ": ")
		if !ok {
			continue
		}
		if label != availability {
			continue
		}
		for value := range strings.FieldsSeq(values) {
			if value == name {
				return true
			}
		}
	}
	return false
}

func commandMetaListContains(meta, label, want string) bool {
	for value := range strings.FieldsSeq(commandMetaValue(meta, label)) {
		if value == want {
			return true
		}
	}
	return false
}

func commandMetaValue(meta, label string) string {
	var marker textbuf.Buffer
	prefix := marker.Str(label).Byte(' ').String()
	for segment := range strings.SplitSeq(meta, "; ") {
		if strings.HasPrefix(segment, prefix) {
			return strings.TrimPrefix(segment, prefix)
		}
	}
	return ""
}

func compareRenderedCommandSurfaces(
	root, publicRoot, expectedRoot string,
	live []publishedCommand,
) []issue {
	expected := map[string]bool{
		"llms.txt":                 true,
		"reference/cli/index.html": true,
		"reference/cli/index.md":   true,
	}
	equivalentsRoot := filepath.Join(expectedRoot, "reference", "command-equivalents")
	walkErr := filepath.WalkDir(equivalentsRoot,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".html" {
				if extension != ".md" {
					return nil
				}
			}
			relative, err := filepath.Rel(expectedRoot, path)
			if err != nil {
				return err
			}
			expected[filepath.ToSlash(relative)] = true
			return nil
		})
	if walkErr != nil {
		return []issue{{
			File:    commandSurfacePath(root, equivalentsRoot),
			Message: "could not enumerate generated command-equivalent surfaces",
			Detail:  walkErr.Error(),
		}}
	}

	issues := validateGeneratedCommandSurfaces(root, publicRoot, live)
	paths := make([]string, 0, len(expected))
	for path := range expected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		publishedPath := filepath.Join(publicRoot, filepath.FromSlash(relative))
		if _, err := os.ReadFile(publishedPath); err != nil { //nolint:gosec // generated sibling artifact
			issues = append(issues, issue{
				File:    commandSurfacePath(root, publishedPath),
				Message: "the published per-command surface is missing or unreadable",
				Detail:  err.Error(),
			})
		}
	}

	publicEquivalentsRoot := filepath.Join(publicRoot, "reference", "command-equivalents")
	if _, err := os.Stat(publicEquivalentsRoot); err != nil {
		return issues
	}
	walkErr = filepath.WalkDir(publicEquivalentsRoot,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".html" {
				if extension != ".md" {
					return nil
				}
			}
			relative, err := filepath.Rel(publicRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if expected[relative] {
				return nil
			}
			issues = append(issues, issue{
				File:    commandSurfacePath(root, path),
				Message: "the published command-equivalent surface is stale",
				Detail:  "the live command catalog no longer generates this file",
			})
			return nil
		})
	if walkErr != nil {
		issues = append(issues, issue{
			File:    commandSurfacePath(root, publicEquivalentsRoot),
			Message: "could not enumerate published command-equivalent surfaces",
			Detail:  walkErr.Error(),
		})
	}
	return issues
}

func commandSurfacePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// writeGeneratedDocs rewrites the generated documentation this tool checks, so
// the writer and the checker are one program and cannot render differently.
func writeGeneratedDocs(root string) error {
	path := filepath.Join(root, pipeOperatorReferencePath)
	return os.WriteFile(path, []byte(command.RenderOperatorReference()), 0o644) //nolint:gosec // generated documentation
}
