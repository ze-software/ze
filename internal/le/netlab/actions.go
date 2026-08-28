// actions.go exposes the native netlab render check and golden rewrite through leaction.
package netlab

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "netlab"

type actionRunner func() (any, int)

func actionTable(runCheck, runUpdate actionRunner) leaction.Area {
	return leaction.New(area,
		leaction.Action{Verb: "render-check", Why: "render contrib/netlab with the installed netlab, compare every node with its golden file," +
			" and prove each golden is accepted by ze config validate",
			Answer: runCheck},
		leaction.Action{
			Verb:   "render-update",
			Why:    "rewrite contrib/netlab/golden from a real netlab render, then validate every rewritten file",
			Writes: true,
			Answer: runUpdate,
		},
	)
}

var actions = actionTable(runCheckHere, runUpdateHere)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line action hint shown by command help.
func Subs() string { return actions.Subs() }

// Answer is the le netlab command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) { return runHere(false) }

func runUpdateHere() (any, int) { return runHere(true) }

func runHere(update bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, code := newChecker(root).Run(update)
	if diagnostics := report.errorText(); diagnostics != "" {
		fmt.Fprint(os.Stderr, diagnostics) //nolint:errcheck // CLI diagnostic output
	}
	return report, code
}
