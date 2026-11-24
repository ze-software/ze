// Design: docs/architecture/exabgp-bridge.md -- ExaBGP compatibility CLI wrapper.
// Detail: main_sdk.go -- SDK/TLS connect-back and query-mode runner for bus endpoints.
//
// Package exabgp provides the ze exabgp subcommand.
package exabgp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	exitOK    = 0
	exitError = 1
)

// pluginOptions holds the validated CLI flags and script command for
// `ze exabgp plugin`.
type pluginOptions struct {
	Families     []string
	RouteRefresh bool
	AddPath      string
	Encoder      bridge.Encoder
	Forms        bridgerun.FormSpecs
	Cmd          []string
}

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
		Command:   "ze exabgp",
		ShortHelp: "ExaBGP compatibility tools",
		Usage:     []string{"ze exabgp <subcommand> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subcommands", Entries: []helpfmt.HelpEntry{
				{Name: "plugin <cmd>", Desc: "Run ExaBGP script with ze (bidirectional translation)"},
				{Name: "migrate <file>", Desc: "Convert ExaBGP config to ze format"},
			}},
		},
		Examples: []string{
			"ze exabgp plugin /path/to/exabgp-plugin.py",
			"ze exabgp migrate /path/to/exabgp.conf > ze-bgp.conf",
			"Use in ze config:",
			"  bus {",
			"      endpoint exabgp-compat {",
			`          external { run "ze exabgp plugin /path/to/plugin.py"; }`,
			"      }",
			"  }",
		},
	}
	p.WriteErr()
}

// familyList is a custom flag type for repeatable --family flags.
type familyList []string

func (f *familyList) String() string { return textbuf.Join(*f, ",") }
func (f *familyList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// formList is a custom flag type for repeatable --form flags.
type formList []string

func (f *formList) String() string { return textbuf.Join(*f, ",") }
func (f *formList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// parsePluginArgs parses and validates `ze exabgp plugin` flags and script
// argv. When help is requested, it returns (pluginOptions{}, true, nil).
func parsePluginArgs(args []string, errOut io.Writer) (pluginOptions, bool, error) {
	fs := flag.NewFlagSet("exabgp plugin", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var families familyList
	fs.Var(&families, "family", "Address family to support (repeatable, e.g., ipv4/unicast)")
	routeRefresh := fs.Bool("route-refresh", false, "Enable route-refresh capability (RFC 2918)")
	addPath := fs.String("add-path", "", "ADD-PATH mode: receive, send, or both (RFC 7911)")
	encoderName := fs.String("encoder", "text", "Script event encoding: text or json (default: text)")
	var forms formList
	fs.Var(&forms, "form", "Message rendering form modifier <[neighbor:]direction-form> (repeatable)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze exabgp plugin",
			ShortHelp: "Run an ExaBGP script as a ze bus endpoint by translating between formats",
			Usage:     []string{"ze exabgp plugin [flags] <cmd> [args...]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Flags", Entries: []helpfmt.HelpEntry{
					{Name: "--family <family>", Desc: "Address family to support (repeatable, default: ipv4/unicast)"},
					{Name: "--route-refresh", Desc: "Enable route-refresh capability (RFC 2918)"},
					{Name: "--add-path <mode>", Desc: "ADD-PATH mode: receive, send, both (RFC 7911)"},
					{Name: "--encoder <text|json>", Desc: "Event encoding written to the script (default: text)"},
					{Name: "--form <[neighbor:]direction-form>", Desc: "Message form modifier: receive|send + parsed|packets|consolidate (repeatable)"},
				}},
			},
			Examples: []string{
				"ze exabgp plugin /path/to/exabgp-plugin.py",
				"ze exabgp plugin --encoder json --route-refresh /path/to/plugin.py",
				"ze exabgp plugin --family ipv4/unicast --family ipv6/unicast /path/to/plugin.py",
				"ze exabgp plugin --add-path receive python3 /path/to/plugin.py",
				"Use in ze config:",
				"  bus {",
				"      endpoint exabgp-compat {",
				`          external { run "ze exabgp plugin --encoder json --route-refresh /path/to/plugin.py"; }`,
				"      }",
				"  }",
			},
		}
		if errOut == os.Stderr {
			p.WriteErr()
			return
		}
		p.WriteTo(errOut, false)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return pluginOptions{}, true, nil
		}
		return pluginOptions{}, false, err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return pluginOptions{}, false, errors.New("missing plugin command")
	}

	if *addPath != "" {
		switch strings.ToLower(*addPath) {
		case "receive", "send", "both":
		default:
			return pluginOptions{}, false, fmt.Errorf("invalid --add-path mode %q (must be: receive, send, both)", *addPath)
		}
	}

	enc, err := bridge.ParseEncoder(*encoderName)
	if err != nil {
		return pluginOptions{}, false, err
	}

	formSpecs, err := bridgerun.ParseFormSpecs(forms)
	if err != nil {
		return pluginOptions{}, false, err
	}

	for _, fam := range families {
		if err := bridge.ValidateFamily(fam); err != nil {
			return pluginOptions{}, false, err
		}
	}

	effectiveFamilies := []string(families)
	if len(effectiveFamilies) == 0 {
		effectiveFamilies = []string{"ipv4/unicast"}
	}

	return pluginOptions{
		Families:     effectiveFamilies,
		RouteRefresh: *routeRefresh,
		AddPath:      strings.ToLower(*addPath),
		Encoder:      enc,
		Forms:        formSpecs,
		Cmd:          fs.Args(),
	}, false, nil
}

// writeStderrError writes one formatted CLI error line to stderr.
func writeStderrError(prefix string, err error) {
	var tb textbuf.Buffer
	tb.Str("error: ")
	if prefix != "" {
		tb.Str(prefix).Str(": ")
	}
	tb.Err(err).Byte('\n')
	_ = tb.StdErr() //nolint:errcheck // best-effort CLI error output
}

// cmdPlugin runs an ExaBGP script with ze.
func cmdPlugin(args []string) int {
	opts, helped, err := parsePluginArgs(args, os.Stderr)
	if helped {
		return exitOK
	}
	if err != nil {
		writeStderrError("", err)
		return exitError
	}

	// Query mode (`ZE_PLUGIN_MODE=declare`) writes the Stage 1 declaration to
	// stdout and exits 0 without connecting to TLS or starting the script.
	if sdk.QueryModeRequested() {
		return runSDKMode(context.Background(), opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// When launched by ze's process manager, ZE_PLUGIN_HUB_TOKEN is set.
	// Use SDK mode (TLS connect-back) instead of standalone stdin/stdout.
	if env.Get("ze.plugin.hub.token") != "" {
		return runSDKMode(ctx, opts)
	}

	// Standalone mode: stdin/stdout with MuxConn framing.
	b := bridge.NewBridge(opts.Cmd)
	b.Families = opts.Families
	b.RouteRefresh = opts.RouteRefresh
	b.AddPathMode = opts.AddPath
	b.SetEncoder(opts.Encoder)
	if runErr := b.Run(ctx); runErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](runErr); ok {
			return exitErr.ExitCode()
		}
		if ctx.Err() == nil {
			writeStderrError("", runErr)
			return exitError
		}
	}

	return exitOK
}

// cmdMigrate converts an ExaBGP config to ze format.
func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("exabgp migrate", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	envFile := fs.String("env", "", "Migrate ExaBGP INI environment file instead of config")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze exabgp migrate",
			ShortHelp: "Convert an ExaBGP configuration file to ze format",
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
					{Name: "process -> bus endpoint", Desc: "External endpoint running ze exabgp plugin"},
					{Name: "api { processes [...] }", Desc: "-> endpoint receive/send bus filters and --form flags"},
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
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	// Handle --env flag: migrate ExaBGP INI environment file.
	if *envFile != "" {
		if *dryRun {
			fmt.Fprintln(os.Stderr, "warning:", migrationEnvironmentDryRunWarning())
		}
		return cmdMigrateEnv(*envFile)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return exitError
	}

	return printMigrationFile("show exabgp migration config", fs.Arg(0), *dryRun, migrationConfigBytes)
}

// cmdMigrateEnv migrates an ExaBGP INI environment file to Ze config output.
func cmdMigrateEnv(envPath string) int {
	return printMigrationFile("show exabgp migration environment", envPath, false, migrationEnvironmentBytes)
}
