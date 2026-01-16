package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/thomas-mangin/zebgp/pkg/exabgp"
)

// cmdExabgp handles the "zebgp exabgp" command and its subcommands.
func cmdExabgp(args []string) int {
	if len(args) < 1 {
		exabgpUsage()
		return exitError
	}

	switch args[0] {
	case "plugin":
		return cmdExabgpPlugin(args[1:])
	case "migrate":
		return cmdExabgpMigrate(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		exabgpUsage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown exabgp subcommand: %s\n", args[0])
		exabgpUsage()
		return exitError
	}
}

func exabgpUsage() {
	fmt.Fprintf(os.Stderr, `Usage: zebgp exabgp <subcommand> [options]

ExaBGP compatibility tools.

Subcommands:
  plugin <cmd>     Run ExaBGP plugin with ZeBGP (bidirectional translation)
  migrate <file>   Convert ExaBGP config to ZeBGP format

Examples:
  zebgp exabgp plugin /path/to/exabgp-plugin.py
  zebgp exabgp migrate /path/to/exabgp.conf > zebgp.conf

Use in ZeBGP config:
  plugin exabgp-compat {
      run "zebgp exabgp plugin /path/to/plugin.py";
  }
`)
}

// cmdExabgpPlugin runs an ExaBGP plugin with ZeBGP.
func cmdExabgpPlugin(args []string) int {
	fs := flag.NewFlagSet("exabgp plugin", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp exabgp plugin <cmd> [args...]

Run an ExaBGP plugin with ZeBGP by translating between formats:
- ZeBGP JSON events → ExaBGP JSON format (to plugin stdin)
- ExaBGP text commands → ZeBGP commands (from plugin stdout)

Examples:
  zebgp exabgp plugin /path/to/exabgp-plugin.py
  zebgp exabgp plugin python3 /path/to/plugin.py --arg1 --arg2

Use in ZeBGP config:
  process exabgp-compat {
      run "zebgp exabgp plugin /path/to/plugin.py";
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
	if err := bridge.Run(ctx); err != nil {
		// Don't print error for normal exit
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	return exitOK
}

// cmdExabgpMigrate converts an ExaBGP config to ZeBGP format.
func cmdExabgpMigrate(args []string) int {
	fs := flag.NewFlagSet("exabgp migrate", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp exabgp migrate [options] <config-file>

Convert an ExaBGP configuration file to ZeBGP format.

The migrated config is written to stdout. Redirect to save:
  zebgp exabgp migrate exabgp.conf > zebgp.conf

Options:
  -dry-run    Show what would be done without output

Transformations applied:
  - neighbor → peer
  - process → plugin (wrapped with zebgp exabgp plugin bridge)
  - api { processes [...] } → process NAME { ... } inside peer
  - capability { route-refresh; } → capability { route-refresh enable; }
  - If GR or route-refresh: inject RIB plugin

Example:
  zebgp exabgp migrate /etc/exabgp/exabgp.conf > /etc/zebgp/zebgp.conf
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
