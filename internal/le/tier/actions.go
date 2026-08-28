// Design: ai/rules/architecture.md -- the tier area, as one command
//
// Overview: tier.go -- the import audit the two gate actions read
//
// actions.go ports the Python area. internal/le/leaction owns dispatch, listing,
// help, and refusals. This file keeps the TABLE and the four checkout actions.
//
// Check, report, and write-baseline are explicit native actions. `| json`
// renders the payload's complete sets, while filters replace the retired
// candidates-only rendering. Optional AREA arguments are absent because no
// native action, hook, or documented procedure uses them.
package tier

import (
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "tier"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "module-tier placement (ai/rules/architecture.md): a config-driven engine lives in internal/component/ when a feature depends on it, and in internal/plugins/ otherwise",
		Answer: runCheck},
	leaction.Action{Verb: "selftest", Why: "the tier gate's own isolated fixtures -- engine placement, and the wired-versus-core classification -- before it judges the live tree",
		Answer: runSelftest},
	leaction.Action{
		// This reverse-dependency report is an explicit native action for
		// package moves.
		Verb:   "report",
		Why:    "who imports each subsystem from outside its own subtree, so a reader can see which packages could move and which the daemon wires",
		Answer: runReport,
	},
	leaction.Action{
		// The script's --write-baseline. ai/rules/architecture.md names it as
		// the way to regenerate the baseline after a move.
		Verb:   "write-baseline",
		Why:    "regenerate the migration baseline from the engines currently in the wrong tier, which is what you run after moving one",
		Writes: true,
		Answer: runWriteBaseline,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le tier` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le tier check` action.
//
// The failure page goes to stderr, and verdicts go to stdout. The script keeps
// this split so callers can compare each stream.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := Check(tree)
	if err != nil {
		// Code 2 means that the gate failed to read its tree. The script uses the
		// same code for its own failures.
		leaction.ReportError(err)
		return nil, 2
	}
	if diagnosis := report.Diagnosis(); diagnosis != "" {
		os.Stderr.WriteString(diagnosis) //nolint:errcheck // CLI output
	}
	return report, report.Failed
}

// runReport is the `le tier report` action, over the three default registries.
func runReport() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := Report(tree, DefaultAreas[:])
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// runWriteBaseline is the `le tier write-baseline` action.
func runWriteBaseline() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	module, err := modulePath(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	edges, err := collectEdges(tree, module)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	misplaced, err := engineMisplacements(tree, module, edges)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if err := writeBaseline(tree, misplaced); err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return baselineReport{File: Baseline, Engines: len(misplaced)}, 0
}

// Report answers the reverse-dependency audit for the named areas.
func Report(tree string, areas []string) (AuditReport, error) {
	module, err := modulePath(tree)
	if err != nil {
		return AuditReport{}, err
	}
	edges, err := collectEdges(tree, module)
	if err != nil {
		return AuditReport{}, err
	}
	engines, err := engineSubsystems(tree)
	if err != nil {
		return AuditReport{}, err
	}

	report := AuditReport{Module: module, Areas: make(map[string][]Row, len(areas)), Order: areas}
	for _, name := range areas {
		rows, classifyErr := Classify(name, tree, module, edges, engines)
		if classifyErr != nil {
			return AuditReport{}, classifyErr
		}
		report.Areas[name] = rows
	}
	return report, nil
}
