// Design: docs/architecture/api/process-protocol.md — plugin debug shell
// Related: main.go — bgp subcommand dispatch
// Related: ../internal/ssh/client/client.go — SSH credentials and protocol sessions

package bgp

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
	fmt.Fprintf(os.Stderr, `ze bgp plugin - Plugin debug shell

Usage:
  ze bgp plugin <command> [options]

Commands:
  cli                  Interactive plugin debug shell (5-stage handshake + commands)
  help                 Show this help

Examples:
  ze bgp plugin cli                          Enter plugin debug shell (defaults)
  ze bgp plugin cli --name my-test           Enter with custom plugin name
`)
}

// cmdPluginCLI runs the plugin debug shell.
// Asks Q&A about handshake parameters (with defaults), connects via SSH,
// runs the 5-stage handshake over the SSH channel, then enters interactive
// command mode for debugging.
func cmdPluginCLI(args []string) int {
	fs := flag.NewFlagSet("plugin cli", flag.ExitOnError)
	name := fs.String("name", "", "Plugin name (default: auto-generated)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin cli [options]

Plugin debug shell. Connects to the daemon via SSH, runs the 5-stage
plugin handshake, then enters interactive command mode.

Hit Enter at each prompt to accept defaults.

Post-handshake commands:
  dispatch-command <command>          Dispatch engine command
  subscribe-events [events...]        Subscribe to events
  unsubscribe-events                 Unsubscribe from events
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

	// Load SSH credentials.
	creds, err := sshclient.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: is the daemon running?\n")
		return 1
	}

	// Shared scanner for all stdin reads (Q&A + interactive).
	// A single scanner avoids losing buffered input when multiple scanners
	// are created over the same reader.
	scanner := bufio.NewScanner(os.Stdin)

	// Q&A phase: ask about handshake parameters on local terminal.
	pluginName := *name
	if pluginName == "" {
		pluginName = promptWithDefault(scanner, "Plugin name", "cli-debug")
	}
	useDefaults := promptYesNo(scanner, "Use default registration?", true)

	var families string
	if !useDefaults {
		families = promptWithDefault(scanner, "Families (comma-separated, e.g., ipv4/unicast)", "")
	}

	fmt.Fprintf(os.Stderr, "\nConnecting to daemon as %q...\n", pluginName)

	// Open persistent SSH session with "plugin protocol" command.
	ps, err := sshclient.OpenProtocolSession(creds, "plugin protocol")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer ps.Close() //nolint:errcheck,gosec // best-effort cleanup

	// Create SDK plugin wrapping the SSH channel.
	// Stdout from SSH is what we read (engine -> plugin).
	// Stdin to SSH is what we write (plugin -> engine).
	p := sdk.NewWithIO(pluginName, io.NopCloser(ps.Stdout), ps.Stdin)
	defer p.Close() //nolint:errcheck,gosec // best-effort cleanup

	// Build registration from Q&A answers.
	reg := sdk.Registration{}
	if families != "" {
		for f := range strings.SplitSeq(families, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				reg.Families = append(reg.Families, sdk.FamilyDecl{Name: f, Mode: "both"})
			}
		}
	}

	// Set up post-handshake callback: start interactive mode.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interactiveDone := make(chan struct{})
	p.OnStarted(func(_ context.Context) error {
		fmt.Fprintf(os.Stderr, "Handshake complete. Interactive mode (type 'bye' to quit).\n\n")
		go func() {
			defer close(interactiveDone)
			runInteractive(ctx, p, scanner)
			cancel() // Signal SDK to shut down when user types 'bye'.
		}()
		return nil
	})

	// Run 5-stage handshake + event loop (blocks until shutdown).
	fmt.Fprintf(os.Stderr, "Running 5-stage handshake...\n")
	if err := p.Run(ctx, reg); err != nil {
		// Context cancellation from interactive bye is expected.
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	<-interactiveDone
	return 0
}

// runInteractive reads commands from the shared scanner and dispatches them via the SDK.
func runInteractive(ctx context.Context, p *sdk.Plugin, scanner *bufio.Scanner) {
	fmt.Fprint(os.Stderr, "> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Fprint(os.Stderr, "> ")
			continue
		}

		if line == "bye" {
			fmt.Fprintln(os.Stderr, "goodbye")
			return
		}

		// Dispatch command via SDK.
		status, data, err := p.DispatchCommand(ctx, line)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		case data != "":
			fmt.Printf("%s: %s\n", status, data)
		default:
			fmt.Println(status)
		}
		fmt.Fprint(os.Stderr, "> ")
	}
}

// promptWithDefault asks a question with a default value.
// Returns the default on empty input. Uses the shared scanner to avoid
// losing buffered input when piped.
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s (default: %s): ", prompt, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}

	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return defaultVal
}

// promptYesNo asks a yes/no question with a default.
// Uses the shared scanner to avoid losing buffered input when piped.
func promptYesNo(scanner *bufio.Scanner, prompt string, defaultYes bool) bool {
	if defaultYes {
		fmt.Fprintf(os.Stderr, "%s (Y/n): ", prompt)
	} else {
		fmt.Fprintf(os.Stderr, "%s (y/N): ", prompt)
	}

	if scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if line == "" {
			return defaultYes
		}
		return line == "y" || line == "yes"
	}
	return defaultYes
}
