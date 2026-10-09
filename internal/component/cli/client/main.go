// Design: docs/architecture/core-design.md — interactive CLI
// Related: ../../../../cmd/ze/internal/cmdutil/cmdutil.go — shared command utilities (uses BuildCommandTree)
//
// Package cli provides the ze cli subcommand.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"

	unicli "github.com/ze-software/ze/internal/component/cli"
	cmd "github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/yang"
	pingcmd "github.com/ze-software/ze/internal/component/ping/cmd" // init() registers ping RPCs; NewPingSession used below
	"github.com/ze-software/ze/internal/component/plugin"

	// plugin/all is GENERATED (internal/le/plugin/imports/pluginimports.go) and blank-
	// imports every schema, RPC command, and plugin package -- including the
	// verb/cmd packages this file used to enumerate by hand. Never re-add
	// per-package blank imports here; regenerate with `make generate`.
	"github.com/ze-software/ze/internal/component/cli/sshclient"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd" // init() registers traceroute RPCs; NewTracerouteSession used below
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"

	tea "charm.land/bubbletea/v2"
)

// Run executes the cli subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	// Check for help first
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return 0
	}

	// Check for subsystem prefix (e.g., "ze cli bgp ...")
	if len(args) > 0 && args[0] == "bgp" {
		return runBGP(args[1:])
	}

	// Default: BGP subsystem (only one for now)
	return runBGP(args)
}

func usage() {
	formatDescription := formatHelpDescription(
		"the daemon's environment cli format default",
	)
	pipeSections := pipeHelpSections()
	sections := make([]helpfmt.HelpSection, 0, 2+len(pipeSections))
	sections = append(
		sections,
		helpfmt.HelpSection{Title: "Subsystems", Entries: []helpfmt.HelpEntry{
			{Name: "bgp", Desc: "BGP daemon (default)"},
		}},
		helpfmt.HelpSection{Title: "Options", Entries: []helpfmt.HelpEntry{
			{Name: "-c <command>", Desc: "Execute single command and exit (like ssh -c)"},
			{Name: "--format <format>", Desc: formatDescription},
		}},
	)
	sections = append(sections, pipeSections...)
	p := helpfmt.Page{
		Command:   "ze cli",
		ShortHelp: "Interactive CLI for Ze daemons",
		Usage:     []string{"ze cli [subsystem] [options]"},
		Sections:  sections,
		Examples: []string{
			"ze cli                           Interactive BGP CLI",
			"ze cli bgp                       Interactive BGP CLI (explicit)",
			`ze cli -c "show bgp peer list"   Execute command and exit`,
			`ze cli -c "show version"         One-shot command`,
		},
	}
	p.WriteErr()
}

// formatOperatorNames returns the mutually exclusive global renderers in
// catalog order. Other global operators are idempotent or composable.
func formatOperatorNames() []string {
	var names []string
	for _, op := range cmd.PipeOperatorCatalog() {
		if op.Class != cmd.ClassGlobal {
			continue
		}
		if op.Repeat != cmd.RepeatRefuse {
			continue
		}
		names = append(names, op.Name)
	}
	return names
}

func formatHelpDescription(defaultDescription string) string {
	var description textbuf.Buffer
	description.Str("Output format: ").Join(formatOperatorNames(), ", ").
		Str(" (default: ").Str(defaultDescription).Byte(')')
	return description.String()
}

func pipeHelpSections() []helpfmt.HelpSection {
	classes := []struct {
		class cmd.PipeClass
		title string
	}{
		{class: cmd.ClassGlobal, title: "Global pipe operators"},
		{class: cmd.ClassData, title: "Data pipe operators (when the answer has rows)"},
		{class: cmd.ClassStream, title: "Stream pipe operators (when the command keeps answering)"},
	}
	sections := make([]helpfmt.HelpSection, 0, len(classes))
	var label textbuf.Buffer
	for _, group := range classes {
		entries := make([]helpfmt.HelpEntry, 0)
		for _, op := range cmd.PipeOperatorCatalog() {
			if op.Class != group.class {
				continue
			}
			label.Reset(64)
			label.Str("<command> | ").Str(op.Name)
			if hint := op.ArgHint(); hint != "" {
				label.Byte(' ').Str(hint)
			}
			name := label.String()

			label.Reset(64)
			label.Str(op.Description)
			if op.LocalOnly {
				label.Str(" (local process only)")
			}
			entries = append(entries, helpfmt.HelpEntry{Name: name, Desc: label.String()})
		}
		sections = append(sections, helpfmt.HelpSection{Title: group.title, Entries: entries})
	}
	return sections
}

// CommandFunc dispatches a CLI command and carries completion to the UI writer.
type CommandFunc func(command string) (unicli.CommandOutput, error)

// RunAttached starts an interactive CLI session using a direct dispatch
// function (no SSH). Called by `ze start --cli` after the daemon is ready.
// The editor gives the console config mode, reached by typing `configure`.
// A nil editor leaves the command-only console, which refuses `configure`.
// The caller builds the editor, because this package is transport and has no
// config storage of its own.
func RunAttached(dispatch CommandFunc, ed *unicli.Editor) int {
	return runInteractiveWithDispatch(dispatch, ed)
}

// newAttachedModel assembles the model the attached console runs. The executor
// is installed before the opening mode is selected, which is the order
// SetStartMode requires.
//
// The console opens operational whichever model it is, so an editor changes
// what `configure` reaches and nothing else about the first screen.
//
// The error is the YANG loader's refusal of the schema: with no command tree
// there is nothing to complete against, and the caller reports it.
func newAttachedModel(dispatch CommandFunc, executor unicli.CommandExecutor, ed *unicli.Editor) (unicli.Model, error) {
	m := unicli.NewCommandModel(unicli.FilesystemAuthorityOperatorLocal)
	if ed != nil {
		editorModel, err := unicli.NewModel(ed, unicli.FilesystemAuthorityOperatorLocal)
		if err != nil {
			slogutil.Logger("cli.attach").Warn("config editor unavailable", "error", err)
		} else {
			m = editorModel
		}
	}

	m.SetCommandExecutor(executor)
	m.SetStartMode(unicli.ModeOperational)

	cmdTree, err := buildRuntimeTreeFromDispatch(dispatch)
	if err != nil {
		return unicli.Model{}, err
	}
	m.SetCommandCompleter(unicli.NewCommandCompleter(cmdTree))

	injectViewFactories(&m, func() (func() (string, error), error) {
		return func() (string, error) {
			output, err := dispatch("show bgp")
			if output.TransportComplete != nil {
				defer output.TransportComplete()
			}
			return output.Text, err
		}, nil
	})

	return m, nil
}

func runInteractiveWithDispatch(dispatch CommandFunc, ed *unicli.Editor) int {
	history := unicli.NewHistory(statestore.Store(), os.Getenv("USER"))

	executor := unicli.CommandExecutor(dispatch)

	if tf := unicli.OpenTranscriptFile(strconv.Itoa(os.Getpid())); tf != nil {
		tw := unicli.NewTranscriptWriter(tf, os.Getenv("USER"), "local")
		defer tw.Close() //nolint:errcheck // best-effort transcript
		executor = unicli.WrapExecutorWithTranscript(executor, tw)
	}

	m, err := newAttachedModel(dispatch, executor, ed)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // one-shot error to stderr
		return 1
	}
	if history != nil {
		m.SetHistory(history)
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open /dev/tty: %v\n", err)
		return 1
	}
	defer tty.Close() //nolint:errcheck // best-effort cleanup

	// Redirect daemon stdout/stderr to /dev/null while the TUI runs.
	// Without this, slog and fmt.Println writes hit the same terminal
	// as Bubble Tea, injecting bytes mid-frame and corrupting the display.
	restoreOutput := silenceDaemonOutput()

	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(tty))
	_, runErr := p.Run()

	restoreOutput()

	if runErr != nil {
		if errors.Is(runErr, tea.ErrProgramPanic) {
			crashlog.HandleCaughtPanic(runErr)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		return 1
	}

	return 0
}

// silenceDaemonOutput redirects fd 1 (stdout) and fd 2 (stderr) to /dev/null.
// Returns a function that restores the original fds.
// Bubble Tea writes through its own /dev/tty handle, so it is unaffected.
func silenceDaemonOutput() func() {
	savedOut, outErr := syscall.Dup(1)
	savedErr, errErr := syscall.Dup(2)
	if outErr != nil || errErr != nil {
		return func() {}
	}

	devNull, nullErr := syscall.Open(os.DevNull, os.O_WRONLY, 0)
	if nullErr != nil {
		syscall.Close(savedOut) //nolint:errcheck // cleanup
		syscall.Close(savedErr) //nolint:errcheck // cleanup
		return func() {}
	}

	unix.Dup2(devNull, 1)  //nolint:errcheck // best-effort redirect
	unix.Dup2(devNull, 2)  //nolint:errcheck // best-effort redirect
	syscall.Close(devNull) //nolint:errcheck // fd already duped

	return func() {
		unix.Dup2(savedOut, 1)  //nolint:errcheck // restore
		unix.Dup2(savedErr, 2)  //nolint:errcheck // restore
		syscall.Close(savedOut) //nolint:errcheck // cleanup
		syscall.Close(savedErr) //nolint:errcheck // cleanup
	}
}

func runInteractiveSession(client *cliClient) int {
	if err := sshclient.RunInteractive(client.creds, "", true); err != nil {
		fmt.Fprintf(os.Stderr, "error: interactive session: %v\n", err)
		return 1
	}
	return 0
}

// runBGP runs the BGP CLI using the unified cli.Model.
// runOfflineFallback serves a read-only command in-process when the daemon is
// unreachable, if an offline fallback was registered for that command path
// (via registry.RegisterOfflineFallback). Returns the command's exit code and
// true when it handled the command; false lets the caller emit the usual
// "daemon unreachable" error. The fallback registry is separate from the local
// command registry, so a fallback is reached only through this daemon-down path
// and never shadows the daemon command while the daemon is up.
func runOfflineFallback(command string) (int, bool) {
	if command == "" {
		return 0, false
	}
	handler, fallbackArgs := registry.LookupOfflineFallback(strings.Fields(command))
	if handler == nil {
		return 0, false
	}
	return handler(fallbackArgs), true
}

func runBGP(args []string) int {
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	runCmd := fs.String("c", "", "Execute single command and exit")
	// Empty means "no override": the daemon renders in its configured default
	// (environment cli format default). The client cannot resolve that default
	// itself, because nothing on this startup path loads the configuration.
	format := fs.String(
		"format",
		"",
		formatHelpDescription("the daemon's configured format"),
	)
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")
	remote := fs.String("remote", "", "Connect to remote daemon (host:port)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// A command served in THIS process needs no daemon and no credentials, so
	// it is answered before either is asked for. It used to reach the pipe
	// layer on no surface: `ze cli -c "show env list | json"` answered
	// `unknown command`, because YANG declares a wire method for it that no
	// daemon handler implements.
	if *runCmd != "" {
		input := commandWithFormat(*runCmd, *format)
		if answer, code, served := cmd.ServeLocal(input, ""); served {
			return emitLocalResult(input, answer, code, nil)
		}
	}

	// Load SSH credentials to connect to daemon
	var creds sshclient.Credentials
	var err error
	if *remote == "" {
		creds, err = sshclient.LoadCredentialsWithFlags(*user)
	} else {
		host, port := parseRemote(*remote)
		creds, err = sshclient.LoadCredentialsForRemote(*user, host, port)
	}
	if err != nil {
		// No usable credentials means no daemon to talk to. Read-only commands
		// that registered an in-process fallback (show crashes, show host) are
		// served locally so host-local data stays readable with no daemon.
		if code, served := runOfflineFallback(*runCmd); served {
			return code
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}

	// Create SSH-based client
	client := newCLIClient(creds)

	// Verify daemon is reachable before entering interactive mode.
	// An authorization refusal proves the daemon is running -- only treat
	// connection-level failures as unreachable. A profile that denies `show
	// version` must not send the operator to the offline fallback with
	// "is the daemon running?".
	if _, err := client.SendCommand("show version"); err != nil {
		if !strings.Contains(err.Error(), plugin.UnauthorizedMessage) {
			// Daemon unreachable: try an in-process offline fallback before
			// giving up. The fallback registry is consulted ONLY here, after a
			// connection-level failure, so it never shadows the daemon command
			// when the daemon is up.
			if code, served := runOfflineFallback(*runCmd); served {
				return code
			}
			fmt.Fprintf(os.Stderr, "error: cannot connect to daemon: %v\n", err)
			fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
			return 1
		}
	}

	// If -c specified, execute single command and exit.
	if *runCmd != "" {
		// Streaming commands (bgp monitor) use StreamCommand for line-by-line output.
		if isMonitorCommand(*runCmd) {
			return client.StreamMonitor(*runCmd)
		}
		if tf := unicli.OpenTranscriptFile(strconv.Itoa(os.Getpid())); tf != nil {
			tw := unicli.NewTranscriptWriter(tf, os.Getenv("USER"), creds.Host+":"+creds.Port)
			defer tw.Close() //nolint:errcheck // best-effort transcript
			return client.executeWithTranscript(*runCmd, *format, tw)
		}
		return client.Execute(*runCmd, *format)
	}

	return runInteractiveSession(client)
}

// cliClient handles communication with the daemon via SSH exec.
type cliClient struct {
	creds sshclient.Credentials
	// send is the SSH exec transport. It is a field rather than a direct call
	// for one reason: a test reads the exact command string a caller put on
	// the channel, `| raw` included, and opens no connection to do it.
	// Every production client comes from newCLIClient. The zero value carries
	// no transport, so a caller MUST NOT build this struct by hand outside a
	// test.
	send func(sshclient.Credentials, string) (string, error)
	// stream is the SSH exec transport for an answer written to the operator
	// as it arrives. It is a separate field from send because it answers a
	// different question: send returns the whole answer to a caller that
	// parses it, and stream writes the answer to a terminal and returns what
	// the answer turned out to be. A test substitutes either one.
	stream func(sshclient.Credentials, string, io.Writer) (sshclient.Answer, error)
	// monitorStream owns a monitor's callback lifecycle. A test substitutes it
	// to drive event and transport failures without a daemon.
	monitorStream func(sshclient.Credentials, string, func(string) error) error
}

func newCLIClient(creds sshclient.Credentials) *cliClient {
	return &cliClient{
		creds: creds, send: sshclient.ExecCommand, stream: sshclient.ExecCommandStream,
		monitorStream: sshclient.StreamCommand,
	}
}

// Execute sends a command to the daemon and prints the answer.
//
// The client does not format. The daemon holds the configuration. It is
// therefore the only process of the pair that can honor
// `environment cli format default`. It runs the whole pipe chain for the same
// reason, so one implementation renders every surface. This client sends the
// command with its pipes intact. It prints what comes back.
func (c *cliClient) Execute(command, format string) int {
	return c.execute(command, format, nil)
}

// executeWithTranscript is like Execute but also records the command and response
// to the given transcript writer. Records the original command (with pipe
// operators) for transcript fidelity, and the daemon's rendering as the answer,
// which is what the operator saw.
func (c *cliClient) executeWithTranscript(command, format string, tw *unicli.TranscriptWriter) int {
	return c.execute(command, format, tw)
}

// execute runs one command and prints its answer as the daemon writes it.
//
// Nothing is collected on the way through. The daemon streams a long answer
// record by record, and holding it here to print it in one call would spend the
// memory again at the last hop. A transcript is the one caller that needs the
// whole text, so it is the one caller that pays for a copy.
//
// A failure is reported after whatever reached the terminal, because a stream
// cannot be taken back: the operator has already read the rows that arrived,
// and the message says why the rest did not.
func (c *cliClient) execute(command, format string, tw *unicli.TranscriptWriter) int {
	// A command served in THIS process never reaches the daemon, and used to
	// reach no pipe layer either: `ze cli -c "show env list | json"` answered
	// `unknown command`, because YANG declares a wire method that no daemon
	// handler implements. Serving it here runs the operator's chain over its
	// answer, through the same pipe layer that renders a daemon answer.
	if answer, code, served := cmd.ServeLocal(command, format); served {
		return emitLocalResult(command, answer, code, tw)
	}

	var transcript *textbuf.Buffer
	if tw != nil {
		transcript = &textbuf.Buffer{}
	}
	out := newDaemonOutput(os.Stdout, os.Stderr, command, transcript)

	// This emitter has no command of its own: the daemon resolves the operator's
	// text against the registry and answers an unknown one.
	// le-ci-dispatch: dynamic -- command is the operator's own typed input
	_, err := c.stream(c.creds, commandWithFormat(command, format), out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	// The transcript is taken before Close, so it holds the rendering and not
	// the newline that ends the terminal line, nor the OK that stands in for an
	// answer of nothing. That is what a transcript has always recorded.
	if tw != nil {
		tw.Record(command, out.Transcript())
	}
	if closeErr := out.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", closeErr)
		return 1
	}
	// The daemon runs the operator's pipe chain, and a chain the answer's shape
	// cannot support is refused AFTER the command has run: the refusal then
	// arrives as the answer itself. daemonOutput has already sent it to stderr,
	// and the status is what a script reads, so it says refused here. This is
	// the reading emitLocalResult does for a locally served answer.
	if out.Refused() {
		return 1
	}
	return 0
}

// emitLocalResult prints a locally served ANSWER on stdout and answers the exit
// code the handler chose. Transcripts keep the same command and result.
//
// The ANSWER and the CODE are independent (registry.LocalDataHandler): the code
// carries the verdict and the payload carries the evidence, so `validate config`
// answers the diagnostics of a config it rejects AND exits 1. Routing on the
// code sent that payload to stderr, where no pipe operator can reach it, and
// `| json`, `| yaml` and `| table` are three renderings of one payload
// (ai/rules/cli.md). A handler with nothing to say returned a nil payload, which
// arrives here as the empty string: its reason is already on stderr, and
// WriteAnswer prints nothing for it.
func emitLocalResult(input, answer string, code int, tw *unicli.TranscriptWriter) int {
	if tw != nil {
		tw.Record(input, answer)
	}
	// A pipe failure is a diagnostic about the operator's own chain, not an
	// answer to their question, so it is the one local result stdout never sees.
	if cmd.IsPipeError(answer) {
		writeLocalDiagnostic(answer)
		return 1
	}
	cmd.WriteAnswer(answer)
	return code
}

func writeLocalDiagnostic(answer string) {
	if answer == "" {
		return
	}
	os.Stderr.WriteString(answer) //nolint:errcheck // CLI diagnostic
	if strings.HasSuffix(answer, "\n") {
		return
	}
	os.Stderr.WriteString("\n") //nolint:errcheck // CLI diagnostic
}

// commandWithFormat appends the --format flag to the command as a format pipe,
// so the daemon applies it. The flag needs no field on the wire. The pipe
// grammar already names every format. Routing the flag through it keeps one
// implementation deciding the format of every surface.
//
// A format operator the command already names is the operator's own choice,
// and it outranks the flag. `show bgp peer list | json compact` is that shape.
// The flag says what to do when the command asks for nothing, and nothing more.
// Without that precedence a `--format yaml` re-renders the JSON the pipe just
// produced. Every consumer that asked for JSON on the command line then got
// YAML with exit code 0 (plan/journal/silent-fall-through.md, 2026-08-14).
//
// An unknown format reaches the daemon as an unknown pipe operator, which
// ValidatePipes refuses with a message naming it.
func commandWithFormat(command, format string) string {
	if format == "" || cmd.HasFormatPipe(command) {
		return command
	}
	var tb textbuf.Buffer
	return tb.Str(command).Str(" | ").Str(format).String()
}

// SendCommand sends a command to the daemon via SSH exec. It returns the
// answer as an operator would see it. The daemon has already applied the pipe
// chain and the configured default format.
func (c *cliClient) SendCommand(command string) (string, error) {
	return c.send(c.creds, command)
}

// sendCommandRaw sends a command and returns the dispatcher's JSON, whatever
// format the operator configured. Every caller in this package that PARSES the
// answer uses this. See sshclient.ExecCommandRaw for why.
func (c *cliClient) sendCommandRaw(command string) (string, error) {
	return c.send(c.creds, sshclient.RawCommand(command))
}

// isMonitorCommand returns true if the command is a streaming monitor command.
func isMonitorCommand(command string) bool {
	return pluginserver.IsStreamingCommand(command)
}

// StreamMonitor runs a streaming monitor command and prints each event.
func (c *cliClient) StreamMonitor(command string) int {
	defaultFmt := pluginserver.MonitorEventFormatter()
	if defaultFmt == nil {
		defaultFmt = func(s string) string { return s }
	}
	cmdStr, formatFn, saves, pipeErr := cmd.ProcessStreamPipesDefaultFunc(command, defaultFmt)
	if pipeErr != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", pipeErr)
		return 1
	}

	err := c.monitorStream(c.creds, cmdStr, func(line string) error {
		formatted := formatFn(line)
		if formatted == "" {
			return nil
		}
		if _, err := fmt.Fprintln(os.Stdout, formatted); err != nil {
			return err
		}
		if err := saves.WriteString(formatted); err != nil {
			return err
		}
		return saves.WriteString("\n")
	})
	if err != nil {
		cleanupErr := saves.Abort()
		if cleanupErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v (and save cleanup failed: %v)\n", err, cleanupErr)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 1
	}
	if err := saves.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func allCLIRPCs() []pluginserver.RPCRegistration {
	return pluginserver.AllBuiltinRPCs()
}

// yangState is everything this package derives from the YANG command schema.
// It is built once, on first use, by loadYANGState.
type yangState struct {
	// wireToPath maps a WireMethod to its shortest CLI dispatch path.
	wireToPath map[string]string
	// wireToPaths maps a WireMethod to every CLI path, aliases included.
	wireToPaths map[string][]string
	// cmdTree is the YANG command tree, with the verb containers (show, set,
	// del, ...) at the top level and their descriptions from YANG modules.
	cmdTree *Command
	// descriptions maps a CLI path to its YANG description.
	descriptions map[string]string
	// argDefs maps a CLI path to its YANG argument definitions.
	argDefs map[string][]cmd.ArgDef
	// declared is the set of ABSOLUTE CLI paths some registered built-in
	// declares: the population AbsoluteVerbPath scans, keyed for a direct
	// question rather than an inverse one, so the two cannot disagree.
	declared map[string]struct{}
}

// loadYANGState builds the YANG state on first use and caches it with its
// error. Safe for concurrent use.
//
// It is built on first use, never at package init, for two reasons. A schema
// yang.DefaultLoader refuses (an extension typo) answers a nil loader plus an
// error, and building at init handed that nil loader to BuildCommandTree, whose
// panic took down every binary importing this package, ze included. And every
// ze:command registration happens in an init() of the owning package, whose
// order across the binary is undefined, while no dispatch runs before init()
// completes, so first use is always after the last registration.
//
// The error is cached with the state: every accessor returns it, so a caller
// reports the refused schema rather than answering from an empty one. A test
// that registers a refused module resets this variable.
var loadYANGState = sync.OnceValues(buildYANGState)

// buildYANGState loads the YANG schema and derives every map and tree this
// package serves from it.
func buildYANGState() (*yangState, error) {
	resolved, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("cli: YANG command schema: %w", err)
	}
	state := &yangState{
		wireToPath:   yang.WireMethodToPath(resolved),
		wireToPaths:  yang.WireMethodToPaths(resolved),
		cmdTree:      yang.BuildCommandTree(resolved),
		descriptions: yang.PathToDescription(resolved),
		argDefs:      yang.PathToArgDefs(resolved),
		declared:     make(map[string]struct{}),
	}
	for _, reg := range allCLIRPCs() {
		for _, cliPath := range state.wireToPaths[reg.WireMethod] {
			state.declared[cliPath] = struct{}{}
		}
	}
	return state, nil
}

// WireToPath returns the YANG-derived WireMethod to CLI dispatch path mapping.
// Used by help generation to show dispatch keys alongside RPC names.
// Returns the shortest path when multiple aliases exist for a wire method.
// The error is the YANG loader's refusal of the schema. The map is shared:
// the caller MUST NOT modify it.
func WireToPath() (map[string]string, error) {
	state, err := loadYANGState()
	if err != nil {
		return nil, err
	}
	return state.wireToPath, nil
}

// WireToPaths returns all CLI paths for each wire method (including aliases).
// The error is the YANG loader's refusal of the schema. The map is shared:
// the caller MUST NOT modify it.
func WireToPaths() (map[string][]string, error) {
	state, err := loadYANGState()
	if err != nil {
		return nil, err
	}
	return state.wireToPaths, nil
}

// YANGCommandTree returns the YANG-derived command tree with descriptions.
// The returned tree has verb containers (show, set, del, etc.) at the top level
// with descriptions from YANG modules. Used for help text generation (verb
// descriptions come from YANG, not RPC registrations). The error is the YANG
// loader's refusal of the schema. The tree is shared: the caller MUST NOT
// modify it.
func YANGCommandTree() (*Command, error) {
	state, err := loadYANGState()
	if err != nil {
		return nil, err
	}
	return state.cmdTree, nil
}

// BuildCommandTree builds the command tree from registered RPCs.
// If readOnly is true, only includes RPCs whose CLI path starts with a read-only verb (for "ze show").
// Descriptions come from the YANG command tree, not from the RPC registration.
// The error is the YANG loader's refusal of the schema.
func BuildCommandTree(readOnly bool) (*Command, error) {
	state, err := loadYANGState()
	if err != nil {
		return nil, err
	}
	rpcs := allCLIRPCs()
	infos := make([]cmd.RPCInfo, 0, len(rpcs))
	for _, reg := range rpcs {
		paths := state.wireToPaths[reg.WireMethod]
		if len(paths) == 0 {
			continue
		}
		for _, cliPath := range paths {
			isRO := pluginserver.IsReadOnlyPath(cliPath)
			if readOnly && !isRO {
				continue
			}
			infos = append(infos, cmd.RPCInfo{
				CLICommand: cliPath,
				ReadOnly:   isRO,
			})
		}
	}
	tree := cmd.BuildTree(infos, false) // readOnly already filtered above
	// Merge both help texts from the YANG command tree into the RPC-built tree.
	// BuildTree creates nodes with neither. YANG modules declare them.
	mergeHelpText(tree, state.cmdTree)
	mergeArgDefs(tree, state.cmdTree)
	cmd.MergeYANGNodes(tree, state.cmdTree)
	wireValueHints(tree)
	return tree, nil
}

func applyArgDefs(root *Command, defsByPath map[string][]cmd.ArgDef) {
	if root == nil || len(defsByPath) == 0 {
		return
	}
	for path, defs := range defsByPath {
		parts := strings.Fields(path)
		node := root
		for _, part := range parts {
			if node.Children == nil {
				node = nil
				break
			}
			child, ok := node.Children[part]
			if !ok {
				node = nil
				break
			}
			node = child
		}
		if node != nil && len(node.ArgDefs) == 0 {
			node.ArgDefs = defs
		}
	}
}

// mergeHelpText copies both declared help texts from the YANG tree into dst.
// It walks the nodes that exist in both trees. Each field is decided on its
// own, because each is declared on its own. A node that already states a
// summary still takes the explanation it has none of.
func mergeHelpText(dst, src *Command) {
	if dst == nil || src == nil {
		return
	}
	for name, dstChild := range dst.Children {
		srcChild, ok := src.Children[name]
		if !ok {
			continue
		}
		if dstChild.ShortHelp == "" && srcChild.ShortHelp != "" {
			dstChild.ShortHelp = srcChild.ShortHelp
		}
		if dstChild.Description == "" && srcChild.Description != "" {
			dstChild.Description = srcChild.Description
		}
		mergeHelpText(dstChild, srcChild)
	}
}

// mergeArgDefs copies ArgDefs from the YANG tree into dst for nodes that
// exist in both trees but have no ArgDefs in dst.
func mergeArgDefs(dst, src *Command) {
	if dst == nil || src == nil {
		return
	}
	for name, dstChild := range dst.Children {
		srcChild, ok := src.Children[name]
		if !ok {
			continue
		}
		if len(dstChild.ArgDefs) == 0 && len(srcChild.ArgDefs) > 0 {
			dstChild.ArgDefs = srcChild.ArgDefs
		}
		mergeArgDefs(dstChild, srcChild)
	}
}

// nodeAtPath returns the node the space-separated command path names, and nil
// when the tree holds no such path. It is the ONE descent in this package. The
// caller that writes a summary and the caller that writes an explanation can
// never find two different nodes for one path.
func nodeAtPath(root *Command, path string) *Command {
	if root == nil {
		return nil
	}
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return nil // A path that names no command names the root.
	}
	node := root
	for _, part := range parts {
		if node.Children == nil {
			return nil
		}
		child, ok := node.Children[part]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}

// applyDescriptions sets Description on each node whose full command path
// matches a key in the descriptions map. A node that already states a summary
// keeps it.
func applyDescriptions(root *Command, descriptions map[string]string) {
	for path, description := range descriptions {
		node := nodeAtPath(root, path)
		if node != nil && node.ShortHelp == "" {
			node.ShortHelp = description
		}
	}
}

// applyCommandText writes both texts the daemon's "system command list" answer
// carries onto the node each entry names. The summary lands on ShortHelp,
// which every one-line surface reads, and the explanation on Description, which
// the command's help page and the `?` key read. It applies to a builtin and to
// a plugin command alike.
//
// ONE walk carries both, so the node that takes the summary is the node that
// takes the explanation. A field already written is left alone, and a path the
// tree does not hold is skipped: injectPluginCommands creates such a node.
func applyCommandText(root *Command, entries []commandEntry) {
	for _, entry := range entries {
		node := nodeAtPath(root, entry.Value)
		if node == nil {
			continue
		}
		if node.ShortHelp == "" {
			node.ShortHelp = entry.ShortHelp
		}
		if node.Description == "" {
			node.Description = entry.Description
		}
	}
}

// wireValueHints attaches ValueHints callbacks to known nodes in the command tree.
// Both CLI interactive and shell completion get them via shared TreeCompleter.
// wireValueHints delegates to command.WireValueHints (shared with SSH sessions).
func wireValueHints(tree *Command) {
	cmd.WireValueHints(tree)
}

// Command is an alias for command.Node. Use command.Node directly in new code.
type Command = cmd.Node

// buildRuntimeTree queries the daemon for available commands and returns a
// command tree filtered to exclude proxy commands whose plugin is not running.
// Falls back to the static BuildCommandTree(false) when the daemon does not
// answer the list. The error is the YANG loader's refusal of the schema, which
// leaves no tree to fall back to.
func buildRuntimeTree(client *cliClient) (*Command, error) {
	wireToPaths, err := WireToPaths()
	if err != nil {
		return nil, err
	}

	// Query daemon for runtime command list
	output, err := client.sendCommandRaw("system command list")
	if err != nil {
		return BuildCommandTree(false)
	}

	// Parse response to get available command names and descriptions
	commands, err := decodeCommandList([]byte(output))
	if err != nil {
		slogutil.Logger("cli.tree").Warn("system command list refused", "error", err)
		return BuildCommandTree(false)
	}

	available := make(map[string]bool, len(commands))
	hidden := make(map[string]bool)
	for _, c := range commands {
		available[strings.ToLower(c.Value)] = true
		if c.Hidden {
			hidden[strings.ToLower(c.Value)] = true
		}
	}

	// Filter: include RPCs that are either not proxy commands,
	// or whose underlying plugin command is available at runtime
	var filtered []cmd.RPCInfo
	for _, reg := range allCLIRPCs() {
		if reg.PluginCommand != "" && !available[strings.ToLower(reg.PluginCommand)] {
			continue // Plugin not running -- skip this proxy command
		}
		paths := wireToPaths[reg.WireMethod]
		if len(paths) == 0 {
			continue
		}
		for _, cliPath := range paths {
			filtered = append(filtered, cmd.RPCInfo{
				CLICommand: cliPath,
				ReadOnly:   pluginserver.IsReadOnlyPath(cliPath),
			})
		}
	}

	tree := cmd.BuildTree(filtered, false)
	applyCommandText(tree, commands)
	wireValueHints(tree)

	// Inject non-hidden plugin commands into the completion tree.
	// Plugin commands not backed by YANG proxy RPCs are missing from the tree
	// unless we add them here.
	injectPluginCommands(tree, commands, hidden)

	// Attach dynamic peer selector completion to the "peer" node.
	// This allows "peer <TAB>" to suggest peer names and IPs.
	if tree.Children != nil {
		if peerNode, ok := tree.Children["peer"]; ok {
			peerNode.DynamicChildren = func() []cmd.Suggestion {
				return fetchPeerSelectors(client)
			}
		}
	}

	return tree, nil
}

// buildRuntimeTreeFromDispatch is like buildRuntimeTree but uses a direct
// dispatch function instead of an SSH client.
func buildRuntimeTreeFromDispatch(dispatch CommandFunc) (*Command, error) {
	wireToPath, err := WireToPath()
	if err != nil {
		return nil, err
	}

	output, err := dispatch("system command list")
	if err != nil {
		return BuildCommandTree(false)
	}
	if output.TransportComplete != nil {
		defer output.TransportComplete()
	}

	commands, err := decodeCommandList([]byte(output.Text))
	if err != nil {
		slogutil.Logger("cli.tree").Warn("system command list refused", "error", err)
		return BuildCommandTree(false)
	}

	available := make(map[string]bool, len(commands))
	hidden := make(map[string]bool)
	for _, c := range commands {
		available[strings.ToLower(c.Value)] = true
		if c.Hidden {
			hidden[strings.ToLower(c.Value)] = true
		}
	}

	var filtered []cmd.RPCInfo
	for _, reg := range allCLIRPCs() {
		if reg.PluginCommand != "" && !available[strings.ToLower(reg.PluginCommand)] {
			continue
		}
		cliPath := wireToPath[reg.WireMethod]
		if cliPath == "" {
			continue
		}
		filtered = append(filtered, cmd.RPCInfo{
			CLICommand: cliPath,
			ReadOnly:   pluginserver.IsReadOnlyPath(cliPath),
		})
	}

	tree := cmd.BuildTree(filtered, false)
	applyCommandText(tree, commands)
	wireValueHints(tree)

	// Inject non-hidden plugin commands into the completion tree.
	injectPluginCommands(tree, commands, hidden)

	if tree.Children != nil {
		if peerNode, ok := tree.Children["peer"]; ok {
			peerNode.DynamicChildren = func() []cmd.Suggestion {
				return fetchPeerSelectorsFromDispatch(dispatch)
			}
		}
	}

	return tree, nil
}

func fetchPeerSelectorsFromDispatch(dispatch CommandFunc) []cmd.Suggestion {
	if time.Since(peerCache.fetchedAt) < peerSelectorCacheTTL {
		return peerCache.suggestions
	}

	// `show bgp peer list` (ze-bgp:peer-list), NOT `peer * list`: the bare form
	// was removed with the verb-first migration (ze-peer-cmd.yang revision note),
	// and it was a builtin whose only registration was that YANG path, so the
	// removal did not deprecate it -- it made this call return "unknown command".
	// The `*` must be dropped too: this node declares no selector leaf, so a
	// selector token breaks the match. Bare defaults to all peers.
	output, err := dispatch("show bgp peer list")
	if err != nil {
		return nil
	}
	if output.TransportComplete != nil {
		defer output.TransportComplete()
	}

	var data struct {
		Peers map[string]struct {
			Name string `json:"name"`
		} `json:"peers"`
	}
	if json.Unmarshal([]byte(output.Text), &data) != nil {
		return nil
	}

	var suggestions []cmd.Suggestion
	for ip, info := range data.Peers {
		suggestions = append(suggestions, cmd.Suggestion{
			Text:        ip,
			Description: "peer",
			Type:        cmd.SuggestionSelector,
		})
		if info.Name != "" {
			suggestions = append(suggestions, cmd.Suggestion{
				Text:        info.Name,
				Description: func() string { var tb textbuf.Buffer; return tb.Str("peer (").Str(ip).Byte(')').String() }(),
				Type:        cmd.SuggestionSelector,
			})
		}
	}

	peerCache = peerSelectorCache{
		suggestions: suggestions,
		fetchedAt:   time.Now(),
	}

	return suggestions
}

// peerSelectorCache holds cached peer selector suggestions with a TTL.
type peerSelectorCache struct {
	suggestions []cmd.Suggestion
	fetchedAt   time.Time
}

// peerSelectorCacheTTL is how long peer selector suggestions are cached.
// Avoids querying the daemon on every tab press.
const peerSelectorCacheTTL = 3 * time.Second

var peerCache peerSelectorCache

// fetchPeerSelectors queries the daemon for peer names and IPs.
// Results are cached for peerSelectorCacheTTL to avoid per-keystroke queries.
func fetchPeerSelectors(client *cliClient) []cmd.Suggestion {
	if time.Since(peerCache.fetchedAt) < peerSelectorCacheTTL {
		return peerCache.suggestions
	}

	// See fetchPeerSelectorsFromDispatch: `show bgp peer list`, no `*`.
	output, err := client.sendCommandRaw("show bgp peer list")
	if err != nil {
		return nil
	}

	var data struct {
		Peers map[string]struct {
			Name string `json:"name"`
		} `json:"peers"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
		return nil
	}

	var suggestions []cmd.Suggestion
	for ip, info := range data.Peers {
		suggestions = append(suggestions, cmd.Suggestion{
			Text:        ip,
			Description: "peer",
			Type:        cmd.SuggestionSelector,
		})
		if info.Name != "" {
			suggestions = append(suggestions, cmd.Suggestion{
				Text:        info.Name,
				Description: func() string { var tb textbuf.Buffer; return tb.Str("peer (").Str(ip).Byte(')').String() }(),
				Type:        cmd.SuggestionSelector,
			})
		}
	}

	peerCache = peerSelectorCache{
		suggestions: suggestions,
		fetchedAt:   time.Now(),
	}

	return suggestions
}

// injectViewFactories injects each registered live view's concrete factory into
// the model by iterating unicli.RegisteredViews() instead of calling per-view
// typed setters. dashboard is passed in because its poller differs per entry
// point (SSH client vs in-process dispatch); ping/traceroute are package funcs.
func injectViewFactories(m *unicli.Model, dashboard unicli.DashboardFactory) {
	for _, v := range unicli.RegisteredViews() {
		switch v.Key {
		case unicli.ViewKeyDashboard:
			m.SetViewFactory(v.Key, dashboard)
		case unicli.ViewKeyTraceroute:
			m.SetViewFactory(v.Key, unicli.TracerouteFactory(streamingTracerouteFactory))
		case unicli.ViewKeyPing:
			m.SetViewFactory(v.Key, unicli.PingFactory(streamingPingFactory))
		}
	}
}

func streamingTracerouteFactory(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	return traceroutecmd.NewTracerouteSession(ctx, target, maxHops)
}

func streamingPingFactory(ctx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
	return pingcmd.NewPingSession(ctx, target, interval, timeout, count, size)
}

func parseRemote(s string) (string, string) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return s, "2222"
	}
	return host, port
}
