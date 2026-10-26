// Design: docs/contributing/running-commands.md -- launcher builds, `./le rollback`
// Related: register.go -- the area's registration
//
// Package rollback publishes `le rollback` in le's command tree, so help,
// completion, `le help ai` and the menu name it. The move itself belongs to
// the `le` launcher script in the checkout: only it knows every build under
// bin/builds/, which build it would run, and how to set one aside. A build of
// le reaches this handler only when it runs without that selection: invoked
// by its path (bin/builds/<name>/le), or as a `./le --name` build, which the
// launcher execs before it reads the rollback word. There it refuses.
package rollback

import (
	"io"
	"os"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// area is the le command this package registers.
const area = "rollback"

// codeUsage is the exit code of a line le cannot act on where it runs, the
// code a parse refusal answers, so a caller reads it apart from work that ran
// and failed.
const codeUsage = 2

var actions = leaction.New(area,
	// The area word is the whole command, as the launcher reads it: the one
	// action is verbless and performs the vocabulary verb rollback
	// (D-P4-areaword, D-P4-launcher).
	leaction.Action{
		Does:    "rollback",
		Summary: "set the newest le build aside",
		Why:     "set the newest le build aside so the previous one runs; use when the newest misbehaves",
		Writes:  true,
		Answer:  answerNewest,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le rollback` command. The table's one action is verbless, so
// the bare line runs it and a stray value is refused by the table.
func Answer(args []string) (any, int) {
	return actions.Answer(args)
}

// answerNewest refuses: this build was started without the launcher, so it
// cannot see the other builds and has none to fall back to.
func answerNewest() (any, int) {
	return nil, refuse(os.Stderr)
}

// refusal says what is wrong, why, and the exact command that does the work.
const refusal = "error: le rollback sets the newest build in bin/builds/ aside, which only the ./le launcher" +
	" does, and this le did not come from its build selection: it was run by its path, or as a --name build.\n" +
	"fix: run it from the checkout with no --name: ./le rollback\n"

// refuse writes the refusal to stderr and answers codeUsage.
func refuse(stderr io.Writer) int {
	io.WriteString(stderr, refusal) //nolint:errcheck // CLI output
	return codeUsage
}
