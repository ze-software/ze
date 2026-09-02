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
		0xa3, 0xd9, 0x13, 0x1c, 0x90, 0x11, 0xb8, 0xde,
		0x1b, 0xeb, 0x33, 0x3c, 0x92, 0x7e, 0x1e, 0xab,
		0x0f, 0x9c, 0xf8, 0x2c, 0x27, 0xfd, 0x53, 0xb5,
		0x47, 0x87, 0x15, 0x50, 0x17, 0xbd, 0xa2, 0x75,
	}
	fixtureCatalogDigest = [sha256.Size]byte{
		0x94, 0x22, 0x6a, 0x4b, 0xb2, 0xae, 0xf9, 0xef,
		0x5b, 0xfb, 0xe1, 0xd2, 0x90, 0x12, 0x25, 0x05,
		0x2b, 0x5f, 0x63, 0xa2, 0xbb, 0xac, 0x10, 0x02,
		0xc8, 0x4b, 0x56, 0x46, 0xe1, 0x2d, 0xcb, 0xe3,
	}
	fixtureCategoryDigest = [sha256.Size]byte{
		0x11, 0x5e, 0xef, 0x79, 0x05, 0x58, 0x9e, 0x40,
		0xa2, 0xca, 0x7a, 0x49, 0x4f, 0x66, 0x92, 0xef,
		0xb2, 0x17, 0x13, 0x0e, 0xc3, 0x29, 0xf3, 0x91,
		0x5f, 0xb8, 0x9e, 0x6b, 0x4a, 0x6e, 0x53, 0x62,
	}
	fixtureBoundaryDigest = [sha256.Size]byte{
		0x5f, 0xe8, 0x52, 0x9b, 0xf0, 0x53, 0x49, 0xb9,
		0xf7, 0x65, 0x27, 0xdd, 0xd0, 0xd8, 0xa9, 0x6d,
		0x4b, 0xf3, 0x3e, 0x0b, 0x93, 0xbf, 0x9a, 0x83,
		0xe0, 0x5b, 0x55, 0x96, 0xd8, 0xf4, 0xf5, 0xfd,
	}
	hookSourcesDigest = [sha256.Size]byte{
		0x4d, 0x30, 0x25, 0x1e, 0x7a, 0xbb, 0xf2, 0x83,
		0xbd, 0x33, 0x98, 0x48, 0x61, 0x81, 0xf9, 0xbf,
		0xff, 0xd6, 0xcf, 0xd3, 0x72, 0xe8, 0x1d, 0xd4,
		0x4e, 0x3e, 0xb8, 0x01, 0x1e, 0xcb, 0x41, 0x8b,
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
	{categoryFormatAlloc, "run_format_alloc", hookWriteEditFile, "func writeGoPatterns(", "return fmt.Errorf(\"x\")", "return fmt.\u0053printf(\"%d\", n)"},
	{categoryDesignRef, "run_design_ref", hookWriteEditFile, "func writeDesignEvidence(", "// Design: docs/x.md\npackage x", "package x"},
	{categoryTestFirst, "run_test_first", hookWriteEditFile, "func writeSpecStatus(", "foo_test.go", "foo.go"},
	{categoryRenderedRule, "run_rendered_rule", hookWriteEditFile, "func writeRenderedRule(", "ai/rules/points/commands/x.md", "ai/rules/commands.md"},
	{categoryRFCLanguage, "run_rfc_language", hookWriteEditFile, "func writePointLanguage(", "The caller MUST stop.", "The caller stops."},
	{categoryValidateSpec, "run_validate_spec", hookLifecycleFile, "func hookValidateSpec(", "# Spec: x\n## Risks & Assumptions\n## Critical Review Checklist", "# Spec: x"},
	{categoryCommitGate, "run_commit_gate", "", "", "implemented now", "deferred to later"},
	{categorySessionID, "run_session_id", "internal/le/hookruntime/session.go", "func resolvedSessionID(", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "../shared"},
	{categoryRFCTestGuard, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, "ordinary test edit", "RFC 4271: MUST accept"},
	{categoryWeakenedHatch, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, "pkg/x_test.go::TestX", "pkg/y_test.go::TestY"},
	{categoryRFCChangedLedger, weakenedProposedFixture, hookWriteEditFile, writeWeakeningAnchor, "pkg/x_test.go::TestRFC", "pkg/x_test.go::TestOther"},
	{categoryDraftIncubator, "run_draft_incubator", hookWriteEditFile, writeWeakeningAnchor, "test/unit/x_test.go", "test/draft/x_test.go"},
	{categoryGovernedDocEdit, "run_governed_doc_edit", hookBashFile, "func bashGovernedWrite(", "cat plan/spec-x.md", "echo x > plan/spec-x.md"},
	{categoryMarkSourceRead, "run_mark_source_read", hookLifecycleFile, "func hookSourceRead(", "internal/x.go", "docs/x.md"},
	{categoryDesignGate, "run_design_gate", hookWriteEditFile, "func writeDesignEvidence(", "go:go", "go:script"},
	{categoryDelegation, "run_delegation", hookLifecycleFile, "func hookStop(", "spawned", "not-spawned"},
	{categorySessionState, "run_session_state", hookLifecycleFile, "func hookEndSummary(", "## Phase 4 handoff\nkept", "## Snapshot\nonly"},
	{categorySessionStateLocation, "run_session_state_location", hookLifecycleFile, "func stateFile(", "tmp/session/2026-08-25-id/state/session-state-x-id.md", "tmp/session/shared/state/x.md"},
	{categorySubagentContext, "run_subagent_context", hookLifecycleFile, "func hookSubagentContext(", "batch reads; state/session-state-x.md", "generic context"},
	{categoryDelegationReminder, "run_delegation_reminder", hookLifecycleFile, "case \"delegation-reminder\":", "needs no permission; planning.md", "permission denied"},
	{categoryPhaseGates, "run_phase_gates", "internal/le/hookruntime/agent.go", "func coveredSkill(", exploreSkill, "raw research agent"},
	{categoryRawJobAdmission, "run_raw_job_admission", hookBashFile, "func bashRawHeavy(", "./le test-unit core", "go test ./..."},
	{categoryJournalRowShape, "run_journal_row_shape", "internal/le/hookruntime/postwrite.go", "func postJournal(", "| 2026-08-22 | spec-x | hooks | symptom | fix |", "| 2026-08-22 | spec-x | hooks | broken |"},
	{categoryScriptWeakeningArms, "run_script_weakening_arms", hookWriteEditFile, writeWeakeningAnchor, "self.assertEqual(1, f())", "@pytest.mark.xfail\nself.assertEqual(1, f())"},
	{categoryCISleepMarker, "run_ci_sleep_marker", hookWriteEditFile, "func writeCISleep(", "# sleep(timer): tracker ticks\ntime.sleep(2)", "time.sleep(2)"},
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
		categoryRFCTestGuard,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedRFCChanges(",
	},
	{
		categoryWeakenedHatch,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedFindings(",
	},
	{
		categoryRFCChangedLedger,
		weakenedActionsFile,
		proposedVerbAnchor,
		weakenedProposedFile,
		"func proposedLedger(",
	},
}

var hookSourcePaths = [...]string{
	"internal/le/hookruntime/runtime.go",
	"internal/le/hookruntime/session.go",
	hookBashFile,
	hookWriteEditFile,
	"internal/le/hookruntime/postwrite.go",
	"internal/le/hookruntime/agent.go",
	hookLifecycleFile,
	weakenedActionsFile,
	weakenedProposedFile,
}

func runFixtures(root string) ([]Result, int) {
	results := make([]Result, 0, len(fixtureCategories)+2)
	results = append(results,
		checkFixturePopulation(fixtureCategories[:], fixtureSites[:], fixtureCatalog),
		checkHookSources(root))
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

// hookSourceContentDigest reads every hook and native producer file under root
// and answers one digest over their paths and their bodies.
func hookSourceContentDigest(root string) ([sha256.Size]byte, error) {
	var tb textbuf.Buffer
	for _, path := range &hookSourcePaths {
		body, err := readCheckoutFile(root, path)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		tb.Str(path).Byte(0).Str(string(body)).Byte(0)
	}
	return sha256.Sum256([]byte(tb.String())), nil
}

func checkHookSources(root string) Result {
	result := Result{Name: "hook-source-drift", Passed: true}
	current, err := hookSourceContentDigest(root)
	if err != nil {
		result.Passed = false
		result.Code = 2
		result.Message = err.Error()
		return result
	}
	if current != hookSourcesDigest {
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
	case categoryFormatAlloc:
		return !strings.Contains(value, "fmt.Sprintf") &&
			!strings.Contains(value, "strconv.Format")
	case categoryDesignRef:
		return strings.Contains(value, "// Design:")
	case categoryTestFirst:
		return strings.HasSuffix(value, "_test.go")
	case categoryRenderedRule:
		return strings.Contains(value, "/points/")
	case categoryRFCLanguage:
		return rfcLanguagePattern.MatchString(value)
	case categoryValidateSpec:
		return strings.Contains(value, "## Risks & Assumptions") &&
			strings.Contains(value, "## Critical Review Checklist")
	case categoryCommitGate:
		return !deferralPattern.MatchString(value)
	case categorySessionID:
		return safeSessionID(value)
	case categoryRFCTestGuard:
		return !strings.Contains(value, "RFC ")
	case categoryWeakenedHatch:
		return strings.Contains(value, "x_test.go::TestX")
	case categoryRFCChangedLedger:
		return strings.HasSuffix(value, "::TestRFC")
	case categoryDraftIncubator:
		return !strings.Contains(value, "/draft/")
	case categoryGovernedDocEdit:
		return !governedWrite(value)
	case categoryMarkSourceRead:
		return sourceKind(value) != ""
	case categoryDesignGate:
		left, right, ok := strings.Cut(value, ":")
		return ok && left == right
	case categoryDelegation:
		return value == "spawned"
	case categorySessionState:
		return strings.Contains(value, "handoff")
	case categorySessionStateLocation:
		return strings.Contains(value, "/state/session-state-") &&
			!strings.Contains(value, "/shared/")
	case categorySubagentContext:
		return strings.Contains(value, "batch") &&
			strings.Contains(value, "session-state-")
	case categoryDelegationReminder:
		return strings.Contains(value, "needs no permission") &&
			strings.Contains(value, planningRule)
	case categoryPhaseGates:
		return strings.HasPrefix(value, "/ze-")
	case categoryRawJobAdmission:
		return strings.HasPrefix(value, "make ") || !rawJob(value)
	case categoryJournalRowShape:
		return journalCells(value) == 5
	case categoryScriptWeakeningArms:
		return !strings.Contains(value, "@pytest.mark.xfail") &&
			!strings.Contains(value, "@unittest.expectedFailure")
	case categoryCISleepMarker:
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
	case strings.HasSuffix(path, goSuffix):
		return "go"
	case strings.HasSuffix(path, ".sh"):
		return kindShell
	case strings.HasSuffix(path, ".yang"):
		return kindYang
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

// The fixture catalog's own vocabulary: the message match kinds, the behavioral
// category names, the producer files a category is bound to, and the message
// fragments and generator labels that more than two rows repeat.
const (
	matchContains                = "contains"
	matchNotContains             = "not-contains"
	matchEquals                  = "equals"
	matchSuffix                  = "suffix"
	categoryFormatAlloc          = "format-alloc"
	categoryDesignRef            = "design-ref"
	categoryTestFirst            = "test-first"
	categoryRenderedRule         = "rendered-rule"
	categoryRFCLanguage          = "rfc-language"
	categoryValidateSpec         = "validate-spec"
	categoryCommitGate           = "commit-gate"
	categorySessionID            = "session-id"
	categoryWeakenedHatch        = "weakened-hatch"
	categoryRFCChangedLedger     = "rfc-changed-ledger"
	categoryDraftIncubator       = "draft-incubator"
	categoryGovernedDocEdit      = "governed-doc-edit"
	categoryDelegation           = "delegation"
	categoryDelegationReminder   = "delegation-reminder"
	categoryPhaseGates           = "phase-gates"
	categoryDesignGate           = "design-gate"
	categoryRFCTestGuard         = "rfc-test-guard"
	categoryJournalRowShape      = "journal-row-shape"
	categoryRawJobAdmission      = "raw-job-admission"
	categoryScriptWeakeningArms  = "script-weakening-arms"
	categoryCISleepMarker        = "ci-sleep-marker"
	categorySessionStateLocation = "session-state-location"
	categorySessionState         = "session-state"
	categoryMarkSourceRead       = "mark-source-read"
	categorySubagentContext      = "subagent-context"
	hookLifecycleFile            = "internal/le/hookruntime/lifecycle.go"
	weakenedActionsFile          = "internal/le/testweakened/actions.go"
	weakenedProposedFile         = "internal/le/testweakened/proposed.go"
	delegationHeading            = "Delegation:"
	planningRule                 = "planning.md"
	goSuffix                     = ".go"
	exploreSkill                 = "/ze-explore"
	governedPrefix               = "governed-"
	kindMakefile                 = "makefile"
	kindShell                    = "shell"
	kindYang                     = "yang"
	labelDot                     = "dot"
	entryPointPlaceholder        = "Entry Point contains placeholder"
	writeWeakeningAnchor         = "func writeWeakening("
	removingExpectations         = "removing expectations"
	statesNoRFCLevel             = "states no RFC 2119 level"
	wouldYouLikeMe               = "would you like me to"
)

// The remaining producer files and repeated fixture strings.
const (
	labelDotDot             = "dot-dot"
	ribHoldsRow             = "| TestRibHolds |"
	hookWriteEditFile       = "internal/le/hookruntime/writeedit.go"
	hookBashFile            = "internal/le/hookruntime/bash.go"
	weakenedProposedFixture = "test-weakened proposed"
)

// proposedVerbAnchor is the registration line that binds the weakened proposed
// action to its command surface.
const proposedVerbAnchor = `Verb:   "proposed"`

// labelTrailingNewline is the generator label for a trailing-newline input.
const (
	labelTrailingNewline = "trailing-newline"
)
