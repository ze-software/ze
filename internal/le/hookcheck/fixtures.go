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
	"github.com/ze-software/ze/internal/le/hookruntime"
)

var (
	deferralPattern = regexp.MustCompile(`(?i)\b(?:defer|deferred|later)\b`)
)

const (
	fixtureSitesExpected       = 457
	fixtureChecksExpected      = 616
	fixtureUniqueNamesExpected = 615
	fixtureCategoriesExpected  = 26
)

var (
	fixtureSiteDigest = [sha256.Size]byte{
		0x15, 0x1b, 0xb0, 0xdb, 0x86, 0xc5, 0xa5, 0x24,
		0x53, 0x32, 0x1f, 0x35, 0x59, 0x93, 0xc3, 0xcf,
		0x8d, 0x49, 0x64, 0xf3, 0x98, 0xda, 0xfa, 0xb4,
		0x24, 0x05, 0x5c, 0xa8, 0xf7, 0xb5, 0x7a, 0xfb,
	}
	fixtureCatalogDigest = [sha256.Size]byte{
		0xf7, 0x6d, 0x2a, 0xb4, 0xfc, 0x49, 0xea, 0x51,
		0x62, 0x37, 0x32, 0xa3, 0x9c, 0x3d, 0xc3, 0xdc,
		0x80, 0x46, 0x4a, 0x88, 0x59, 0xee, 0x06, 0x16,
		0xd9, 0x99, 0x78, 0x11, 0x87, 0xb1, 0x14, 0x2a,
	}
	fixtureCategoryDigest = [sha256.Size]byte{
		0xc7, 0x61, 0xa4, 0xcc, 0x07, 0x6d, 0x71, 0xb5,
		0xdb, 0x10, 0x30, 0xaf, 0xf2, 0x37, 0x71, 0x39,
		0xf6, 0xb1, 0x05, 0xc1, 0x2d, 0xb1, 0x5f, 0xb3,
		0x7f, 0x34, 0xaf, 0x31, 0xd1, 0xed, 0xa8, 0xc7,
	}
	fixtureBoundaryDigest = [sha256.Size]byte{
		0x5f, 0xe8, 0x52, 0x9b, 0xf0, 0x53, 0x49, 0xb9,
		0xf7, 0x65, 0x27, 0xdd, 0xd0, 0xd8, 0xa9, 0x6d,
		0x4b, 0xf3, 0x3e, 0x0b, 0x93, 0xbf, 0x9a, 0x83,
		0xe0, 0x5b, 0x55, 0x96, 0xd8, 0xf4, 0xf5, 0xfd,
	}
	hookSourcesDigest = [sha256.Size]byte{
		0xf1, 0x17, 0xb9, 0x78, 0xf7, 0xf1, 0x68, 0x2d,
		0x64, 0xd9, 0x86, 0x89, 0x12, 0x13, 0x8c, 0x94,
		0xf2, 0x6d, 0xb3, 0x57, 0x92, 0xfe, 0x7e, 0x43,
		0x94, 0x7b, 0x48, 0x8f, 0x31, 0x97, 0x45, 0x13,
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
	{categoryDesignRef, "run_design_ref", "internal/le/consistency/consistency.go", "func (c *checker) checkDesignRefs(", "// Design: docs/x.md\npackage x", "package x"},
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
	{categoryDesignGate, "run_design_gate", hookWriteEditFile, "func writeDesignEvidence(", "source-read", "no-source-read"},
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
	{categoryYangDescription, "run_yang_description", hookWriteEditFile, "func writeYangDescription(", yangProbeAllow, yangProbeRefuse},
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

// yangProbeAllow and yangProbeRefuse are the two YANG bodies the description
// gate must separate: a summary inside the bounds, and one past the 96-character
// cap writeYangDescription enforces.
const (
	yangProbeAllow  = "leaf a {\n  description \"A short summary.\";\n}"
	yangProbeRefuse = "leaf a {\n  description \"" +
		"A summary written far past the ninety-six character cap so the gate has something it must refuse here.\";\n}"
)

// categoryProbe binds one fixture category to the check that PRODUCES its
// verdict. check is the Go function name registered in hookruntime's
// nativeHookActions, resolved there rather than listed twice. tool is the
// Claude tool the payload claims, file the path it carries, and slot the
// payload field the category's allow/refuse value fills. wrap is a format
// string with one %s when the producer needs the value inside a larger
// document; an empty wrap passes the value through.
type categoryProbe struct {
	check string
	tool  string
	file  string
	slot  string
	wrap  string
}

const (
	slotContent = "content"
	slotPath    = "path"
	slotCommand = "command"
)

// categoryProbes is the set of categories whose verdict asks hookruntime
// instead of restating it. A category is here or on the ungroundedCategories
// baseline in fixture_grounding_test.go, and that test refuses a third state.
//
// Absent categories are the ones no synthetic payload can decide. Three shapes
// account for every one of them: a check that reads session markers or state
// files from the running checkout (design-gate, test-first, delegation,
// session-state), a check that reads the real file from disk rather than the
// payload content (journal-row-shape, through journal.ValidateFile), and a
// category whose producer is not a hook at all (design-ref, commit-gate).
//
//nolint:gochecknoglobals // fixture data, read by the category probes
var categoryProbes = map[string]categoryProbe{
	categoryFormatAlloc:     {check: "writeGoPatterns", tool: "Write", file: "internal/probe/probe.go", slot: slotContent},
	categoryRenderedRule:    {check: "writeRenderedRule", tool: "Write", slot: slotPath},
	categoryRFCLanguage:     {check: "writePointLanguage", tool: "Write", file: "ai/rules/points/probe/probe/probe.md", slot: slotContent, wrap: "---\nkind: directive\nstage:\n---\n%s\n"},
	categoryCISleepMarker:   {check: "writeCISleep", tool: "Write", file: "./test/probe/probe.ci", slot: slotContent},
	categoryGovernedDocEdit: {check: "bashGovernedWrite", tool: "Bash", slot: slotCommand},
	categoryRawJobAdmission: {check: "bashRawHeavy", tool: "Bash", slot: slotCommand},
	categoryYangDescription: {check: "writeYangDescription", tool: "Write", file: "internal/plugins/probe/yang/probe.yang", slot: slotContent},
}

// probeVerdict asks the producing check whether it allows this value. It
// reports false when the check refuses, and false when no check of that name is
// registered, so a renamed producer fails the selftest rather than passing it
// silently. TestEveryProbeNamesARegisteredCheck reports the name itself.
func probeVerdict(probe categoryProbe, value string) bool {
	if probe.wrap != "" {
		value = fmt.Sprintf(probe.wrap, value)
	}
	input := map[string]any{}
	switch probe.slot {
	case slotContent:
		input["file_path"] = probe.file
		input["content"] = value
	case slotPath:
		input["file_path"] = value
		input["content"] = "probe"
	case slotCommand:
		input["command"] = value
	}
	code, _, found := hookruntime.Probe(probe.check, hookruntime.Payload{ToolName: probe.tool, ToolInput: input})
	return found && code == 0
}

func categoryVerdict(category, value string) bool {
	if probe, bound := categoryProbes[category]; bound {
		return probeVerdict(probe, value)
	}
	switch category {
	case categoryDesignRef:
		return strings.Contains(value, "// Design:")
	case categoryTestFirst:
		return strings.HasSuffix(value, "_test.go")
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
	case categoryMarkSourceRead:
		return sourceKind(value) != ""
	case categoryDesignGate:
		return value == "source-read"
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
	case categoryJournalRowShape:
		return journalCells(value) == 5
	case categoryScriptWeakeningArms:
		return !strings.Contains(value, "@pytest.mark.xfail") &&
			!strings.Contains(value, "@unittest.expectedFailure")
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
	categoryYangDescription      = "yang-description"
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
