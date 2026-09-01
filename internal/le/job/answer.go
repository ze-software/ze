// Design: docs/architecture/core-design.md -- job admission as a command
// Related: job.go -- the admission this command drives
//
// `le job run label <label> command <argv...>` restates the command line of
// internal/le/job/answer.go in the grammar that every other le command uses. This
// grammar requires a closed keyword before every value (ai/rules/cli.md). The
// shell accepted its label and command as bare positionals. The port changed
// only this aspect.

package job

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
)

// name is the root command, and the word a developer types.
const name = "job"

// The keywords this command takes. Each one types the value that follows it,
// so a label that happens to spell a keyword is still a label.
const (
	runVerb        = "run"
	labelKeyword   = "label"
	quietKeyword   = "quiet"
	commandKeyword = "command"
)

// usageLine is what a refusal points at.
const usageLine = "usage: le job run label <label> [quiet] command <argv...>"

// Answer is the command. It admits and runs one job. It reports the result and
// the job's exit code.
//
// The exit code belongs to the JOB, and this command returns it unchanged. The
// discovery-index check exits 0 for fresh, 3 for stale and 1 when the generator
// itself fails. internal/le/commit/actions.go blocks on 3 but treats 1 as a
// warning.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return actions(), 0
	}
	if args[0] != runVerb {
		return nil, refuse("no such action in job: ", args[0])
	}

	asked, ok := parseRun(args[1:])
	if !ok {
		// 2, which is what the shell half's usage answered for a missing
		// label or an empty command, and what an unknown action answers here.
		// A caller reads it apart from a job that ran and failed.
		return nil, 2
	}

	adm, err := New()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	if asked.Quiet {
		return runQuiet(adm, asked.Label, asked.Argv)
	}

	report, code := adm.Run(asked.Label, asked.Argv, "", nil)
	return report, code
}

// runArgs is one parsed invocation: what to run, under what name, and where
// its output goes.
type runArgs struct {
	Label string
	Argv  []string
	// Quiet sends the child's output to this session's scratch log rather than
	// to the terminal, and answers a summary of it.
	Quiet bool
}

// parseRun reads `label <label> [quiet] command <argv...>` and reports whether
// parsing succeeded. If parsing fails, it has already printed a refusal.
//
// Everything after the command keyword is the job's argv, regardless of its
// form. A wrapped recipe passes a make invocation with its own flags. A job
// flag is not a flag of this command, and neither is a `quiet` the child
// itself takes: this one is read before the command keyword and nowhere else.
func parseRun(args []string) (runArgs, bool) {
	if len(args) < 2 || args[0] != labelKeyword {
		return refused(labelKeyword, " names the job, and it comes first")
	}
	asked := runArgs{Label: args[1]}

	rest := args[2:]
	if len(rest) > 0 && rest[0] == quietKeyword {
		asked.Quiet = true
		rest = rest[1:]
	}

	if len(rest) < 2 || rest[0] != commandKeyword {
		return refused(commandKeyword, " names what to run, and everything after it is that command")
	}
	asked.Argv = rest[1:]
	return asked, true
}

// actions is what a bare `le job` answers: the one thing this command does,
// as data. It is the shape every le area answers to the same question
// (internal/le/leaction, List), so a reader who has seen one has seen this.
func actions() leaction.List {
	return leaction.List{
		Area: name,
		Actions: []leaction.Row{{
			Verb: runVerb,
			Why: "admit one heavy job, run it, and answer its exit code. Several sessions share this machine." +
				" `quiet` before the command keyword writes the child's output to this session's scratch log" +
				" instead of the terminal, and answers the verdict, that log's path and its failure lines",
		}},
	}
}

// refuse reports a word that this command does not accept. It returns the code
// that the ported le areas return for the same error: 2. This code distinguishes
// the error from a job that ran and failed.
func refuse(what, got string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(what).Str(got).Byte('\n').StdErr() //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine)                       //nolint:errcheck // CLI output
	return 2
}

// refused reports a missing keyword, in the shape parseRun answers with.
func refused(keyword, why string) (runArgs, bool) {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(keyword).Str(why).Byte('\n').StdErr() //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usageLine)                          //nolint:errcheck // CLI output
	return runArgs{}, false
}
