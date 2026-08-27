// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// actions.go is the Python area, ported. `le rfc --extraction-status` selected
// one mode out of an argv; `le rfc extraction-status` selects one action out of
// the table below. The three fields the Gate carried travel with it: the Make
// target it still is, the reason the listing printed, and whether it WRITES.
//
// Four of the five `ze-rfc-*` gates are here: extraction-status, check,
// reseal, and index-update. Selftest joins this area with its own fixture
// suite. The verb is derived by removing the area's prefix from the gate name,
// so one gate-name family is one area whatever stands behind it.
package rfc

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-rfc-extraction-status",
		Why: "the machine-readable extraction counts the umbrella's drain quota consumes: " +
			"signed and enrolled counts, the per-register split, and the unsigned backlog",
		Answer: extractionStatusAnswer,
	},
	leaction.Action{
		Gate:   "ze-rfc-check",
		Why:    "verify RFC requirement coverage, evidence strength, public status, audit verdicts, extraction sign-off, and generated ledger freshness without writing",
		Answer: checkAnswer,
	},
	leaction.Action{
		Gate: "ze-rfc-selftest",
		Why: "exercise every RFC engine concern against in-process fixtures and report one " +
			"structured row per property",
		Answer: selftestAnswer,
	},
	leaction.Action{
		Gate: "ze-rfc-reseal",
		Why: "rewrite the file-level fingerprints of the audit verdicts a mechanical edit " +
			"staled: the tagged unit is byte-identical and only the file around it moved, " +
			"so nothing was re-judged and no human should be asked to re-read. A verdict " +
			"whose unit, cited producer code, or requirement text MOVED is refused and " +
			"stays stale: that one needs /ze-rfc-audit <rfc>, then ze-rfc-index-update",
		Writes: true,
		Answer: resealAnswer,
	},
	leaction.Action{
		Gate: "ze-rfc-index-update",
		Why: "regenerate ai/RFC-REQUIREMENTS.md and one requirement table per RFC under " +
			"rfc/requirements/, from the summaries and the `RFC requirement:` tags the " +
			"tests themselves carry. It DELETES a table the render no longer produces, " +
			"so it refuses outright when a summary did not parse: that RFC's rows would " +
			"be absent from the render and its file removed as an orphan",
		Writes: true,
		Answer: indexUpdateAnswer,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le rfc` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// extractionStatusAnswer derives the envelope over this checkout.
//
// It answers 2 for anything that stopped it, which is the code the script
// answers: a malformed summary, an unreadable enrolled list, a malformed
// artifact and a tag in a carrier nothing runs are all "the gate could not
// run", and "clean" must never mean "I compared nothing".
func extractionStatusAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	collected, err := Collect(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	status, err := ExtractionStatus(NewDeriver(tree), collected.Requirements, collected.Enrolled)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return status, 0
}

// checkAnswer runs the read-only RFC requirement gate over this checkout.
func checkAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		return CheckReport{CannotRun: err.Error()}, 2
	}
	return Check(tree)
}

// resealAnswer re-stamps the shifted verdicts of this checkout.
//
// It answers 0 whether or not anything was re-stamped, and whether or not
// anything was refused, which is what the script answers. A refusal is a
// verdict a human must re-read, not a broken tree: reporting it as a failure
// would make the ledger generator that follows look like the remedy. Only a
// tree the writer could not READ answers 2.
func resealAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Reseal(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// indexUpdateAnswer regenerates the ledger and its per-RFC tables.
//
// It answers 2 for everything that stopped it, and the two REFUSALS are among
// them: a summary that did not parse and a render with no rows are both states
// where the prune would delete a tracked file the generator still owns, so
// neither may report success. Nothing has been written when either fires.
func indexUpdateAnswer() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := IndexUpdate(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
