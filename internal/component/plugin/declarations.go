// Design: docs/architecture/api/commands.md -- where a command is served
// Related: register.go -- the init() that registers this command, beside `show plugin list`
// Related: registry/registry.go -- Registration.Commands and Registration.Pipes, the in-tree declaration
//
// declarations.go answers `show plugin declarations`, the command that reports
// what each plugin declares rather than which plugins exist. The two questions
// share one noun and take two actions: `show plugin list` is what the binary
// carries, and this command is what each plugin declares.
//
// A plugin the binary carries declares into `registry.Registration`, which
// this process already holds. A plugin only a config file names has its code
// in another binary, so its declaration is reachable only by starting that
// binary in query mode. A running daemon starts it. A command with no daemon
// starts no external plugin (owner ruling: "plugin command requiring external
// plugins which can not be started from the code when no daemon is running to
// pass it to are not available"), so there the row says the declaration is not
// available and names the two commands that read it. Both populations answer
// here, one row each, and a row is never dropped: the state field says why a
// row carries no declaration.
//
// A row is never dropped and never silently empty. The state says which
// answer the row carries: the plugin declared something, it declared
// nothing, it ran and wrote no declaration, it could not be started (which
// covers a declaration the daemon refuses at Stage 1), it started and said
// nothing inside the budget, it declared in a binary that cannot judge the
// declaration, its program was not started because no daemon runs here, or
// its caller stopped the query before it completed.

package plugin

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// keyDeclarations is the envelope key the rows travel under, so a caller
// parses one shape whichever format it asked for.
const keyDeclarations = "declarations"

// keywordConfig types the one value this command takes, so the operator names
// what the path is before they give it (`ai/rules/cli.md`, keyword before
// value).
const keywordConfig = "config"

// The kinds a row can carry. They are the two words the config file already
// uses for the same split, `plugin { internal ... }` against
// `plugin { external ... }`, so an operator reads one vocabulary in both
// places.
const (
	kindInternal = "internal"
	kindExternal = "external"
)

// The states a row can carry. They are exhaustive: every plugin gets one,
// and none of them is the absence of an answer.
//
// The pair that must stay apart is stateDeclaredNone and stateNoAnswer. A
// plugin that declares no command and no pipe still writes its declaration
// line, so the two are different facts: the first is a plugin with nothing to
// declare, and the second is a plugin that did not answer at all, which is what
// a binary with no query mode in it looks like. A reader that folded them
// together would report every unaware plugin as a plugin with nothing to say
// (`ai/rules/principles.md`).
const (
	stateDeclared     = "declared"
	stateDeclaredNone = "declared-none"
	stateNoAnswer     = "no-answer"
	stateUnstartable  = "unstartable"
	stateTimeout      = "timeout"
	stateCanceled     = "canceled"
	// stateUnjudged is a declaration that was read in a binary that links no
	// Stage 1 check table (declaration_judge.go). It is a state of its own so
	// an unjudged declaration is never listed as declared.
	stateUnjudged = "not-judged"
	// stateNotAvailable is an external plugin a config file declares, read by
	// a command with no daemon: ze starts no external plugin then, so nobody
	// asked the program what it declares. It is a state of its own because
	// every other state says something about the program (it answered, it
	// stayed silent, it could not start), and nothing is known of a program
	// that was never started.
	stateNotAvailable = "not-available"
)

// The two commands the reason of a stateNotAvailable row names. A running ze
// starts the program in query mode, so the same command typed in a session it
// serves reads the declaration. `ze cli -c "<command>"` is not that session:
// it answers a local-data command in its own process (command.ServeLocal),
// which is the process that starts nothing. The conformance checker starts the
// program the operator names on its own command line, with no daemon, because
// running that program is its purpose.
const (
	daemonSession       = "ze cli"
	daemonDeclarations  = "show plugin declarations config "
	conformanceCheckRun = "ze show plugin conformance run "
)

// envQueryTimeout names the budget one queried plugin has to write its
// declaration, and defaultQueryTimeout is the same 5 seconds a startup stage
// gets (defaultStageTimeout, plugin/server/server.go). A query answers one
// message where a stage answers one exchange, so the two are the same size of
// wait and there is no second number to keep in step.
const (
	envQueryTimeout     = "ze.plugin.query.timeout"
	defaultQueryTimeout = 5 * time.Second
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         envQueryTimeout,
	Type:        "duration",
	Default:     "5s",
	Description: "How long a plugin started for a declaration query has to write its declaration before the reader stops it",
})

// queryDrainDelay bounds the wait for the child's output AFTER the child is
// killed. A run string under a shell can leave a grandchild holding the write
// end of the pipe, and a read of that pipe ends when the last writer goes, not
// when the child does. exec.Cmd.WaitDelay closes the pipes once this expires,
// so the whole query is bounded by the budget plus this.
const queryDrainDelay = 200 * time.Millisecond

// The interpreter a run string is given to is Shell (shell.go), the one the
// daemon's live start forks ((*Process).startExternal), because a run string is
// written for a shell: it carries quoting, and a plugin startable by the daemon
// must be startable by the reader (A-4).

// exitCommandNotFound and exitNotExecutable are what a POSIX shell exits with
// when it could not run the command it was given. They are the shell's way of
// saying the process never started, which is stateUnstartable rather than a
// plugin that ran and stayed silent.
const (
	exitCommandNotFound = 127
	exitNotExecutable   = 126
)

// childEnvPluginNamespace is the namespace every plugin variable is registered
// under. Each inherited variable in it is dropped from a queried child's
// environment, which is how the child gets no hub host, no hub port, no token
// and no CA: a plugin that ignores query mode and tries to start for real then
// fails to connect instead of joining a live daemon's hub (R-1).
//
// It is the DOTTED key namespace rather than an OS spelling, because a
// variable in it reaches the environment under either: the daemon's own fork
// writes `ZE_PLUGIN_HUB_TOKEN` and env.Set writes `ze.plugin.hub.token`, and
// env.Get reads both. The drop is decided by env.InNamespace for that reason.
const childEnvPluginNamespace = "ze.plugin."

// declarationRow is one row of `show plugin declarations`: which plugin the
// row answers for, whether the plugin's code is in this binary or in another
// one, whether a declaration was read, and the declaration itself.
//
// Commands and Pipes are the two declarations a plugin makes about its command
// surface (`pkg/plugin/rpc/types.go`). They are empty until a declaration is
// read, and State says which of those two an empty value means.
type declarationRow struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	State    string            `json:"state"`
	Commands []rpc.CommandDecl `json:"commands,omitempty"`
	Pipes    []rpc.PipeDecl    `json:"pipes,omitempty"`
	Reason   string            `json:"reason,omitempty"`
}

// ConfigProcess says which process read a config file for this command: a
// command with no daemon, or a running daemon. The zero value is no answer,
// and a reader that left it is refused (errNoConfigProcess).
type ConfigProcess uint8

const (
	// ConfigProcessUnspecified is the zero value, which no reader answers.
	ConfigProcessUnspecified ConfigProcess = iota
	// ConfigProcessOffline is a command that runs with no daemon. It starts
	// no external plugin, so the declaration of one is not available in it.
	ConfigProcessOffline
	// ConfigProcessDaemon is a running daemon, which starts each external
	// plugin the file declares in query mode to read its declaration.
	ConfigProcessDaemon
)

// ConfiguredPlugins is what a ConfiguredPluginReader answers: the plugin
// blocks a config file declares, and the process that read the file.
type ConfiguredPlugins struct {
	// Plugins holds one entry for each plugin the file declares.
	Plugins []PluginConfig
	// Process says whether a daemon runs in the process that read the file.
	// The reader MUST set it: it decides whether an external plugin's program
	// is started.
	Process ConfigProcess
}

// ConfiguredPluginReader answers the plugin blocks a config file declares, and
// which process read it.
//
// It is a seam rather than a call because the dependency runs the other way:
// `internal/component/config` parses the file and already imports this
// package, for `PluginConfig`, so this package cannot import it back. The
// config component registers its reader from an init().
//
// The process comes through the same seam on purpose. The config component
// already decides "this command runs with no daemon" for every command that
// parses a config (config.DeclaredSchemas.Offline), so this command asks that
// answer and keeps no second test of its own.
type ConfiguredPluginReader func(path string) (ConfiguredPlugins, error)

var (
	configuredPluginsMu sync.RWMutex
	configuredPlugins   ConfiguredPluginReader
)

// errNoConfigReader is what `show plugin declarations config <path>` answers in
// a binary that links no config parser.
//
// A nil reader is a refusal, never an empty answer: a binary with no config
// parser in it says that it cannot read the file, rather than reporting that
// the file names no plugin (`ai/rules/principles.md`).
var errNoConfigReader = errors.New("this binary links no config parser, so the plugins a config file names cannot be listed")

// errNoConfigProcess is what `show plugin declarations config <path>` answers
// when the reader named no process (ConfigProcessUnspecified).
//
// It is a refusal because both readings of the missing answer are wrong: one
// starts an operator's program in a process that MUST NOT, and the other
// reports every external plugin as not available inside a daemon. Only a
// defect in the reader produces it.
var errNoConfigProcess = errors.New("BUG: the config reader did not say whether a daemon runs in this process, " +
	"so ze cannot tell whether it may start the external plugins the file declares: report this as a defect of ze")

// SetConfiguredPluginReader registers the function that answers which plugins
// a config file names. The config component calls it once, from an init().
//
// Safe for concurrent use.
func SetConfiguredPluginReader(read ConfiguredPluginReader) {
	if read == nil {
		return
	}
	configuredPluginsMu.Lock()
	defer configuredPluginsMu.Unlock()
	configuredPlugins = read
}

// dataDeclarations answers `show plugin declarations` with one row for each
// plugin this binary carries, in name order.
//
// It takes no argument: the answer is the whole set, and a reader who wants one
// plugin narrows it with `| match <name>`.
func dataDeclarations(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) > 0 {
		var tb textbuf.Buffer
		return declarationRefusal(errors.New(tb.Str("show plugin declarations takes no value, and ").
			Str(args[0]).Str(" follows it: name a config file with ").Str(keywordConfig).
			Str(" <path>").String()))
	}
	return answerDeclarations(validated.Context(), "")
}

// dataDeclarationsConfig answers `show plugin declarations config <path>`: the
// same rows, plus one for each plugin the named config file declares.
//
// The keyword is a command word rather than a value, so the CLI tree carries it
// and completion offers it (`ai/rules/cli.md`, keyword before value). What
// reaches this handler is the path alone.
func dataDeclarationsConfig(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	// On the daemon route the file must lie where the daemon reads
	// configuration from, and that is judged before the file is looked at, so
	// a refusal says nothing about a file outside those directories.
	if len(args) == 1 {
		readPath, err := command.CallerPath(validated.Route(), "show plugin declarations config", args, args[0])
		if err != nil {
			return declarationRefusal(err)
		}
		args = []string{readPath}
	}
	path, err := declarationConfigPath(args)
	if err != nil {
		return declarationRefusal(err)
	}
	return answerDeclarations(validated.Context(), path)
}

// answerDeclarations builds the payload both handlers answer with, so the two
// commands are two entry points into one answer rather than two answers.
func answerDeclarations(ctx context.Context, configPath string) command.LocalAnswer {
	rows, err := declarationRows(ctx, configPath)
	if err != nil {
		return declarationRefusal(err)
	}
	return command.LocalAnswer{Payload: Map{keyDeclarations: rows}}
}

// declarationConfigPath reads the path typed after the `config` keyword.
//
// The path is checked here rather than at the read, so an operator who
// mistyped a filename is told that, and is not told that the file names no
// plugin.
func declarationConfigPath(args []string) (string, error) {
	var tb textbuf.Buffer
	if len(args) == 0 {
		return "", errors.New("show plugin declarations config needs the path of a config file after it")
	}
	if len(args) > 1 {
		return "", errors.New(tb.Str("show plugin declarations config takes one path, and ").
			Str(args[1]).Str(" follows a second one").String())
	}

	// A config arriving on stdin has no path to check, and "-" is the token
	// every Ze command spells it with (`internal/core/cliio`). Everything else
	// is checked here, so an operator who mistyped a filename is told that.
	path := args[0]
	if cliio.IsStdin(path) {
		return path, nil
	}
	if _, err := os.Stat(path); err != nil {
		return "", errors.New(tb.Str("cannot read the config file ").Str(path).
			Str(": ").Err(err).String())
	}
	return path, nil
}

// declarationRows answers one row for every plugin this binary carries, plus
// one for every plugin the named config file declares, in name order.
//
// A plugin named in both places gets ONE row, and the CONFIG BLOCK decides
// where that row's declaration comes from. An `internal` block names code this
// binary carries, so the row is answered from the registration and no process
// is started (AC-9). An `external` block names ANOTHER binary, whatever the
// name it gives that binary, so Ze's own declaration under that name is never
// its answer: a block that reads `external mrt { run "/opt/vendor/my-mrt" }`
// is a vendor's program and Ze's own mrt declaration would be a wrong answer
// with the right shape. Only that program can answer, and the process that
// read the file decides whether it is asked: a running daemon starts it in
// query mode, and a command with no daemon starts nothing and says the
// declaration is not available (configuredDeclarationRow).
func declarationRows(ctx context.Context, configPath string) ([]declarationRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	budget, err := declarationBudget()
	if err != nil {
		return nil, err
	}

	byName := inTreeDeclarationRows()
	carried := make(map[string]bool, len(byName))
	for name := range byName {
		carried[name] = true
	}

	if configPath != "" {
		configured, readErr := readConfiguredPlugins(configPath)
		if readErr != nil {
			return nil, readErr
		}
		for _, configuredPlugin := range configured.Plugins {
			// A compiled-in plugin that this binary carries is already
			// answered from the registration walk above, whether running
			// in-daemon or as a separate process.
			if configuredPlugin.IsCompiled() && carried[configuredPlugin.CompiledIdentity()] {
				continue
			}
			row, rowErr := configuredDeclarationRow(ctx, configuredPlugin, configured.Process, configPath, budget)
			if rowErr != nil {
				return nil, rowErr
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			byName[configuredPlugin.Name] = row
		}
	}

	rows := make([]declarationRow, 0, len(byName))
	for _, row := range byName {
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(first, second declarationRow) int {
		return strings.Compare(first.Name, second.Name)
	})
	return rows, nil
}

// QueryBudget answers how long one declare-mode run has, for a caller outside
// this package: the conformance checker gives a program the budget `show
// plugin declarations` gives it, read from the same setting.
func QueryBudget() (time.Duration, error) {
	return declarationBudget()
}

// declarationBudget answers how long a queried plugin has to write its
// declaration.
//
// A budget of zero or less is REFUSED rather than obeyed. Waiting no time at
// all reports every started plugin as a timeout, which reads as a fleet of
// broken plugins and hides the one setting that produced it.
func declarationBudget() (time.Duration, error) {
	budget := env.GetDuration(envQueryTimeout, defaultQueryTimeout)
	if budget > 0 {
		return budget, nil
	}

	var tb textbuf.Buffer
	return 0, errors.New(tb.Str(envQueryTimeout).Str(" is ").Str(budget.String()).
		Str(", and a plugin cannot answer in no time at all: give it a positive duration").String())
}

// inTreeDeclarationRows answers one row for every plugin this binary carries,
// keyed by name.
//
// The population is registry.SetupResults, the set `show plugin list` answers
// for (pluginRows, register.go), and the two commands answer for one set on
// purpose: a plugin missing from one answer and present in the other tells an
// operator that the two disagree about which plugins this binary has. That set
// is every registered plugin UNION every plugin that recorded a setup outcome,
// so it holds the plugin whose init() recorded and whose Register never
// completed. Dropping that plugin here would read as "not built into this
// binary", which is the silence the never-omit rule exists to remove.
func inTreeDeclarationRows() map[string]declarationRow {
	registered := make(map[string]*registry.Registration)
	for _, registration := range registry.All() {
		registered[registration.Name] = registration
	}

	results := registry.SetupResults()
	rows := make(map[string]declarationRow, len(results))
	for _, result := range results {
		registration, complete := registered[result.Plugin]
		if !complete {
			rows[result.Plugin] = unregisteredDeclarationRow(result)
			continue
		}
		rows[result.Plugin] = inTreeDeclarationRow(registration)
	}
	return rows
}

// unregisteredDeclarationRow answers for a plugin whose own init() recorded a
// setup outcome and whose Register call never completed.
//
// Nothing of the plugin declared, so there is no declaration to read and none
// to start a process for: this binary holds the plugin's code and holds no
// answer from it. The row says that, and carries the outcome the plugin
// recorded, so the operator is sent to the setup that failed rather than to a
// plugin that looks like it declares nothing.
func unregisteredDeclarationRow(result registry.SetupResult) declarationRow {
	var tb textbuf.Buffer
	tb.Str("the plugin recorded setup outcome ").Str(result.Outcome.String()).
		Str(" and never completed its registration, so this binary holds no declaration for it")
	if result.Reason != "" {
		tb.Str(": ").Str(result.Reason)
	}

	return declarationRow{
		Name:   result.Plugin,
		Kind:   kindInternal,
		State:  stateUnstartable,
		Reason: tb.String(),
	}
}

// inTreeDeclarationRow answers for a plugin this binary carries, from the
// registration this process already holds. No process is started: the
// declaration is a field of the compiled-in registration, and starting the
// plugin to be told what the binary already knows would be a second answer that
// can disagree with the first.
func inTreeDeclarationRow(registration *registry.Registration) declarationRow {
	row := declarationRow{
		Name:     registration.Name,
		Kind:     kindInternal,
		State:    stateDeclared,
		Commands: registration.Commands,
		Pipes:    registration.Pipes,
	}
	if len(row.Commands) == 0 && len(row.Pipes) == 0 {
		row.State = stateDeclaredNone
		row.Reason = "the plugin's registration declares no command and no pipe"
	}
	return row
}

// configuredDeclarationRow answers for a plugin a config file declares and the
// registration walk did not answer, in the process that read the file.
//
// The process decides one thing: whether the program an external block names
// is started. A running daemon starts it in query mode. A command with no
// daemon starts no external plugin, so the row says the declaration is not
// available there. A reader that named no process is refused: neither reading
// of the missing answer is safe (errNoConfigProcess).
func configuredDeclarationRow(ctx context.Context, configured PluginConfig, process ConfigProcess, configPath string, budget time.Duration) (declarationRow, error) {
	switch process {
	case ConfigProcessOffline:
		return unavailableDeclarationRow(configured, configPath), nil
	case ConfigProcessDaemon:
		return queriedDeclarationRow(ctx, configured, budget), nil
	case ConfigProcessUnspecified:
		return declarationRow{}, errNoConfigProcess
	}
	// A value outside the named set is the same defect as the zero value: a
	// reader that did not answer.
	return declarationRow{}, errNoConfigProcess
}

// blockWithNoProgramRow answers for a config block that names no program to
// start, and reports whether the block is one. Such a row needs no process in
// either mode, so a daemon and a command with no daemon answer it alike.
//
// A config block that declares the plugin internal, for a plugin this binary
// does not carry, has nothing to start and nothing to read: it names code that
// is in no binary here, which is a refusal rather than a plugin with nothing to
// declare.
func blockWithNoProgramRow(configured PluginConfig) (row declarationRow, answered bool) {
	row = declarationRow{Name: configured.Name, Kind: configuredKind(configured), State: stateUnstartable}
	if configured.IsCompiled() {
		row.Reason = "the config declares this plugin internal and this binary does not carry it"
		return row, true
	}
	if configured.Run == "" {
		row.Reason = "the config block names no run command, so there is nothing to start"
		return row, true
	}
	return declarationRow{}, false
}

// unavailableDeclarationRow answers for an external plugin a config file
// declares, in a command with no daemon. The program is NOT started: with no
// daemon running ze starts no external plugin, and inspecting a config someone
// else wrote MUST NOT run the commands it names.
//
// The row keeps the plugin, says that its declaration was not read and why,
// and names the two commands that read it: the same command in a session a
// running ze serves, and the conformance checker for an author who wants the
// program started now.
func unavailableDeclarationRow(configured PluginConfig, configPath string) declarationRow {
	if row, answered := blockWithNoProgramRow(configured); answered {
		return row
	}

	// A config read from standard input has no name a daemon could be given.
	file := "<config-file>"
	if !cliio.IsStdin(configPath) {
		file = configPath
	}

	var tb textbuf.Buffer
	tb.Str("the declaration of this plugin is not available with no daemon running: its program is ").
		Str("declared in the config and ze starts no external plugin here, so nobody asked it what it ").
		Str("declares. A running ze reads it: type `").Str(daemonDeclarations).Str(file).
		Str("` in the session `").Str(daemonSession).Str("` opens, with the file in the configuration ").
		Str("directory of that ze. To start the program yourself, with no daemon: ").
		Str(conformanceCheckRun).Str(quotedRunString(configured.Run))
	return declarationRow{
		Name:   configured.Name,
		Kind:   kindExternal,
		State:  stateNotAvailable,
		Reason: tb.String(),
	}
}

// quotedRunString writes a run string as the one quoted token the conformance
// checker takes after `run`, for an operator to paste into a shell. A run
// string that holds a double quote cannot sit inside double quotes unchanged,
// so the row then names the value instead of spelling a command that would not
// parse.
func quotedRunString(run string) string {
	if strings.Contains(run, `"`) {
		return `"<the run string of this plugin's config block>"`
	}
	return `"` + run + `"`
}

// queriedDeclarationRow answers for a plugin whose code is in another binary,
// by starting that binary in query mode and reading its declaration. Only a
// running daemon calls it (configuredDeclarationRow).
func queriedDeclarationRow(ctx context.Context, configured PluginConfig, budget time.Duration) declarationRow {
	if row, answered := blockWithNoProgramRow(configured); answered {
		return row
	}
	row := declarationRow{Name: configured.Name, Kind: configuredKind(configured)}

	query := QueryDeclaration(ctx, configured, budget)
	if query.Exit == QueryExitCanceled {
		row.State = query.silence.state
		row.Reason = query.silence.reason
		return row
	}
	if !query.Declared {
		row.State = query.silence.state
		row.Reason = query.silence.reason
		return row
	}
	// A declaration the daemon's decoder refuses, for a key ze does not know,
	// is refused here with the same text before any check reads it.
	if query.Refusal != nil {
		row.State = stateUnstartable
		row.Reason = query.Refusal.Error()
		return row
	}

	// The declaration is judged by the daemon's own Stage 1 checks, so a row is
	// listed as declared only when the daemon would start the plugin. A binary
	// that links no check table says the declaration was not judged: it never
	// reports an unjudged declaration as declared.
	judgement, err := JudgeDeclaration(configured.Name, &query.Declaration)
	if err != nil {
		var tb textbuf.Buffer
		row.State = stateUnjudged
		row.Reason = tb.Str("the plugin answered, and ").Err(err).
			Str(": run this command from the ze binary").String()
		return row
	}
	// A declaration the daemon refuses at Stage 1 is refused here with the
	// daemon's own reason (server/stage1_checks.go), never listed as declared.
	if refusal := judgement.Refusal(); refusal != nil {
		row.State = stateUnstartable
		row.Reason = refusal.Error()
		return row
	}

	row.Commands = query.Declaration.Commands
	row.Pipes = query.Declaration.Pipes
	row.State = stateDeclared
	if len(row.Commands) == 0 && len(row.Pipes) == 0 {
		row.State = stateDeclaredNone
		row.Reason = "the plugin answered and declares no command and no pipe"
	}
	return row
}

// QueryExit says how a declare-mode run of a plugin ended. The zero value is
// no answer: every run QueryDeclaration returns carries one of the named
// values.
type QueryExit uint8

const (
	// QueryExitUnspecified is the zero value, which no run carries.
	QueryExitUnspecified QueryExit = iota
	// QueryExitClean is a program that exited 0 and left nothing running.
	QueryExitClean
	// QueryExitFailed is a program that ran and exited with another status.
	QueryExitFailed
	// QueryExitTimeout is a program that was still running when the budget
	// ended, and was stopped.
	QueryExitTimeout
	// QueryExitNotStarted is a run string that produced no plugin process:
	// the fork failed, or the shell found no such command or could not
	// execute it (status 127 or 126).
	QueryExitNotStarted
	// QueryExitOutlived is a program that exited 0 while a process it started
	// still held its standard output.
	QueryExitOutlived
	// QueryExitCanceled is a query stopped because its caller ended, including
	// a caller deadline. It is distinct from the independent query budget.
	QueryExitCanceled
)

// DeclarationQuery is everything one declare-mode run of a plugin produced:
// what the program wrote, how it ended, and the declaration read from what it
// wrote.
type DeclarationQuery struct {
	// Stdout is every byte the program wrote to standard output, up to
	// rpc.MaxMessageSize.
	Stdout []byte
	// Exit says how the run ended.
	Exit QueryExit
	// ExitCode is the status the program exited with. It is meaningful for
	// QueryExitFailed and for a QueryExitNotStarted the shell reported, and
	// -1 for a failed run a signal ended. It is not meaningful for a canceled
	// query.
	ExitCode int
	// Cause is the text of the start, wait or caller interruption error, as
	// evidence. It is empty for QueryExitClean.
	Cause string
	// Declaration is the Stage 1 declaration read from Stdout. It is
	// meaningful only when Declared is true.
	Declaration rpc.DeclareRegistrationInput
	// Declared says a declaration line was read. A program that answered has
	// answered whatever its exit was.
	Declared bool
	// Refusal is the decoder's refusal of the declaration line read: one that
	// carries a key ze does not know (rpc.DecodeDeclaration). Declared is true
	// with it, and Declaration is then empty. The daemon refuses such a
	// plugin at Stage 1 with the same text.
	Refusal error

	// silence is the row `show plugin declarations` writes when no
	// declaration was read or the caller stopped the query before it completed.
	silence queryOutcome
}

// queryOutcome is what running the child says about the child itself, as
// opposed to what it wrote. A missing declaration or a caller-canceled query
// reads this outcome; a completed answer is retained whatever its exit status.
type queryOutcome struct {
	state  string
	reason string
}

// QueryDeclaration starts one plugin's run string in query mode and returns
// whatever it wrote to stdout, how the run ended, and the declaration read
// from that output. It is the one declare-mode fork: `show plugin
// declarations` and the conformance checker both read its answer.
// The caller MUST provide a non-nil context. Caller cancellation stops the
// isolated process group and joins the child before returning QueryExitCanceled;
// only the query's own budget is QueryExitTimeout.
//
// The start mirrors (*Process).startExternal: the run string under a shell,
// because it is shell-quoted and no argv can be appended to it, and the engine
// binary's directory on PATH as a fallback so `run "ze plugin mrt"` finds ze in
// a checkout. It forks here rather than calling that method because
// internal/component/plugin/process imports this package, so a call from here
// is an import cycle; and because that path discards the child's exit status,
// which is the one fact separating a plugin that could not start from a plugin
// that ran and said nothing.
func QueryDeclaration(ctx context.Context, configured PluginConfig, budget time.Duration) DeclarationQuery {
	queryCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	// #nosec G204 -- the run string comes from the operator's own config, the
	// same string the daemon starts, or from the command line of an operator
	// who runs the conformance checker in their own process and under their
	// own identity (conformance/register.go refuses the daemon route).
	child := exec.CommandContext(queryCtx, Shell, "-c", configured.Run)
	if configured.WorkDir != "" {
		child.Dir = configured.WorkDir
	}
	child.Env = queryChildEnv()

	// The plugin is started in its own process group and a caller or budget stop
	// is aimed at that group, so the stop reaches the plugin and not only the
	// shell that was given the run string. A shell keeps the plugin as its own
	// child for every run string it does not exec-optimize, and a stop that
	// reached the shell alone would leave the plugin running its LIVE start
	// past the budget, which is the one outcome query mode exists to prevent.
	// The daemon's live start takes the same stop from the same function
	// (KillGroupOnCancel, sysproc.go).
	KillGroupOnCancel(child)

	// The child's stdout is held rather than streamed, so a plugin that answers
	// and then fails to exit still has its answer read: the bytes are here
	// whatever the wait reports.
	stdout := boundedBuffer{limit: rpc.MaxMessageSize}
	child.Stdout = &stdout

	// A run string under a shell can leave a grandchild holding the write end
	// of the pipe, so the read of it would outlive the killed child. WaitDelay
	// closes the pipes once the child is gone, which is what bounds Wait.
	child.WaitDelay = queryDrainDelay

	if err := StartChild(child); err != nil {
		if stopped := ctx.Err(); stopped != nil {
			return canceledDeclarationQuery(&DeclarationQuery{}, stopped)
		}
		return DeclarationQuery{
			Exit:    QueryExitNotStarted,
			Cause:   err.Error(),
			silence: unstartableOutcome(err, ShellAvailable()),
		}
	}

	waitErr := child.Wait()
	query := DeclarationQuery{Stdout: stdout.held}
	query.Declaration, query.Declared, query.Refusal = declarationFromStdout(configured.Name, stdout.held)
	if err := ctx.Err(); err != nil {
		return canceledDeclarationQuery(&query, err)
	}
	if queryCtx.Err() != nil {
		var tb textbuf.Buffer
		query.Exit = QueryExitTimeout
		query.Cause = queryCtx.Err().Error()
		query.silence = queryOutcome{
			state: stateTimeout,
			reason: tb.Str("the plugin wrote no declaration within ").Str(budget.String()).
				Str(" and was stopped; ").Str(envQueryTimeout).Str(" sets that budget").String(),
		}
		return query
	}
	query.Exit, query.ExitCode = queryExitOf(waitErr)
	if waitErr != nil {
		query.Cause = waitErr.Error()
	}
	query.silence = queryOutcomeOfExit(waitErr)
	return query
}

// canceledDeclarationQuery retains any declaration already read, but records
// the caller's interruption rather than a violation of the plugin's budget.
// query MUST be non-nil.
func canceledDeclarationQuery(query *DeclarationQuery, cause error) DeclarationQuery {
	query.Exit = QueryExitCanceled
	query.Cause = cause.Error()
	var tb textbuf.Buffer
	query.silence = queryOutcome{
		state: stateCanceled,
		reason: tb.Str("the caller stopped the declaration query before it completed: ").Err(cause).
			Str("; retry the declaration query when its caller can wait for completion").String(),
	}
	return *query
}

// queryExitOf reads what Wait returned as the way the run ended.
//
// exec.ErrWaitDelay is what Wait answers for a program that exited 0 while a
// process it started still held the pipe: the wait closed the pipe after
// queryDrainDelay. A program that exited with another status answers its
// *exec.ExitError whether or not a descendant held the pipe, so that case
// reads as QueryExitFailed.
func queryExitOf(waitErr error) (QueryExit, int) {
	if waitErr == nil {
		return QueryExitClean, 0
	}
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		return QueryExitOutlived, 0
	}
	exitErr, isExit := errors.AsType[*exec.ExitError](waitErr)
	if !isExit {
		// The wait failed for a reason that is no exit status, such as a
		// failed copy of the output. The run did not end clean.
		return QueryExitFailed, -1
	}
	code := exitErr.ExitCode()
	if code == exitCommandNotFound {
		return QueryExitNotStarted, code
	}
	if code == exitNotExecutable {
		return QueryExitNotStarted, code
	}
	return QueryExitFailed, code
}

// unstartableOutcome says why the fork never produced a plugin process.
//
// A host with no shell fails every external plugin start for one reason, so the
// row names the absent shell. A row that read "cannot start the plugin" would
// send the operator to the plugin's own code, where there is nothing to repair
// (`ai/rules/principles.md`).
//
// The shell answer is a parameter so a test states it: no test can take /bin/sh
// away from the host it runs on.
func unstartableOutcome(startErr, shellErr error) queryOutcome {
	var tb textbuf.Buffer
	if shellErr != nil {
		return queryOutcome{
			state:  stateUnstartable,
			reason: tb.Err(shellErr).String(),
		}
	}
	return queryOutcome{
		state:  stateUnstartable,
		reason: tb.Str("cannot start the plugin: ").Err(startErr).String(),
	}
}

// queryOutcomeOfExit reads the child's exit status as a statement about whether
// the plugin ran at all.
//
// A shell that could not find the command, or found one it could not execute,
// exits 126 or 127 and nothing of the plugin ran: that is stateUnstartable, and
// reporting it as silence would blame a plugin for a run string that names a
// binary which is not there. Every other exit means the process ran and wrote
// no declaration, which is what a binary with no query mode in it does.
//
// The run string is given to a shell, and a shell reports its own failure with
// the same status a plugin can exit with, so a plugin that itself exits 126 or
// 127 reads as a plugin that never ran. The reason says so, because the two
// repairs are in different places and the row cannot tell the operator which
// one to make.
func queryOutcomeOfExit(waitErr error) queryOutcome {
	if exitErr, isExit := errors.AsType[*exec.ExitError](waitErr); isExit {
		switch exitErr.ExitCode() {
		case exitCommandNotFound, exitNotExecutable:
			var tb textbuf.Buffer
			return queryOutcome{
				state: stateUnstartable,
				reason: tb.Str("the run command exited ").Int(int64(exitErr.ExitCode())).
					Str(": no such command, or it is not executable; a plugin that itself exits ").
					Int(int64(exitErr.ExitCode())).Str(" reads the same way, because the shell reports both with this status").String(),
			}
		}
	}
	if waitErr != nil {
		var tb textbuf.Buffer
		return queryOutcome{
			state:  stateNoAnswer,
			reason: tb.Str("the plugin ran and wrote no declaration: ").Err(waitErr).String(),
		}
	}
	return queryOutcome{
		state:  stateNoAnswer,
		reason: "the plugin ran, wrote no declaration and exited 0, so it does not support query mode",
	}
}

// queryChildEnv composes the environment a queried plugin is started with: this
// process's environment without any plugin variable, the engine directory on
// PATH as a fallback, and the mode.
//
// Dropping the whole ze.plugin. namespace is what makes the guarantee cheap to
// state and impossible to get half right: the child receives no hub host, no
// port, no token, no CA and no plugin name, so a plugin that ignores the mode
// and starts for real fails to connect rather than joining a live daemon.
//
// The run string is started by a shell, as the live start is, so every other
// ze key is passed under its shell spelling (env.ShellSafe): dash drops a
// dotted or hyphenated name, and the queried plugin would declare from the
// key's default.
func queryChildEnv() []string {
	inherited := env.ShellSafe(os.Environ())
	childEnv := make([]string, 0, len(inherited)+2)
	for _, entry := range inherited {
		name, _, named := strings.Cut(entry, "=")
		if !named {
			continue
		}
		if env.InNamespace(name, childEnvPluginNamespace) {
			continue
		}
		childEnv = append(childEnv, entry)
	}
	if pathEnv := ChildPathEnv(EngineBinDir(), os.Getenv("PATH")); pathEnv != "" {
		childEnv = append(childEnv, pathEnv)
	}

	var tb textbuf.Buffer
	// The OS spelling is derived from the registered key, so the variable the
	// child reads and the key the SDK registered stay one declaration.
	name := strings.ToUpper(strings.ReplaceAll(sdk.EnvPluginMode, ".", "_"))
	return append(childEnv, tb.Str(name).Byte('=').Str(sdk.ModeDeclare).String())
}

// declarationFromStdout finds the plugin's declaration in what it wrote, and
// reports whether one was there.
//
// Everything that is not the declaration is ignored, because a plugin's stdout
// is not a protocol stream: a banner, a log line or a warning can sit before
// the answer and must not corrupt it (R-3). That is also why the lines are
// framed with rpc.ScanLinesKeepingReturns rather than rpc.ScanAnswerLines: the
// second reads a line that opens with an answer kind word by the width its
// fields state, and refuses the whole stream when the arithmetic lands
// somewhere other than a newline. A declaration line holds no raw newline,
// because json.Marshal escapes one, so the newline frames it exactly.
//
// The line is decoded as the daemon decodes it (rpc.DecodeDeclaration), for
// the plugin named name. A line that decodes and carries a key ze does not
// know is a declaration the daemon refuses: it is read, with that refusal as
// refusal and no declaration, so the plugin is never listed as declaring
// what the daemon would not start.
func declarationFromStdout(name string, stdout []byte) (declaration rpc.DeclareRegistrationInput, declared bool, refusal error) {
	lines := bufio.NewScanner(bytes.NewReader(stdout))
	lines.Buffer(make([]byte, 0, initialQueryLineSize), rpc.MaxMessageSize+1)
	lines.Split(rpc.ScanLinesKeepingReturns)
	for lines.Scan() {
		_, method, payload, err := rpc.ParseLine(lines.Bytes())
		if err != nil {
			continue
		}
		if method != rpc.MethodDeclareRegistration {
			continue
		}
		decoded, decodeErr := rpc.DecodeDeclaration(name, payload)
		if decodeErr == nil {
			return decoded, true, nil
		}
		if _, refused := errors.AsType[*opererr.Error](decodeErr); refused {
			return rpc.DeclareRegistrationInput{}, true, decodeErr
		}
	}
	return rpc.DeclareRegistrationInput{}, false, nil
}

// initialQueryLineSize is the buffer a line starts at. A declaration of a
// dozen commands fits inside it, and the scanner grows to rpc.MaxMessageSize
// for a plugin that declares more.
const initialQueryLineSize = 64 * 1024

// boundedBuffer holds what a child wrote, up to limit bytes, and drops the
// rest. A queried plugin is another program, so what it writes is not this
// process's to hold without a bound: one message's worth is the most a
// declaration can be (rpc.MaxMessageSize), so more than that cannot be an
// answer and is never read as one.
//
// Not safe for concurrent use. exec.Cmd writes to it from one goroutine and
// hands it back at Wait.
type boundedBuffer struct {
	held  []byte
	limit int
}

// Write appends what fits and reports every byte as written, so a child that
// writes past the limit is not killed by a short-write error: it is simply not
// read past the limit.
func (b *boundedBuffer) Write(data []byte) (int, error) {
	if room := b.limit - len(b.held); room > 0 {
		b.held = append(b.held, data[:min(room, len(data))]...)
	}
	return len(data), nil
}

// configuredKind answers where a configured plugin's code lives. A compiled-in
// block names a plugin this binary carries, and an external block names a
// program the daemon starts.
func configuredKind(configuredPlugin PluginConfig) string {
	if configuredPlugin.IsCompiled() {
		return kindInternal
	}
	return kindExternal
}

// readConfiguredPlugins asks the registered reader which plugins a config file
// names and which process read it, and refuses when no component registered
// one.
func readConfiguredPlugins(path string) (ConfiguredPlugins, error) {
	configuredPluginsMu.RLock()
	read := configuredPlugins
	configuredPluginsMu.RUnlock()
	if read == nil {
		return ConfiguredPlugins{}, errNoConfigReader
	}
	return read(path)
}

// declarationRefusal is the answer of a declarations handler with nothing to
// answer. The reason travels as the answer's diagnostic, which the local routes
// print on stderr: stdout carries the payload, so a pipe operator reaches the
// answer and never the complaint (docs/architecture/api/commands.md). The
// handler writes nothing itself (command.LocalDataHandler).
func declarationRefusal(err error) command.LocalAnswer {
	return command.LocalRefusal(1, command.LocalErrorLine(err))
}
