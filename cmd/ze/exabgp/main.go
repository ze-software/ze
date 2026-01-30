// Package exabgp provides the ze exabgp subcommand.
package exabgp

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/exabgp"
)

const (
	exitOK    = 0
	exitError = 1
)

// Run executes the exabgp subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return exitError
	}

	switch args[0] {
	case "plugin":
		return cmdPlugin(args[1:])
	case "migrate":
		return cmdMigrate(args[1:])
	case "help", "-h", "--help":
		usage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown exabgp subcommand: %s\n", args[0])
		usage()
		return exitError
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze exabgp <subcommand> [options]

ExaBGP compatibility tools.

Subcommands:
  plugin <cmd>     Run ExaBGP plugin with ze (bidirectional translation)
  migrate <file>   Convert ExaBGP config to ze format

Examples:
  ze exabgp plugin /path/to/exabgp-plugin.py
  ze exabgp migrate /path/to/exabgp.conf > ze-bgp.conf

Use in ze config:
  plugin exabgp-compat {
      run "ze exabgp plugin /path/to/plugin.py";
  }
`)
}

// familyList is a custom flag type for repeatable --family flags.
type familyList []string

func (f *familyList) String() string { return strings.Join(*f, ",") }
func (f *familyList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// cmdPlugin runs an ExaBGP plugin with ze.
func cmdPlugin(args []string) int {
	fs := flag.NewFlagSet("exabgp plugin", flag.ExitOnError)

	var families familyList
	fs.Var(&families, "family", "Address family to support (repeatable, e.g., ipv4/unicast)")
	routeRefresh := fs.Bool("route-refresh", false, "Enable route-refresh capability (RFC 2918)")
	addPath := fs.String("add-path", "", "ADD-PATH mode: receive, send, or both (RFC 7911)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze exabgp plugin [flags] <cmd> [args...]

Run an ExaBGP plugin with ze by translating between formats:
- ze-bgp JSON events -> ExaBGP JSON format (to plugin stdin)
- ExaBGP text commands -> ze commands (from plugin stdout)

Flags:
  --family <family>     Address family to support (repeatable)
                        Default: ipv4/unicast
                        Examples: ipv4/unicast, ipv6/unicast, ipv4/flow
  --route-refresh       Enable route-refresh capability (RFC 2918)
  --add-path <mode>     ADD-PATH mode: receive, send, both (RFC 7911)

Examples:
  ze exabgp plugin /path/to/exabgp-plugin.py
  ze exabgp plugin --route-refresh /path/to/plugin.py
  ze exabgp plugin --family ipv4/unicast --family ipv6/unicast /path/to/plugin.py
  ze exabgp plugin --add-path receive python3 /path/to/plugin.py

Use in ze config:
  plugin exabgp-compat {
      run "ze exabgp plugin --route-refresh /path/to/plugin.py";
  }
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing plugin command\n")
		fs.Usage()
		return exitError
	}

	// Validate add-path mode if specified
	if *addPath != "" {
		switch strings.ToLower(*addPath) {
		case "receive", "send", "both":
			// valid
		default:
			fmt.Fprintf(os.Stderr, "error: invalid --add-path mode %q (must be: receive, send, both)\n", *addPath)
			return exitError
		}
	}

	// Validate families
	for _, fam := range families {
		if err := exabgp.ValidateFamily(fam); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	pluginCmd := fs.Args()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Run the bridge
	bridge := exabgp.NewBridge(pluginCmd)
	if len(families) > 0 {
		bridge.Families = families
	}
	bridge.RouteRefresh = *routeRefresh
	bridge.AddPathMode = *addPath
	if err := bridge.Run(ctx); err != nil {
		// Don't print error for normal exit
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	return exitOK
}

// cmdMigrate converts an ExaBGP config to ze format.
func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("exabgp migrate", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze exabgp migrate [options] <config-file>

Convert an ExaBGP configuration file to ze format.

The migrated config is written to stdout. Redirect to save:
  ze exabgp migrate exabgp.conf > ze-bgp.conf

Options:
  -dry-run    Show what would be done without output

Transformations applied:
  - neighbor -> peer
  - process -> plugin (wrapped with ze exabgp plugin bridge)
  - api { processes [...] } -> process NAME { ... } inside peer
  - capability { route-refresh; } -> capability { route-refresh enable; }
  - If GR or route-refresh: inject RIB plugin

Example:
  ze exabgp migrate /etc/exabgp/exabgp.conf > /etc/ze/bgp/ze-bgp.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return exitError
	}

	configFile := fs.Arg(0)

	// Read config file.
	data, err := os.ReadFile(configFile) //nolint:gosec // User-specified config file.
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		return exitError
	}

	// Parse with ExaBGP schema.
	tree, err := exabgp.ParseExaBGPConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing config: %v\n", err)
		return exitError
	}

	// Migrate.
	result, err := exabgp.MigrateFromExaBGP(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error migrating config: %v\n", err)
		return exitError
	}

	// Print warnings.
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	if result.RIBInjected {
		fmt.Fprintf(os.Stderr, "info: RIB plugin injected (required for GR/route-refresh)\n")
	}

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would migrate %s\n", configFile)
		return exitOK
	}

	// Serialize and output.
	output := exabgp.SerializeTree(result.Tree)
	fmt.Print(output)

	return exitOK
}
