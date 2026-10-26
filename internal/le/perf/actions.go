// Design: docs/architecture/core-design.md -- the perf area, as one command
// Overview: nudge.go -- the nudge the suggest and record actions run
// Related: bench.go -- run, and the record targets that execute a benchmark
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/le/action, which every ported area shares.
//
// The area is perf. suggest and bare record read the checkout and answer at once;
// run, record history and record evidence run the benchmark chain in bench.go;
// send, compare and track run internal/perf/cli with every word after the verb.
// The action table is the sole command surface.

package perf

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	perfcli "github.com/ze-software/ze/internal/perf/cli"
)

// The verbs that run the benchmark program. Each one forwards the words after
// it to one internal/perf/cli subcommand (programVerbs).
const (
	// sendVerb sends the benchmark traffic at one DUT and writes the result.
	sendVerb = "send"
	// compareVerb renders result files as a comparison.
	compareVerb = "compare"
	// trackVerb reads a result history and detects regressions.
	trackVerb = "track"
	// suggestVerb is the nudge that says a perf run is overdue.
	suggestVerb = "suggest"
)

// programVerbs maps each verb that runs the benchmark program to the
// internal/perf/cli subcommand it runs. `send` is the subcommand the retired
// perf program called `run`: `perf run` is the multi-DUT suite, so the one-DUT sender
// takes the word for what it does. `compare` runs the `report` subcommand,
// because what it does to the files it is given is compare them.
var programVerbs = map[string]string{
	sendVerb:    "run",
	compareVerb: "report",
	trackVerb:   "track",
}

// runParameters is the grammar of `perf run`: the DUT selection of perf-bench
// run and the step selection that replaced the retired perf-run program's --build and --test.
var runParameters = []leaction.Parameter{
	{Keyword: dutKeyword, Kind: leaction.KindList, Help: "the DUTs to measure, comma-separated", Value: "names", Requirement: leaction.Optional},
	{Keyword: stepKeyword, Kind: leaction.KindEnum, Help: "run only the build step or only the test step", Value: "step", Values: []string{stepBuild, stepTest},
		ValueHelp: map[string]string{
			stepBuild: "build the DUT images and measure nothing",
			stepTest:  "measure with the existing DUT images, building none",
		},
		Requirement: leaction.Optional},
}

var actions = leaction.New(area,
	leaction.Action{Verb: suggestVerb, Summary: "suggest a perf run when the data plane changed", Why: "suggest a perf run when BGP data-plane code changed since the last one." +
		" A NUDGE, never a gate -- always exits 0. The heavy suite needs Docker and" +
		" minutes, so it is not run every edit; this notices when a Docker perf run" +
		" is overdue on THIS machine, beside the nightly Docker-free regression check",
		Answer: suggestHere},
	leaction.Action{
		Verb:    recordVerb,
		Summary: "record HEAD as the commit perf last ran at",
		Why: "record the current HEAD as the commit perf last ran at, which clears the suggestion." +
			" `history` instead appends every result of the last measurement to its DUT's committed" +
			" NDJSON history under " + historyDir + ", then records the run; `evidence` is the release" +
			" gate: measure the ze DUT, append its result to the committed history, and exit non-zero" +
			" when that history shows a regression",
		Writes: true,
		Parameters: []leaction.Parameter{{
			Kind: leaction.KindEnum, Value: recordTarget, Requirement: leaction.Optional,
			Help:   "record a measurement instead of only the commit perf ran at",
			Values: []string{recordHistory, recordEvidence},
			ValueHelp: map[string]string{
				recordHistory:  "append the last results to the committed history",
				recordEvidence: "run the release perf gate on the ze DUT",
			},
		}},
		AnswerArgs: recordSelection,
	},
	leaction.Action{
		Verb:    runVerb,
		Summary: "measure BGP convergence, throughput and latency",
		Why: "build the DUT images and measure BGP convergence, UPDATE throughput and p99" +
			" latency against every DUT, then record the run. Needs Docker and minutes." +
			" `dut ze` measures one, `dut \"ze bird\"` measures several;" +
			" `step build` only builds the images, `step test` only measures",
		Writes:     true,
		Parameters: runParameters,
		AnswerArgs: runHere,
	},
	leaction.Action{
		Verb:    sendVerb,
		Summary: "send benchmark traffic at one DUT",
		Why: "send benchmark traffic at one DUT and write the result, with the options" +
			" typed after the verb. The perf runner calls it inside the sender container",
		AnswerWords: func(words []string) (any, int) { return runProgram(sendVerb, words) },
	},
	leaction.Action{
		Verb:        compareVerb,
		Summary:     "compare benchmark result files",
		Why:         "render benchmark result files as a comparison, with the options and files typed after the verb",
		AnswerWords: func(words []string) (any, int) { return runProgram(compareVerb, words) },
	},
	leaction.Action{
		Verb:    trackVerb,
		Summary: "report a benchmark history's trend",
		Why: "read a benchmark history and report its trend; `--check <history>` exits" +
			" non-zero on a regression",
		AnswerWords: func(words []string) (any, int) { return runProgram(trackVerb, words) },
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le perf` command. The program verbs forward their words
// through the table (leaction.Action.AnswerWords). A help word typed last
// never reaches them, because leroot renders the verb's usage first.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runProgram runs the internal/perf/cli subcommand behind verb with argv and
// answers its exit code. The program writes its own output, so the answer
// carries no document.
func runProgram(verb string, argv []string) (any, int) {
	words := make([]string, 0, 1+len(argv))
	words = append(words, programVerbs[verb])
	return nil, perfcli.Dispatch(append(words, argv...))
}

// suggestHere runs the nudge over the checkout this command was run in.
func suggestHere() (any, int) {
	run, err := here()
	if err != nil {
		leaction.ReportError(err)
		// This advisory always exits 0 because it must not block a build.
		// It reports a missing checkout instead of converting that condition into a failure.
		return Report{Error: err.Error()}, 0
	}
	return run.Suggest()
}

// The targets `perf record` takes in its first slot. Bare, it records HEAD.
const (
	recordTarget   = "target"
	recordHistory  = "history"
	recordEvidence = "evidence"
)

// recordSelection is the `le perf record [history|evidence]` action.
func recordSelection(args leaction.Arguments) (any, int) {
	switch args.One(recordTarget) {
	case "":
		return recordHere()
	case recordHistory:
		return historyHere()
	case recordEvidence:
		return evidenceHere()
	}
	panic("BUG: perf: a declared record target has no answer: " + args.One(recordTarget))
}

// recordHere writes the marker for the checkout this command was run in.
func recordHere() (any, int) {
	run, err := here()
	if err != nil {
		leaction.ReportError(err)
		// 1, unlike the nudge: a caller asked for the marker to be written and
		// it was not, which is a failure rather than an advisory.
		return Report{Error: err.Error()}, 1
	}
	return run.Record()
}

// runHere runs the steps and measures the DUTs the invocation named, over this
// checkout. A step value it does not know answers 2, before any work starts.
func runHere(args leaction.Arguments) (any, int) {
	steps, err := stepsOf(args)
	if err != nil {
		leaction.ReportError(err)
		return RunReport{Action: runVerb, Error: err.Error(), Code: 2}, 2
	}
	bench, err := newBench()
	if err != nil {
		leaction.ReportError(err)
		return RunReport{Action: runVerb, Error: err.Error(), Code: 1}, 1
	}
	return signalRooted(func(ctx context.Context) (any, int) {
		return bench.Run(ctx, steps, splitDUTs(args.One(dutKeyword)))
	})
}

// historyHere appends the last measurement's results over this checkout.
func historyHere() (any, int) {
	bench, err := newBench()
	if err != nil {
		leaction.ReportError(err)
		return RunReport{Action: historyVerb, Error: err.Error(), Code: 1}, 1
	}
	return bench.HistoryRecord()
}

// evidenceHere runs the release evidence gate over this checkout.
func evidenceHere() (any, int) {
	bench, err := newBench()
	if err != nil {
		leaction.ReportError(err)
		return RunReport{Action: evidenceVerb, Error: err.Error(), Code: 1}, 1
	}
	return signalRooted(func(ctx context.Context) (any, int) {
		return bench.EvidenceRecord(ctx)
	})
}

// signalRooted runs one benchmark verb under a root context that SIGINT or
// SIGTERM cancels, so the containers and the network it started are removed
// before the process exits, rather than left behind by the default action.
func signalRooted(run func(context.Context) (any, int)) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

// here answers a runner over the checkout this command was run in.
func here() (*Runner, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	return New(root), nil
}
