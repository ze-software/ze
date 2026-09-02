// Design: docs/architecture/core-design.md -- native verifier work is callable through le
// Detail: verifylint.go -- the full lint plan and direct process execution
package verifylint

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// jobLabel is the name this stage claims in the shared job registry. Every lint
// on this machine claims it, so two of them queue instead of oversubscribing
// the box, and two asking for the SAME work share one run.
const jobLabel = "lint"

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

// runHere admits this lint through the shared job registry and then runs it.
//
// Several sessions work one checkout, and each lint uses cores allocated for
// the whole machine. Admission decides how many run at once. It also answers a
// second session asking for the SAME work over the SAME tree with the running
// job's output and verdict, so one run serves both rather than the tree being
// linted twice.
func runHere(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}
	admission, err := job.NewIn(root)
	if err != nil {
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}

	// A run we attach to replays its output through Out, and its findings name
	// the same files ours would have. Reading them back out of that replay is
	// what lets the shared verdict still say WHICH files it was about: a red
	// that names none is charged to every commit in the checkout.
	shared := newPathCollector()
	admission.Out = io.MultiWriter(os.Stdout, shared)

	ticket, err := admission.Admit(jobLabel, jobArgv(arguments))
	if err != nil {
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}
	if ticket.Kind == job.KindAttached {
		return Report{Code: ticket.Code, FailingPaths: shared.paths()}, ticket.Code
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runner, err := NewRunner(ctx, root)
	if err != nil {
		ticket.Release(cannotPlan)
		leaction.ReportError(err)
		return Report{Code: cannotPlan, Error: err.Error()}, cannotPlan
	}

	// The slot's log is opened before the run and closed after it, because
	// Release removes the file: a writer still holding it would keep the bytes
	// alive in a path nothing can read.
	if ticket.Log != "" {
		logFile, openErr := os.OpenFile(filepath.Join(root, filepath.FromSlash(ticket.Log)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // this job's own registry log, named from a validated label
		if openErr != nil {
			leaction.ReportError(openErr)
		} else {
			runner.Progress(logFile)
			defer func() {
				if closeErr := logFile.Close(); closeErr != nil {
					leaction.ReportError(closeErr)
				}
			}()
		}
	}

	payload, code := runRunner(runner, arguments)
	ticket.Release(code)
	return payload, code
}

// jobArgv is what the registry fingerprints as this job's work. A full run and
// a scoped run do different work, so they must not share one verdict.
func jobArgv(arguments leaction.Arguments) []string {
	argv := []string{"le", "verify", "lint", actionRun}
	if scope, scoped := arguments["scope"]; scoped {
		argv = append(argv, strings.Fields(scope)...)
	}
	return argv
}

func runRunner(runner *Runner, arguments leaction.Arguments) (any, int) {
	if scope, scoped := arguments["scope"]; scoped {
		return runner.runScope(strings.Fields(scope))
	}
	return runner.Run()
}
