// Design: docs/architecture/core-design.md — interactive CLI
// Related: ../run/main.go — run convenience command (uses BuildCommandTree)
// Related: ../show/main.go — show convenience command (uses BuildCommandTree)
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities (uses BuildCommandTree)
//
// Package cli provides the ze cli subcommand.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/cache"             // init() registers cache command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/commit"            // init() registers commit command RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/meta"              // init() registers help/discovery RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"              // init() registers peer management RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"               // init() registers raw message RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"               // init() registers RIB proxy RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/subscribe"         // init() registers subscribe/unsubscribe RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"            // init() registers update parsing RPCs
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler" // init() registers route-refresh command RPCs
	unicli "codeberg.org/thomas-mangin/ze/internal/component/cli"
	cmd "codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"

	tea "github.com/charmbracelet/bubbletea"
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
	fmt.Fprintf(os.Stderr, `Usage: ze cli [subsystem] [options]

Interactive CLI for Ze daemons.

Subsystems:
  bgp    BGP daemon (default)

Options:
  --socket <path>    Path to API socket (default: auto-detected)
  --run <command>    Execute single command and exit

Pipe operators (interactive mode only, Tab completes after |):
  <command> | match <pattern>    Filter lines matching pattern
  <command> | count              Count output lines
  <command> | table              Render as nushell-style table
  <command> | json               Pretty-print JSON (default)
  <command> | json compact       Single-line JSON
  <command> | no-more            Disable paging

Examples:
  ze cli                           Interactive BGP CLI
  ze cli bgp                       Interactive BGP CLI (explicit)
  ze cli --run "peer list"         Execute command
  ze cli bgp --run "daemon status" Execute command (explicit)
`)
}

// runBGP runs the BGP CLI using the unified cli.Model.
func runBGP(args []string) int {
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	socketPath := fs.String("socket", config.DefaultSocketPath(), "Path to API socket")
	runCmd := fs.String("run", "", "Execute single command and exit")
	format := fs.String("format", "yaml", "Output format: yaml, json, table")
	user := fs.String("user", "", "Username for authorization (simulates authenticated user)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Try to connect to daemon
	client, err := newCLIClient(*socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to %s: %v\n", *socketPath, err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}
	defer func() { _ = client.Close() }()
	client.username = *user

	// If --run specified, execute single command and exit
	if *runCmd != "" {
		return client.Execute(*runCmd, *format)
	}

	// Create unified model in command-only mode
	m := unicli.NewCommandModel()

	// Wire command executor: sends commands to daemon, returns raw JSON.
	// Pipe processing (| table, | json, etc.) is handled by the unified model.
	m.SetCommandExecutor(func(input string) (string, error) {
		resp, err := client.SendCommand(input)
		if err != nil {
			return "", err
		}
		if resp.Error != "" {
			return "", fmt.Errorf("%s", resp.ErrorMessage())
		}
		if len(resp.Result) == 0 || string(resp.Result) == "null" {
			return "OK", nil
		}
		return string(resp.Result), nil
	})

	// Wire command completer from runtime-filtered command tree.
	cmdTree := buildRuntimeTree(client)
	m.SetCommandCompleter(unicli.NewCommandCompleter(cmdTree))

	// Run the bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// rpcResponse covers both success and error JSON RPC wire formats.
type rpcResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
	Params json.RawMessage `json:"params,omitempty"` // Error detail from server (message field)
}

// ErrorMessage returns the human-readable error message.
// Prefers Params.message (structured detail) over the kebab-case Error code.
func (r *rpcResponse) ErrorMessage() string {
	if len(r.Params) > 0 {
		var detail struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(r.Params, &detail) == nil && detail.Message != "" {
			return detail.Message
		}
	}
	return r.Error
}

// cliClient handles communication with the API server using NUL-framed JSON RPC.
type cliClient struct {
	conn     net.Conn
	reader   *rpc.FrameReader
	writer   *rpc.FrameWriter
	cmdMap   map[string]string // lowercase CLI command → wire method
	cmdKeys  []string          // sorted by length descending for longest-match
	username string            // authenticated username for authorization (empty = no auth)
}

func newCLIClient(socketPath string) (*cliClient, error) {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, err
	}

	// Build command map from all registered RPCs (builtins + BGP handlers)
	cmdMap := make(map[string]string)
	for _, reg := range AllCLIRPCs() {
		cmdMap[strings.ToLower(reg.CLICommand)] = reg.WireMethod
	}

	// Sort keys by length descending for longest-match
	keys := make([]string, 0, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	return &cliClient{
		conn:    conn,
		reader:  rpc.NewFrameReader(conn),
		writer:  rpc.NewFrameWriter(conn),
		cmdMap:  cmdMap,
		cmdKeys: keys,
	}, nil
}

func (c *cliClient) Close() error {
	return c.conn.Close()
}

// resolveCommand finds the wire method for a text command.
// Tries "bgp <input>" first (default subsystem), then raw input.
// Returns wire method and any remaining args after the matched prefix.
func (c *cliClient) resolveCommand(input string) (method string, args []string) {
	// Try with "bgp " prefix first (default subsystem)
	if m, a := c.matchCommand("bgp " + input); m != "" {
		return m, a
	}
	// Try raw input for system/rib/plugin commands
	return c.matchCommand(input)
}

// matchCommand does longest-prefix matching against registered CLI commands.
func (c *cliClient) matchCommand(input string) (method string, args []string) {
	lower := strings.ToLower(strings.TrimSpace(input))
	for _, key := range c.cmdKeys {
		if strings.HasPrefix(lower, key) {
			// Check word boundary
			if len(lower) == len(key) || lower[len(key)] == ' ' {
				remaining := strings.TrimSpace(input[len(key):])
				if remaining != "" {
					args = strings.Fields(remaining)
				}
				return c.cmdMap[key], args
			}
		}
	}
	return "", nil
}

// Execute sends a command and prints the response in the given format.
// Valid formats: "yaml" (default), "json", "table".
func (c *cliClient) Execute(command, format string) int {
	resp, err := c.SendCommand(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	c.printFormatted(resp, format)

	if resp.Error != "" {
		return 1
	}
	return 0
}

// SendCommand sends a command as JSON RPC and returns the response.
func (c *cliClient) SendCommand(command string) (*rpcResponse, error) {
	method, args := c.resolveCommand(command)
	if method == "" {
		return &rpcResponse{Error: "unknown command: " + command}, nil
	}

	// Build JSON RPC request.
	// Extract peer selector from args: if the first arg looks like an IP/glob,
	// it's a peer selector that the server expects in params.selector.
	req := rpc.Request{Method: method}
	var selector string
	if len(args) > 0 && looksLikePeerSelector(args[0]) {
		selector = args[0]
		args = args[1:]
	}
	if len(args) > 0 || selector != "" || c.username != "" {
		params := struct {
			Selector string   `json:"selector,omitempty"`
			Args     []string `json:"args,omitempty"`
			Username string   `json:"username,omitempty"`
		}{Selector: selector, Args: args, Username: c.username}
		paramBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = paramBytes
	}

	// Send NUL-framed JSON
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := c.writer.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Read NUL-framed response
	respBytes, err := c.reader.Read()
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	var resp rpcResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &resp, nil
}

// printFormatted formats and prints a response in the given format.
func (c *cliClient) printFormatted(resp *rpcResponse, format string) {
	if resp.Error != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.ErrorMessage())
		return
	}

	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		fmt.Println("OK")
		return
	}

	switch format {
	case "json":
		fmt.Println(cmd.ApplyJSON(string(resp.Result), "pretty"))
	case "table":
		fmt.Print(cmd.ApplyTable(string(resp.Result)))
	default: // yaml
		var data any
		if err := json.Unmarshal(resp.Result, &data); err != nil {
			fmt.Println(string(resp.Result))
			return
		}
		fmt.Print(cmd.RenderYAML(data))
	}
}

// looksLikePeerSelector returns true if the string looks like an IP address or glob.
func looksLikePeerSelector(s string) bool {
	if s == "*" {
		return true
	}
	return strings.ContainsAny(s, ".:")
}

// AllCLIRPCs returns all RPCs needed for CLI command mapping.
// All RPCs self-register via init() + pluginserver.RegisterRPCs().
// Exported so other CLI commands (e.g., ze show) can build from the same source.
func AllCLIRPCs() []pluginserver.RPCRegistration {
	return pluginserver.AllBuiltinRPCs()
}

// BuildCommandTree builds the command tree from registered RPCs.
// Strips the "bgp " prefix for BGP commands to create the user-facing tree.
// If readOnly is true, only includes RPCs marked ReadOnly (for "ze show").
func BuildCommandTree(readOnly bool) *Command {
	rpcs := AllCLIRPCs()
	infos := make([]cmd.RPCInfo, len(rpcs))
	for i, reg := range rpcs {
		infos[i] = cmd.RPCInfo{
			CLICommand: reg.CLICommand,
			Help:       reg.Help,
			ReadOnly:   reg.ReadOnly,
		}
	}
	return cmd.BuildTree(infos, readOnly)
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
	resp, err := client.SendCommand("system command list")
	if err != nil || resp.Error != "" {
		return commandTree
	}

	// Parse response to get available command names
	var data struct {
		Commands []struct {
			Value string `json:"value"`
		} `json:"commands"`
	}
	if json.Unmarshal(resp.Result, &data) != nil {
		return commandTree
	}

	available := make(map[string]bool, len(data.Commands))
	for _, c := range data.Commands {
		available[strings.ToLower(c.Value)] = true
	}

	// Filter: include RPCs that are either not proxy commands,
	// or whose underlying plugin command is available at runtime
	var filtered []cmd.RPCInfo
	for _, reg := range AllCLIRPCs() {
		if reg.PluginCommand != "" && !available[strings.ToLower(reg.PluginCommand)] {
			continue // Plugin not running — skip this proxy command
		}
		filtered = append(filtered, cmd.RPCInfo{
			CLICommand: reg.CLICommand,
			Help:       reg.Help,
			ReadOnly:   reg.ReadOnly,
		})
	}

	return cmd.BuildTree(filtered, false)
}
