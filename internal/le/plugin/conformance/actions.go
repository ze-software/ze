// Design: docs/architecture/core-design.md -- the plugin-conformance area, as one command
// Related: ../../../component/plugin/conformance/conformance.go -- Run, the checker both binaries call
//
// actions.go holds the TABLE. The dispatch, the listing, the help line and the
// two refusals live in internal/le/le/action, which every le area shares.
//
// `le plugin conformance check run <command> [name <name>]` is the development
// launcher's entry to the plugin conformance checker. The shipped binary
// reaches the same conformance.Run through `ze show plugin conformance run`,
// so a developer and an operator read one report.

package pluginconformance

import (
	"context"
	"os"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/conformance"
	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// area is the name this command is typed as.
const area = "plugin conformance"

// The keywords of the check action.
const (
	keywordRun  = "run"
	keywordName = "name"
)

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb: "check",
		// The action starts a program and waits for it, so the main thread
		// schedules it like any other run.
		Heavy:   true,
		Summary: "check that a program behaves as a Ze plugin",
		Why:     "a program started in declare mode writes exactly the Stage 1 declaration line, exits 0 and declares what the daemon accepts, so a plugin author finds a refusal before the plugin is configured",
		Parameters: []leaction.Parameter{
			{
				Keyword: keywordRun, Kind: leaction.KindString, Value: "command",
				Help:        "the run string that starts the plugin, as one quoted token",
				Requirement: leaction.Required,
			},
			{
				Keyword: keywordName, Kind: leaction.KindLabel, Value: "name",
				Help:        "the plugin name in the report, the file name when absent",
				Requirement: leaction.Optional,
			},
		},
		AnswerArgs: runCheck,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le plugin conformance` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le plugin conformance check` action. It answers the report
// with the checker's own exit code: 0 when no row failed, 1 when one did, and
// 2 when the run could not happen.
func runCheck(args leaction.Arguments) (any, int) {
	run := args.One(keywordRun)
	name := conformance.DefaultName(run)
	if args.Has(keywordName) {
		name = args.One(keywordName)
	}

	// The program starts in the directory the developer typed the command
	// in, so a relative path in the run string is read from there.
	workDir, err := os.Getwd()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"plugin conformance check could not read the directory it was started in",
			"the program starts in that directory and a relative path in the run string is read from it, so nothing was started",
			"cd into a directory that exists, then run ./le plugin conformance check run \"<run string>\" again"))
		return nil, conformance.ExitNoRun
	}
	budget, err := plugin.QueryBudget()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"plugin conformance check has no time budget to give the program",
			"the budget is read from ze.plugin.query.timeout, and a program cannot answer in no time, so nothing was started",
			"ZE_PLUGIN_QUERY_TIMEOUT=5s ./le plugin conformance check run \"<run string>\""))
		return nil, conformance.ExitNoRun
	}

	report, err := conformance.Run(context.Background(),
		conformance.Target{Run: run, Name: name, WorkDir: workDir}, budget)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"plugin conformance check could not start the check",
			"no program was started, so there is no report to read",
			"correct what the cause names, then run ./le plugin conformance check run \"<run string>\" again"))
		return nil, conformance.ExitNoRun
	}
	return report, report.ExitCode()
}
