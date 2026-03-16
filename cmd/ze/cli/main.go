// Design: docs/architecture/core-design.md — interactive CLI
// Related: ../run/main.go — run convenience command (uses BuildCommandTree)
// Related: ../show/main.go — show convenience command (uses BuildCommandTree)
// Related: ../internal/cmdutil/cmdutil.go — shared command utilities (uses BuildCommandTree)
//
// Package cli provides the ze cli subcommand.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/sshclient"
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
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

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
	runCmd := fs.String("run", "", "Execute single command and exit")
	format := fs.String("format", "yaml", "Output format: yaml, json, table")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Load SSH credentials to connect to daemon
	creds, err := sshclient.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}

	// Create SSH-based client
	client := newCLIClient(creds)

	// If --run specified, execute single command and exit
	if *runCmd != "" {
		return client.Execute(*runCmd, *format)
	}

	// Create unified model in command-only mode
	m := unicli.NewCommandModel()

	// Wire command executor: sends commands to daemon via SSH, returns response.
	// Pipe processing (| table, | json, etc.) is handled by the unified model.
	m.SetCommandExecutor(func(input string) (string, error) {
		return client.SendCommand(input)
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
	output, err := c.SendCommand(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printFormatted(output, format)
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
	output, err := client.SendCommand("system command list")
	if err != nil {
		return commandTree
	}

	// Parse response to get available command names
	var data struct {
		Commands []struct {
			Value string `json:"value"`
		} `json:"commands"`
	}
	if json.Unmarshal([]byte(output), &data) != nil {
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

	tree := cmd.BuildTree(filtered, false)

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

	output, err := client.SendCommand("bgp peer * list")
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
