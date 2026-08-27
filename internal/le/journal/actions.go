// Design: docs/architecture/core-design.md -- the journal area, as one command
//
// actions.go keeps the producer registry row as data. The gate name, writes
// flag, purpose, help, listing, dispatch, and parity claim all read this table.
package journal

import "github.com/ze-software/ze/internal/le/leaction"

const journalWhy = "every problem class in plan/journal/ with 2+ occurrences, its row count and the span between first and last date. Prints nothing when every class has one row"

var actions = leaction.New(area,
	leaction.Action{
		Gate:   "ze-journal-report",
		Why:    journalWhy,
		Writes: false,
		Answer: reportHere,
	},
)

// Actions returns the complete command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the action hint that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le journal` command. `le journal report` runs the gate.
func Answer(args []string) (any, int) { return actions.Answer(args) }
