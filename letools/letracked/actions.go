// Design: docs/architecture/testing/tracked-build-gate.md -- the tracked-import area as one command
//
// actions.go contains the port of the Python area. The area has ONE action.
// Therefore, the command uses a root handler instead of an leaction table.
// `le tracked` judges the commit and does not take a verb.
//
// THE REVISION COMES FROM THE ENVIRONMENT. It is not a value typed after the
// command. The tree is the checkout. The rendering is a pipe operator. Thus, no
// le command takes a value of its own (ai/rules/cli.md). The key belongs to le.
//
// It is not the bare REV that letools/trackedbuild already owns. By default, the
// two gates judge the same commit. A caller that names one for the build gate
// has not requested it here.

package letracked

import (
	"context"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
	"github.com/ze-software/ze/letools/leroot"
)

// area is the name this command is typed as.
const area = "tracked"

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

// Answer implements the `le tracked` command.
//
// The three codes have separate meanings, as they did in the script. Code 0
// means le works for the commit. Code 1 means it does not work. Code 2 means the
// run failed to judge the commit.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(area, args[0])
	}

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
