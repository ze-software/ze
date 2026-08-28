// Design: ai/rules/architecture.md -- the tier gate, proved against fixtures
//
// Overview: tier.go -- the import audit these fixtures exercise
//
// selftest.go proves the five checks independently from the live tree. It uses
// three fixture checkouts and one row for each property.
//
// Clean output cannot distinguish a valid gate from broken detection. Fixtures
// therefore place required failures beside permitted placements.
//
// One script fixture cannot return. It wrote plugin_imports.go into the fixture
// and then parsed plugin roots from that file. Thus, the fixture controlled the
// roots. This port calls internal/le/pluginimports.PluginSearchRoots, which returns
// the generator's actual list. The nested-namespace fixture uses a root from
// that list. TestThePluginDirsAreTheGeneratorsOwnList links both derivations.
package tier

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// fixtureModule is the module path every fixture checkout declares.
const fixtureModule = "example.com/m"

// nestedFixtureRoot is a real plugin search root and a nested sub-plugin
// namespace. An engine there is a correctly placed sub-plugin of its host.
const nestedFixtureRoot = "internal/component/firewall/plugins"

// agentWorktree represents a sibling Claude checkout nested in this tree. The
// edge scan once included such a checkout and reported another session's
// in-progress violation against this tree.
const agentWorktree = ".claude/worktrees/agent-fake/internal/app/use2.go"

// writeFixture writes one fixture checkout from a path -> content map.
func writeFixture(root string, files map[string]string) error {
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// blankImport renders a Go file that blank-imports one package of the fixture
// module, which is how every wiring fixture is written.
func blankImport(pkg, imported string) string {
	var tb textbuf.Buffer
	return tb.Str("package ").Str(pkg).Str("\nimport _ ").Byte('"').Str(fixtureModule).
		Byte('/').Str(imported).Byte('"').Byte('\n').String()
}

// engineFile renders a Go file whose code constructs a config-driven engine.
func engineFile(pkg string) string {
	var tb textbuf.Buffer
	return tb.Str("package ").Str(pkg).Str("\nfunc R(){ sdk.NewWithConn() }\n").String()
}

// gatedImport renders a Go file whose import is reachable only behind a build
// tag, which is the correct way to reach a compile-out-able feature.
func gatedImport(tag, pkg, imported string) string {
	var tb textbuf.Buffer
	return tb.Str("//go:build ").Str(tag).Str("\n\n").Str(blankImport(pkg, imported)).String()
}

// allImports renders the generated composition root, blank-importing each
// package named.
func allImports(imported ...string) string {
	var tb textbuf.Buffer
	tb.Str("package all\n")
	for _, pkg := range imported {
		tb.Str("import _ ").Byte('"').Str(fixtureModule).Byte('/').Str(pkg).Byte('"').Byte('\n')
	}
	return tb.String()
}

// engineImporting renders an engine that also imports one package, which is
// what makes a component engine depended upon.
func engineImporting(pkg, imported string) string {
	var tb textbuf.Buffer
	return tb.Str(blankImport(pkg, imported)).Str("func R(){ sdk.NewWithConn() }\n").String()
}

// placementFixture answers the first fixture checkout: engine placement, the
// wired-versus-core classification, and the disableable-import audit.
func placementFixture() map[string]string {
	var tb textbuf.Buffer
	nested := tb.Str(nestedFixtureRoot).Str("/sub/r.go").String()

	return map[string]string{
		"go.mod": "module example.com/m\n",

		// An edge engine in component that nothing depends on belongs in
		// plugins.
		"internal/component/edgeproto/r.go": engineFile("edgeproto"),
		// A platform engine in component that a feature depends on is correct.
		"internal/component/platform/r.go": engineImporting("platform", "internal/plugins/platformx"),
		"internal/plugins/consumer/r.go":   blankImport("consumer", "internal/component/platform"),
		// A nested sub-plugin engine is excluded by the search roots.
		nested: engineFile("sub"),
		// An edge engine already in plugins is correct.
		"internal/plugins/edge1/r.go": engineFile("edge1"),
		// A platform engine stuck in plugins belongs in component.
		"internal/plugins/platformx/r.go": engineFile("platformx"),

		// A command plugin wired ONLY through the cmd/ze dispatch: the shape
		// the all.go-only signal used to mis-send to core.
		"internal/plugins/cmdverb/r.go": "package cmdverb\n",
		"cmd/ze/ze_core_dispatch.go":    blankImport("main", "internal/plugins/cmdverb"),
		// A plugin wired through the generated composition root, and an ENGINE
		// wired the same way. The engine is what proves a registration import
		// is not a feature dependency: reading it as one would send every
		// registered plugin engine to internal/component.
		"internal/plugins/genverb/r.go":        "package genverb\n",
		"internal/plugins/regengine/r.go":      engineFile("regengine"),
		"internal/component/plugin/all/all.go": allImports("internal/plugins/genverb", "internal/plugins/regengine"),
		// A setup feature package, wired by cmd/ze/setup_features_*.go.
		"internal/plugins/setupcmd/r.go":  "package setupcmd\n",
		"cmd/ze/setup_features_distro.go": blankImport("main", "internal/plugins/setupcmd"),
		// A genuine leaf, and a shared library a feature imports.
		"internal/plugins/leaflib/lib.go":   "package leaflib\n",
		"internal/plugins/sharedlib/lib.go": "package sharedlib\n",
		"internal/plugins/edge1/use.go":     blankImport("edge1", "internal/plugins/sharedlib"),

		// The non-engine category fixtures.
		"internal/component/frameworklib/lib.go": "package frameworklib\n",
		"internal/component/hostsvc/lib.go":      "package hostsvc\n",
		"internal/component/l2tp/lib.go":         "package l2tp\n",
		"internal/plugins/oldleaf/lib.go":        "package oldleaf\n",
		"internal/component/unclassified/lib.go": "package unclassified\n",

		// Disableable-feature fixtures include gated, always-on, test, and
		// nonproduction importers. A sibling agent worktree verifies that the
		// edge scan never enters it.
		"internal/component/widget/w.go":     "package widget\n",
		"cmd/app/svc_widget.go":              gatedImport("ze_widget", "app", "internal/component/widget"),
		"internal/app/use.go":                blankImport("app", "internal/component/widget"),
		"internal/app/use_test.go":           blankImport("app", "internal/component/widget"),
		"internal/chaos/orchestrator/use.go": blankImport("orchestrator", "internal/component/widget"),
		agentWorktree:                        blankImport("app", "internal/component/widget"),

		// The manifest fixtures that keep the disableable and drift gates clean
		// while the sequence below exercises the other three.
		FeatureGatesManifest: "# no gates in this fixture\n",
		Golangci:             "run:\n  build-tags:\n    - ze_core\nlinters: {}\n",
		NonEngineCategories: strings.Join([]string{
			"# path category rationale",
			"internal/component/frameworklib\tframework\tselftest framework package",
			"internal/component/plugin\tframework\tselftest composition root",
			"internal/component/hostsvc\thost-service\tselftest host service package",
			"internal/component/firewall\thost-service\tselftest nested plugin root",
			"internal/component/l2tp\tdomain-library\tselftest domain library package",
			"internal/plugins/leaflib\tplanned-violation\tspec-tiers-5 selftest misplaced leaf",
			"internal/component/widget\tframework\tselftest disableable feature package",
			"internal/plugins/consumer\tplanned-violation\tspec-tiers-5 selftest feature importer fixture",
			"internal/plugins/oldleaf\tplanned-violation\tspec-tiers-5 selftest planned violation",
			"internal/plugins/sharedlib\tplanned-violation\tspec-tiers-5 selftest shared plugin lib",
			"internal/plugins/setupcmd\tframework\tselftest setup feature registration",
			"",
		}, "\n"),
	}
}

// manifestFixture answers the second fixture checkout: the feature-gate
// manifest and the lint build-tag list.
func manifestFixture() map[string]string {
	return map[string]string{
		FeatureGatesManifest: "# header comment\nze_alpha   internal/component/alpha\nze_beta    internal/component/beta\n",
		Golangci: strings.Join([]string{
			"run:", "  build-tags:", "    - ze_core  # base tag", "    # the gates:",
			"    - ze_alpha", "    - ze_beta", "linters:", "  default: none", "",
		}, "\n"),
	}
}

// coreFixture answers the third fixture checkout: the core import direction.
func coreFixture() map[string]string {
	return map[string]string{
		"go.mod": "module example.com/m\n",
		// A core-to-core import is never a violation.
		"internal/core/clean/lib.go": "package clean\n",
		"internal/core/ok/self.go":   blankImport("ok", "internal/core/clean"),
		// A core-to-component import is one, and so is a core-to-plugins
		// import from a TEST file: the real grandfathered set holds one.
		"internal/core/bad/uses.go":      blankImport("bad", "internal/component/thing"),
		"internal/core/bad/uses_test.go": blankImport("bad", "internal/plugins/thingp"),
		// A component-to-component import is not what this gate judges.
		"internal/component/other/lib.go": blankImport("other", "internal/component/thing"),
	}
}

// caseResult answers a passing or failing row for one selftest case.
func caseResult(name string, ok bool, detail string) leroot.SelftestResult {
	if ok {
		return leroot.Pass(name)
	}
	return leroot.Fail(name, detail)
}

// Selftest writes three fixture checkouts and runs all checks. It answers one row
// for each property.
//
// A fixture write or scan error differs from failed detection. The function
// returns that error separately instead of adding another failing row.
func Selftest() (leroot.SelftestReport, error) {
	var results []leroot.SelftestResult
	for _, stage := range []func() ([]leroot.SelftestResult, error){
		runPlacementCases, runManifestCases, runCoreCases,
	} {
		rows, err := stage()
		if err != nil {
			return leroot.SelftestReport{}, err
		}
		results = append(results, rows...)
	}
	return leroot.NewSelftestReport("dep_audit selftest OK", "dep_audit selftest FAILED:", results...), nil
}

// runPlacementCases exercises engine placement, the classification, the
// disableable audit and the four-step gate sequence.
func runPlacementCases() ([]leroot.SelftestResult, error) {
	root, err := os.MkdirTemp("", "tier-selftest-placement")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temp fixture

	if err := writeFixture(root, placementFixture()); err != nil {
		return nil, err
	}
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		return nil, err
	}
	misplaced, err := engineMisplacements(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}

	want := map[string]string{
		"internal/component/edgeproto": "internal/plugins",
		"internal/plugins/platformx":   "internal/component",
	}
	var tb textbuf.Buffer
	nestedEngine := tb.Str(nestedFixtureRoot).Str("/sub").String()
	results := []leroot.SelftestResult{
		caseResult("misplaced-engines", sameMap(misplaced, want),
			"the misplaced engines are not the two the fixture declares"),
		caseResult("nested-sub-plugin-excluded", !hasKey(misplaced, nestedEngine),
			"an engine inside a nested sub-plugin namespace was flagged"),
		caseResult("depended-component-engine", !hasKey(misplaced, "internal/component/platform"),
			"a component engine a feature depends on was flagged"),
		caseResult("edge-plugin-engine", !hasKey(misplaced, "internal/plugins/edge1"),
			"an edge engine already in plugins was flagged"),
		caseResult("registration-wired-engine", !hasKey(misplaced, "internal/plugins/regengine"),
			"an engine the composition root blank-imports was read as a feature dependency and sent to component"),
	}

	wiring, err := runWiringCases(root, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, wiring...)
	results = append(results, runDisableableCases(root, edges)...)

	sequence, err := runGateSequence(root, edges, misplaced)
	if err != nil {
		return nil, err
	}
	return append(results, sequence...), nil
}

// runWiringCases proves the wired-versus-core determination reads every
// composition root.
func runWiringCases(root string, edges Edges) ([]leroot.SelftestResult, error) {
	engines, err := engineSubsystems(root)
	if err != nil {
		return nil, err
	}
	rows, err := Classify("internal/plugins", root, fixtureModule, edges, engines)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]Row, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}

	cases := []struct {
		name, subsystem, detail string
		ok                      func(Row) bool
	}{
		{"dispatch-wired-plugin", "cmdverb", "a *-cmd plugin wired only through the cmd/ze dispatch is not seen as wired",
			func(row Row) bool { return row.IsRegistered && !row.CoreCandidate }},
		{"all-go-wired-plugin", "genverb", "a plugin wired through the generated composition root is not seen as wired",
			func(row Row) bool { return row.IsRegistered && !row.CoreCandidate }},
		{"setup-feature-wired-plugin", "setupcmd", "a plugin wired by cmd/ze/setup_features_*.go is not seen as wired",
			func(row Row) bool { return row.IsRegistered && !row.CoreCandidate }},
		{"genuine-leaf", "leaflib", "a leaf nothing wires and nothing imports is not a core candidate",
			func(row Row) bool { return row.CoreCandidate && !row.IsRegistered }},
		{"shared-library", "sharedlib", "a library a feature imports reads as a core candidate",
			func(row Row) bool { return !row.CoreCandidate && len(row.External) > 0 }},
	}

	results := make([]leroot.SelftestResult, 0, len(cases))
	for _, testCase := range cases {
		row, ok := byName[testCase.subsystem]
		results = append(results, caseResult(testCase.name, ok && testCase.ok(row), testCase.detail))
	}
	return results, nil
}

// runDisableableCases proves the compile-out audit flags an always-on importer
// and leaves the four legitimate shapes alone.
func runDisableableCases(root string, edges Edges) []leroot.SelftestResult {
	gates := map[string]string{"internal/component/widget": "ze_widget"}
	flagged := make(map[string]bool)
	for _, violation := range disableableViolations(root, fixtureModule, edges, gates) {
		flagged[violation.Importer] = true
	}

	cases := []struct {
		name, file, detail string
		want               bool
	}{
		{"always-on-importer", "internal/app/use.go", "an always-on direct import of a disableable feature is not flagged", true},
		{"tag-gated-importer", "cmd/app/svc_widget.go", "a build-tag-gated importer is flagged", false},
		{"test-importer", "internal/app/use_test.go", "a test importer is flagged", false},
		{"non-production-importer", "internal/chaos/orchestrator/use.go", "a ze-chaos importer is flagged", false},
	}

	results := make([]leroot.SelftestResult, 0, len(cases)+1)
	for _, testCase := range cases {
		results = append(results, caseResult(testCase.name, flagged[testCase.file] == testCase.want, testCase.detail))
	}

	worktree := false
	for file := range flagged {
		if strings.Contains(file, ".claude") {
			worktree = true
		}
	}
	return append(results, caseResult("sibling-worktree-not-scanned", !worktree,
		"a sibling agent worktree was scanned and flagged"))
}

// runGateSequence walks the baseline's whole life: empty, written, classified,
// and stale.
func runGateSequence(root string, edges Edges, misplaced map[string]string) ([]leroot.SelftestResult, error) {
	code, err := quietCheck(root, edges)
	if err != nil {
		return nil, err
	}
	results := []leroot.SelftestResult{
		caseResult("empty-baseline-fails", code == 2, "an empty baseline passed while the tree holds a new misplacement"),
	}

	if err := writeBaseline(root, misplaced); err != nil {
		return nil, err
	}
	code, err = quietCheck(root, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("unclassified-placement-fails", code == 2,
		"an unclassified non-engine placement passed"))

	if err := appendLine(filepath.Join(root, NonEngineCategories),
		"internal/component/unclassified\tplanned-violation\tspec-tiers-5 selftest illegal package made explicit\n"); err != nil {
		return nil, err
	}
	code, err = quietCheck(root, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("matching-baseline-passes", code == 0,
		"a matching baseline and a fully classified tree did not pass"))

	if err := appendLine(filepath.Join(root, Baseline),
		"internal/component/gone\tinternal/plugins\tspec-tiers-2\n"); err != nil {
		return nil, err
	}
	code, err = quietCheck(root, edges)
	if err != nil {
		return nil, err
	}
	return append(results, caseResult("stale-baseline-entry-fails", code == 2,
		"a stale baseline entry passed, so the baseline can grow")), nil
}

// quietCheck runs the five gates and answers the code alone. Nothing is
// printed: a selftest that wrote its fixtures' pages to the terminal would bury
// its own verdict.
func quietCheck(root string, edges Edges) (int, error) {
	report, err := CheckWith(root, fixtureModule, edges)
	if err != nil {
		return 0, err
	}
	return report.Failed, nil
}

// appendLine adds one row to a fixture manifest.
func appendLine(path, line string) error {
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // a fixture file this package created
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(line); err != nil {
		handle.Close() //nolint:errcheck,gosec // the write error is the one that matters
		return err
	}
	return handle.Close()
}

// runManifestCases exercises the feature-gate manifest parse and the lint
// build-tag drift gate.
func runManifestCases() ([]leroot.SelftestResult, error) {
	root, err := os.MkdirTemp("", "tier-selftest-manifest")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temp fixture

	if err := writeFixture(root, manifestFixture()); err != nil {
		return nil, err
	}

	gates, err := loadFeatureGates(root)
	if err != nil {
		return nil, err
	}
	wantGates := map[string]string{
		"internal/component/alpha": "ze_alpha",
		"internal/component/beta":  "ze_beta",
	}
	results := []leroot.SelftestResult{
		caseResult("feature-gate-manifest-parse", sameMap(gates, wantGates),
			"the feature-gate manifest did not parse into its two gates"),
	}

	tags, err := parseGolangciBuildTags(root)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("golangci-build-tags-parse",
		len(tags) == 3 && tags["ze_core"] && tags["ze_alpha"] && tags["ze_beta"],
		"the build-tags list did not survive an inline comment and a comment-only line"))

	clean, err := golangciDriftGate(root)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("golangci-matching-passes", clean.Code == 0,
		"a build-tags list matching the manifest was reported as drift"))

	if err := writeFixture(root, map[string]string{
		Golangci: "run:\n  build-tags:\n    - ze_core\n    - ze_alpha\nlinters:\n  default: none\n",
	}); err != nil {
		return nil, err
	}
	drifted, err := golangciDriftGate(root)
	if err != nil {
		return nil, err
	}
	return append(results, caseResult("golangci-missing-gate-fails", drifted.Code == 2,
		"a gate missing from the build-tags list was not reported as drift")), nil
}

// runCoreCases exercises the core import-direction gate over its own fixture.
func runCoreCases() ([]leroot.SelftestResult, error) {
	root, err := os.MkdirTemp("", "tier-selftest-core")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temp fixture

	if err := writeFixture(root, coreFixture()); err != nil {
		return nil, err
	}
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		return nil, err
	}

	want := []corePair{
		{File: "internal/core/bad/uses.go", Package: "internal/component/thing"},
		{File: "internal/core/bad/uses_test.go", Package: "internal/plugins/thingp"},
	}
	results := []leroot.SelftestResult{
		caseResult("core-direction-violations", samePairs(coreDirectionViolations(fixtureModule, edges), want),
			"the upward imports out of internal/core are not the two the fixture declares"),
	}

	missing, err := coreDirectionGate(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("missing-core-baseline-fails", missing.Code == 2,
		"a new upward import passed with no baseline"))

	baselineRows := strings.Join([]string{
		"# core import-direction baseline -- selftest fixture",
		"internal/core/bad/uses.go\tinternal/component/thing\thand-fixable: selftest fixture",
		"internal/core/bad/uses_test.go\tinternal/plugins/thingp\tneeds-design: selftest fixture",
		"",
	}, "\n")
	if err := writeFixture(root, map[string]string{CoreImportBaseline: baselineRows}); err != nil {
		return nil, err
	}
	baselined, err := coreDirectionGate(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("fully-baselined-passes", baselined.Code == 0,
		"a fully baselined set of pairs did not pass"))

	// A NEW pair in an already-baselined FILE must still fail: the baseline is
	// at pair granularity rather than file granularity.
	if err := writeFixture(root, map[string]string{
		"internal/core/bad/more.go": blankImport("bad", "internal/component/second"),
	}); err != nil {
		return nil, err
	}
	edges, err = collectEdges(root, fixtureModule)
	if err != nil {
		return nil, err
	}
	newPair, err := coreDirectionGate(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("new-pair-in-baselined-file-fails", newPair.Code == 2,
		"a new pair in an already-baselined file passed"))

	if err := os.Remove(filepath.Join(root, "internal", "core", "bad", "more.go")); err != nil {
		return nil, err
	}
	if err := appendLine(filepath.Join(root, CoreImportBaseline),
		"internal/core/gone/x.go\tinternal/component/thing\thand-fixable: stale fixture\n"); err != nil {
		return nil, err
	}
	edges, err = collectEdges(root, fixtureModule)
	if err != nil {
		return nil, err
	}
	stale, err := coreDirectionGate(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}
	results = append(results, caseResult("stale-core-baseline-row-fails", stale.Code == 2,
		"a stale baseline row passed, so the baseline can grow"))

	// Every pair is baselined here, so the ONLY thing wrong with the file is
	// the fix route one row names. A gate that stopped checking the route would
	// otherwise still answer 2 for the pairs it no longer holds.
	if err := writeFixture(root, map[string]string{
		CoreImportBaseline: strings.Join([]string{
			"internal/core/bad/uses.go\tinternal/component/thing\twontfix: not a legal route",
			"internal/core/bad/uses_test.go\tinternal/plugins/thingp\tneeds-design: selftest fixture",
			"",
		}, "\n"),
	}); err != nil {
		return nil, err
	}
	illegal, err := coreDirectionGate(root, fixtureModule, edges)
	if err != nil {
		return nil, err
	}
	return append(results, caseResult("illegal-fix-route-fails",
		illegal.Code == 2 && strings.Contains(illegal.Diagnosis, "malformed"),
		"a baseline row naming no legal fix route passed")), nil
}

// hasKey reports whether a map holds a key, which reads better than a two-value
// assignment inside a case table.
func hasKey(items map[string]string, key string) bool {
	_, ok := items[key]
	return ok
}

// sameMap reports whether two string maps hold the same pairs.
func sameMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

// samePairs reports whether two pair slices hold the same pairs.
func samePairs(left, right []corePair) bool {
	if len(left) != len(right) {
		return false
	}
	sortPairs(left)
	sortPairs(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// sortedNames answers a row set's names in order, which the package test reads.
func sortedNames(rows []Row) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	sort.Strings(names)
	return names
}

// runSelftest is the `le tier selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		// Code 2 distinguishes an unwritable fixture from failed detection.
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.Code(1)
}
