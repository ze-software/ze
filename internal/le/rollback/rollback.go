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

// newestVerb is the one action: set the newest build aside. A bare
// `le rollback` means it, so the bare line and the explicit one are one action.
const newestVerb = "newest"

// codeUsage is the exit code of a line le cannot act on where it runs, the
// code a parse refusal answers, so a caller reads it apart from work that ran
// and failed.
const codeUsage = 2

var actions = leaction.New(area,
	leaction.Action{
		Verb:   newestVerb,
		Why:    "set the newest le build aside so the previous one runs; use when the newest misbehaves",
		Writes: true,
		Answer: answerNewest,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le rollback` command. A help word or a declared verb travels
// as typed; any other line, the bare one included, is the newest action, so
// one parser reads it and a stray value is refused by the table.
func Answer(args []string) (any, int) {
	if len(args) > 0 && (actions.Holds(args[0]) || leaction.IsHelpArg(args[0])) {
		return actions.Answer(args)
	}

	line := make([]string, 0, len(args)+1)
	line = append(line, newestVerb)
	line = append(line, args...)
	return actions.Answer(line)
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
