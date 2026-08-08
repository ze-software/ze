// Design: docs/architecture/system-architecture.md -- ze core dispatch and commands
//
// Ze personality: YANG verbs, config file dispatch, global flags, root commands.
// Included only in ze_core builds (ze, ze-appliance).

//go:build ze_core

package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/cmd/ze/hub"
	"github.com/ze-software/ze/cmd/ze/internal/cmdutil"
	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/cmd/ze/internal/suggest"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"

	_ "github.com/ze-software/ze/internal/component/plugin/all"

	_ "github.com/ze-software/ze/internal/component/firewall/cli"
	_ "github.com/ze-software/ze/internal/component/iface/cli"
	_ "github.com/ze-software/ze/internal/component/sysctl/cli"

	_ "github.com/ze-software/ze/internal/component/resolve/cli"

	// Routing-protocol CLI registration is gated per protocol in this same
	// dispatch composition root; see dispatch_isis.go / dispatch_ospf.go
	// (//go:build ze_core && ze_<proto>). With a protocol's tag off, its CLI
	// imports drop from BOTH this root and the generated all.go, so the package
	// unlinks (the two-composition-root reality this spec exists to handle).

	_ "github.com/ze-software/ze/internal/component/config/yang/cli"
	_ "github.com/ze-software/ze/internal/component/plugin/cli"
	_ "github.com/ze-software/ze/internal/component/traffic/cli"

	_ "github.com/ze-software/ze/internal/component/config/cli"
	_ "github.com/ze-software/ze/internal/component/config/schema/cli"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli"

	_ "github.com/ze-software/ze/internal/component/doctor"
	_ "github.com/ze-software/ze/internal/plugins/completion"
	_ "github.com/ze-software/ze/internal/plugins/crashes"
	_ "github.com/ze-software/ze/internal/plugins/debug"
	_ "github.com/ze-software/ze/internal/plugins/diag"
	_ "github.com/ze-software/ze/internal/plugins/explain"
	_ "github.com/ze-software/ze/internal/plugins/host"
	_ "github.com/ze-software/ze/internal/plugins/init"
	_ "github.com/ze-software/ze/internal/plugins/passwd"
	_ "github.com/ze-software/ze/internal/plugins/signal"
	_ "github.com/ze-software/ze/internal/plugins/skills"
	_ "github.com/ze-software/ze/internal/plugins/support"

	_ "github.com/ze-software/ze/internal/component/aaa/all"
)

var (
	errAuthRejected           = errors.New("auth rejected")
	errHubReturnedEmptyConfig = errors.New("hub returned empty config")
)

// cmdHelp is the root "help" command name, used both when normalizing the
// -h/--help flags to the help verb and when registering the handler.
const cmdHelp = "help"

var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.storage.blob", Type: "bool", Default: "true", Description: "Use blob storage (false = filesystem)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.server", Type: "string", Description: "Override hub address (host:port) for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.name", Type: "string", Description: "Override client name for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.token", Type: "string", Description: "Override auth token for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.connect.timeout", Type: "duration", Default: "5s", Description: "Connection timeout for managed hub"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.tls.insecure", Type: "bool", Default: "false", Description: "Skip TLS certificate verification for hub connection (INSECURE)"})
)

// zeGlobalFlags holds values parsed from ze's global flags. Populated by
// zeSetup, read by zeDispatch and newZeRuntimeContext.
type zeGlobalFlags struct {
	plugins      []string
	chaosSeed    int64
	chaosRate    float64
	pprofAddr    string
	fileOverride string
	mcpAddr      string
	mcpToken     string
	webPort      string
	insecureWeb  bool
	webOnly      bool
}

var zeFlags zeGlobalFlags

func init() {
	binarySetup = zeSetup
	binaryDispatch = zeDispatch
	binaryUsage = zeUsage
}

// printVersion writes the version line through a RenderWriter and returns the
// exit code (non-zero on a broken pipe).
func printVersion(extended bool) int {
	rw := helpfmt.NewRenderWriter(os.Stdout)
	if extended {
		rw.Line(zeversion.Extended())
	} else {
		rw.Line(zeversion.Short())
	}
	return rw.ExitCode()
}

func withPanicCapture(fn func() int) (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			crashlog.HandlePanic(r)
			exitCode = 2
		}
	}()
	return fn()
}

func zeSetup(args []string) ([]string, int) {
	if isShellInvocation(binaryName()) {
		return nil, loginMain()
	}

	pluginserver.SetVersion(version, buildDate)
	diagnostic.RegisterBuiltinCodes()
	registerLocalCommands()

	zeFlags.chaosRate = -1
	return zeParseGlobalFlags(args)
}

func zeParseGlobalFlags(args []string) ([]string, int) {
	for len(args) > 0 && (strings.HasPrefix(args[0], "--") || args[0] == "-d" || args[0] == "-V" || args[0] == "-f") {
		switch args[0] {
		case "-f":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: -f requires a file path\n")
				return nil, 1
			}
			zeFlags.fileOverride = args[1]
			args = args[2:]
		case "--server":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --server requires host:port\n")
				return nil, 1
			}
			_ = env.Set("ze.managed.server", args[1])
			args = args[2:]
		case "--name":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --name requires client name\n")
				return nil, 1
			}
			_ = env.Set("ze.managed.name", args[1])
			args = args[2:]
		case "--token":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --token requires auth token\n")
				return nil, 1
			}
			_ = env.Set("ze.managed.token", args[1])
			args = args[2:]
		case "-d", "--debug":
			_ = env.Set("ze.log", "debug")
			_ = env.Set("ze.log.relay", "debug")
			args = args[1:]
		case "--plugin":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --plugin requires an argument\n")
				return nil, 1
			}
			zeFlags.plugins = append(zeFlags.plugins, args[1])
			args = args[2:]
		case "--pprof":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --pprof requires an address (e.g. :6060)\n")
				return nil, 1
			}
			zeFlags.pprofAddr = args[1]
			args = args[2:]
		case "--chaos-seed":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-seed requires an argument\n")
				return nil, 1
			}
			n, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-seed: %v\n", err)
				return nil, 1
			}
			zeFlags.chaosSeed = n
			args = args[2:]
		case "--chaos-rate":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate requires an argument\n")
				return nil, 1
			}
			f, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-rate: %v\n", err)
				return nil, 1
			}
			if f < 0 || f > 1.0 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate must be 0.0-1.0, got %.2f\n", f)
				return nil, 1
			}
			zeFlags.chaosRate = f
			args = args[2:]
		case "--mcp":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				return nil, 1
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[1])
				return nil, 1
			}
			var tb textbuf.Buffer
			zeFlags.mcpAddr = tb.Str("127.0.0.1:").Str(args[1]).String()
			args = args[2:]
		case "--mcp-token":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --mcp-token requires a value\n")
				return nil, 1
			}
			zeFlags.mcpToken = args[1]
			args = args[2:]
		case "--web":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				return nil, 1
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[1])
				return nil, 1
			}
			zeFlags.webPort = args[1]
			args = args[2:]
		case "--insecure-web":
			zeFlags.insecureWeb = true
			args = args[1:]
		case "--web-only":
			zeFlags.webOnly = true
			args = args[1:]
		case "--color":
			_ = env.Set("ze.log.color", "true")
			args = args[1:]
		case "--no-color":
			_ = env.Set("ze.log.color", "false")
			args = args[1:]
		case "--plugins":
			return args, 0
		case "--version", "-V":
			return nil, printVersion(false)
		case "--extended-version":
			return nil, printVersion(true)
		case "--help", "-h": //nolint:goconst // consistent pattern across cmd files
			return args, 0
		default:
			return args, 0
		}
	}
	return args, 0
}

func zeDispatch(args []string) int {
	pprofAddr := zeFlags.pprofAddr
	if pprofAddr == "" {
		pprofAddr = env.Get("ze.pprof")
	}
	if pprofAddr != "" {
		startPprof(pprofAddr)
	}

	if zeFlags.fileOverride != "" {
		store := storage.NewFilesystem()
		zeFlags.fileOverride = config.ResolveConfigPath(zeFlags.fileOverride)
		switch detectConfigType(store, zeFlags.fileOverride) {
		case config.ConfigTypeBGP, config.ConfigTypeHub, config.ConfigTypeUnknown:
			return withPanicCapture(func() int {
				return hub.Run(store, zeFlags.fileOverride, zeFlags.plugins, zeFlags.chaosSeed, zeFlags.chaosRate, false, "", false, "", "")
			})
		}
	}

	if len(args) < 1 {
		if stdinIsTerminal() {
			chosen := runTUILauncher()
			if chosen == "" {
				return 0
			}
			args = strings.Fields(chosen)
		} else {
			zeUsage()
			return 1
		}
	}

	arg := args[0]

	if isYANGVerb(arg) {
		if helpPath := extractHelpPath(args); helpPath != nil {
			yangTree := cli.YANGCommandTree()
			yangNode := command.FindNode(yangTree, helpPath)

			var tb textbuf.Buffer
			tb.Str("ze ").Str(textbuf.Join(helpPath, " "))
			cmdPath := tb.String()

			summary := ""
			var entries []helpfmt.HelpEntry
			if yangNode != nil {
				summary = yangNode.Description
				entries = make([]helpfmt.HelpEntry, 0, len(yangNode.Children))
				for _, e := range command.HelpEntries(yangNode, nil) {
					entries = append(entries, helpfmt.HelpEntry{Name: e.Name, Desc: e.Desc})
				}
			}

			tb.Reset()
			tb.Str(cmdPath).Str(" <command> [options]")

			p := helpfmt.Page{
				Command: cmdPath,
				Summary: summary,
				Usage:   []string{tb.String()},
				Sections: []helpfmt.HelpSection{
					{Title: "Commands", Entries: entries},
				},
			}
			p.WriteErr()
			return 0
		}
		// RunCommand answers -1 only for `ze <verb> <format-keyword>`, where
		// every word after the verb was yaml/json/table and no path is left to
		// resolve. A BARE verb never reaches here: extractHelpPath above returns
		// args unchanged when len(args) == 1, so `ze show` prints the verb help
		// page and exits 0. The tail below is therefore never empty.
		code := cmdutil.RunCommand(args, arg)
		if code == -1 {
			fmt.Fprintf(os.Stderr, "unknown %s command: %s\n", arg, textbuf.Join(args[1:], " "))
			fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", arg)
			return 1
		}
		return code
	}

	switch arg {
	case "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		arg = cmdHelp
	}

	// `ze run` was removed (Phase 6: cmd/ze/run): every verb now dispatches
	// directly. Keep a deprecation stub so callers of the old wrapper get
	// migration hints instead of a bare "unknown command: run".
	if arg == "run" {
		fmt.Fprintf(os.Stderr, "error: 'ze run' has been replaced by direct verb dispatch\n")
		fmt.Fprintf(os.Stderr, "hint: use 'ze show <command>' for read-only commands\n")
		fmt.Fprintf(os.Stderr, "hint: use 'ze set/delete/update <command>' for mutations\n")
		fmt.Fprintf(os.Stderr, "hint: run 'ze help' for available verbs\n")
		return 1
	}

	rctx := newZeRuntimeContext()
	if code, handled := dispatchRegisteredRoot(arg, rctx, args[1:]); handled {
		return code
	}

	webEnabled := zeFlags.webPort != ""
	webListenAddr := ""
	if webEnabled {
		var wb textbuf.Buffer
		webListenAddr = wb.Str("0.0.0.0:").Str(zeFlags.webPort).String()
		if zeFlags.insecureWeb {
			webListenAddr = wb.Reset().Str("127.0.0.1:").Str(zeFlags.webPort).String()
		}
	}
	if zeFlags.insecureWeb && !webEnabled && !zeFlags.webOnly {
		fmt.Fprintf(os.Stderr, "error: --insecure-web requires --web <port> or --web-only\n")
		return 1
	}
	if zeFlags.webOnly && arg == "-" {
		fmt.Fprintf(os.Stderr, "error: --web-only cannot be used with a config file (use 'ze start --web-only' instead)\n")
		return 1
	}

	// `-` is a closed position-1 sentinel: read the config from stdin (the
	// universal Unix convention). It cannot collide with any command name, so it
	// satisfies R1's keywords-before-values invariant (ai/rules/cli.md).
	// A free-form config PATH at position 1 was REMOVED by
	// spec-fixit-config-file-positional-grammar: use `ze start <config-file>`.
	if arg == "-" {
		return withPanicCapture(func() int {
			return hub.Run(resolveStorage(), arg, zeFlags.plugins, zeFlags.chaosSeed, zeFlags.chaosRate, webEnabled, webListenAddr, zeFlags.insecureWeb, zeFlags.mcpAddr, zeFlags.mcpToken)
		})
	}

	// cli.IsDeclaredCommand keeps this root fallback under the same shadow rule
	// as the verb dispatch (registry.LookupLocal): a handler registered at a
	// short path must not answer a ze:command declared below it.
	if handler, remaining := registry.LookupLocal(args, cli.IsDeclaredCommand); handler != nil {
		return handler(remaining)
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	known := knownCommands()
	if suggestion := suggest.Command(arg, known); suggestion != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
	}
	zeUsage()
	return 1
}

func registerLocalCommands() {
	registry.MustRegisterLocalMeta("show version", func(args []string) int {
		return printVersion(slices.Contains(args, "--extended"))
	}, registry.Meta{
		Description: "Show the running Ze version and build date",
		Mode:        "offline",
	})

	registry.MustRegisterRootHandler("start", func(rctx *registry.RuntimeContext, args []string) int {
		if len(args) > 0 && isHelpArg(args[0]) {
			startUsage()
			return 0
		}
		return cmdStart(args, rctx.Plugins, rctx.ChaosSeed, rctx.ChaosRate, rctx.MCPAddr, rctx.MCPToken, rctx.WebPort, rctx.InsecureWeb, rctx.WebOnly)
	}, registry.Meta{
		Description: "Start the Ze daemon from blob storage config",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		Subs:        "--web <port>, --web-only, --insecure-web, --mcp <port>",
	})
	registry.MustRegisterRootHandler("version", func(rctx *registry.RuntimeContext, args []string) int {
		rctx.PrintVersion(slices.Contains(args, "--extended"))
		return 0
	}, registry.Meta{
		Description: "Show the running Ze version and build date",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--extended",
	})
	registry.MustRegisterRootHandler("help", func(_ *registry.RuntimeContext, args []string) int {
		return dispatchHelp(args)
	}, registry.Meta{
		Description: "Show available commands and how to use them",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "command [<filter>] [--json], ai [cli|api|mcp|dispatch|all] [--json]",
	})
	registry.MustRegisterRootHandler("--plugins", func(_ *registry.RuntimeContext, args []string) int {
		printPlugins(len(args) > 0 && args[0] == "--json")
		return 0
	}, registry.Meta{
		Description: "List loaded plugins",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--json",
	})
	registry.MustRegisterRootHandler("pipe", func(_ *registry.RuntimeContext, args []string) int {
		return runPipe(args)
	}, registry.Meta{
		Description: "Apply pipe operators to stdin (format: json/table/yaml; filter: match/count/first/last; resolve)",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "json, table, text, yaml, ndjson, match <pattern>, count, first <n>, last <n>, resolve",
	})
	registry.MustRegisterLocalMeta("help command", printHelpCommand, registry.Meta{
		Description: "List every command with its description. Use a filter to narrow the list.",
		Mode:        "offline",
	})
	registry.MustRegisterLocalMeta("help ai", printAIHelp, registry.Meta{
		Description: "AI reference generated from the binary. Sections: cli, api, mcp, dispatch, all (add --json).",
		Mode:        "offline",
	})
	// `update` is a YANG verb, so a root handler named `update` would be
	// unreachable behind the isYANGVerb branch in zeDispatch. RunCommand consults
	// the local-handler registry before the YANG tree (cmdutil.go), so
	// `update serve` lives here as a local meta -- the same mechanism `show
	// version` uses to run a local command under a YANG verb.
	registry.MustRegisterLocalMeta("update serve", runUpdateServe, registry.Meta{
		Description: "Run a local update server for firmware checks",
		Mode:        "offline",
	})

	registry.SetRuntimeStorage(func() any { return resolveStorage() })
}

func newZeRuntimeContext() *registry.RuntimeContext {
	return &registry.RuntimeContext{
		ResolveStorage: func() any { return resolveStorage() },
		Plugins:        zeFlags.plugins,
		ConfigOverride: zeFlags.fileOverride,
		PrintVersion:   versionPrinter,
		WebPort:        zeFlags.webPort,
		WebOnly:        zeFlags.webOnly,
		InsecureWeb:    zeFlags.insecureWeb,
		MCPAddr:        zeFlags.mcpAddr,
		MCPToken:       zeFlags.mcpToken,
		ChaosSeed:      zeFlags.chaosSeed,
		ChaosRate:      zeFlags.chaosRate,
	}
}

// versionPrinter adapts printVersion (which returns an exit code) to the void
// RuntimeContext.PrintVersion field. The `ze version` root command returns 0
// regardless; the non-zero-exit-on-write-error contract is honored on the
// primary `ze --version` / `ze -V` path.
func versionPrinter(extended bool) { printVersion(extended) }

func dispatchRegisteredRoot(arg string, rctx *registry.RuntimeContext, rest []string) (code int, handled bool) {
	handler := registry.LookupRoot(arg)
	if handler == nil {
		return 0, false
	}
	return handler(rctx, rest), true
}

func knownCommands() []string {
	roots := registry.ListRoot()
	names := make([]string, 0, len(yangVerbs)+len(roots))
	for verb := range yangVerbs {
		names = append(names, verb)
	}
	for _, rc := range roots {
		names = append(names, rc.Name)
	}
	return names
}

var yangVerbs = map[string]bool{
	"show": true, "set": true, "clear": true, "request": true,
	"delete": true, "update": true, "validate": true, "monitor": true,
}

func isYANGVerb(arg string) bool {
	return yangVerbs[arg]
}

func extractHelpPath(args []string) []string {
	if len(args) < 1 {
		return nil
	}
	if len(args) == 1 {
		return args
	}
	last := args[len(args)-1]
	if isHelpArg(last) {
		return args[:len(args)-1]
	}
	return nil
}

func detectConfigType(store storage.Storage, path string) config.ConfigType {
	data, err := store.ReadFile(path)
	if err != nil {
		return config.ConfigTypeUnknown
	}
	return config.ProbeConfigType(string(data))
}

func dispatchHelp(args []string) int {
	switch {
	case len(args) > 0 && args[0] == "command":
		if slices.Contains(args[1:], "--help") || slices.Contains(args[1:], "-h") {
			helpCommandUsage()
			return 0
		}
		return printHelpCommand(args[1:])
	case len(args) > 0 && args[0] == "ai":
		// Canonical form: ze help ai [cli|api|mcp|dispatch|all] [--json].
		return printAIHelp(args[1:])
	case aiHelpRequested(args):
		// Deprecated alias: ze help --ai [--cli|--api|...]. Still accepted.
		return printAIHelp(args)
	case slices.Contains(args, "--help") || slices.Contains(args, "-h"):
		helpUsage()
		return 0
	default:
		zeUsage()
		return 0
	}
}
