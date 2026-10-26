// Design: docs/architecture/core-design.md -- the rules area, as one command
// Overview: rules.go -- the corpus every action here reads
// Detail: lint.go -- the format linter, render.go -- the render and the round trip
// Detail: coverage_report.go -- the gate map's answer
// Detail: session_coverage.go -- the transcript rule-miss detector
// Detail: index.go -- the rule index
// Detail: artifacts.go -- the two digest artifacts and the payload they cost
// Detail: router.go -- the routing measurement
//
// actions.go ports the retired rules area. `le ai rules check metadata` now
// selects one action and target from the table below, while retired gate
// metadata remains only for the migration census.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/le/action. What stays here is the TABLE, because the table is the only
// part of an area that is about the rule corpus.
//
// The table carries the eleven `ze-rules-*` gates and one hook-facing action.
// The five gates that judge an artifact against its source are the targets of
// `check`, and the three generators are the targets of `update`; the reports
// keep their own actions. Transcript coverage declares its native verb directly.

package airules

import (
	"os"
	"path/filepath"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The targets of the check and update actions. Each is the positional value of
// its action, so one verb carries every artifact the area judges or writes.
const (
	targetCondensed       = "condensed"
	targetIndex           = "index"
	targetMetadata        = "metadata"
	targetPointsRoundtrip = "points-roundtrip"
	targetRender          = "render"
	targetValue           = "target"
)

// checkTargets answers each check target's gate. None of them writes.
var checkTargets = map[string]func() (any, int){
	targetCondensed:       func() (any, int) { return condensedAnswer(true) },
	targetIndex:           func() (any, int) { return indexAnswer(true) },
	targetMetadata:        lintAnswer,
	targetPointsRoundtrip: roundTripAnswer,
	targetRender:          func() (any, int) { return renderAnswer(true) },
}

// updateTargets answers each update target's generator.
var updateTargets = map[string]func() (any, int){
	targetCondensed: func() (any, int) { return condensedAnswer(false) },
	targetIndex:     func() (any, int) { return indexAnswer(false) },
	targetRender:    func() (any, int) { return renderAnswer(false) },
}

// checkParameters is the grammar of `ai rules check <target>`.
var checkParameters = []leaction.Parameter{{
	Kind: leaction.KindEnum, Value: targetValue, Requirement: leaction.Required,
	Help:   "the rule artifact to check",
	Values: []string{targetCondensed, targetIndex, targetMetadata, targetPointsRoundtrip, targetRender},
	ValueHelp: map[string]string{
		targetCondensed:       "TRIGGERS.md and CORE.md match the rules they derive from",
		targetIndex:           "INDEX.md names every rule and its trigger",
		targetMetadata:        "every rule carries its When and Severity block",
		targetPointsRoundtrip: "every rule splits back into its points byte for byte",
		targetRender:          "the rendered rules agree with their points",
	},
}}

// updateParameters is the grammar of `ai rules update <target>`.
var updateParameters = []leaction.Parameter{{
	Kind: leaction.KindEnum, Value: targetValue, Requirement: leaction.Required,
	Help:   "the rule artifact to regenerate",
	Values: []string{targetCondensed, targetIndex, targetRender},
	ValueHelp: map[string]string{
		targetCondensed: "regenerate the trigger index and the always-on core",
		targetIndex:     "regenerate ai/rules/INDEX.md from each rule's heading",
		targetRender:    "render the rule points into ai/rules/*.md",
	},
}}

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Summary: "check one rule artifact against its source", Why: "the named rule artifact agrees with the source it derives from: " +
		"the trigger index and core (condensed; a stale trigger index routes a session to a rule that moved), " +
		"the rule index (index), the **When:** / **Severity:** block every rule carries (metadata, " +
		"ai/rules/rule-format.md, so tooling parses triggers rather than guessing them), the byte-identical " +
		"split back into points (points-roundtrip; a lossy split is silent instruction loss), or the rendered " +
		"ai/rules/*.md against ai/rules/points/ (render)",
		Parameters: checkParameters,
		AnswerArgs: func(args leaction.Arguments) (any, int) { return targetAnswer(checkTargets, args) }},
	leaction.Action{Verb: "update", Summary: "regenerate one rule artifact from its source", Why: "regenerate the named rule artifact: the trigger index and the " +
		"always-on core from one parse of ai/rules/ (condensed), ai/rules/INDEX.md from each rule's own heading " +
		"and trigger (index), or ai/rules/*.md from ai/rules/points/ (render)",
		Writes: true, Parameters: updateParameters,
		AnswerArgs: func(args leaction.Arguments) (any, int) { return targetAnswer(updateTargets, args) }},
	leaction.Action{Verb: "gate-map-report", Summary: "report which rule point each hook check enforces", Why: "which rule point each hook check enforces. Gated and ungated are " +
		"MEASUREMENTS and exit 0: an ungated point is a rule no machine enforces yet. " +
		"Dangling FAILS, and so do a check that named a point at HEAD and declares none " +
		"now while that point is still on disk, and a rationale or excepted-by naming nothing",
		Answer: coverageAnswer},
	leaction.Action{Verb: "payload-report", Summary: "measure what a session loads against the token budget", Why: "what a session loads before it reads anything else, measured against the " +
		"token budget. A MEASUREMENT, except that a payload file which is not there " +
		"fails: an absent file is not a smaller one",
		Answer: payloadAnswer},
	leaction.Action{Verb: "router-report", Summary: "report blocking rules no past task would surface", Why: "which blocking rule no past task description would surface. A MEASUREMENT " +
		"and exit 0: each name it prints is a candidate for the always-on core. " +
		"An unreadable precedence ladder FAILS, because the core this subtracts is " +
		"derived from it",
		Answer: routerAnswer},
	leaction.Action{
		Verb:    "coverage-report",
		Summary: "report blocking rules matched but never read",
		Why:     "which blocking rule matched this session's edited file types but was never read",
		Writes:  true,
		Parameters: []leaction.Parameter{
			{Keyword: "quiet", Kind: leaction.KindSwitch, Help: "print one line, only when the coverage changed"},
			{Keyword: "transcript", Kind: leaction.KindPath, Help: "the session transcript to read", Value: "path", Requirement: leaction.Optional},
			{Keyword: "session", Kind: leaction.KindLabel, Help: "the session id whose transcript is read", Value: "id", Requirement: leaction.Optional},
			{Keyword: "rules-dir", Kind: leaction.KindPath, Help: "the rules directory to judge against", Value: "path", Requirement: leaction.Optional},
			{Keyword: "no-append", Kind: leaction.KindSwitch, Help: "do not record this run in the coverage history"},
		},
		AnswerArgs: coverageReportAnswer,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le ai rules` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// targetAnswer runs the answer of the target the invocation named. The parser
// refuses a missing or undeclared target before this runs, so a miss here is a
// table that declares a value no answer serves.
func targetAnswer(targets map[string]func() (any, int), args leaction.Arguments) (any, int) {
	answer, known := targets[args.One(targetValue)]
	if !known {
		panic("BUG: airules: a declared target has no answer: " + args.One(targetValue))
	}
	return answer()
}

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
	"the declared plan category directories are readable"

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
	tb.Str("warning: ").Str(emptyCorpusWarning).Byte('\n').StdErr() //nolint:errcheck // CLI output
}
