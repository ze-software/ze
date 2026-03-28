// Design: docs/architecture/api/process-protocol.md — plugin CLI simulator
// Related: main.go — bgp subcommand dispatch
// Related: ../internal/ssh/client/client.go — SSH credentials from zefs and ExecCommand

package bgp

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
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
`)
}

// cmdPluginCLI runs the interactive plugin CLI.
// Connects to the daemon via SSH and enters interactive command mode
// with plugin SDK method completion.
func cmdPluginCLI(args []string) int {
	fs := flag.NewFlagSet("plugin cli", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin cli [options]

Interactive plugin CLI. Connects to the daemon via SSH and enters
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

	// Load SSH credentials
	creds, err := sshclient.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}

	// Create unified model in command-only mode with plugin SDK method completions
	m := cli.NewCommandModel()
	m.SetCommandCompleter(cli.NewPluginCompleter())
	m.SetCommandExecutor(func(input string) (string, error) {
		return sshclient.ExecCommand(creds, input)
	})

	// Run the bubbletea program
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
