// Design: docs/architecture/core-design.md -- the documentation drift gate
// Overview: report.go -- the answer this file produces
// Detail: scan.go -- the reads and the counts these checks compare against
// Detail: actions.go -- the command that runs this gate
//
// drift.go compares what the documentation CLAIMS against what the tree, the
// plugin registry and the operator catalog HOLD. Each check reads one document
// and reports the claims that no longer match.
//
// A check reads a document that is ABSENT as "no claim was made", which is why
// every check here returns early on a read failure rather than reporting one.
// The file that exists and cannot be read is a different fact and is reported
// (scan.go, readLines).

package docvalid

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
)

// Drift answers every documentation claim in root that the tree disagrees
// with. An empty report is a tree whose documents all still hold.
func Drift(root string) DriftReport {
	c := &checker{root: root}
	return DriftReport{Issues: c.run()}
}

// run walks every check in the order the gate printed them.
func (c *checker) run() []Issue {
	issues := make([]Issue, 0, len(c.unreadable))

	pluginNames := registryPluginNames()
	familyNames := registryFamilyNames()

	ciTotal, _ := c.countCITests(filepath.Join(c.root, "test"))
	interopCount := c.countInteropScenarios(filepath.Join(c.root, "test", "interop", "scenarios"))
	fuzzCount := c.countFuzzTargets(c.root)
	goTestCount := c.countGoTestFunctions(c.root)
	releaseGateSuites := functionalGateSuites(c.root)

	issues = append(issues, c.checkDesignMD(pluginNames, familyNames, ciTotal, interopCount, fuzzCount, goTestCount)...)
	issues = append(issues, c.checkComparisonMD(familyNames)...)
	issues = append(issues, c.checkReadmeMD(ciTotal, interopCount, fuzzCount, goTestCount)...)
	issues = append(issues, c.checkFeaturesMD()...)
	issues = append(issues, c.checkFunctionalTestsMD(releaseGateSuites)...)
	issues = append(issues, c.checkMakefileHelp(releaseGateSuites)...)
	issues = append(issues, c.checkForbiddenDocClaims()...)
	issues = append(issues, c.checkPipeOperatorReference()...)
	issues = append(issues, c.checkPublishedCommandSurfaces("")...)
	issues = append(issues, c.unreadable...)

	return issues
}

// forbiddenDocClaim is a sentence a document must no longer carry, because the
// code it describes has moved on.
type forbiddenDocClaim struct {
	File    string
	Needle  string
	Message string
	Detail  string
}

// The documents this gate judges, each named once so a check and its finding
// cannot spell a path two ways.
const (
	textParserDoc      = "docs/architecture/api/text-parser.md"
	designDoc          = "docs/DESIGN.md"
	readmeDoc          = "README.md"
	featuresDoc        = "docs/features.md"
	functionalTestsDoc = "docs/functional-tests.md"
	makefileName       = "Makefile"
)

var forbiddenDocClaims = []forbiddenDocClaim{
	{
		File:    textParserDoc,
		Needle:  "strings.Fields",
		Message: "stale text parser claim references strings.Fields",
		Detail:  "The route-server text parser uses textparse.NewScanner; update this doc to source-linked scanner wording.",
	},
	{
		File:    textParserDoc,
		Needle:  "All functions allocate via",
		Message: "stale text parser allocation summary",
		Detail:  "Describe scanner tokenization and the remaining result allocations separately.",
	},
	{
		File:    textParserDoc,
		Needle:  "No manual byte scanning or zero-allocation parsing exists",
		Message: "stale text parser allocation summary",
		Detail:  "Describe scanner tokenization and the remaining result allocations separately.",
	},
}

func (c *checker) checkForbiddenDocClaims() []Issue {
	var issues []Issue
	for _, claim := range forbiddenDocClaims {
		lines, err := c.readLines(filepath.Join(c.root, claim.File))
		if err != nil {
			continue
		}
		for i, line := range lines {
			if !strings.Contains(line, claim.Needle) {
				continue
			}
			issues = append(issues, Issue{
				File:    claim.File,
				Line:    i + 1,
				Message: claim.Message,
				Detail:  claim.Detail,
			})
		}
	}
	return issues
}

// functionalGateSuites returns the suites the functional gate runs from its
// native owner. An invalid run list returns empty, and both callers turn that
// into a loud derivation finding rather than a silent pass.
func functionalGateSuites(_ string) []string {
	resolved, err := functional.GatingSuites(functional.Gating, functional.Suites)
	if err != nil {
		return nil
	}
	suites := make([]string, 0, len(resolved))
	for _, suite := range resolved {
		suites = append(suites, suite.Name)
	}
	return suites
}

// registryPluginNames answers every plugin name the registry holds, sorted.
func registryPluginNames() []string {
	all := registry.All()
	names := make([]string, 0, len(all))
	for _, reg := range all {
		names = append(names, reg.Name)
	}
	sort.Strings(names)
	return names
}

// registryFamilyNames answers every address family this build carries, sorted,
// including the four the engine holds without a plugin.
func registryFamilyNames() []string {
	fam := registry.FamilyMap()
	names := make(map[string]bool)
	for name := range fam {
		names[name] = true
	}
	for _, builtin := range []string{
		"ipv4/unicast", "ipv6/unicast",
		"ipv4/multicast", "ipv6/multicast",
	} {
		names[builtin] = true
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func (c *checker) checkDesignMD(pluginNames, familyNames []string, ciTotal, interopCount, fuzzCount, goTestCount int) []Issue {
	path := filepath.Join(c.root, "docs", "DESIGN.md")
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}

	var issues []Issue
	var tb textbuf.Buffer

	for i, line := range lines {
		lineNum := i + 1

		if m := extractCount(line, `(\d+) address families`); m > 0 {
			actual := len(familyNames)
			if m != actual {
				tb.Reset()
				message := tb.Str("claims ").Int(int64(m)).Str(" address families, registry has ").
					Int(int64(actual)).String()
				tb.Reset()
				issues = append(issues, Issue{
					File:    designDoc,
					Line:    lineNum,
					Message: message,
					Detail:  tb.Str("registry: ").Join(familyNames, ", ").String(),
				})
			}
		}

		if m := extractApprox(line, `~([\d,]+).*functional test`); m > 0 && !withinThreshold(m, ciTotal, 0.20) {
			tb.Reset()
			issues = append(issues, Issue{
				File: designDoc, Line: lineNum,
				Message: tb.Str("claims ~").Int(int64(m)).Str(" functional tests, actual is ").
					Int(int64(ciTotal)).String(),
			})
		}
		if m := extractApprox(line, `~([\d,]+).*[Gg]o test function`); m > 0 && !withinThreshold(m, goTestCount, 0.20) {
			tb.Reset()
			issues = append(issues, Issue{
				File: designDoc, Line: lineNum,
				Message: tb.Str("claims ~").Int(int64(m)).Str(" Go test functions, actual is ").
					Int(int64(goTestCount)).String(),
			})
		}
		if m := extractApprox(line, `~([\d,]+).*[Ff]uzz target`); m > 0 && !withinThreshold(m, fuzzCount, 0.30) {
			tb.Reset()
			issues = append(issues, Issue{
				File: designDoc, Line: lineNum,
				Message: tb.Str("claims ~").Int(int64(m)).Str(" fuzz targets, actual is ").
					Int(int64(fuzzCount)).String(),
			})
		}

		// A FLOOR claim ("100+ interop scenarios") is satisfied by any real
		// count at or above it; an exact claim ("106 interop scenarios") still
		// has to match. Prose pinning a number that grows on its own schedule
		// costs a doc edit and a red gate every time somebody adds a scenario:
		// that one line was corrected twice in a single day, for no reader's
		// benefit. The floor keeps the guarantee that matters, which is that
		// the page never overclaims, and drops the churn. A bare number is
		// still accepted and still checked exactly, for counts an author does
		// want pinned.
		if m, isFloor := extractFloorCount(line, `(\d+)(\+?) interop scenario`); m > 0 {
			tb.Reset()
			switch {
			case isFloor && interopCount < m:
				issues = append(issues, Issue{
					File: designDoc, Line: lineNum,
					Message: tb.Str("claims at least ").Int(int64(m)).
						Str(" interop scenarios, actual is ").Int(int64(interopCount)).String(),
				})
			case !isFloor && m != interopCount:
				issues = append(issues, Issue{
					File: designDoc, Line: lineNum,
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
			tb.Reset()
			issues = append(issues, Issue{
				File:    designDoc,
				Line:    0,
				Message: tb.Str("plugin ").Quoted(name).Str(" registered but missing from Shipped Plugins table").String(),
			})
		}
	}

	return issues
}

// comparisonLabels maps a row label in the comparison page to the family name
// the registry holds it under.
var comparisonLabels = map[string]string{
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

func (c *checker) checkComparisonMD(familyNames []string) []Issue {
	path := filepath.Join(c.root, "docs", "comparison.md")
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}

	var issues []Issue
	var tb textbuf.Buffer
	familySet := make(map[string]bool)
	for _, f := range familyNames {
		familySet[f] = true
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

		regFamily, mapped := comparisonLabels[label]
		if !mapped {
			continue
		}

		inRegistry := familySet[regFamily]

		tb.Reset()
		switch {
		case zeClaim == "yes" && !inRegistry:
			issues = append(issues, Issue{
				File: "docs/comparison.md", Line: lineNum,
				Message: tb.Str("claims Ze has ").Quoted(cells[0]).Str(" but ").
					Quoted(regFamily).Str(" not in registry").String(),
			})
		case zeClaim == "no" && inRegistry:
			issues = append(issues, Issue{
				File: "docs/comparison.md", Line: lineNum,
				Message: tb.Str("claims Ze lacks ").Quoted(cells[0]).Str(" but ").
					Quoted(regFamily).Str(" IS in registry").String(),
				Detail: "Change to Yes or Decode as appropriate",
			})
		}
	}

	return issues
}

func (c *checker) checkReadmeMD(ciTotal, interopCount, fuzzCount, goTestCount int) []Issue {
	path := filepath.Join(c.root, "README.md")
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}

	var issues []Issue
	var tb textbuf.Buffer
	for i, line := range lines {
		lineNum := i + 1
		if iss, ok := checkReadmeCount(line, lineNum, `unit tests`, "unit tests", goTestCount); ok {
			issues = append(issues, iss)
		}
		if m := extractApprox(line, `(?i)roughly ([\d,]+).*functional tests`); m > 0 && !withinThreshold(m, ciTotal, 0.20) {
			tb.Reset()
			issues = append(issues, Issue{
				File: readmeDoc, Line: lineNum,
				Message: tb.Str("claims roughly ").Int(int64(m)).
					Str(" functional tests, actual is ").Int(int64(ciTotal)).String(),
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
//     anti-re-drift default -- headroom above the claimed floor is intended and
//     tolerated as the tree grows.
//   - bare `N <unit>` (exact): drift when actual != N in EITHER direction. This
//     catches bare over-claims AND undercounts that a `+`-only pattern cannot
//     see (a bare `57 fuzz targets`, or a bare `10,000 unit tests`).
//
// RE2 has no look-ahead, so the optional `+` is captured as its own group and
// inspected to tell bare from at-least apart.
func checkReadmeCount(line string, lineNum int, unit, label string, actual int) (Issue, bool) {
	var pattern textbuf.Buffer
	re := regexp.MustCompile(pattern.Str(`([\d,]+)(\+?)\s+`).Str(unit).String())
	m := re.FindStringSubmatch(line)
	if len(m) < 3 {
		return Issue{}, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil || n <= 0 {
		return Issue{}, false
	}
	var tb textbuf.Buffer
	if m[2] == "+" {
		if actual < n {
			tb.Str("claims ").Int(int64(n)).Str("+ ").Str(label).Str(", actual is ").Int(int64(actual))
			return Issue{File: readmeDoc, Line: lineNum, Message: tb.String()}, true
		}
		return Issue{}, false
	}
	if n != actual {
		tb.Str("claims ").Int(int64(n)).Byte(' ').Str(label).
			Str(" (bare exact count), actual is ").Int(int64(actual))
		return Issue{File: readmeDoc, Line: lineNum, Message: tb.String()}, true
	}
	return Issue{}, false
}

// featureStatuses are the statuses a feature inventory row may carry.
var featureStatuses = map[string]bool{
	"supported":    true,
	"partial":      true,
	"experimental": true,
	"stub-backed":  true,
	"rejected":     true,
	"future":       true,
}

func (c *checker) checkFeaturesMD() []Issue {
	path := filepath.Join(c.root, "docs", "features.md")
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}

	var issues []Issue
	var tb textbuf.Buffer
	foundHeader := false
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| Feature |") {
			foundHeader = true
			if !strings.Contains(trimmed, "| Status |") || !strings.Contains(trimmed, "| Description |") {
				issues = append(issues, Issue{
					File: featuresDoc, Line: lineNum,
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
			issues = append(issues, Issue{
				File: featuresDoc, Line: lineNum,
				Message: "feature inventory row must include status",
			})
			continue
		}
		status := strings.ToLower(strings.TrimSpace(cells[1]))
		if !featureStatuses[status] {
			tb.Reset()
			issues = append(issues, Issue{
				File: featuresDoc, Line: lineNum,
				Message: tb.Str("unknown feature status ").Quoted(cells[1]).String(),
				Detail:  "valid statuses: supported, partial, experimental, stub-backed, rejected, future",
			})
		}
	}
	if !foundHeader {
		issues = append(issues, Issue{
			File:    featuresDoc,
			Line:    0,
			Message: "feature inventory table not found",
		})
	}
	return issues
}

// suiteDerivationFailed is what both suite checks report when the functional
// owner cannot resolve its gating list. It names the derivation rather than the
// document, because the document is not what failed.
const suiteDerivationFailed = "could not derive ze-functional-test suites from the native functional catalog"

func (c *checker) checkFunctionalTestsMD(gateSuites []string) []Issue {
	path := filepath.Join(c.root, "docs", "functional-tests.md")
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}
	// Same order and the same reason as checkMakefileHelp: the read decides
	// whether there is anything here to check, and only then does an empty
	// population mean the derivation failed.
	if len(gateSuites) == 0 {
		return []Issue{{
			File:    functionalTestsDoc,
			Line:    0,
			Message: suiteDerivationFailed,
		}}
	}

	var issues []Issue
	var tb textbuf.Buffer
	joined := strings.Join(lines, " ")
	re := regexp.MustCompile(`functional test target runs (\d+) suites: ([^.]+)\.`)
	m := re.FindStringSubmatch(joined)
	if len(m) < 3 {
		issues = append(issues, Issue{
			File:    functionalTestsDoc,
			Line:    0,
			Message: "could not find release gate suite list",
			Detail:  "expected: functional test target runs N suites: a, b, c.",
		})
	} else {
		claimedCount, _ := strconv.Atoi(m[1]) //nolint:errcheck // a non-number cannot reach here: the group is \d+
		claimedSuites := splitSuiteList(m[2])
		if claimedCount != len(gateSuites) {
			tb.Reset()
			message := tb.Str("claims ").Int(int64(claimedCount)).
				Str(" release-gate suites, Makefile has ").Int(int64(len(gateSuites))).String()
			tb.Reset()
			issues = append(issues, Issue{
				File: functionalTestsDoc, Line: lineNumberContaining(lines, m[0]),
				Message: message,
				Detail:  tb.Str("Makefile: ").Join(gateSuites, ", ").String(),
			})
		}
		if !sameStrings(claimedSuites, gateSuites) {
			tb.Reset()
			issues = append(issues, Issue{
				File: functionalTestsDoc, Line: lineNumberContaining(lines, m[0]),
				Message: "release-gate suite list does not match Makefile",
				Detail: tb.Str("docs: ").Join(claimedSuites, ", ").
					Str("; Makefile: ").Join(gateSuites, ", ").String(),
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
			tb.Reset()
			issues = append(issues, Issue{
				File: functionalTestsDoc, Line: 0,
				Message: tb.Str("gated suite ").Quoted(suite).Str(" is listed as manual-only").String(),
			})
		}
	}
	return issues
}

func (c *checker) checkMakefileHelp(gateSuites []string) []Issue {
	path := filepath.Join(c.root, makefileName)
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}
	// Emptiness is judged AFTER the read and separately from it. The read
	// answers "is there anything here to check", and a fixture root with no
	// Makefile must stay silent. Once the file exists, an empty population
	// means the derivation FAILED, and reading that as "no suites to check"
	// would disable this check on exactly the failure it must report.
	if len(gateSuites) == 0 {
		return []Issue{{
			File:    makefileName,
			Line:    0,
			Message: suiteDerivationFailed,
		}}
	}

	var tb textbuf.Buffer
	listRe := regexp.MustCompile(`ze-functional-test\s+- Run ze functional tests \(([^)]*)\)`)
	countRe := regexp.MustCompile(`ze-functional-test\s+All\s+(\d+)\s+gating suites`)
	for i, line := range lines {
		m := listRe.FindStringSubmatch(line)
		if len(m) < 2 {
			count := countRe.FindStringSubmatch(line)
			if len(count) != 2 {
				continue
			}
			claimed, _ := strconv.Atoi(count[1]) //nolint:errcheck // a non-number cannot reach here: the group is \d+
			if claimed == len(gateSuites) {
				return nil
			}
			message := tb.Str("ze-functional-test help claims ").Int(int64(claimed)).
				Str(" suites, target has ").Int(int64(len(gateSuites))).String()
			tb.Reset()
			return []Issue{{
				File:    makefileName,
				Line:    i + 1,
				Message: message,
				Detail:  tb.Str("target: ").Join(gateSuites, ", ").String(),
			}}
		}
		claimed := splitSuiteList(m[1])
		if sameStrings(claimed, gateSuites) {
			return nil
		}
		tb.Reset()
		return []Issue{{
			File:    makefileName,
			Line:    i + 1,
			Message: "ze-functional-test help suite list does not match target",
			Detail: tb.Str("help: ").Join(claimed, ", ").
				Str("; target: ").Join(gateSuites, ", ").String(),
		}}
	}

	return []Issue{{
		File:    makefileName,
		Line:    0,
		Message: "ze-functional-test help line not found",
	}}
}

// splitSuiteList answers the suite names in a comma-separated prose list.
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

// sameStrings reports whether two lists hold the same names in the same order.
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

// pipeOperatorReferencePath is the generated operator table every documentation
// page links to instead of listing the operators itself.
const pipeOperatorReferencePath = "docs/features/pipe-operators.generated.md"

// checkPipeOperatorReference fails when the published operator table and the
// operator catalog disagree.
//
// This is the gate the whole surface needed. The set used to be hand-copied
// into five places and no two agreed: Tab completion had all sixteen, two
// different pages had two different tens, and `display` and `fill` appeared in
// none of the lists a user or a tool could reach. Nothing could see that,
// because no check compared a published list against the product.
//
// The comparison is exact rather than a name-by-name search, so a description
// or an argument kind that changes in the catalog and not on the page is caught
// as well as a missing operator.
func (c *checker) checkPipeOperatorReference() []Issue {
	// This gate is also run over a fixture tree holding one or two documents,
	// to check a single claim. Such a tree owes no generated operator table,
	// and reporting one missing there is a finding about the fixture rather
	// than about the documentation. The directory the table lives in is the
	// sentinel: absent, there is nothing here to judge.
	if info, statErr := os.Stat(filepath.Join(c.root, "docs", "features")); statErr != nil || !info.IsDir() {
		return nil
	}

	path := filepath.Join(c.root, pipeOperatorReferencePath)
	published, err := os.ReadFile(path) //nolint:gosec // repository-relative documentation path
	if err != nil {
		return []Issue{{
			File:    pipeOperatorReferencePath,
			Message: "the generated pipe operator reference is missing",
			Detail:  "run `make ze-docs-pipe-operators-update`",
		}}
	}
	if string(published) == command.RenderOperatorReference() {
		return nil
	}
	return []Issue{{
		File:    pipeOperatorReferencePath,
		Message: "the published pipe operator table and the operator catalog disagree",
		Detail: "the catalog in internal/component/command/pipe_catalog.go is the source; " +
			"run `make ze-docs-pipe-operators-update`",
	}}
}

// WriteGenerated rewrites the generated documentation the drift gate checks, so
// the writer and the checker are one program and cannot render differently.
func WriteGenerated(root string) (WriteReport, error) {
	path := filepath.Join(root, pipeOperatorReferencePath)
	if err := os.WriteFile(path, []byte(command.RenderOperatorReference()), 0o644); err != nil { //nolint:gosec // generated documentation
		return WriteReport{}, err
	}
	return WriteReport{Path: pipeOperatorReferencePath}, nil
}
