// Design: docs/architecture/core-design.md -- native verifier work is callable through le
// Detail: verifylint.go -- the full lint plan and direct process execution
package verifylint

import (
	"context"
	"os"
	"os/signal"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var actions = leaction.New(area, leaction.Action{
	Verb: actionRun,
	Why:  "lint the full tree or a declared package scope through the host, Linux integration, platform, capability, personality, and compile-out builds",
	Parameters: []leaction.Parameter{
		{Keyword: "scope", Value: "packages"},
	},
	AnswerArgs: runHere,
})

// Actions returns the gateless command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the one-line action hint for command help.
func Subs() string { return actions.Subs() }

// Answer is the `le verify lint` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runHere(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runner, err := NewRunner(ctx, root)
	if err != nil {
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}
	return runRunner(runner, arguments)
}

func runRunner(runner *Runner, arguments leaction.Arguments) (any, int) {
	if scope, scoped := arguments["scope"]; scoped {
		return runner.runScope(strings.Fields(scope))
	}
	return runner.Run()
}
