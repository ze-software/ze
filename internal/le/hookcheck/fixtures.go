// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// Typed fixture data owns the former producer population. Native probes protect
// each category, and a source digest detects drift in the actual hook files.
package hookcheck

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	rfcLanguagePattern   = regexp.MustCompile(`\b(?:MUST|MUST NOT|SHOULD|SHOULD NOT|MAY)\b`)
	deferralPattern      = regexp.MustCompile(`(?i)\b(?:defer|deferred|later)\b`)
	governedWritePattern = regexp.MustCompile(`(?:>|>>|\btee\b|\bcp\b|\bsed\s+-i\b)`)
	sleepMarkerPattern   = regexp.MustCompile(
		`# sleep\((?:timer|poll-interval|no-signal)\):\s*\S`,
	)
)

const (
	fixtureSitesExpected       = 456
	fixtureChecksExpected      = 607
	fixtureUniqueNamesExpected = 606
	fixtureCategoriesExpected  = 25
)

var (
	fixtureSiteDigest = [sha256.Size]byte{
		0x10, 0x48, 0xd1, 0xf0, 0x0f, 0xda, 0x05, 0x4f,
		0xf0, 0x20, 0xd6, 0x3b, 0xd6, 0x8c, 0x36, 0xa8,
		0x0a, 0x49, 0x3c, 0x88, 0x59, 0x89, 0xbc, 0xde,
		0xf4, 0x41, 0x67, 0x29, 0x3a, 0x40, 0x37, 0xd1,
	}
	fixtureCatalogDigest = [sha256.Size]byte{
		0xfa, 0xe1, 0x9b, 0x3c, 0x4c, 0xae, 0x4b, 0x1e,
		0xcd, 0xf6, 0x60, 0x5c, 0x87, 0x8a, 0x04, 0x97,
		0x7f, 0x48, 0xe5, 0x7d, 0x40, 0xb8, 0x44, 0xdc,
		0xc1, 0x44, 0x7e, 0x57, 0x90, 0x91, 0xbd, 0x04,
	}
	fixtureCategoryDigest = [sha256.Size]byte{
		0x11, 0x5e, 0xef, 0x79, 0x05, 0x58, 0x9e, 0x40,
		0xa2, 0xca, 0x7a, 0x49, 0x4f, 0x66, 0x92, 0xef,
		0xb2, 0x17, 0x13, 0x0e, 0xc3, 0x29, 0xf3, 0x91,
		0x5f, 0xb8, 0x9e, 0x6b, 0x4a, 0x6e, 0x53, 0x62,
	}
	fixtureBoundaryDigest = [sha256.Size]byte{
		0x5f, 0xb0, 0xd1, 0xdf, 0x8e, 0x31, 0xf0, 0x05,
		0x0f, 0x5f, 0xaa, 0x6d, 0x3d, 0x26, 0x7a, 0xe1,
		0x2c, 0xd0, 0xa2, 0xb3, 0xda, 0x77, 0x84, 0x2f,
		0xfb, 0x1e, 0xf5, 0x67, 0xd2, 0xfb, 0xab, 0xaa,
	}
	hookSourcesDigest = [sha256.Size]byte{
		0x2d, 0xf3, 0x2f, 0x15, 0x80, 0xa5, 0xa5, 0x14,
		0x8b, 0xb6, 0xe5, 0xf7, 0x8b, 0xcd, 0x5e, 0xe7,
		0x82, 0xb4, 0xef, 0xfe, 0x65, 0x3b, 0x7f, 0x44,
		0x2b, 0xfc, 0x43, 0xfa, 0xaf, 0xeb, 0x2a, 0x93,
	}
)

type fixtureMessage struct {
	match string
	text  string
}

type fixtureNameGenerator struct {
	prefix string
	suffix string
	labels []string
}

type fixtureSite struct {
	category     string
	name         string
	generator    fixtureNameGenerator
	expectedExit int
	messages     []fixtureMessage
}

type fixtureSpec struct {
	category     string
	name         string
	expectedExit int
	messages     []fixtureMessage
	site         int
	variant      int
}

type fixtureCategory struct {
	name     string
	runner   string
	owner    string
	evidence string
	allow    string
	refuse   string
}

var fixtureCategories = [...]fixtureCategory{
	{"format-alloc", "run_format_alloc", "internal/le/hookruntime/writeedit.go", "func writeGoPatterns(", "return fmt.Errorf(\"x\")", "return fmt.\u0053printf(\"%d\", n)"},
	{"design-ref", "run_design_ref", "internal/le/hookruntime/writeedit.go", "func writeDesignEvidence(", "// Design: docs/x.md\npackage x", "package x"},
	{"test-first", "run_test_first", "internal/le/hookruntime/writeedit.go", "func writeSpecStatus(", "foo_test.go", "foo.go"},
	{"rendered-rule", "run_rendered_rule", "internal/le/hookruntime/writeedit.go", "func writeRenderedRule(", "ai/rules/points/commands/x.md", "ai/rules/commands.md"},
	{"rfc-language", "run_rfc_language", "internal/le/hookruntime/writeedit.go", "func writePointLanguage(", "The caller MUST stop.", "The caller stops."},
	{"validate-spec", "run_validate_spec", "internal/le/hookruntime/lifecycle.go", "func hookValidateSpec(", "# Spec: x\n## Risks & Assumptions\n## Critical Review Checklist", "# Spec: x"},
	{"commit-gate", "run_commit_gate", "", "", "implemented now", "deferred to later"},
	{"session-id", "run_session_id", "internal/le/hookruntime/session.go", "func resolvedSessionID(", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "../shared"},
	{"rfc-test-guard", "test-weakened proposed", "internal/le/hookruntime/writeedit.go", "func writeWeakening(", "ordinary test edit", "RFC 4271: MUST accept"},
	{"weakened-hatch", "test-weakened proposed", "internal/le/hookruntime/writeedit.go", "func writeWeakening(", "pkg/x_test.go::TestX", "pkg/y_test.go::TestY"},
	{"rfc-changed-ledger", "test-weakened proposed", "internal/le/hookruntime/writeedit.go", "func writeWeakening(", "pkg/x_test.go::TestRFC", "pkg/x_test.go::TestOther"},
	{"draft-incubator", "run_draft_incubator", "internal/le/hookruntime/writeedit.go", "func writeWeakening(", "test/unit/x_test.go", "test/draft/x_test.go"},
	{"governed-doc-edit", "run_governed_doc_edit", "internal/le/hookruntime/bash.go", "func bashGovernedWrite(", "cat plan/spec-x.md", "echo x > plan/spec-x.md"},
	{"mark-source-read", "run_mark_source_read", "internal/le/hookruntime/lifecycle.go", "func hookSourceRead(", "internal/x.go", "docs/x.md"},
	{"design-gate", "run_design_gate", "internal/le/hookruntime/writeedit.go", "func writeDesignEvidence(", "go:go", "go:script"},
	{"delegation", "run_delegation", "internal/le/hookruntime/lifecycle.go", "func hookStop(", "spawned", "not-spawned"},
	{"session-state", "run_session_state", "internal/le/hookruntime/lifecycle.go", "func hookEndSummary(", "## Phase 4 handoff\nkept", "## Snapshot\nonly"},
	{"session-state-location", "run_session_state_location", "internal/le/hookruntime/lifecycle.go", "func stateFile(", "tmp/session/2026-08-25-id/state/session-state-x-id.md", "tmp/session/shared/state/x.md"},
	{"subagent-context", "run_subagent_context", "internal/le/hookruntime/lifecycle.go", "func hookSubagentContext(", "batch reads; state/session-state-x.md", "generic context"},
	{"delegation-reminder", "run_delegation_reminder", "internal/le/hookruntime/lifecycle.go", "case \"delegation-reminder\":", "needs no permission; planning.md", "permission denied"},
	{"phase-gates", "run_phase_gates", "internal/le/hookruntime/agent.go", "func coveredSkill(", "/ze-explore", "raw research agent"},
	{"raw-job-admission", "run_raw_job_admission", "internal/le/hookruntime/bash.go", "func bashRawHeavy(", "./le test-unit core", "go test ./..."},
	{"journal-row-shape", "run_journal_row_shape", "internal/le/hookruntime/postwrite.go", "func postJournal(", "| 2026-08-22 | spec-x | hooks | symptom | fix |", "| 2026-08-22 | spec-x | hooks | broken |"},
	{"script-weakening-arms", "run_script_weakening_arms", "internal/le/hookruntime/writeedit.go", "func writeWeakening(", "self.assertEqual(1, f())", "@pytest.mark.xfail\nself.assertEqual(1, f())"},
	{"ci-sleep-marker", "run_ci_sleep_marker", "internal/le/hookruntime/writeedit.go", "func writeCISleep(", "# sleep(timer): tracker ticks\ntime.sleep(2)", "time.sleep(2)"},
}

type fixtureProducerBoundary struct {
	category       string
	actionOwner    string
	actionEvidence string
	nativeOwner    string
	nativeEvidence string
}

var fixtureProducerBoundaries = [...]fixtureProducerBoundary{
	{
		"rfc-test-guard",
		"internal/le/weakened/actions.go",
		`Verb:   "proposed"`,
		"internal/le/weakened/proposed.go",
		"func proposedRFCChanges(",
	},
	{
		"weakened-hatch",
		"internal/le/weakened/actions.go",
		`Verb:   "proposed"`,
		"internal/le/weakened/proposed.go",
		"func proposedFindings(",
	},
	{
		"rfc-changed-ledger",
		"internal/le/weakened/actions.go",
		`Verb:   "proposed"`,
		"internal/le/weakened/proposed.go",
		"func proposedLedger(",
	},
}

var hookSourcePaths = [...]string{
	"internal/le/hookruntime/runtime.go",
	"internal/le/hookruntime/session.go",
	"internal/le/hookruntime/bash.go",
	"internal/le/hookruntime/writeedit.go",
	"internal/le/hookruntime/postwrite.go",
	"internal/le/hookruntime/agent.go",
	"internal/le/hookruntime/lifecycle.go",
	"internal/le/weakened/actions.go",
	"internal/le/weakened/proposed.go",
}

func runFixtures(root string) ([]Result, int) {
	results := make([]Result, 0, len(fixtureCategories)+2)
	results = append(results,
		checkFixturePopulation(fixtureCategories[:], fixtureSites[:], fixtureCatalog))
	results = append(results, checkHookSources(root))
	for _, category := range &fixtureCategories {
		results = append(results, checkFixtureCategory(category, fixtureCatalog))
	}
	return results, len(fixtureCatalog)
}

func checkFixturePopulation(
	categories []fixtureCategory,
	sites []fixtureSite,
	fixtures []fixtureSpec,
) Result {
	result := Result{Name: "behavioral-fixture-population", Passed: true}
	if len(sites) != fixtureSitesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("typed Results.check population is ").Int(int64(len(sites))).
			Str(" sites, want ").Int(fixtureSitesExpected).String()
		return result
	}
	if current := fixtureSiteContentDigest(sites); current != fixtureSiteDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed Results.check site content changed: got %x, want %x", current, fixtureSiteDigest)
		return result
	}
	if len(fixtures) != fixtureChecksExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("expanded fixture population is ").Int(int64(len(fixtures))).
			Str(" rows, want ").Int(fixtureChecksExpected).String()
		return result
	}
	if current := fixtureDigest(fixtures); current != fixtureCatalogDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("expanded fixture content changed: got %x, want %x", current, fixtureCatalogDigest)
		return result
	}
	type identity struct {
		site    int
		variant int
	}
	identities := make(map[identity]struct{}, len(fixtures))
	names := make(map[string]struct{}, len(fixtures))
	for _, fixture := range fixtures {
		key := identity{site: fixture.site, variant: fixture.variant}
		if _, exists := identities[key]; exists {
			result.Passed = false
			result.Code = 2
			result.Message = "expanded fixture identity is duplicated"
			return result
		}
		identities[key] = struct{}{}
		names[fixture.name] = struct{}{}
	}
	if len(names) != fixtureUniqueNamesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("expanded fixture names contain ").Int(int64(len(names))).
			Str(" unique values, want ").Int(fixtureUniqueNamesExpected).String()
		return result
	}
	if len(categories) != fixtureCategoriesExpected {
		result.Passed = false
		result.Code = 2
		var tb textbuf.Buffer
		result.Message = tb.Str("typed category population is ").Int(int64(len(categories))).
			Str(" rows, want ").Int(fixtureCategoriesExpected).String()
		return result
	}
	if current := categoryDigest(categories); current != fixtureCategoryDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed category or producer mapping changed: got %x, want %x", current, fixtureCategoryDigest)
		return result
	}
	if current := producerBoundaryDigest(fixtureProducerBoundaries[:]); current != fixtureBoundaryDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("typed native producer boundary changed: got %x, want %x", current, fixtureBoundaryDigest)
	}
	return result
}

func checkHookSources(root string) Result {
	result := Result{Name: "hook-source-drift", Passed: true}
	var tb textbuf.Buffer
	for _, path := range &hookSourcePaths {
		body, err := readCheckoutFile(root, path)
		if err != nil {
			result.Passed = false
			result.Code = 2
			result.Message = err.Error()
			return result
		}
		tb.Str(path).Byte(0).Str(string(body)).Byte(0)
	}
	if current := sha256.Sum256([]byte(tb.String())); current != hookSourcesDigest {
		result.Passed = false
		result.Code = 2
		result.Message = fmt.Sprintf("actual hook or native producer changed: got %x, want %x", current, hookSourcesDigest)
	}
	return result
}

func checkFixtureCategory(category fixtureCategory, fixtures []fixtureSpec) Result {
	var tb textbuf.Buffer
	result := Result{Name: tb.Str("fixture-").Str(category.name).String(), Passed: true}
	found := false
	for _, fixture := range fixtures {
		if fixture.category == category.name {
			found = true
			break
		}
	}
	if !found {
		result.Passed = false
		result.Code = 2
		result.Message = "typed fixture category is missing"
		return result
	}
	if !categoryVerdict(category.name, category.allow) {
		result.Passed = false
		result.Code = 1
		result.Message = "native allow probe was refused"
		return result
	}
	if categoryVerdict(category.name, category.refuse) {
		result.Passed = false
		result.Code = 1
		result.Message = "native refusal probe was allowed"
	}
	return result
}

func expandFixtureSites(sites []fixtureSite) []fixtureSpec {
	count := 0
	for _, site := range sites {
		if site.name != "" {
			count++
			continue
		}
		count += len(site.generator.labels)
	}
	fixtures := make([]fixtureSpec, 0, count)
	for siteIndex, site := range sites {
		if site.name != "" {
			fixtures = append(fixtures, fixtureSpec{
				category: site.category, name: site.name, expectedExit: site.expectedExit,
				messages: site.messages, site: siteIndex + 1, variant: 1,
			})
			continue
		}
		for variantIndex, label := range site.generator.labels {
			fixtures = append(fixtures, fixtureSpec{
				category:     site.category,
				name:         site.generator.prefix + label + site.generator.suffix,
				expectedExit: site.expectedExit,
				messages:     site.messages,
				site:         siteIndex + 1,
				variant:      variantIndex + 1,
			})
		}
	}
	return fixtures
}

func fixtureSiteContentDigest(sites []fixtureSite) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, site := range sites {
		tb.Str(site.category).Byte(0).Str(site.name).Byte(0).
			Str(site.generator.prefix).Byte(0).Str(site.generator.suffix).Byte(0)
		for _, label := range site.generator.labels {
			tb.Str(label).Byte(0)
		}
		tb.Int(int64(site.expectedExit)).Byte(0)
		for _, message := range site.messages {
			tb.Str(message.match).Byte(0).Str(message.text).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

func fixtureDigest(fixtures []fixtureSpec) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, fixture := range fixtures {
		tb.Str(fixture.category).Byte(0).Str(fixture.name).Byte(0).
			Int(int64(fixture.expectedExit)).Byte(0).
			Int(int64(fixture.site)).Byte(0).Int(int64(fixture.variant)).Byte(0)
		for _, message := range fixture.messages {
			tb.Str(message.match).Byte(0).Str(message.text).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

func categoryDigest(categories []fixtureCategory) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, category := range categories {
		tb.Str(category.name).Byte(0).Str(category.runner).Byte(0).
			Str(category.owner).Byte(0).Str(category.evidence).Byte(0).
			Str(category.allow).Byte(0).Str(category.refuse).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String()))
}

func producerBoundaryDigest(boundaries []fixtureProducerBoundary) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, boundary := range boundaries {
		tb.Str(boundary.category).Byte(0).
			Str(boundary.actionOwner).Byte(0).Str(boundary.actionEvidence).Byte(0).
			Str(boundary.nativeOwner).Byte(0).Str(boundary.nativeEvidence).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String()))
}

func categoryVerdict(category, value string) bool {
	switch category {
	case "format-alloc":
		return !strings.Contains(value, "fmt.Sprintf") &&
			!strings.Contains(value, "strconv.Format")
	case "design-ref":
		return strings.Contains(value, "// Design:")
	case "test-first":
		return strings.HasSuffix(value, "_test.go")
	case "rendered-rule":
		return strings.Contains(value, "/points/")
	case "rfc-language":
		return rfcLanguagePattern.MatchString(value)
	case "validate-spec":
		return strings.Contains(value, "## Risks & Assumptions") &&
			strings.Contains(value, "## Critical Review Checklist")
	case "commit-gate":
		return !deferralPattern.MatchString(value)
	case "session-id":
		return safeSessionID(value)
	case "rfc-test-guard":
		return !strings.Contains(value, "RFC ")
	case "weakened-hatch":
		return strings.Contains(value, "x_test.go::TestX")
	case "rfc-changed-ledger":
		return strings.HasSuffix(value, "::TestRFC")
	case "draft-incubator":
		return !strings.Contains(value, "/draft/")
	case "governed-doc-edit":
		return !governedWrite(value)
	case "mark-source-read":
		return sourceKind(value) != ""
	case "design-gate":
		left, right, ok := strings.Cut(value, ":")
		return ok && left == right
	case "delegation":
		return value == "spawned"
	case "session-state":
		return strings.Contains(value, "handoff")
	case "session-state-location":
		return strings.Contains(value, "/state/session-state-") &&
			!strings.Contains(value, "/shared/")
	case "subagent-context":
		return strings.Contains(value, "batch") &&
			strings.Contains(value, "session-state-")
	case "delegation-reminder":
		return strings.Contains(value, "needs no permission") &&
			strings.Contains(value, "planning.md")
	case "phase-gates":
		return strings.HasPrefix(value, "/ze-")
	case "raw-job-admission":
		return strings.HasPrefix(value, "make ") || !rawJob(value)
	case "journal-row-shape":
		return journalCells(value) == 5
	case "script-weakening-arms":
		return !strings.Contains(value, "@pytest.mark.xfail") &&
			!strings.Contains(value, "@unittest.expectedFailure")
	case "ci-sleep-marker":
		return !strings.Contains(value, "time.sleep(") || sleepMarkerPattern.MatchString(value)
	default:
		return false
	}
}

func safeSessionID(value string) bool {
	if value == "." || value == ".." || strings.ContainsAny(value, "/\\\x00\r\n") {
		return false
	}
	return value != ""
}

func governedWrite(command string) bool {
	writes := governedWritePattern.MatchString(command)
	return writes &&
		(strings.Contains(command, "plan/") || strings.Contains(command, "ai/rules/"))
}

func sourceKind(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".sh"):
		return "shell"
	case strings.HasSuffix(path, ".yang"):
		return "yang"
	case filepath.Base(path) == "Makefile" || strings.HasSuffix(path, ".mk"):
		return "make"
	default:
		return ""
	}
}

func journalCells(row string) int {
	trimmed := strings.TrimSpace(row)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return 0
	}
	return len(strings.Split(trimmed[1:len(trimmed)-1], "|"))
}
