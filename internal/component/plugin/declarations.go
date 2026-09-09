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
// binary in query mode. Both populations answer here, one row each, and a row
// is never dropped: the state field says why a row carries no declaration.
//
// A row is never dropped and never silently empty. The state says which of
// five answers the row carries: the plugin declared something, it declared
// nothing, it ran and wrote no declaration, it could not be started, or it
// started and said nothing inside the budget.

package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/env"
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

// The five states a row can carry. They are exhaustive: every plugin gets one,
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

// ConfiguredPluginReader answers the plugin blocks a config file declares.
//
// It is a seam rather than a call because the dependency runs the other way:
// `internal/component/config` parses the file and already imports this
// package, for `PluginConfig`, so this package cannot import it back. The
// config component registers its reader from an init().
type ConfiguredPluginReader func(path string) ([]PluginConfig, error)

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
func dataDeclarations(args []string) (any, int) {
	if len(args) > 0 {
		var tb textbuf.Buffer
		writeDeclarationError(errors.New(tb.Str("show plugin declarations takes no value, and ").
			Str(args[0]).Str(" follows it: name a config file with ").Str(keywordConfig).
			Str(" <path>").String()))
		return nil, 1
	}
	return answerDeclarations("")
}

// dataDeclarationsConfig answers `show plugin declarations config <path>`: the
// same rows, plus one for each plugin the named config file declares.
//
// The keyword is a command word rather than a value, so the CLI tree carries it
// and completion offers it (`ai/rules/cli.md`, keyword before value). What
// reaches this handler is the path alone.
func dataDeclarationsConfig(args []string) (any, int) {
	path, err := declarationConfigPath(args)
	if err != nil {
		writeDeclarationError(err)
		return nil, 1
	}
	return answerDeclarations(path)
}

// answerDeclarations builds the payload both handlers answer with, so the two
// commands are two entry points into one answer rather than two answers.
func answerDeclarations(configPath string) (any, int) {
	rows, err := declarationRows(configPath)
	if err != nil {
		writeDeclarationError(err)
		return nil, 1
	}
	return Map{keyDeclarations: rows}, 0
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
// name it gives that binary, so the row is answered by starting it: a block
// that reads `external mrt { run "/opt/vendor/my-mrt" }` is a vendor's program
// and Ze's own mrt declaration would be a wrong answer with the right shape.
func declarationRows(configPath string) ([]declarationRow, error) {
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
		for _, configuredPlugin := range configured {
			// An internal block that names a plugin this binary carries is
			// already answered, from the registration walk above.
			if configuredPlugin.Internal && carried[configuredPlugin.Name] {
				continue
			}
			byName[configuredPlugin.Name] = queriedDeclarationRow(configuredPlugin, budget)
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

// queriedDeclarationRow answers for a plugin whose code is in another binary,
// by starting that binary in query mode and reading its declaration.
//
// A config block that declares the plugin internal, for a plugin this binary
// does not carry, has nothing to start and nothing to read: it names code that
// is in no binary here, which is a refusal rather than a plugin with nothing to
// declare.
func queriedDeclarationRow(configured PluginConfig, budget time.Duration) declarationRow {
	row := declarationRow{Name: configured.Name, Kind: configuredKind(configured)}

	if configured.Internal {
		row.State = stateUnstartable
		row.Reason = "the config declares this plugin internal and this binary does not carry it"
		return row
	}
	if configured.Run == "" {
		row.State = stateUnstartable
		row.Reason = "the config block names no run command, so there is nothing to start"
		return row
	}

	stdout, outcome := runQuery(configured, budget)
	declaration, answered := declarationFromStdout(stdout)
	if answered {
		row.Commands = declaration.Commands
		row.Pipes = declaration.Pipes
		row.State = stateDeclared
		if len(row.Commands) == 0 && len(row.Pipes) == 0 {
			row.State = stateDeclaredNone
			row.Reason = "the plugin answered and declares no command and no pipe"
		}
		return row
	}

	row.State = outcome.state
	row.Reason = outcome.reason
	return row
}

// queryOutcome is what running the child says about the child itself, as
// opposed to what the child wrote. It is only read when no declaration arrived,
// because a plugin that answered has answered whatever its exit status was.
type queryOutcome struct {
	state  string
	reason string
}

// runQuery starts one plugin's run string in query mode and returns whatever it
// wrote to stdout, with the outcome to report if that output holds no
// declaration.
//
// The start mirrors (*Process).startExternal: the run string under a shell,
// because it is shell-quoted and no argv can be appended to it, and the engine
// binary's directory on PATH as a fallback so `run "ze plugin mrt"` finds ze in
// a checkout. It forks here rather than calling that method because
// internal/component/plugin/process imports this package, so a call from here
// is an import cycle; and because that path discards the child's exit status,
// which is the one fact separating a plugin that could not start from a plugin
// that ran and said nothing.
func runQuery(configured PluginConfig, budget time.Duration) ([]byte, queryOutcome) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	// #nosec G204 -- the run string comes from the operator's own config, the
	// same string the daemon starts, and no part of it is caller input.
	child := exec.CommandContext(ctx, Shell, "-c", configured.Run)
	if configured.WorkDir != "" {
		child.Dir = configured.WorkDir
	}
	child.Env = queryChildEnv()

	// The plugin is started in its own process group and the budget's stop is
	// aimed at that group, so the stop reaches the plugin and not only the
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

	if err := child.Start(); err != nil {
		return nil, unstartableOutcome(err, ShellAvailable())
	}

	waitErr := child.Wait()
	if ctx.Err() != nil {
		var tb textbuf.Buffer
		return stdout.held, queryOutcome{
			state: stateTimeout,
			reason: tb.Str("the plugin wrote no declaration within ").Str(budget.String()).
				Str(" and was stopped; ").Str(envQueryTimeout).Str(" sets that budget").String(),
		}
	}
	return stdout.held, queryOutcomeOfExit(waitErr)
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
func queryChildEnv() []string {
	inherited := os.Environ()
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
func declarationFromStdout(stdout []byte) (rpc.DeclareRegistrationInput, bool) {
	var declaration rpc.DeclareRegistrationInput

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
		if json.Unmarshal(payload, &declaration) != nil {
			continue
		}
		return declaration, true
	}
	return declaration, false
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

// configuredKind answers where a configured plugin's code lives. An internal
// block names a plugin this binary carries, and an external block names a
// program the daemon starts.
func configuredKind(configuredPlugin PluginConfig) string {
	if configuredPlugin.Internal {
		return kindInternal
	}
	return kindExternal
}

// readConfiguredPlugins asks the registered reader which plugins a config file
// names, and refuses when no component registered one.
func readConfiguredPlugins(path string) ([]PluginConfig, error) {
	configuredPluginsMu.RLock()
	read := configuredPlugins
	configuredPluginsMu.RUnlock()
	if read == nil {
		return nil, errNoConfigReader
	}
	return read(path)
}

// writeDeclarationError puts the reason on stderr, where a local-data command's
// diagnostics go: stdout carries the payload, so a pipe operator reaches the
// answer and never the complaint (docs/architecture/api/commands.md).
func writeDeclarationError(err error) {
	var tb textbuf.Buffer
	//nolint:errcheck // a failed write to stderr has nowhere left to report it
	tb.Str("error: ").Err(err).Byte('\n').StdErr()
}
