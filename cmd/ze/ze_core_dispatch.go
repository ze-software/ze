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

	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	cli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"

	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	_ "codeberg.org/thomas-mangin/ze/internal/component/firewall/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/cli"

	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/resolve/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/tacacs/cli"

	_ "codeberg.org/thomas-mangin/ze/internal/component/config/yang/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/traffic/cli"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/schema/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/storage/cli"

	_ "codeberg.org/thomas-mangin/ze/internal/component/doctor"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/completion"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/crashes"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/debug"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/diag"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/exabgp"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/explain"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/host"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/init"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/passwd"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/signal"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/skills"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/support"

	_ "codeberg.org/thomas-mangin/ze/internal/component/aaa/all"
)

var (
	errAuthRejected           = errors.New("auth rejected")
	errHubReturnedEmptyConfig = errors.New("hub returned empty config")
)

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

func printVersion(extended bool) {
	if extended {
		fmt.Println(zeversion.Extended())
	} else {
		fmt.Println(zeversion.Short())
	}
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
			printVersion(false)
			return nil, 0
		case "--extended-version":
			printVersion(true)
			return nil, 0
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

			pathStr := textbuf.Join(helpPath, " ")
			fmt.Fprintf(os.Stderr, "Usage: ze %s <command> [options]\n\n", pathStr)
			if yangNode != nil && yangNode.Description != "" {
				var lb textbuf.Buffer
				label := lb.Str(strings.ToUpper(helpPath[len(helpPath)-1][:1])).Str(helpPath[len(helpPath)-1][1:]).String()
				fmt.Fprintf(os.Stderr, "%s (%s).\n\n", label, yangNode.Description)
			}
			fmt.Fprintf(os.Stderr, "Available commands:\n")
			if yangNode != nil && len(yangNode.Children) > 0 {
				command.WriteHelp(os.Stderr, yangNode, nil)
			} else {
				fmt.Fprintf(os.Stderr, "  (no commands registered)\n")
			}
			fmt.Fprintln(os.Stderr)
			return 0
		}
		readOnly := command.IsReadOnlyVerb(arg)
		code := cmdutil.RunCommand(args, readOnly, arg)
		if code == -1 {
			fmt.Fprintf(os.Stderr, "unknown %s command: %s\n", arg, textbuf.Join(args[1:], " "))
			fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", arg)
			return 1
		}
		return code
	}

	switch arg {
	case "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		arg = "help"
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
	if zeFlags.webOnly && looksLikeConfig(arg) {
		fmt.Fprintf(os.Stderr, "error: --web-only cannot be used with a config file (use 'ze start --web-only' instead)\n")
		return 1
	}

	if looksLikeConfig(arg) {
		if arg == "-" {
			return withPanicCapture(func() int {
				return hub.Run(resolveStorage(), arg, zeFlags.plugins, zeFlags.chaosSeed, zeFlags.chaosRate, webEnabled, webListenAddr, zeFlags.insecureWeb, zeFlags.mcpAddr, zeFlags.mcpToken)
			})
		}
		store := resolveStorage()
		arg = config.ResolveConfigPath(arg)
		if storage.IsBlobStorage(store) && !store.Exists(arg) {
			if _, statErr := os.Stat(arg); statErr != nil {
				store.Close() //nolint:errcheck // closing blob before filesystem fallback
				store = storage.NewFilesystem()
			}
		}
		switch detectConfigType(store, arg) {
		case config.ConfigTypeBGP, config.ConfigTypeHub, config.ConfigTypeUnknown:
			return withPanicCapture(func() int {
				return hub.Run(store, arg, zeFlags.plugins, zeFlags.chaosSeed, zeFlags.chaosRate, webEnabled, webListenAddr, zeFlags.insecureWeb, zeFlags.mcpAddr, zeFlags.mcpToken)
			})
		}
	}

	if handler, remaining := registry.LookupLocal(args); handler != nil {
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
		printVersion(slices.Contains(args, "--extended"))
		return 0
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
	registry.MustRegisterRootHandler("update-serve", func(_ *registry.RuntimeContext, args []string) int {
		return runUpdateServe(args)
	}, registry.Meta{
		Description: "Run a local update server for firmware checks",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--listen <addr>",
	})
	registry.MustRegisterRootHandler("help", func(_ *registry.RuntimeContext, args []string) int {
		dispatchHelp(args)
		return 0
	}, registry.Meta{
		Description: "Show available commands and how to use them",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "command [<filter>] [--json], --ai [--cli|--api|--mcp|--dispatch|--all]",
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
	registry.MustRegisterRootHandler("format", func(_ *registry.RuntimeContext, args []string) int {
		return runFormat(args)
	}, registry.Meta{
		Description: "Apply pipe formatting to stdin (json, table, yaml, match, count)",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "json, table, text, yaml, ndjson, match <pattern>, count, first <n>, last <n>, resolve",
	})
	registry.MustRegisterLocalMeta("help command", func(args []string) int {
		printHelpCommand(args)
		return 0
	}, registry.Meta{
		Description: "List every command with its description. Use a filter to narrow the list.",
		Mode:        "offline",
	})

	registry.SetRuntimeStorage(func() any { return resolveStorage() })
}

func newZeRuntimeContext() *registry.RuntimeContext {
	return &registry.RuntimeContext{
		ResolveStorage: func() any { return resolveStorage() },
		Plugins:        zeFlags.plugins,
		ConfigOverride: zeFlags.fileOverride,
		PrintVersion:   printVersion,
		WebPort:        zeFlags.webPort,
		WebOnly:        zeFlags.webOnly,
		InsecureWeb:    zeFlags.insecureWeb,
		MCPAddr:        zeFlags.mcpAddr,
		MCPToken:       zeFlags.mcpToken,
		ChaosSeed:      zeFlags.chaosSeed,
		ChaosRate:      zeFlags.chaosRate,
	}
}

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

func looksLikeConfig(arg string) bool {
	if arg == "-" {
		return true
	}
	if strings.HasSuffix(arg, ".conf") ||
		strings.HasSuffix(arg, ".cfg") ||
		strings.HasSuffix(arg, ".yaml") ||
		strings.HasSuffix(arg, ".yml") ||
		strings.HasSuffix(arg, ".json") {
		return true
	}
	if strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") {
		if _, err := os.Stat(arg); err == nil {
			return true
		}
	}
	return false
}

func detectConfigType(store storage.Storage, path string) config.ConfigType {
	data, err := store.ReadFile(path)
	if err != nil {
		return config.ConfigTypeUnknown
	}
	return config.ProbeConfigType(string(data))
}

func dispatchHelp(args []string) {
	switch {
	case len(args) > 0 && args[0] == "command":
		if slices.Contains(args[1:], "--help") || slices.Contains(args[1:], "-h") {
			helpCommandUsage()
		} else {
			printHelpCommand(args[1:])
		}
	case aiHelpRequested(args):
		printAIHelp(args)
	case slices.Contains(args, "--help") || slices.Contains(args, "-h"):
		helpUsage()
	default:
		zeUsage()
	}
}
