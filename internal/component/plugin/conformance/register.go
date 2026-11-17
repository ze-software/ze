// Design: docs/architecture/api/commands.md -- where a command is served
// Overview: conformance.go -- Run, the report and the check registry
// Related: checks.go -- the checks registered here
//
// register.go registers the checks of this version and the command that runs
// them from the shipped binary, `show plugin conformance run <command>`. The
// development launcher runs the same Run through `le plugin conformance check`
// (internal/le/plugin/conformance).

package conformance

import (
	"github.com/ze-software/ze/internal/component/command"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyChecks is the envelope key the rows travel under, so a caller parses one
// shape whichever format it asked for.
const keyChecks = "checks"

// The two leaves the command's YANG node declares (ze-plugin-show-cmd.yang).
const (
	leafCommand = "command"
	leafName    = "name"
)

func init() {
	Register(Check{Name: "program-starts", Order: 10, Rows: programStarts})
	Register(Check{Name: "exits-clean", Order: 20, Rows: exitsClean})
	Register(Check{Name: "declaration-line", Order: 30, Rows: declarationLine})
	Register(Check{Name: "declaration-decodes", Order: 40, Rows: declarationDecodes})
	Register(Check{Name: "stage-1-declaration", Order: 50, Rows: stage1Rows})
	Register(Check{Name: "stage-1-daemon-state", Order: 60, Rows: daemonStateRows})

	Register(notJudged(100, "tls-connect-back", stageTransport,
		"the checker starts no hub, so the TLS connect-back and the #0 auth exchange of a live start are not run"))
	Register(notJudged(110, "stage-2-config", "2",
		"the checker sends no configure message, so how the plugin answers its config is not seen"))
	Register(notJudged(120, "stage-3-capabilities", "3",
		"the checker reads no declare-capabilities message, because the plugin is never started live"))
	Register(notJudged(130, "stage-4-registry", "4",
		"the checker sends no share-registry message, so the plugin's answer to it is not seen"))
	Register(notJudged(140, "stage-5-ready", stageFive,
		"the checker reads no ready message, so the subscriptions the plugin asks for are not judged"))
	Register(notJudged(150, "config-callbacks", stageRuntime,
		"the checker opens no config transaction, so the verify, apply, rollback and committed callbacks are not called"))
	Register(notJudged(160, "execute-command-framing", stageRuntime,
		"the checker sends no execute-command, so the framing of the plugin's answer lines is not read"))
	Register(notJudged(170, "bye-and-exit", stageShutdown,
		"the checker sends no bye, so whether the plugin stops and exits when the daemon asks is not seen"))
	Register(notJudged(180, "schema", stageSchema,
		"the YANG modules in the declaration's schemas list are not compiled, so a module the daemon would refuse is not found"))

	command.MustRegisterLocalData("show plugin conformance run", dataConformance, cmdregistry.Meta{
		ShortHelp: "Check that a program behaves as a Ze plugin.",
		Description: "The program is started in declare mode and judged against the documented " +
			"plugin protocol. One row is written for each check, with a verdict of pass, fail or " +
			"not-judged, the reason and the change that fixes a failure. The command exits 1 " +
			"when a check failed.",
		Mode: "offline",
		// Answering starts the program the command line names, so it is
		// process execution: authorized as more than a read.
		NotReadOnly: true,
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show plugin conformance run"}, command.ShapeTab)
	command.RegisterColumns([]string{"show plugin conformance run"},
		command.ColumnOrder{"check", "stage", "verdict", "reason", "fix"})
}

// dataConformance answers `show plugin conformance run <command> [name
// <plugin-name>]` with one row for each check.
//
// It runs on the local route alone. The run string is typed on the command
// line, and `ai/allowed-system-commands.md` authorizes the declare-mode fork
// for the operator's own configured plugin, started in the operator's own
// process. A daemon that started a program a remote caller named would run
// that caller's command under the daemon's identity, which nothing
// authorizes. refuseRemoteCaller is that gate, and the one place to lift it.
func dataConformance(validated command.ValidatedArgs) command.LocalAnswer {
	run, _ := validated.Value(leafCommand)
	if refusal := refuseRemoteCaller(validated.Route(), run); refusal != nil {
		return command.LocalRefusal(ExitNoRun, command.LocalErrorLine(refusal))
	}

	name, named := validated.Value(leafName)
	if !named {
		name = DefaultName(run)
	}
	budget, err := plugin.QueryBudget()
	if err != nil {
		return command.LocalRefusal(ExitNoRun, command.LocalErrorLine(opererr.Wrap(err,
			"show plugin conformance run has no time budget to give the program",
			"the budget is read from ze.plugin.query.timeout, and a program cannot answer in no time, so nothing was started",
			localCommand("ZE_PLUGIN_QUERY_TIMEOUT=5s ", run))))
	}

	// WorkDir stays empty, so the program starts in the caller's own
	// directory and a relative path in the run string is read from there.
	report, err := Run(validated.Context(), Target{Run: run, Name: name}, budget)
	if err != nil {
		return command.LocalRefusal(ExitNoRun, command.LocalErrorLine(opererr.Wrap(err,
			"show plugin conformance run could not complete the check",
			"no complete conformance report was produced, so no verdict can be read",
			"correct what the cause names, then run "+localCommand("", run)+" again")))
	}
	return command.LocalAnswer{Payload: plugin.Map{keyChecks: report.Rows}, Code: report.ExitCode()}
}

// refuseRemoteCaller is the daemon-route gate of the conformance command: it
// answers nil for the operator's own process and a refusal for every other
// route, the unmarked one included. The refusal names the reason and the
// command to run instead.
func refuseRemoteCaller(route command.Route, run string) error {
	if route.CallerLocal() {
		return nil
	}
	return opererr.New(
		"show plugin conformance run was refused: it starts the program named on the command line",
		"the daemon starts no program a remote caller names, so run the check in your own shell on the host that holds the program",
		localCommand("", run))
}

// localCommand is the command an operator types in their own shell to check
// the run string run, after prefix: the fix line of each refusal here.
func localCommand(prefix, run string) string {
	var tb textbuf.Buffer
	return tb.Str(prefix).Str("ze show plugin conformance run ").Quoted(run).String()
}
