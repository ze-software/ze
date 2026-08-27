// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// Each category keeps three things together: its producer runner, the owning
// hook rule, and a native discriminator. The producer census prevents a fixture
// category or assertion from disappearing while a positive/negative probe
// prevents a present-but-constant Go port from passing.
package hookcheck

import (
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

const fixtureChecksExpected = 456

type fixtureCategory struct {
	name     string
	runner   string
	owner    string
	evidence string
	allow    string
	refuse   string
}

var fixtureCategories = [...]fixtureCategory{
	{"format-alloc", "run_format_alloc", ".claude/hooks/pretool-writeedit.py", "def c_format_alloc(", "return fmt.Errorf(\"x\")", "return fmt.\u0053printf(\"%d\", n)"},
	{"design-ref", "run_design_ref", ".claude/hooks/pretool-writeedit.py", "def c_require_design_ref(", "// Design: docs/x.md\npackage x", "package x"},
	{"test-first", "run_test_first", ".claude/hooks/pretool-writeedit.py", "def c_require_test_first(", "foo_test.go", "foo.go"},
	{"rendered-rule", "run_rendered_rule", ".claude/hooks/pretool-writeedit.py", "def c_rendered_rules(", "ai/rules/points/commands/x.md", "ai/rules/commands.md"},
	{"rfc-language", "run_rfc_language", ".claude/hooks/pretool-writeedit.py", "def c_rule_point_rfc_language(", "The caller MUST stop.", "The caller stops."},
	{"validate-spec", "run_validate_spec", ".claude/hooks/validate-spec.sh", "Risks & Assumptions", "# Spec: x\n## Risks & Assumptions\n## Critical Review Checklist", "# Spec: x"},
	{"commit-gate", "run_commit_gate", "scripts/dev/commit_helper.py", "def deferral_in_diff_problems(", "implemented now", "deferred to later"},
	{"session-id", "run_session_id", ".claude/hooks/lib/session_id.py", "def session_id(", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "../shared"},
	{"rfc-test-guard", "run_rfc_test_guard", ".claude/hooks/pretool-writeedit.py", "def _rfc_tagged_change_err(", "ordinary test edit", "RFC 4271: MUST accept"},
	{"weakened-hatch", "run_weakened_hatch", ".claude/hooks/pretool-writeedit.py", "def _weakened_hatch(", "pkg/x_test.go::TestX", "pkg/y_test.go::TestY"},
	{"rfc-changed-ledger", "run_rfc_changed_ledger", ".claude/hooks/pretool-writeedit.py", "def _rfc_changed_hatch(", "pkg/x_test.go::TestRFC", "pkg/x_test.go::TestOther"},
	{"draft-incubator", "run_draft_incubator", ".claude/hooks/pretool-writeedit.py", "def _is_draft(", "test/unit/x_test.go", "test/draft/x_test.go"},
	{"governed-doc-edit", "run_governed_doc_edit", ".claude/hooks/pretool-bash.py", "def check_governed_doc_edit(", "cat plan/spec-x.md", "echo x > plan/spec-x.md"},
	{"mark-source-read", "run_mark_source_read", ".claude/hooks/mark-source-read.sh", "source-read-", "internal/x.go", "docs/x.md"},
	{"design-gate", "run_design_gate", ".claude/hooks/pretool-writeedit.py", "def c_design_without_lsp(", "go:go", "go:python"},
	{"delegation", "run_delegation", ".claude/hooks/block-premature-stop.sh", "agent-spawned", "spawned", "not-spawned"},
	{"session-state", "run_session_state", ".claude/hooks/session-end-summary.sh", "handoff", "## Phase 4 handoff\nkept", "## Snapshot\nonly"},
	{"session-state-location", "run_session_state_location", ".claude/hooks/lib/state-file.sh", "session-state-", "tmp/session/2026-08-25-id/state/session-state-x-id.md", "tmp/session/shared/state/x.md"},
	{"subagent-context", "run_subagent_context", ".claude/hooks/subagent-context.sh", "context-economy", "batch reads; state/session-state-x.md", "generic context"},
	{"delegation-reminder", "run_delegation_reminder", ".claude/hooks/delegation-reminder.sh", "planning.md", "needs no permission; planning.md", "permission denied"},
	{"phase-gates", "run_phase_gates", ".claude/hooks/pretool-agent-skill.py", "ze-explore", "/ze-explore", "raw research agent"},
	{"raw-job-admission", "run_raw_job_admission", ".claude/hooks/pretool-bash.py", "def check_raw_test_invocation(", "make ze-unit-pkg-test", "go test ./..."},
	{"journal-row-shape", "run_journal_row_shape", ".claude/hooks/posttool-writeedit.py", "def c_journal_row_shape(", "| 2026-08-22 | spec-x | hooks | symptom | fix |", "| 2026-08-22 | spec-x | hooks | broken |"},
	{"python-weakening-arms", "run_python_weakening_arms", ".claude/hooks/pretool-writeedit.py", "def _is_python_test(", "self.assertEqual(1, f())", "@pytest.mark.xfail\nself.assertEqual(1, f())"},
	{"ci-sleep-marker", "run_ci_sleep_marker", ".claude/hooks/pretool-writeedit.py", "def c_ci_sleep_justification(", "# sleep(timer): tracker ticks\ntime.sleep(2)", "time.sleep(2)"},
}

func runFixtures(root string) ([]Result, int) {
	body, err := readCheckoutFile(root, fixtureProducer)
	if err != nil {
		return []Result{{Name: "behavioral-fixture-population", Code: 2, Message: err.Error()}}, 0
	}
	fixtureText := string(body)
	fixtureChecks := strings.Count(fixtureText, "results.check(")
	results := make([]Result, 0, len(fixtureCategories)+1)
	population := Result{Name: "behavioral-fixture-population", Passed: fixtureChecks == fixtureChecksExpected}
	if !population.Passed {
		population.Code = 2
		var tb textbuf.Buffer
		population.Message = tb.Str("producer has ").Int(int64(fixtureChecks)).
			Str(" check callsites, want ").Int(fixtureChecksExpected).String()
	}
	results = append(results, population)

	for _, category := range &fixtureCategories {
		result := checkFixtureCategory(root, fixtureText, category)
		results = append(results, result)
	}
	return results, fixtureChecks
}

func checkFixtureCategory(root, fixtureBody string, category fixtureCategory) Result {
	var tb textbuf.Buffer
	result := Result{Name: tb.Str("fixture-").Str(category.name).String(), Passed: true}
	tb.Reset()
	mapping := tb.Byte('\'').Str(category.name).Str("': ").Str(category.runner).String()
	if !strings.Contains(fixtureBody, mapping) {
		result.Passed = false
		result.Code = 2
		result.Message = "producer section mapping is missing"
		return result
	}
	tb.Reset()
	runnerDeclaration := tb.Str("def ").Str(category.runner).Byte('(').String()
	if !strings.Contains(fixtureBody, runnerDeclaration) {
		result.Passed = false
		result.Code = 2
		result.Message = "producer runner is missing"
		return result
	}
	owner, err := readCheckoutFile(root, category.owner)
	if err != nil {
		result.Passed = false
		result.Code = 2
		result.Message = err.Error()
		return result
	}
	if !strings.Contains(string(owner), category.evidence) {
		result.Passed = false
		result.Code = 2
		result.Message = "owning rule evidence is missing"
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
	case "python-weakening-arms":
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
	case strings.HasSuffix(path, ".py"):
		return "python"
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
