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
	"net"
	"os"
	"strings"
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

	// plugin/all is GENERATED (scripts/codegen/plugin_imports.go) and blank-
	// imports every schema, RPC command, and plugin package -- including the
	// verb/cmd packages this file used to enumerate by hand. Never re-add
	// per-package blank imports here; regenerate with `make generate`.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd" // init() registers traceroute RPCs; NewTracerouteSession used below
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/helpfmt"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/zefs"

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
	p := helpfmt.Page{
		Command: "ze cli",
		Summary: "Interactive CLI for Ze daemons",
		Usage:   []string{"ze cli [subsystem] [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subsystems", Entries: []helpfmt.HelpEntry{
				{Name: "bgp", Desc: "BGP daemon (default)"},
			}},
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "-c <command>", Desc: "Execute single command and exit (like ssh -c)"},
			}},
			{Title: "Pipe operators (interactive mode only, Tab completes after |)", Entries: []helpfmt.HelpEntry{
				{Name: "<command> | match <pattern>", Desc: "Filter lines matching pattern"},
				{Name: "<command> | count", Desc: "Count output lines"},
				{Name: "<command> | table", Desc: "Render as nushell-style table"},
				{Name: "<command> | json", Desc: "Pretty-print JSON (default)"},
				{Name: "<command> | json compact", Desc: "Single-line JSON"},
				{Name: "<command> | no-more", Desc: "Disable paging"},
			}},
		},
		Examples: []string{
			"ze cli                           Interactive BGP CLI",
			"ze cli bgp                       Interactive BGP CLI (explicit)",
			`ze cli -c "show bgp peer list"   Execute command and exit`,
			`ze cli -c "show version"         One-shot command`,
		},
	}
	p.WriteErr()
}

// CommandFunc dispatches a CLI command and returns the response.
type CommandFunc func(command string) (string, error)

// RunAttached starts an interactive CLI session using a direct dispatch
// function (no SSH). Called by `ze start --cli` after the daemon is ready.
func RunAttached(dispatch CommandFunc) int {
	return runInteractiveWithDispatch(dispatch)
}

func runInteractiveWithDispatch(dispatch CommandFunc) int {
	m := unicli.NewCommandModel()

	if dbPath := sshclient.ResolveDBPath(); dbPath != "" {
		if store, storeErr := zefs.Open(dbPath); storeErr == nil {
			defer store.Close() //nolint:errcheck // best-effort history
			m.SetHistory(unicli.NewHistory(store, os.Getenv("USER")))
		}
	}

	executor := func(input string) (string, error) {
		return dispatch(input)
	}

	if tf := openTranscriptFile(); tf != nil {
		tw := unicli.NewTranscriptWriter(tf, os.Getenv("USER"), "local")
		defer tw.Close() //nolint:errcheck // best-effort transcript
		executor = unicli.WrapExecutorWithTranscript(executor, tw)
	}

	m.SetCommandExecutor(executor)

	cmdTree := buildRuntimeTreeFromDispatch(dispatch)
	m.SetCommandCompleter(unicli.NewCommandCompleter(cmdTree))

	injectViewFactories(&m, func() (func() (string, error), error) {
		return func() (string, error) {
			return dispatch("show bgp summary")
		}, nil
	})

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
	m := unicli.NewCommandModel()

	if dbPath := sshclient.ResolveDBPath(); dbPath != "" {
		if store, storeErr := zefs.Open(dbPath); storeErr == nil {
			defer store.Close() //nolint:errcheck // best-effort history
			m.SetHistory(unicli.NewHistory(store, os.Getenv("USER")))
		}
	}

	executor := func(input string) (string, error) {
		return client.SendCommand(input)
	}

	if tf := openTranscriptFile(); tf != nil {
		tw := unicli.NewTranscriptWriter(tf, os.Getenv("USER"), client.creds.Host+":"+client.creds.Port)
		defer tw.Close() //nolint:errcheck // best-effort transcript
		executor = unicli.WrapExecutorWithTranscript(executor, tw)
	}

	m.SetCommandExecutor(executor)

	cmdTree := buildRuntimeTree(client)
	m.SetCommandCompleter(unicli.NewCommandCompleter(cmdTree))

	injectViewFactories(&m, func() (func() (string, error), error) {
		return func() (string, error) {
			return client.SendCommand("show bgp summary")
		}, nil
	})

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		if errors.Is(err, tea.ErrProgramPanic) {
			crashlog.HandleCaughtPanic(err)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
	format := fs.String("format", "yaml", "Output format: yaml, json, table")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")
	remote := fs.String("remote", "", "Connect to remote daemon (host:port)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Load SSH credentials to connect to daemon
	var creds sshclient.Credentials
	var err error
	if *remote != "" {
		host, port := parseRemote(*remote)
		creds, err = sshclient.LoadCredentialsForRemote(*user, host, port)
	} else {
		creds, err = sshclient.LoadCredentialsWithFlags(*user)
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
		if tf := openTranscriptFile(); tf != nil {
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
}

func newCLIClient(creds sshclient.Credentials) *cliClient {
	return &cliClient{creds: creds}
}

// Execute sends a command via SSH and prints the response in the given format.
// Valid formats: "yaml" (default), "json", "table".
func (c *cliClient) Execute(command, format string) int {
	cmdStr, formatFn, pipeErr := cmd.ProcessPipesChecked(command)
	if pipeErr != "" {
		fmt.Fprintf(os.Stderr, "pipe error: %s\n", pipeErr)
		return 1
	}
	output, err := c.SendCommand(cmdStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	formatted := formatFn(output)
	printFormatted(formatted, format)
	return 0
}

// executeWithTranscript is like Execute but also records the command and response
// to the given transcript writer. Records the original command (with pipe
// operators) for transcript fidelity.
func (c *cliClient) executeWithTranscript(command, format string, tw *unicli.TranscriptWriter) int {
	cmdStr, formatFn, pipeErr := cmd.ProcessPipesChecked(command)
	if pipeErr != "" {
		fmt.Fprintf(os.Stderr, "pipe error: %s\n", pipeErr)
		return 1
	}
	output, err := c.SendCommand(cmdStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	tw.Record(command, output)

	formatted := formatFn(output)
	printFormatted(formatted, format)
	return 0
}

// SendCommand sends a command to the daemon via SSH exec and returns the response.
func (c *cliClient) SendCommand(command string) (string, error) {
	return sshclient.ExecCommand(c.creds, command)
}

// printFormatted formats and prints output in the given format.
func printFormatted(output, format string) {
	if output == "" {
		fmt.Println("OK")
		return
	}

	switch format {
	case "json":
		fmt.Println(cmd.ApplyJSON(output, "pretty"))
	case "table":
		fmt.Print(cmd.ApplyTable(output))
	default: // yaml
		var data any
		if err := json.Unmarshal([]byte(output), &data); err != nil {
			fmt.Println(output)
			return
		}
		fmt.Print(cmd.RenderYAML(data))
	}
}

// isMonitorCommand returns true if the command is a streaming monitor command.
func isMonitorCommand(command string) bool {
	return pluginserver.IsStreamingCommand(command)
}

// StreamMonitor runs a streaming monitor command, printing each event line.
// Default output is a compact one-liner per event (registered monitor formatter).
// Users can override with explicit pipes: "monitor event | json", "| table", etc.
func (c *cliClient) StreamMonitor(command string) int {
	// Pipe operators are extracted before streaming.
	// Default to the registered compact one-liner formatter instead of table
	// because table produces multi-line output per event, unsuitable for streaming.
	// The formatter is registered by the monitor plugin's init() via pluginserver.
	defaultFmt := pluginserver.MonitorEventFormatter()
	if defaultFmt == nil {
		// Fallback: pass through raw JSON if no formatter registered.
		defaultFmt = func(s string) string { return s }
	}
	cmdStr, formatFn := cmd.ProcessPipesDefaultFunc(command, defaultFmt)

	err := sshclient.StreamCommand(c.creds, cmdStr, func(line string) error {
		// Apply formatting (pipe operators or default text rendering).
		formatted := formatFn(line)
		if formatted != "" {
			fmt.Println(formatted)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// AllCLIRPCs returns all RPCs needed for CLI command mapping.
// All RPCs self-register via init() + pluginserver.RegisterRPCs().
// Exported so other CLI commands (e.g., ze show) can build from the same source.
func AllCLIRPCs() []pluginserver.RPCRegistration {
	return pluginserver.AllBuiltinRPCs()
}

// cliLoader is the shared YANG loader, built once at init.
var cliLoader = func() *yang.Loader {
	loader, err := yang.DefaultLoader()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli: %v\n", err)
	}
	return loader
}()

// cliWireToPath is the YANG-derived WireMethod -> CLI path mapping.
// Built once at package init from the shared DefaultLoader.
var cliWireToPath = yang.WireMethodToPath(cliLoader)

var cliWireToPaths = yang.WireMethodToPaths(cliLoader)

// WireToPath returns the YANG-derived WireMethod to CLI dispatch path mapping.
// Used by help generation to show dispatch keys alongside RPC names.
// Returns the shortest path when multiple aliases exist for a wire method.
func WireToPath() map[string]string {
	return cliWireToPath
}

// WireToPaths returns all CLI paths for each wire method (including aliases).
func WireToPaths() map[string][]string {
	return cliWireToPaths
}

// yangCmdTree is the YANG command tree with descriptions from YANG modules.
// Used for help text generation (verb descriptions come from YANG, not RPC registrations).
var yangCmdTree = yang.BuildCommandTree(cliLoader)

// YANGCommandTree returns the YANG-derived command tree with descriptions.
// The returned tree has verb containers (show, set, del, etc.) at the top level
// with descriptions from YANG modules.
func YANGCommandTree() *Command {
	return yangCmdTree
}

// BuildCommandTree builds the command tree from registered RPCs.
// If readOnly is true, only includes RPCs whose CLI path starts with a read-only verb (for "ze show").
// Descriptions come from the YANG command tree, not from the RPC registration.
func BuildCommandTree(readOnly bool) *Command {
	rpcs := AllCLIRPCs()
	infos := make([]cmd.RPCInfo, 0, len(rpcs))
	for _, reg := range rpcs {
		paths := cliWireToPaths[reg.WireMethod]
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
	// Merge descriptions from the YANG command tree into the RPC-built tree.
	// BuildTree creates nodes without descriptions; YANG modules define them.
	if yangCmdTree != nil {
		mergeDescriptions(tree, yangCmdTree)
		mergeArgDefs(tree, yangCmdTree)
	}
	cmd.MergeYANGNodes(tree, yangCmdTree)
	wireValueHints(tree)
	return tree
}

var (
	pathDescriptions = yang.PathToDescription(cliLoader)
	pathArgDefs      = yang.PathToArgDefs(cliLoader)
)

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

// mergeDescriptions copies Description fields from the YANG tree into dst
// for nodes that exist in both trees but have an empty description in dst.
func mergeDescriptions(dst, src *Command) {
	if dst == nil || src == nil {
		return
	}
	for name, dstChild := range dst.Children {
		srcChild, ok := src.Children[name]
		if !ok {
			continue
		}
		if dstChild.Description == "" && srcChild.Description != "" {
			dstChild.Description = srcChild.Description
		}
		mergeDescriptions(dstChild, srcChild)
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

// applyDescriptions walks the command tree and sets Description on leaf nodes
// whose full command path matches a key in the descriptions map. This propagates
// help text from plugin CommandDecl through the daemon's "system command list".
func applyDescriptions(root *Command, descriptions map[string]string) {
	if root == nil || len(descriptions) == 0 {
		return
	}
	for path, desc := range descriptions {
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
		if node != nil && node.Description == "" {
			node.Description = desc
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

// commandTree holds all available commands for completion (compile-time fallback).
var commandTree = BuildCommandTree(false)

// buildRuntimeTree queries the daemon for available commands and returns a
// command tree filtered to exclude proxy commands whose plugin is not running.
// Falls back to the static commandTree on any error.
func buildRuntimeTree(client *cliClient) *Command {
	// Query daemon for runtime command list
	output, err := client.SendCommand("system command list")
	if err != nil {
		return commandTree
	}

	// Parse response to get available command names and descriptions
	var data struct {
		Commands []commandEntry `json:"commands"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
		return commandTree
	}

	available := make(map[string]bool, len(data.Commands))
	descriptions := make(map[string]string, len(data.Commands))
	hidden := make(map[string]bool)
	for _, c := range data.Commands {
		available[strings.ToLower(c.Value)] = true
		if c.Help != "" {
			descriptions[c.Value] = c.Help
		}
		if c.Hidden {
			hidden[strings.ToLower(c.Value)] = true
		}
	}

	// Filter: include RPCs that are either not proxy commands,
	// or whose underlying plugin command is available at runtime
	var filtered []cmd.RPCInfo
	for _, reg := range AllCLIRPCs() {
		if reg.PluginCommand != "" && !available[strings.ToLower(reg.PluginCommand)] {
			continue // Plugin not running -- skip this proxy command
		}
		paths := cliWireToPaths[reg.WireMethod]
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
	applyDescriptions(tree, descriptions)
	wireValueHints(tree)

	// Inject non-hidden plugin commands into the completion tree.
	// Plugin commands not backed by YANG proxy RPCs are missing from the tree
	// unless we add them here.
	injectPluginCommands(tree, data.Commands, hidden)

	// Attach dynamic peer selector completion to the "peer" node.
	// This allows "peer <TAB>" to suggest peer names and IPs.
	if tree.Children != nil {
		if peerNode, ok := tree.Children["peer"]; ok {
			peerNode.DynamicChildren = func() []cmd.Suggestion {
				return fetchPeerSelectors(client)
			}
		}
	}

	return tree
}

// buildRuntimeTreeFromDispatch is like buildRuntimeTree but uses a direct
// dispatch function instead of an SSH client.
func buildRuntimeTreeFromDispatch(dispatch CommandFunc) *Command {
	output, err := dispatch("system command list")
	if err != nil {
		return commandTree
	}

	var data struct {
		Commands []commandEntry `json:"commands"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
		return commandTree
	}

	available := make(map[string]bool, len(data.Commands))
	descriptions := make(map[string]string, len(data.Commands))
	hidden := make(map[string]bool)
	for _, c := range data.Commands {
		available[strings.ToLower(c.Value)] = true
		if c.Help != "" {
			descriptions[c.Value] = c.Help
		}
		if c.Hidden {
			hidden[strings.ToLower(c.Value)] = true
		}
	}

	var filtered []cmd.RPCInfo
	for _, reg := range AllCLIRPCs() {
		if reg.PluginCommand != "" && !available[strings.ToLower(reg.PluginCommand)] {
			continue
		}
		cliPath := cliWireToPath[reg.WireMethod]
		if cliPath == "" {
			continue
		}
		filtered = append(filtered, cmd.RPCInfo{
			CLICommand: cliPath,
			ReadOnly:   pluginserver.IsReadOnlyPath(cliPath),
		})
	}

	tree := cmd.BuildTree(filtered, false)
	applyDescriptions(tree, descriptions)
	wireValueHints(tree)

	// Inject non-hidden plugin commands into the completion tree.
	injectPluginCommands(tree, data.Commands, hidden)

	if tree.Children != nil {
		if peerNode, ok := tree.Children["peer"]; ok {
			peerNode.DynamicChildren = func() []cmd.Suggestion {
				return fetchPeerSelectorsFromDispatch(dispatch)
			}
		}
	}

	return tree
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
			Type:        "selector",
		})
		if info.Name != "" {
			suggestions = append(suggestions, cmd.Suggestion{
				Text:        info.Name,
				Description: func() string { var tb textbuf.Buffer; return tb.Str("peer (").Str(ip).Byte(')').String() }(),
				Type:        "selector",
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
	output, err := client.SendCommand("show bgp peer list")
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
			Type:        "selector",
		})
		if info.Name != "" {
			suggestions = append(suggestions, cmd.Suggestion{
				Text:        info.Name,
				Description: func() string { var tb textbuf.Buffer; return tb.Str("peer (").Str(ip).Byte(')').String() }(),
				Type:        "selector",
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
