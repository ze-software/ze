// Design: docs/architecture/core-design.md — external format bridge CLI
// Detail: main_sdk.go — SDK/TLS connect-back mode for engine-launched bridge
//
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

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/exabgp/bridge"
	"codeberg.org/thomas-mangin/ze/internal/exabgp/migration"
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
		if s := suggest.Command(args[0], []string{"plugin", "migrate", "help"}); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		usage()
		return exitError
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze exabgp",
		Summary: "ExaBGP compatibility tools",
		Usage:   []string{"ze exabgp <subcommand> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subcommands", Entries: []helpfmt.HelpEntry{
				{Name: "plugin <cmd>", Desc: "Run ExaBGP plugin with ze (bidirectional translation)"},
				{Name: "migrate <file>", Desc: "Convert ExaBGP config to ze format"},
			}},
		},
		Examples: []string{
			"ze exabgp plugin /path/to/exabgp-plugin.py",
			"ze exabgp migrate /path/to/exabgp.conf > ze-bgp.conf",
			`Use in ze config:`,
			`  plugin exabgp-compat {`,
			`      run "ze exabgp plugin /path/to/plugin.py";`,
			`  }`,
		},
	}
	p.Write()
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
		p := helpfmt.Page{
			Command: "ze exabgp plugin",
			Summary: "Run an ExaBGP plugin with ze by translating between formats",
			Usage:   []string{"ze exabgp plugin [flags] <cmd> [args...]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Flags", Entries: []helpfmt.HelpEntry{
					{Name: "--family <family>", Desc: "Address family to support (repeatable, default: ipv4/unicast)"},
					{Name: "--route-refresh", Desc: "Enable route-refresh capability (RFC 2918)"},
					{Name: "--add-path <mode>", Desc: "ADD-PATH mode: receive, send, both (RFC 7911)"},
				}},
			},
			Examples: []string{
				"ze exabgp plugin /path/to/exabgp-plugin.py",
				"ze exabgp plugin --route-refresh /path/to/plugin.py",
				"ze exabgp plugin --family ipv4/unicast --family ipv6/unicast /path/to/plugin.py",
				"ze exabgp plugin --add-path receive python3 /path/to/plugin.py",
				`Use in ze config:`,
				`  plugin exabgp-compat {`,
				`      run "ze exabgp plugin --route-refresh /path/to/plugin.py";`,
				`  }`,
			},
		}
		p.Write()
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
		if err := bridge.ValidateFamily(fam); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	pluginCmd := fs.Args()

	// Determine effective families list.
	effectiveFamilies := []string(families)
	if len(effectiveFamilies) == 0 {
		effectiveFamilies = []string{"ipv4/unicast"}
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// When launched by ze's process manager, ZE_PLUGIN_HUB_TOKEN is set.
	// Use SDK mode (TLS connect-back) instead of stdin/stdout.
	// NOTE: os.Getenv is correct here -- the bridge runs as a subprocess
	// before ze's env registry is initialized.
	if os.Getenv("ZE_PLUGIN_HUB_TOKEN") != "" { //nolint:forbidigo // system env var, not ze env registry
		return runSDKMode(ctx, pluginCmd, effectiveFamilies, *routeRefresh, *addPath)
	}

	// Standalone mode: stdin/stdout with MuxConn framing.
	b := bridge.NewBridge(pluginCmd)
	b.Families = effectiveFamilies
	b.RouteRefresh = *routeRefresh
	b.AddPathMode = *addPath
	if err := b.Run(ctx); err != nil {
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
	envFile := fs.String("env", "", "Migrate ExaBGP INI environment file instead of config")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze exabgp migrate",
			Summary: "Convert an ExaBGP configuration file to ze format",
			Usage: []string{
				"ze exabgp migrate [options] <config-file>",
				"ze exabgp migrate --env <env-file>",
			},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--dry-run", Desc: "Show what would be done without output"},
					{Name: "--env <file>", Desc: "Migrate ExaBGP INI environment file"},
				}},
				{Title: "Transformations applied", Entries: []helpfmt.HelpEntry{
					{Name: "neighbor -> peer", Desc: ""},
					{Name: "process -> plugin", Desc: "Wrapped with ze exabgp plugin bridge"},
					{Name: "api { processes [...] }", Desc: "-> process NAME { ... } inside peer"},
					{Name: "capability { route-refresh; }", Desc: "-> capability { route-refresh enable; }"},
					{Name: "If GR or route-refresh", Desc: "Inject RIB plugin"},
				}},
			},
			Examples: []string{
				"ze exabgp migrate exabgp.conf > ze-bgp.conf",
				"ze exabgp migrate /etc/exabgp/exabgp.conf > /etc/ze/bgp/ze-bgp.conf",
				"ze exabgp migrate --env /etc/exabgp/exabgp.env",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	// Handle --env flag: migrate ExaBGP INI environment file.
	if *envFile != "" {
		return cmdMigrateEnv(*envFile)
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
	tree, err := migration.ParseExaBGPConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing config: %v\n", err)
		return exitError
	}

	// Migrate.
	result, err := migration.MigrateFromExaBGP(tree)
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

	// Output external process info for the wrapper to handle.
	for _, proc := range result.Processes {
		fmt.Fprintf(os.Stderr, "process:%s:%s\n", proc.Name, proc.RunCmd)
	}

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would migrate %s\n", configFile)
		return exitOK
	}

	// Serialize and output.
	output := migration.SerializeTree(result.Tree)
	fmt.Print(output)

	return exitOK
}

// cmdMigrateEnv migrates an ExaBGP INI environment file to Ze config output.
func cmdMigrateEnv(envPath string) int {
	data, err := os.ReadFile(envPath) //nolint:gosec // User-specified env file.
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading env file: %v\n", err)
		return exitError
	}

	entries, err := migration.ParseExaBGPEnv(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing env file: %v\n", err)
		return exitError
	}

	if err := migration.ValidateEnvEntries(entries); err != nil {
		fmt.Fprintf(os.Stderr, "error validating env file: %v\n", err)
		return exitError
	}

	output := migration.MapEnvToZe(entries)
	fmt.Print(output)

	return exitOK
}
