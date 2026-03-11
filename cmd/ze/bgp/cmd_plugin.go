// Design: docs/architecture/api/process-protocol.md — plugin CLI simulator
// Related: main.go — bgp subcommand dispatch

package bgp

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// cmdPlugin dispatches plugin subcommands.
func cmdPlugin(args []string) int {
	if len(args) < 1 {
		pluginUsage()
		return 1
	}

	switch args[0] {
	case "cli":
		return cmdPluginCLI(args[1:])
	case "help", "-h", "--help":
		pluginUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin command: %s\n", args[0])
		pluginUsage()
		return 1
	}
}

func pluginUsage() {
	fmt.Fprintf(os.Stderr, `ze bgp plugin - Interactive plugin simulator

Usage:
  ze bgp plugin <command> [options]

Commands:
  cli                  Interactive plugin CLI (simulates a text-mode plugin)
  help                 Show this help

Examples:
  ze bgp plugin cli                          Enter interactive plugin command mode
  ze bgp plugin cli --socket /tmp/ze.sock    Connect to specific daemon socket
`)
}

// cmdPluginCLI runs the interactive plugin CLI.
// Connects to the daemon API socket and enters interactive command mode
// with plugin SDK method completion. Sends commands as JSON-RPC.
func cmdPluginCLI(args []string) int {
	fs := flag.NewFlagSet("plugin cli", flag.ExitOnError)
	socketPath := fs.String("socket", config.DefaultSocketPath(), "Path to API socket")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin cli [options]

Interactive plugin CLI. Connects to the daemon API socket and enters
command mode with tab completion for plugin SDK methods.

Commands:
  update-route <selector> <command>   Inject route update
  dispatch-command <command>          Dispatch engine command
  subscribe-events [events...]        Subscribe to events
  decode-nlri <family> <hex>          Decode NLRI
  encode-nlri <family> <args...>      Encode NLRI
  bye                                 Disconnect

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Connect to daemon
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", *socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to %s: %v\n", *socketPath, err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup

	reader := rpc.NewFrameReader(conn)
	writer := rpc.NewFrameWriter(conn)

	// Create unified model in command-only mode with plugin SDK method completions
	m := cli.NewCommandModel()
	m.SetCommandCompleter(cli.NewPluginCompleter())
	m.SetCommandExecutor(func(input string) (string, error) {
		// Send as JSON-RPC to daemon (plugin commands are forwarded)
		req := rpc.Request{Method: "ze-plugin-engine:" + input}
		reqBytes, merr := json.Marshal(req)
		if merr != nil {
			return "", fmt.Errorf("marshal: %w", merr)
		}
		if werr := writer.Write(reqBytes); werr != nil {
			return "", fmt.Errorf("send: %w", werr)
		}
		respBytes, rerr := reader.Read()
		if rerr != nil {
			return "", fmt.Errorf("receive: %w", rerr)
		}
		return string(respBytes), nil
	})

	// Run the bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
