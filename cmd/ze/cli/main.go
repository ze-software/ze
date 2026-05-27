// Design: docs/architecture/core-design.md — interactive CLI
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities (uses BuildCommandTree)
//
// Package cli provides the ze cli subcommand.
package cli

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

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor"           // init() registers monitor streaming RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"              // init() registers peer management RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"               // init() registers raw message RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"               // init() registers RIB proxy RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"            // init() registers update parsing RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler" // init() registers route-refresh command RPCs
	unicli "codeberg.org/thomas-mangin/ze/internal/component/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/archive"   // init() registers config archive trigger RPC
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/bfd"       // init() registers BFD show RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/cache"     // init() registers cache command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/commit"    // init() registers commit command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/del"       // init() registers del verb RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/log"       // init() registers log show/set RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/meta"      // init() registers help/discovery RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics"   // init() registers metrics show/list RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/set"       // init() registers set verb RPCs
	show "codeberg.org/thomas-mangin/ze/internal/component/cmd/show"   // init() registers show verb RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/subscribe" // init() registers subscribe/unsubscribe RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update"    // init() registers update verb RPCs
	cmd "codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface/cmd"  // init() registers interface show/migrate RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all" // init() registers all YANG schemas
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"

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
			`ze cli -c "peer list"            Execute command and exit`,
			`ze cli -c "show version"         One-shot command`,
		},
	}
	p.Write()
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

	m.SetDashboardFactory(func() (func() (string, error), error) {
		return func() (string, error) {
			return dispatch("bgp summary")
		}, nil
	})

	m.SetTracerouteFactory(streamingTracerouteFactory)
	m.SetPingFactory(streamingPingFactory)

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

	m.SetDashboardFactory(func() (func() (string, error), error) {
		return func() (string, error) {
			return client.SendCommand("bgp summary")
		}, nil
	})

	m.SetTracerouteFactory(streamingTracerouteFactory)
	m.SetPingFactory(streamingPingFactory)

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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}

	// Create SSH-based client
	client := newCLIClient(creds)

	// Verify daemon is reachable before entering interactive mode.
	// An "unauthorized" error proves the daemon is running -- only treat
	// connection-level failures as unreachable.
	if _, err := client.SendCommand("show version"); err != nil {
		if !strings.Contains(err.Error(), "unauthorized") {
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
			return client.ExecuteWithTranscript(*runCmd, *format, tw)
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

// ExecuteWithTranscript is like Execute but also records the command and response
// to the given transcript writer. Records the original command (with pipe
// operators) for transcript fidelity.
func (c *cliClient) ExecuteWithTranscript(command, format string, tw *unicli.TranscriptWriter) int {
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
func WireToPath() map[string]string {
	return cliWireToPath
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
	wireValueHints(tree)
	return tree
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
		Commands []struct {
			Value string `json:"value"`
			Help  string `json:"help"`
		} `json:"commands"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
		return commandTree
	}

	available := make(map[string]bool, len(data.Commands))
	descriptions := make(map[string]string, len(data.Commands))
	for _, c := range data.Commands {
		available[strings.ToLower(c.Value)] = true
		if c.Help != "" {
			descriptions[c.Value] = c.Help
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
		Commands []struct {
			Value string `json:"value"`
			Help  string `json:"help"`
		} `json:"commands"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
		return commandTree
	}

	available := make(map[string]bool, len(data.Commands))
	descriptions := make(map[string]string, len(data.Commands))
	for _, c := range data.Commands {
		available[strings.ToLower(c.Value)] = true
		if c.Help != "" {
			descriptions[c.Value] = c.Help
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

	output, err := dispatch("peer * list")
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
				Description: "peer (" + ip + ")",
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

	output, err := client.SendCommand("peer * list")
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
				Description: "peer (" + ip + ")",
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

func streamingTracerouteFactory(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	return show.NewTracerouteSession(ctx, target, maxHops)
}

func streamingPingFactory(ctx context.Context, target string, interval, timeout time.Duration) (<-chan map[string]any, context.CancelFunc, error) {
	return show.NewPingSession(ctx, target, interval, timeout)
}

func parseRemote(s string) (string, string) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return s, "2222"
	}
	return host, port
}
