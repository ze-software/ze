// Design: docs/architecture/api/process-protocol.md — plugin debug shell
// Related: main.go — bgp subcommand dispatch
// Related: ../../../core/ssh/client/client.go — SSH credentials and protocol sessions

package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/helpfmt"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/plugin/sdk"
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
	case bgpCmdHelp, "-h", "--help":
		pluginUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin command: %s\n", args[0])
		pluginUsage()
		return 1
	}
}

func pluginUsage() {
	p := helpfmt.Page{
		Command: "ze bgp plugin",
		Summary: "Plugin debug shell",
		Usage:   []string{"ze bgp plugin <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "cli", Desc: "Interactive plugin debug shell (5-stage handshake + commands)"},
				{Name: bgpCmdHelp, Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze bgp plugin cli                          Enter plugin debug shell (defaults)",
			"ze bgp plugin cli --name my-test           Enter with custom plugin name",
		},
	}
	p.WriteErr()
}

// cmdPluginCLI runs the plugin debug shell.
// Asks Q&A about handshake parameters (with defaults), connects via SSH,
// runs the 5-stage handshake over the SSH channel, then enters interactive
// command mode for debugging.
func cmdPluginCLI(args []string) int {
	fs := flag.NewFlagSet("plugin cli", flag.ExitOnError)
	name := fs.String("name", "", "Plugin name (default: auto-generated)")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze bgp plugin cli",
			Summary: "Plugin debug shell. Connects to the daemon via SSH, runs the 5-stage plugin handshake, then enters interactive command mode",
			Usage:   []string{"ze bgp plugin cli [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Post-handshake commands", Entries: []helpfmt.HelpEntry{
					{Name: "dispatch-command <command>", Desc: "Dispatch engine command"},
					{Name: "subscribe-events [events...]", Desc: "Subscribe to events"},
					{Name: "unsubscribe-events", Desc: "Unsubscribe from events"},
					{Name: "decode-nlri <family> <hex>", Desc: "Decode NLRI"},
					{Name: "encode-nlri <family> <args...>", Desc: "Encode NLRI"},
					{Name: "bye", Desc: "Disconnect"},
				}},
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "--name <name>", Desc: "Plugin name (default: auto-generated)"},
				}},
			},
		}
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nHit Enter at each prompt to accept defaults.\n")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Load SSH credentials.
	creds, err := sshclient.LoadCredentialsWithFlags(*user)
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
		answer, perr := promptWithDefault(scanner, "Plugin name", "cli-debug")
		if perr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", perr)
			return 1
		}
		pluginName = answer
	}
	useDefaults, err := promptYesNo(scanner, "Use default registration?", true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	var families string
	if !useDefaults {
		answer, perr := promptWithDefault(scanner, "Families (comma-separated, e.g., ipv4/unicast)", "")
		if perr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", perr)
			return 1
		}
		families = answer
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
			if f == "" {
				continue
			}
			fam, ok := family.LookupFamily(f)
			if !ok {
				fmt.Fprintf(os.Stderr, "error: unknown family: %s\n", f)
				return 1
			}
			reg.Families = append(reg.Families, sdk.FamilyDecl{
				Name: f,
				Mode: "both",
				AFI:  uint16(fam.AFI),
				SAFI: uint8(fam.SAFI),
			})
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
		case len(data) > 0:
			fmt.Fprintf(os.Stdout, "%s: %s\n", status, string(data)) //nolint:errcheck // CLI output
		default:
			fmt.Fprintln(os.Stdout, status) //nolint:errcheck // CLI output
		}
		fmt.Fprint(os.Stderr, "> ")
	}

	// The loop also ends on a broken pipe and on a command line above
	// bufio.MaxScanTokenSize. Neither is the operator typing bye.
	if err := scanErr(scanner); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}

// promptWithDefault asks a question with a default value.
// Returns the default on empty input and on EOF. Uses the shared scanner to
// avoid losing buffered input when piped.
//
// A read failure is an error, never the default: substituting the default for
// an answer the operator did give registers the plugin under settings nobody
// chose. See scanErr for why Err is read back after a successful Scan.
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s (default: %s): ", prompt, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}

	ok := scanner.Scan()
	if err := scanErr(scanner); err != nil {
		return "", err
	}
	if ok {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line, nil
		}
	}
	return defaultVal, nil
}

// scanErr returns the scanner's error, wrapped for an operator.
//
// It is read back even when Scan SUCCEEDED. Scan returns false on EOF, on a
// read error, and on a line above bufio.MaxScanTokenSize alike, and a read that
// fails part way through a line still returns the buffered prefix as a final
// token. So a truncated answer otherwise reads as a whole one.
func scanErr(scanner *bufio.Scanner) error {
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading from stdin: %w", err)
	}
	return nil
}

// promptYesNo asks a yes/no question with a default.
// Uses the shared scanner to avoid losing buffered input when piped.
func promptYesNo(scanner *bufio.Scanner, prompt string, defaultYes bool) (bool, error) {
	if defaultYes {
		fmt.Fprintf(os.Stderr, "%s (Y/n): ", prompt)
	} else {
		fmt.Fprintf(os.Stderr, "%s (y/N): ", prompt)
	}

	ok := scanner.Scan()
	if err := scanErr(scanner); err != nil {
		return false, err
	}
	if ok {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if line == "" {
			return defaultYes, nil
		}
		return line == "y" || line == "yes", nil
	}
	return defaultYes, nil
}
