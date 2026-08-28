// Design: docs/architecture/core-design.md -- the yang-glue area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// runCheck and runWrite are where the tree is found and a failure becomes an
// exit code. Check and Write take the tree as an argument instead, so a test
// names a fixture by calling the function (internal/le/vendorweb, the same split).

package yangglue

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "yang-glue"

var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "the generated yang/*/register.go and embed.go agree with the .yang tree." +
		" A stale one leaves a module the loader never registers, so a config leaf" +
		" the tree declares is refused as unknown",
		Answer: runCheckHere},
	leaction.Action{
		Verb:   "write",
		Why:    "regenerate the embed and register glue beside every .yang file",
		Writes: true,
		Answer: runWriteHere,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le yang-glue` command.
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

// runCheck answers the check over one tree. A stale file is a verdict rather
// than an error, so it answers 1 with the report; a tree that could not be read
// is an error and answers 1 with none.
func runCheck(root string) (CheckReport, int) {
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return CheckReport{}, 1
	}
	if len(report.Stale) > 0 {
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
