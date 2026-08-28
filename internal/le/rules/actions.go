// Design: docs/architecture/core-design.md -- the rules area, as one command
// Overview: rules.go -- the corpus every action here reads
// Detail: lint.go -- the format linter, render.go -- the render and the round trip
// Detail: coverage_report.go -- the gate map's answer
// Detail: session_coverage.go -- the transcript rule-miss detector
// Detail: index.go -- the rule index
// Detail: artifacts.go -- the two digest artifacts and the payload they cost
// Detail: router.go -- the routing measurement
//
// actions.go ports the retired rules area. `le rules lint` now selects one
// action from the table below, while retired gate metadata remains only for the
// migration census.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE, because the table is the only
// part of an area that is about the rule corpus.
//
// The table contains the eleven `ze-rules-*` gates and one hook-facing action.
// Five gates came from rules_lint.py and rules_points.py. Six came from
// rules_index.py, rules_condensed.py, and rules_router.py. Transcript coverage
// declares its native verb directly.

package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "lint", Why: "every rule carries the **When:** / **Severity:** metadata block " +
		"(ai/rules/rule-format.md), so tooling parses triggers rather than guessing them",
		Answer: lintAnswer},
	leaction.Action{Verb: "render-check", Why: "the rendered ai/rules/*.md agree with ai/rules/points/",
		Answer: func() (any, int) { return renderAnswer(true) }},
	leaction.Action{Verb: "render-update", Why: "render ai/rules/points/ into ai/rules/*.md",
		Writes: true,
		Answer: func() (any, int) { return renderAnswer(false) }},
	leaction.Action{Verb: "points-roundtrip-check", Why: "every rendered rule can be split back into points byte-identically; " +
		"a lossy split is silent instruction loss",
		Answer: roundTripAnswer},
	leaction.Action{Verb: "gate-map-report", Why: "which rule point each hook check enforces. Gated and ungated are " +
		"MEASUREMENTS and exit 0: an ungated point is a rule no machine enforces yet. " +
		"Dangling FAILS, and so do a check that named a point at HEAD and declares none " +
		"now, a rule holding fewer points than HEAD with no row in ai/rules/points/RETIRED.md, " +
		"and a rationale or excepted-by naming nothing",
		Answer: coverageAnswer},
	leaction.Action{Verb: "index-check", Why: "ai/rules/INDEX.md names every rule and its trigger, so an agent can " +
		"tell which rule covers a topic without opening all of them",
		Answer: func() (any, int) { return indexAnswer(true) }},
	leaction.Action{Verb: "index-update", Why: "regenerate ai/rules/INDEX.md from each rule's own heading and trigger",
		Writes: true,
		Answer: func() (any, int) { return indexAnswer(false) }},
	leaction.Action{Verb: "condensed-check", Why: "ai/rules/TRIGGERS.md and ai/rules/CORE.md agree with the rules they are " +
		"derived from; a stale trigger index routes a session to a rule that moved",
		Answer: func() (any, int) { return condensedAnswer(true) }},
	leaction.Action{Verb: "condensed-update", Why: "regenerate the trigger index and the always-on core from one parse of ai/rules/",
		Writes: true,
		Answer: func() (any, int) { return condensedAnswer(false) }},
	leaction.Action{Verb: "payload-report", Why: "what a session loads before it reads anything else, measured against the " +
		"token budget. A MEASUREMENT, except that a payload file which is not there " +
		"fails: an absent file is not a smaller one",
		Answer: payloadAnswer},
	leaction.Action{Verb: "router-report", Why: "which blocking rule no past task description would surface. A MEASUREMENT " +
		"and exit 0: each name it prints is a candidate for the always-on core. " +
		"An unreadable precedence ladder FAILS, because the core this subtracts is " +
		"derived from it",
		Answer: routerAnswer},
	leaction.Action{
		Verb:   "coverage-report",
		Why:    "which blocking rule matched this session's edited file types but was never read",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "quiet"},
			{Keyword: "transcript", Value: "path"},
			{Keyword: "session", Value: "id"},
			{Keyword: "rules-dir", Value: "path"},
			{Keyword: "no-append"},
		},
		AnswerArgs: coverageReportAnswer,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le rules` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// lintAnswer runs the format linter over this checkout.
//
// It answers 1 for a violation or an empty corpus, as the script does. A tree
// with no ai/rules also answers 1. The script's failure text stays unchanged.
// Nothing was read, so no report exists to render.
func lintAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Lint(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// renderAnswer renders the point tree, or compares it against the rendered
// rules and writes nothing.
func renderAnswer(check bool) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := RenderAll(tree, rulesDir(tree), pointsDir(tree), check)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// roundTripAnswer splits every rendered rule into a scratch directory and
// renders it straight back.
//
// The scratch directory is the operating system's, removed on every exit path,
// and nothing under ai/rules/ is written. That is what makes this a check
// rather than a second renderer.
func roundTripAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	out, err := os.MkdirTemp("", "ze-rules-points-")
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	defer os.RemoveAll(out) //nolint:errcheck // a scratch directory this run created

	report, err := RoundTrip(rulesDir(tree), out)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// coverageAnswer joins the hook checks against the points they enforce.
func coverageAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Coverage(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report.writeDiagnosis()
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// rulesDir and pointsDir name the two directories every action reads, so the
// join happens in one place rather than at each call site.
func rulesDir(tree string) string {
	return filepath.Join(tree, filepath.FromSlash(rulesRel))
}

func pointsDir(tree string) string {
	return filepath.Join(tree, filepath.FromSlash(pointsRel))
}

// indexAnswer regenerates ai/rules/INDEX.md, or compares it and writes nothing.
//
// It answers 1 for a stale index or a rule with no derivable summary. The script
// uses these two codes. It also answers 1 when the rules directory contains no
// rules. The script instead writes an empty header and reports success.
func indexAnswer(check bool) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Index(tree, check)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// condensedAnswer regenerates the trigger index and the always-on core, or
// compares both and writes nothing.
func condensedAnswer(check bool) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Digest(tree, check)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.EmptyCorpus {
		reportEmptyCorpus()
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// payloadAnswer measures the always-loaded payload against its budget.
//
// The script always answers 0, and an absent file contributes zero characters.
// Thus, deletion of ai/rules/CORE.md makes the budget look MET. This function
// answers 1 because an absent payload is not a smaller payload.
func payloadAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Payload(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// routerAnswer measures which rules a trigger index would surface for past
// work.
//
// It answers 0 because an operator reads the measurement. This is not a gate.
// It answers nonzero only when it cannot read the precedence ladder. The report
// derives its core from that ladder. An unreadable ladder invalidates every
// number, so the report refuses to print them.
func routerAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Router(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

// emptyCorpusWarning is the sentence rules_condensed.py prints when the
// reachability derivation read no task description. It is spelled here word for
// word, and digest_constants_python_test.go compares it to the Python by value.
const emptyCorpusWarning = "the task corpus is empty, so no blocking rule can be shown " +
	"unreachable and ai/rules/CORE.md loses that derivation -- check that " +
	"plan/spec-*.md is readable"

// reportEmptyCorpus writes that sentence.
//
// It is a WARNING, not an error. The exit code shows the difference: both
// artifacts are generated, and the run answers 0. ONE of the four core
// derivations is missing. The word "error" would describe a failure that did
// not occur.
//
// The script prints it TWICE, once in each artifact builder. This function
// prints it once. The `empty-corpus` payload fact makes it available to `| json`.
func reportEmptyCorpus() {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("warning: ").Str(emptyCorpusWarning).String()) //nolint:errcheck // CLI output
}
