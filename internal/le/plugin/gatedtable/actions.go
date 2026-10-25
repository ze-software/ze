// Design: docs/architecture/plugin/feature-gates.md -- the gated-table area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/le/action, which every ported area shares.

package gatedtable

import (
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "plugin gated-table"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Summary: "check the gated-plugin table is current", Why: "the generated table of gated plugins agrees with feature-gates.txt and every" +
		" registration, so a build without a plugin names the tag that would carry it. A stale" +
		" table names a wrong tag, or none, for a plugin a stripped build refuses",
		Answer: runCheckHere},
	leaction.Action{
		Verb:    "update",
		Summary: "regenerate the gated-plugin table",
		Why:     "regenerate the gated-plugin table from feature-gates.txt and every registration",
		Writes:  true,
		Answer:  runWriteHere,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin gated-table` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runCheck(root)
}

func runWriteHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runWrite(root)
}

// runCheck answers the check over one tree. A stale or missing table is a
// verdict and answers 1 with the report; a tree the run could not read is an
// error and answers 1 with none.
func runCheck(root string) (CheckReport, int) {
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return CheckReport{}, 1
	}
	if report.Reason != "" {
		return report, 1
	}
	return report, 0
}

// runWrite answers the write over one tree.
func runWrite(root string) (WriteReport, int) {
	report, err := Write(root)
	if err != nil {
		leaction.ReportError(err)
		return WriteReport{}, 1
	}
	return report, 0
}
