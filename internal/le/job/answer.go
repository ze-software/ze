// Design: docs/architecture/core-design.md -- job admission as a command
// Related: job.go -- the admission this command drives
// Related: overview.go -- what `le job report` answers
//
// `le job run label <label> [quiet] command <argv...>` and `le job report` are
// declared once, in the action table below, so the listing, the usage, the
// help pages and the parse all read one grammar (internal/le/le/action). This
// grammar requires a closed keyword before every value (ai/rules/cli.md). The
// shell accepted its label and command as bare positionals.

package job

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// name is the root command, and the word a developer types.
const name = "job"

// The keywords this command takes. Each one types the value that follows it,
// so a label that happens to spell a keyword is still a label.
const (
	runVerb = "run"
	// reportVerb reads the registry and the machine and prints them
	// (overview.go). It takes no keyword.
	reportVerb     = "report"
	labelKeyword   = "label"
	quietKeyword   = "quiet"
	commandKeyword = "command"
)

// area is the whole command surface. It is built by a function rather than held
// in a variable because the run's own refusals print its usage, and a variable
// whose initializer names a function that reads it is an initialization cycle.
func area() leaction.Area {
	return leaction.New(name,
		leaction.Action{
			Verb:    runVerb,
			Heavy:   true,
			Summary: "admit one heavy job, run it and answer its code",
			Why: "admit one heavy job, run it, and answer its exit code. Several sessions share this machine." +
				" `quiet` before the command keyword writes the child's output to this session's scratch log" +
				" instead of the terminal, and answers the verdict, that log's path and its failure lines",
			Parameters: []leaction.Parameter{
				{Keyword: labelKeyword, Kind: leaction.KindLabel, Value: "label", Requirement: leaction.Required,
					Help: "the name the job is admitted, logged and attached under"},
				{Keyword: quietKeyword, Kind: leaction.KindSwitch,
					Help: "log the child's output to scratch and answer its summary"},
				{Keyword: commandKeyword, Kind: leaction.KindArgv, Value: "argv", Requirement: leaction.Required,
					Help: "the command to run: every word after this keyword"},
			},
			AnswerArgs: answerRun,
		},
		leaction.Action{
			Verb:    reportVerb,
			Summary: "show the job slots, their holders, waiters and the machine",
			Why: "show how many job slots this checkout has, which jobs hold them and which wait for one, each with" +
				" its session and agent, beside the machine's load, memory and free disk. Read it before starting a" +
				" heavy job. It reads the registry and the process table, and writes and signals nothing. The slots" +
				" count only jobs admitted through this checkout's registry",
			Answer: answerReport,
		})
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return area().Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return area().Subs() }

// Answer is the command. It admits and runs one job. It reports the result and
// the job's exit code.
//
// The exit code belongs to the JOB, and this command returns it unchanged. A
// generator says which failure it met through that code: `le repo package-map
// update` exits 1 when the tree holds no ai/ directory and 2 when the tree
// cannot be read, and a caller that flattened the two could not tell an
// incomplete scan from a tree with nowhere to write.
func Answer(args []string) (any, int) { return area().Answer(args) }

// answerRun runs one parsed `run` line.
func answerRun(args leaction.Arguments) (any, int) {
	asked, err := runArgsOf(args)
	if err != nil {
		// 2, which is what the shell half's usage answered for a missing
		// label or an empty command, and what an unknown action answers here.
		// A caller reads it apart from a job that ran and failed.
		return nil, area().Refuse(runVerb, err)
	}

	adm, err := New()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	if asked.Quiet {
		return adm.RunQuiet(asked.Label, asked.Argv, "", nil)
	}

	report, code := adm.Run(asked.Label, asked.Argv, "", nil)
	return report, code
}

// answerReport answers `le job report`: the slots, their holders and waiters,
// and the machine (Admission.Overview). It exits 0 whenever it answered. A
// checkout that cannot be found and a policy the environment states wrongly
// are 2, as they are for `run`, and a registry that cannot be listed is 1:
// there is then no slot count to state.
//
// Each refusal is an operator error: what failed, why the report cannot be
// answered without it, and the command that fixes it, with the cause kept as
// its evidence.
func answerReport() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"job report: no Ze checkout was found from this directory",
			"the job registry is the checkout's "+JobsDir+", so there is no slot or holder to report",
			"cd into the Ze checkout and run ./le job report again"))
		return nil, 2
	}

	adm, err := NewIn(root)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"job report: the job admission policy in the environment is out of range",
			"the slot count is read from that policy, so there is none to report",
			"unset ZE_RUN_SLOTS ZE_JOB_STALL_SECONDS and run ./le job report again"))
		return nil, 2
	}

	view, err := adm.Overview()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"job report: the job registry "+JobsDir+" could not be listed",
			"its entries are the holders and the slot count, and an empty report would read as an idle machine",
			"run ls -ld "+JobsDir+" to see what blocks the read, restore read access, and run ./le job report again"))
		return nil, 1
	}
	return view, 0
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

// parseRun reads the words after `run` through the action table and reports
// whether they name a job. A refused line has already been reported.
//
// Everything after the command keyword is the job's argv, regardless of its
// form (leaction.KindArgv). A wrapped recipe passes a make invocation with its
// own flags. A job flag is not a flag of this command, and neither is a `quiet`
// the child itself takes: this one is read before the command keyword and
// nowhere else.
func parseRun(words []string) (runArgs, bool) {
	parsed, err := area().Parse(runVerb, words)
	if err != nil {
		leaction.ReportError(err)
		return runArgs{}, false
	}
	asked, err := runArgsOf(parsed)
	if err != nil {
		leaction.ReportError(err)
		return runArgs{}, false
	}
	return asked, true
}

// runArgsOf turns the parsed keywords into the run. The parser does not yet
// refuse a missing required keyword (D-P2-required in
// spec-le-cli-usability), so the run does:
// a job with no name or nothing to run is a mistake, never a default.
func runArgsOf(args leaction.Arguments) (runArgs, error) {
	if !args.Has(labelKeyword) {
		return runArgs{}, errors.New("job run needs label <label>: it names the job")
	}
	if !args.Has(commandKeyword) {
		return runArgs{}, errors.New("job run needs command <argv...>: everything after it is the command to run")
	}
	return runArgs{
		Label: args.One(labelKeyword),
		Argv:  args.Values(commandKeyword),
		Quiet: args.Has(quietKeyword),
	}, nil
}
