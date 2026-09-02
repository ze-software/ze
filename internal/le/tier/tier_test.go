package tier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
)

// fixtureTree writes a fixture checkout and answers its root.
func fixtureTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := writeFixture(root, files); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return root
}

// VALIDATES: the plugin search roots this gate scans are the composition-root
// generator's own list, obtained by a CALL.
// PREVENTS: the step-14 landmine. The script parsed the roots out of
// internal/le/plugin/imports/pluginimports.go's source text, so deleting that
// script leaves the gate with no roots and every nested sub-plugin
// engine reads as misplaced.
func TestThePluginDirsAreTheGeneratorsOwnList(t *testing.T) {
	got, want := PluginDirs(), pluginimports.PluginSearchRoots()
	if !slices.Equal(got, want) {
		t.Fatalf("PluginDirs() = %q, the generator says %q", got, want)
	}
	if len(got) == 0 {
		t.Fatal("the generator names no plugin search root, so the gate would flag every engine")
	}
	if !slices.Contains(got, nestedFixtureRoot) {
		t.Fatalf("the selftest's nested fixture sits under %s, which the generator does not name: %q",
			nestedFixtureRoot, got)
	}
}

// VALIDATES: a nested sub-plugin namespace is one that sits under
// internal/component and is deeper than a top-level subsystem.
// PREVENTS: a top-level plugin root such as internal/plugins being read as a
// nested namespace, which would exclude every plugin engine from the
// gate.
func TestOnlyADeepComponentRootIsANestedNamespace(t *testing.T) {
	nested := nestedNamespaces([]string{
		AreaPlugins,
		"internal/component/iface",
		"internal/component/bgp/plugins",
		"internal/component/bgp/reactor/filter",
	})
	want := []string{"internal/component/bgp/plugins", "internal/component/bgp/reactor/filter"}
	if !slices.Equal(nested, want) {
		t.Fatalf("nested namespaces %q, want %q", nested, want)
	}
}

// VALIDATES: every selftest case passes, so each of the five checks still
// detects what it exists to detect.
// PREVENTS: a check whose detection broke printing the same clean page as a
// check over a clean tree.
func TestTheSelftestPasses(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("running the selftest: %v", err)
	}
	for _, failure := range report.Failures() {
		t.Errorf("selftest case %s: %s", failure.Case, failure.Detail)
	}
	if len(report.Results) < 20 {
		t.Fatalf("the selftest ran %d cases, and the script's fixtures carry more than that", len(report.Results))
	}
	if report.Code(1) != 0 {
		t.Fatalf("the selftest answers %d over its own fixtures", report.Code(1))
	}
	if got := report.Text(); got != "dep_audit selftest OK\n" {
		t.Fatalf("the selftest page is %q, and the script prints \"dep_audit selftest OK\"", got)
	}
}

// VALIDATES: each selftest case would FAIL if the fixture it judges changed, so
// the table is not a list of assertions nothing can break.
// PREVENTS: a selftest that passes over a fixture it never read, which is the
// failure the selftest itself exists to catch one level down.
func TestTheSelftestCasesAreNamedAndDistinct(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("running the selftest: %v", err)
	}
	seen := make(map[string]bool, len(report.Results))
	for _, row := range report.Results {
		if row.Case == "" {
			t.Error("a selftest row carries no case name")
		}
		if seen[row.Case] {
			t.Errorf("two selftest rows are both named %s, so one of them cannot be looked up", row.Case)
		}
		seen[row.Case] = true
	}
	for _, name := range []string{
		"misplaced-engines", "nested-sub-plugin-excluded", "dispatch-wired-plugin",
		"always-on-importer", "sibling-worktree-not-scanned", "stale-baseline-entry-fails",
		"golangci-missing-gate-fails", "new-pair-in-baselined-file-fails", "illegal-fix-route-fails",
	} {
		if !seen[name] {
			t.Errorf("the selftest no longer carries the %s case", name)
		}
	}
}

// VALIDATES: the three registration shapes are recognized and nothing else is.
// PREVENTS: a gated ENGINE's blank import moving from all.go into all_<tag>.go
// reading as a feature dependency, which would send it to the wrong
// tier.
func TestOnlyACompositionRootIsARegistrationImporter(t *testing.T) {
	cases := []struct {
		file string
		want bool
	}{
		{"internal/component/plugin/all/all.go", true},
		{"internal/component/plugin/all/all_ze_isis.go", true},
		{"cmd/ze/ze_core_dispatch.go", true},
		{"cmd/ze/plugin_imports.go", true},
		{"cmd/ze/setup_features_distro.go", true},
		{"internal/component/plugin/all/helper.go", false},
		{"internal/component/other/all/all_ze_isis.go", false},
		{"cmd/ze-test/ze_core_dispatch.go", false},
		{"internal/plugins/log/register.go", false},
		{"cmd/ze/setup.go", false},
	}
	for _, testCase := range cases {
		if got := isRegistrationImporter(testCase.file); got != testCase.want {
			t.Errorf("IsRegistrationImporter(%q) = %v, want %v", testCase.file, got, testCase.want)
		}
	}
}

// VALIDATES: the edge scan skips the module cache and every sibling agent
// worktree, and it reads the imports of everything else.
// PREVENTS: another session's in-progress violation being reported against this
// tree, which is what happened the first time the boundary gate ran
// inside the live pre-commit path.
func TestTheEdgeScanSkipsTheModuleCacheAndSiblingWorktrees(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		"go.mod":                          "module example.com/m\n",
		"internal/a/x.go":                 blankImport("a", "internal/component/thing"),
		"gokrazy/modcache/copy/y.go":      blankImport("copy", "internal/component/thing"),
		agentWorktree:                     blankImport("app", "internal/component/thing"),
		"vendor/other/z.go":               blankImport("other", "internal/component/thing"),
		"tmp/scratch/t.go":                blankImport("scratch", "internal/component/thing"),
		"internal/node_modules/deep/n.go": blankImport("deep", "internal/component/thing"),
	})
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		t.Fatalf("collecting edges: %v", err)
	}
	importers := edges["example.com/m/internal/component/thing"]
	if !slices.Equal(importers, []string{"internal/a/x.go"}) {
		t.Fatalf("importers %q, want internal/a/x.go alone", importers)
	}
}

// VALIDATES: a Go file the scan cannot READ stops the run.
// PREVENTS: the script's fail-open, where an unreadable file contributes no
// import edge and a LOWER violation count is what passing looks like.
func TestAnUnreadableGoFileStopsTheEdgeScan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so this case cannot be staged")
	}
	root := fixtureTree(t, map[string]string{
		"go.mod":          "module example.com/m\n",
		"internal/a/x.go": blankImport("a", "internal/component/thing"),
	})
	path := filepath.Join(root, "internal", "a", "x.go")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skipf("cannot make a file unreadable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	if _, err := collectEdges(root, fixtureModule); err == nil {
		t.Fatal("an unreadable Go file was passed over, so its import edges never reach the direction gate")
	}
}

// VALIDATES: an engine is misplaced when the area it sits in disagrees with the
// area its dependants put it in, and correctly placed otherwise.
// PREVENTS: the dependency test being dropped, which would send every component
// engine to plugins.
func TestAnEngineIsPlacedByWhoDependsOnIt(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		t.Fatalf("collecting edges: %v", err)
	}
	misplaced, err := engineMisplacements(root, fixtureModule, edges)
	if err != nil {
		t.Fatalf("computing misplacements: %v", err)
	}
	want := map[string]string{
		"internal/component/edgeproto": AreaPlugins,
		"internal/plugins/platformx":   AreaComponent,
	}
	if !sameMap(misplaced, want) {
		t.Fatalf("misplacements %v, want %v", misplaced, want)
	}
}

// VALIDATES: the engine scan reads only component and plugin areas.
// PREVENTS: moving the development tools under internal from making an
// engine-shaped fixture in internal/le part of the product tier population.
func TestTheEngineScanIgnoresInternalLe(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		"go.mod":                          "module example.com/m\n",
		"internal/le/tool/tool.go":        engineFile("tool"),
		"internal/plugins/edge/engine.go": engineFile("edge"),
	})

	dirs, err := findEngineDirs(root, nil)
	if err != nil {
		t.Fatalf("finding engine directories: %v", err)
	}
	if !slices.Equal(dirs, []string{"internal/plugins/edge"}) {
		t.Fatalf("engine directories %q, want internal/plugins/edge alone", dirs)
	}
}

// VALIDATES: the baseline the writer emits is the one the reader reads back,
// and each row names the child spec that removes it.
// PREVENTS: a round trip that loses a row, which would make every regenerated
// baseline look stale on the next run.
func TestTheBaselineRoundTripsThroughItsOwnWriter(t *testing.T) {
	root := t.TempDir()
	misplaced := map[string]string{
		"internal/component/edgeproto": AreaPlugins,
		"internal/plugins/platformx":   AreaComponent,
	}
	if err := writeBaseline(root, misplaced); err != nil {
		t.Fatalf("writing the baseline: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, Baseline))
	if err != nil {
		t.Fatalf("reading the baseline: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "internal/component/edgeproto\tinternal/plugins\tspec-tiers-2\n") {
		t.Errorf("an engine bound for plugins does not cite spec-tiers-2:\n%s", text)
	}
	if !strings.Contains(text, "internal/plugins/platformx\tinternal/component\tspec-tiers-3\n") {
		t.Errorf("an engine bound for component does not cite spec-tiers-3:\n%s", text)
	}

	read, err := ReadBaseline(root)
	if err != nil {
		t.Fatalf("reading the baseline back: %v", err)
	}
	if len(read) != 2 || !read["internal/component/edgeproto"] || !read["internal/plugins/platformx"] {
		t.Fatalf("the baseline read back as %v", read)
	}
}

// VALIDATES: a category is legal only WHERE the rule allows it, and the reason
// names the constraint.
// PREVENTS: a category becoming a universal escape hatch, which is what the
// manifest exists to prevent in the first place.
func TestACategoryIsLegalOnlyWhereItsRuleAllowsIt(t *testing.T) {
	setupWired := Row{Registration: []string{"cmd/ze/setup_features_distro.go"}}
	plainWired := Row{Registration: []string{"internal/component/plugin/all/all.go"}}
	cases := []struct {
		rel, category string
		row           Row
		want          bool
	}{
		{"internal/component/frameworklib", "framework", Row{}, true},
		{"internal/plugins/setupcmd", "framework", setupWired, true},
		{"internal/plugins/other", "framework", plainWired, false},
		{"internal/component/host", "host-service", Row{}, true},
		{"internal/plugins/host", "host-service", Row{}, false},
		{"internal/component/l2tp/session", "domain-library", Row{}, true},
		{"internal/component/ike", "domain-library", Row{}, true},
		{"internal/component/bgp", "domain-library", Row{}, false},
		{"internal/plugins/oldleaf", "planned-violation", Row{}, true},
		{"internal/component/x", "made-up", Row{}, false},
	}
	for _, testCase := range cases {
		got, reason := validNonEngineCategory(testCase.rel, testCase.category, testCase.row)
		if got != testCase.want {
			t.Errorf("ValidNonEngineCategory(%q, %q) = %v (%s), want %v",
				testCase.rel, testCase.category, got, reason, testCase.want)
		}
		if !got && reason == "" {
			t.Errorf("%q as %q was refused with no reason", testCase.rel, testCase.category)
		}
	}
}

// VALIDATES: the manifest's own problems are reported with the file and line,
// and a planned-violation must cite a spec.
// PREVENTS: a malformed row being passed over, which would silently unclassify
// whatever it was supposed to declare.
func TestTheManifestReportsItsOwnProblems(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		NonEngineCategories: strings.Join([]string{
			"# a comment",
			"internal/component/a\tframework\tfine",
			"internal/component/a\tframework\tduplicate",
			"internal/component/b\tinvented\tno such category",
			"internal/component/c\tplanned-violation\tno spec cited here",
			"internal/component/d\tframework",
			"",
		}, "\n"),
	})
	_, problems, err := loadNonEngineCategories(root)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	want := []string{
		"duplicate row for internal/component/a",
		"unknown category 'invented'; expected one of domain-library, framework, host-service, planned-violation",
		"planned-violation rationale must cite a spec-* reference",
		"expected '<path> <category> <rationale>'",
	}
	if len(problems) != len(want) {
		t.Fatalf("problems %q, want %d of them", problems, len(want))
	}
	for i, fragment := range want {
		if !strings.Contains(problems[i], fragment) {
			t.Errorf("problem %d is %q, want it to carry %q", i, problems[i], fragment)
		}
		if !strings.HasPrefix(problems[i], NonEngineCategories+":") {
			t.Errorf("problem %d does not name the manifest and its line: %q", i, problems[i])
		}
	}
}

// VALIDATES: a manifest row naming a directory that is not there is reported,
// and so is one naming a path outside the two registry areas.
// PREVENTS: a stale manifest row surviving the package it was written for,
// which turns the manifest into a list nobody has to keep true.
func TestAManifestRowMustNameADirectoryThatExists(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		"go.mod":                         "module example.com/m\n",
		"internal/component/here/lib.go": "package here\n",
		// Both registry areas must exist: the classification lists each one,
		// and an area that is absent stops the walk in both halves.
		"internal/plugins/keep/lib.go": "package keep\n",
		FeatureGatesManifest:           "# none\n",
		Golangci:                       "run:\n  build-tags:\n    - ze_core\nlinters: {}\n",
		NonEngineCategories: strings.Join([]string{
			"internal/component/here\tframework\tthe package that is there",
			"internal/component/gone\tframework\tthe package that is not",
			"internal/core/elsewhere\tframework\toutside both registries",
			"internal/plugins/keep\tplanned-violation\tspec-tiers-5 the second area's own package",
			"",
		}, "\n"),
	})
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		t.Fatalf("collecting edges: %v", err)
	}
	problems, err := nonEngineCategoryProblems(root, fixtureModule, edges)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	var sawMissing, sawOutside bool
	for _, problem := range problems {
		if strings.Contains(problem, "internal/component/gone does not exist") {
			sawMissing = true
		}
		if strings.Contains(problem, "internal/core/elsewhere is outside") {
			sawOutside = true
		}
		if strings.Contains(problem, "internal/component/here") {
			t.Errorf("a row naming a package that is there was reported: %s", problem)
		}
	}
	if !sawMissing {
		t.Errorf("a manifest row naming a directory that is gone was not reported: %q", problems)
	}
	if !sawOutside {
		t.Errorf("a manifest row outside both registries was not reported: %q", problems)
	}
}

// VALIDATES: a build constraint that NEGATES a tag does not require it.
// PREVENTS: `//go:build !ze_isis` reading as a gated importer, which would
// exempt the one file that proves the feature can be compiled out.
func TestANegatedTagDoesNotRequireIt(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		"gated.go":     "//go:build ze_widget\n\npackage a\n",
		"negated.go":   "//go:build !ze_widget\n\npackage a\n",
		"compound.go":  "//go:build ze_widget && linux\n\npackage a\n",
		"other.go":     "//go:build ze_other\n\npackage a\n",
		"plain.go":     "package a\n",
		"comment.go":   "// a comment first\n//go:build ze_widget\n\npackage a\n",
		"codefirst.go": "package a\n\n//go:build ze_widget\n",
	})
	cases := map[string]bool{
		"gated.go": true, "negated.go": false, "compound.go": true, "other.go": false,
		"plain.go": false, "comment.go": true, "codefirst.go": false,
	}
	for file, want := range cases {
		if got := fileRequiresTag(root, file, "ze_widget"); got != want {
			t.Errorf("FileRequiresTag(%q) = %v, want %v", file, got, want)
		}
	}
}

// VALIDATES: the feature-gate manifest refuses a malformed line rather than
// dropping it.
// PREVENTS: a gate map that came back short, which finds no violation for the
// package whose line was skipped.
func TestAMalformedFeatureGateLineIsRefused(t *testing.T) {
	root := fixtureTree(t, map[string]string{FeatureGatesManifest: "ze_alpha\n"})
	if _, err := loadFeatureGates(root); err == nil {
		t.Fatal("a line naming a tag and no package was accepted")
	}
	if _, err := loadFeatureGates(t.TempDir()); err == nil {
		t.Fatal("a tree with no manifest answered a gate map instead of an error")
	}
}

// VALIDATES: the lint build-tags list stops at the next key and tolerates a
// comment.
// PREVENTS: a later list in the file being read as build tags, which would
// report drift for tags nobody declared.
func TestTheBuildTagsListStopsAtTheNextKey(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		Golangci: strings.Join([]string{
			"run:",
			"  build-tags:",
			"    - ze_core  # base tag",
			"    # the gates:",
			"    - ze_alpha",
			"linters:",
			"  enable:",
			"    - govet",
			"",
		}, "\n"),
	})
	tags, err := parseGolangciBuildTags(root)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(tags) != 2 || !tags["ze_core"] || !tags["ze_alpha"] {
		t.Fatalf("tags %v, want ze_core and ze_alpha alone", tags)
	}
}

// VALIDATES: the core direction gate reads TEST files too.
// PREVENTS: the grandfathered set losing its one test-file pair, which would
// make that row look stale and red the gate.
func TestTheCoreDirectionGateReadsTestFiles(t *testing.T) {
	root := fixtureTree(t, coreFixture())
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		t.Fatalf("collecting edges: %v", err)
	}
	pairs := coreDirectionViolations(fixtureModule, edges)
	if len(pairs) != 2 {
		t.Fatalf("pairs %+v, want two", pairs)
	}
	if pairs[1].File != "internal/core/bad/uses_test.go" {
		t.Fatalf("the test-file pair is missing: %+v", pairs)
	}
}

// VALIDATES: the check's verdicts go to the page and its failures to the
// diagnosis, never both to one stream.
// PREVENTS: a merged capture, whose interleaving no runner can order and no
// comparison can settle.
func TestTheVerdictAndTheFailureAreDifferentStreams(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if report.Failed != 2 {
		t.Fatalf("an empty baseline answered %d, want 2", report.Failed)
	}
	if !strings.Contains(report.Diagnosis(), "FAIL: new misplaced engine(s)") {
		t.Errorf("the diagnosis does not name the failure:\n%s", report.Diagnosis())
	}
	if strings.Contains(report.Text(), "FAIL:") {
		t.Errorf("a failure reached the verdict stream:\n%s", report.Text())
	}
	for _, gate := range report.Checks {
		if gate.Page != "" && gate.Diagnosis != "" {
			t.Errorf("gate %s wrote to both streams", gate.Name)
		}
	}
}

// VALIDATES: the run's code is the FIRST failing check's, not the last one's.
// PREVENTS: a later check's code overwriting the first, which would tell a
// caller to look at the wrong thing.
func TestTheFirstFailingCheckDecidesTheCode(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	report, err := Check(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	first := 0
	for _, gate := range report.Checks {
		if gate.Code != 0 {
			first = gate.Code
			break
		}
	}
	if report.Failed != first {
		t.Fatalf("the run answers %d and the first failing check answers %d", report.Failed, first)
	}
	if report.Checks[0].Name != "engine-placement" || len(report.Checks) != 5 {
		t.Fatalf("the checks ran as %+v, want the script's five in order", report.Checks)
	}

	// The five checks all answer 0 or 2, so first-versus-last is unobservable
	// from their own output. The rule is asserted where it can be seen.
	if got := firstFailure([]CheckResult{{Code: 0}, {Code: 3}, {Code: 1}}); got != 3 {
		t.Errorf("FirstFailure answered %d, want the first non-zero code", got)
	}
	if got := firstFailure([]CheckResult{{Code: 0}, {Code: 0}}); got != 0 {
		t.Errorf("FirstFailure answered %d for a run that passed", got)
	}
}

// VALIDATES: the audit answer is structured data with kebab-case keys.
// PREVENTS: a payload `| json` cannot render into the document the CLI contract
// promises.
func TestTheAuditReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	report, err := Report(root, DefaultAreas[:2])
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{"module", "areas", "order"} {
		if _, ok := document[key]; !ok {
			t.Errorf("the payload has no %q key", key)
		}
	}
	areas, _ := document["areas"].(map[string]any)
	rows, _ := areas[AreaPlugins].([]any)
	if len(rows) == 0 {
		t.Fatal("the report holds no row for internal/plugins")
	}
	row, _ := rows[0].(map[string]any)
	for _, key := range []string{"name", "external", "registration", "tests", "is-candidate", "is-registered", "is-engine", "core-candidate"} {
		if _, ok := row[key]; !ok {
			t.Errorf("a row has no %q key: %s", key, raw)
		}
	}
	for key := range row {
		if strings.Contains(key, "_") {
			t.Errorf("key %q is not kebab-case", key)
		}
	}
}

// VALIDATES: the report page groups the subsystems the way the script grouped
// them, and orders the shared libraries by importer count.
// PREVENTS: a page a reader cannot compare with the script's, which is the only
// rendering this action has.
func TestTheReportPageGroupsAndOrdersTheScriptsWay(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	report, err := Report(root, []string{AreaPlugins})
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	page := report.Text()
	for _, heading := range []string{
		"AREA: internal/plugins",
		"-- REGISTERED PLUGINS (wired by the generator / all.go): 4 --",
		"-- CORE CANDIDATES (0 external, not registered, not an engine): 3 --",
		"-- SHARED LIBRARIES (external importers, not a registered plugin): ",
	} {
		if !strings.Contains(page, heading) {
			t.Errorf("the page is missing %q:\n%s", heading, page)
		}
	}
	if !strings.Contains(page, "  leaflib                  registration=0 tests=0\n") {
		t.Errorf("the core-candidate row is not the script's shape:\n%s", page)
	}

	// PAGE order uses importer count, not only helper order. edge1 has no
	// importers and sorts last, although its name sorts first.
	_, shared, found := strings.Cut(page, "-- SHARED LIBRARIES")
	if !found {
		t.Fatalf("the page has no shared-libraries block:\n%s", page)
	}
	first, last := strings.Index(shared, "platformx"), strings.Index(shared, "edge1")
	if first < 0 || last < 0 || first > last {
		t.Errorf("the shared libraries are not ordered by importer count:\n%s", shared)
	}

	// The helper's own contract, beside the page it produces.
	rows := []Row{
		{Name: "one", External: []string{"a"}},
		{Name: "three", External: []string{"a", "b", "c"}},
		{Name: "two", External: []string{"a", "b"}},
	}
	sortByExternalCount(rows)
	if rows[0].Name != "three" || rows[1].Name != "two" || rows[2].Name != "one" {
		t.Fatalf("shared libraries ordered %s, %s, %s", rows[0].Name, rows[1].Name, rows[2].Name)
	}
}

// VALIDATES: the area publishes all four native actions, and only
// write-baseline writes.
// PREVENTS: an action leaving the command surface or the listing calling a
// check a write.
func TestTheAreaPublishesFourNativeActions(t *testing.T) {
	list := Actions()
	verbs := make([]string, 0, len(list.Actions))
	writes := make([]string, 0, 1)
	for _, row := range list.Actions {
		verbs = append(verbs, row.Verb)
		if row.Writes {
			writes = append(writes, row.Verb)
		}
	}
	if !slices.Equal(verbs, []string{"check", "selftest", "report", "write-baseline"}) {
		t.Fatalf("verbs %q", verbs)
	}
	if !slices.Equal(writes, []string{"write-baseline"}) {
		t.Fatalf("the actions that write are %q, and only write-baseline does", writes)
	}
}

// VALIDATES: an action this area does not hold answers 2, and an action given a
// value answers 1.
// PREVENTS: the two refusals collapsing into one code.
func TestTheAreaRefusesAnUnknownActionApartFromAValue(t *testing.T) {
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answered %d, want 2", code)
	}
	if _, code := Answer([]string{"check", AreaComponent}); code != 2 {
		t.Errorf("a value after an action answered %d, want 2", code)
	}
}

// VALIDATES: the classification names every top-level subsystem of an area,
// including one that is neither wired nor a candidate.
// PREVENTS: a subsystem falling out of the audit, which the manifest gate then
// never asks to classify.
func TestEveryTopLevelSubsystemIsClassified(t *testing.T) {
	root := fixtureTree(t, placementFixture())
	edges, err := collectEdges(root, fixtureModule)
	if err != nil {
		t.Fatalf("collecting edges: %v", err)
	}
	engines, err := engineSubsystems(root)
	if err != nil {
		t.Fatalf("finding engines: %v", err)
	}
	rows, err := Classify(AreaPlugins, root, fixtureModule, edges, engines)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	want := []string{"cmdverb", "consumer", "edge1", "genverb", "leaflib", "oldleaf", "platformx", "regengine", "setupcmd", "sharedlib"}
	if !slices.Equal(sortedNames(rows), want) {
		t.Fatalf("subsystems %q, want %q", sortedNames(rows), want)
	}
}
