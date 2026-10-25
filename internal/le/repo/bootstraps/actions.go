// Design: docs/architecture/testing/tracked-build-gate.md -- the tracked-import area as one command
//
// actions.go contains the port of the Python area. The area has ONE action.
// Therefore, the command uses a root handler instead of an leaction table.
// `le repo bootstraps` judges the commit and does not take a verb.
//
// THE REVISION COMES FROM THE ENVIRONMENT. It is not a value typed after the
// command. The tree is the checkout. The rendering is a pipe operator. Thus, no
// le command takes a value of its own (ai/rules/cli.md). The key belongs to le.
//
// It is not the bare REV that internal/le/repo/compiles already owns. By default, the
// two gates judge the same commit. A caller that names one for the build gate
// has not requested it here.

package repobootstraps

import (
	"context"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "repo bootstraps"

// RevKey names the commit this gate judges.
const RevKey = "ze.le.tracked.rev"

var revEntry = env.MustRegister(env.EnvEntry{
	Key:         RevKey,
	Type:        "string",
	Default:     "HEAD",
	Description: "the commit le is built and run from; a past sha reproduces a break that has already landed",
	// Private keeps the key out of `ze env list`. It names a build-host commit
	// and an operator has nothing to do with it.
	Private: true,
})

// Answer implements the `le repo bootstraps` command.
//
// The three codes have separate meanings, as they did in the script. Code 0
// means le works for the commit. Code 1 means it does not work. Code 2 means the
// run failed to judge the commit.
//
// A bare line runs the gate, as hooks and verify stages have always typed it
// (D-P3-bare in plan/post-release/engineering/spec-le-cli-usability.md), so the
// verb is supplied here and the table reads the rest of the line. A word the
// area declares travels as it stands, so `le repo bootstraps check` is the same
// gate.
func Answer(args []string) (any, int) {
	if len(args) > 0 && actions.Holds(args[0]) {
		return actions.Answer(args)
	}
	return actions.Answer(append([]string{checkVerb}, args...))
}

// checkVerb is the one action: the gate itself.
const checkVerb = "check"

// actions is the whole command surface. The gate takes no arguments: the
// commit it judges comes from ze.le.tracked.rev.
var actions = leaction.New(area, leaction.Action{
	Verb:    checkVerb,
	Summary: "check le builds from what git holds",
	Why:     "does le still work when built from what git holds, rather than from the working tree",
	Answer:  answerCheck,
})

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// answerCheck builds le from the judged commit and runs it.
func answerCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	rev := env.Get(revEntry.Key)
	if rev == "" {
		rev = revEntry.Default
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDeadline)
	defer cancel()

	verdict, code, err := Run(ctx, root, rev)
	if err != nil {
		leaction.ReportError(err)
		return nil, code
	}
	return verdict, code
}
