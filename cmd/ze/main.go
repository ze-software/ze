// Design: docs/architecture/system-architecture.md — ze main entry point
//
// Package main provides the ze command entry point.
package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	internalresolve "codeberg.org/thomas-mangin/ze/cmd/ze/internal/resolve"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	cli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/managed"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"

	// Import all plugins to trigger init() registration.
	// Must happen at the binary entry point (not in internal/plugin)
	// to avoid import cycles: format → plugin → all → bgp-rs → format.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	// Blank import: the interface command owner registers the `interface` root
	// handler and `show interface` shortcut with the command registry from its
	// init(). The owner lives under internal/component/iface (not cmd/ze) per
	// the command-surface-ownership model. Phase 7 moves this link into the
	// generated command-provider aggregator.
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface/cli"

	// Blank import: the firewall command owner registers the `firewall` root
	// handler with the command registry. Owner lives under
	// internal/component/firewall per command-surface-ownership.
	_ "codeberg.org/thomas-mangin/ze/internal/component/firewall/cli"

	// Blank import: the sysctl command owner registers the `sysctl` root handler
	// with the command registry. Owner lives under internal/plugins/sysctl per
	// command-surface-ownership.
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/cli"

	// Blank imports: the tacacs, resolve, and l2tp command owners register their
	// root handlers with the command registry from init(). Owners live under
	// internal/component/{tacacs,resolve,l2tp} per command-surface-ownership.
	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/resolve/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/tacacs/cli"

	// Blank imports: the traffic-control, plugin, and yang command owners
	// register their root handlers with the command registry from init().
	// Owners live under internal/component/{traffic,plugin,config/yang} per
	// command-surface-ownership.
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/yang/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/traffic/cli"

	// Blank imports: the data (ZeFS) and env command owners register their root
	// handlers with the command registry from init(). Owners live under
	// internal/component/config/storage and internal/core/env per
	// command-surface-ownership.
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/schema/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/config/storage/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/core/env/cli"

	// Blank imports: root command owners register their handlers via
	// init() and are dispatched by dispatchRegisteredRoot.
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/completion"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/crashes"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/debug"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/diag"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/doctor"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/explain"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/host"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/init"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/passwd"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/signal"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/skills"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/support"

	// Import all AAA backends so their init() fires and aaa.Default
	// contains the backend factories before the hub calls aaa.Default.Build.
	_ "codeberg.org/thomas-mangin/ze/internal/component/aaa/all"
)

var (
	errAuthRejected           = errors.New("auth rejected")
	errHubReturnedEmptyConfig = errors.New("hub returned empty config")
)

// Env var registrations for storage and config.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.storage.blob", Type: "bool", Default: "true", Description: "Use blob storage (false = filesystem)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.server", Type: "string", Description: "Override hub address (host:port) for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.name", Type: "string", Description: "Override client name for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.token", Type: "string", Description: "Override auth token for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.connect.timeout", Type: "duration", Default: "5s", Description: "Connection timeout for managed hub"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.tls.insecure", Type: "bool", Default: "false", Description: "Skip TLS certificate verification for hub connection (INSECURE)"})
)

// version and buildDate are set via ldflags at build time.
// Format: -ldflags "-X main.version=YY.MM.DD -X main.buildDate=YYYY-MM-DD".
var (
	version   = "dev"
	buildDate = "unknown"
)

func printVersion(extended bool) {
	if extended {
		fmt.Println(zeversion.Extended())
	} else {
		fmt.Println(zeversion.Short())
	}
}

// registerLocalCommands wires the small set of local commands that
// belong to main itself (not to any subcommand package). Other local
// commands are registered by their owning package's init() via
// registry -- see e.g. cmd/ze/bgp/register.go, cmd/ze/diag/register.go.
// Root commands (`ze bgp`, `ze ping`, ...) are also registered by
// their package's init() for the same reason; main.go's dispatch
// switch stays, but help enumeration is driven by the registry.
func exit(code int) {
	crashlog.Flush()
	os.Exit(code)
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

// Storage-dependent config subcommands are bound here via
// zeconfig.BindStorageCommands because the blob store is opened only
// after global flag parsing.
func registerLocalCommands() {
	// Commands specific to cmd/ze/main (no subcommand package home).
	registry.MustRegisterLocalMeta("show version", func(args []string) int {
		printVersion(slices.Contains(args, "--extended"))
		return 0
	}, registry.Meta{
		Description: "Show the running Ze version and build date",
		Mode:        "offline",
	})

	// Root commands that live in main() itself (not a package).
	registry.MustRegisterRootHandler("start", func(rctx *registry.RuntimeContext, args []string) int {
		if len(args) > 0 && isHelpArg(args[0]) {
			startUsage()
			return 0
		}
		return cmdStart(args, rctx.Plugins, rctx.ChaosSeed, rctx.ChaosRate, rctx.MCPAddr, rctx.MCPToken, rctx.WebPort, rctx.InsecureWeb)
	}, registry.Meta{
		Description: "Start the Ze daemon from blob storage config",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		Subs:        "--web <port>, --insecure-web, --mcp <port>",
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
	registry.MustRegisterLocalMeta("help command", func(args []string) int {
		printHelpCommand(args)
		return 0
	}, registry.Meta{
		Description: "List every command with its description. Use a filter to narrow the list.",
		Mode:        "offline",
	})

	// Install the process storage resolver so storage-backed local command
	// handlers (e.g. the config owner's `show config history/ls/cat`) can open
	// the blob store lazily at dispatch time. The blob store is opened only
	// after global flag parsing, so handlers must resolve it on demand.
	registry.SetRuntimeStorage(func() any { return resolveStorage() })
}

// newRuntimeContext assembles the process-entry dependencies passed to
// owner-backed root handlers. Storage is resolved lazily so that registering
// and dispatching commands which never touch storage do not open the blob
// store. The returned context is leaf-safe: the registry package does not
// import storage, so owners type-assert ResolveStorage's result (see
// registry.StorageAs).
func newRuntimeContext(plugins []string, configOverride, webPort string, insecureWeb bool, mcpAddr, mcpToken string, chaosSeed int64, chaosRate float64) *registry.RuntimeContext {
	return &registry.RuntimeContext{
		ResolveStorage: func() any { return resolveStorage() },
		Plugins:        plugins,
		ConfigOverride: configOverride,
		PrintVersion:   printVersion,
		WebPort:        webPort,
		InsecureWeb:    insecureWeb,
		MCPAddr:        mcpAddr,
		MCPToken:       mcpToken,
		ChaosSeed:      chaosSeed,
		ChaosRate:      chaosRate,
	}
}

// dispatchRegisteredRoot runs the owner-backed root handler registered for arg,
// if any. It returns the handler's exit code and handled=true when the registry
// owns the command; (0, false) means no owner registered arg and the caller
// must fall through to the legacy static switch.
func dispatchRegisteredRoot(arg string, rctx *registry.RuntimeContext, rest []string) (code int, handled bool) {
	handler := registry.LookupRoot(arg)
	if handler == nil {
		return 0, false
	}
	return handler(rctx, rest), true
}

func main() {
	crashlog.Init()

	zeversion.Stamp(version, buildDate)
	pluginserver.SetVersion(version, buildDate)
	diagnostic.RegisterBuiltinCodes()
	registerLocalCommands()

	if len(os.Args) < 2 {
		usage()
		exit(1)
	}

	// Parse global flags before command dispatch
	var plugins []string
	var chaosSeed int64
	var chaosRate float64 = -1 // -1 means "not set by CLI"
	var pprofAddr string
	var fileOverride string // -f flag: bypass blob, use filesystem directly
	var mcpAddr string      // --mcp <port>: start MCP server on 127.0.0.1:<port>
	var mcpToken string     // --mcp-token <token>: bearer token for MCP auth
	var webPort string      // --web <port>: start web server
	var insecureWeb bool
	args := os.Args[1:]
	for len(args) > 0 && (strings.HasPrefix(args[0], "--") || args[0] == "-d" || args[0] == "-V" || args[0] == "-f") {
		switch args[0] {
		case "-f":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: -f requires a file path\n")
				exit(1)
			}
			fileOverride = args[1]
			args = args[2:]
		case "--server":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --server requires host:port\n")
				exit(1)
			}
			_ = env.Set("ze.managed.server", args[1])
			args = args[2:]
		case "--name":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --name requires client name\n")
				exit(1)
			}
			_ = env.Set("ze.managed.name", args[1])
			args = args[2:]
		case "--token":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --token requires auth token\n")
				exit(1)
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
				exit(1)
			}
			plugins = append(plugins, args[1])
			args = args[2:]
		case "--pprof":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --pprof requires an address (e.g. :6060)\n")
				exit(1)
			}
			pprofAddr = args[1]
			args = args[2:]
		case "--chaos-seed":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-seed requires an argument\n")
				exit(1)
			}
			n, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-seed: %v\n", err)
				exit(1)
			}
			chaosSeed = n
			args = args[2:]
		case "--chaos-rate":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate requires an argument\n")
				exit(1)
			}
			f, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-rate: %v\n", err)
				exit(1)
			}
			if f < 0 || f > 1.0 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate must be 0.0-1.0, got %.2f\n", f)
				exit(1)
			}
			chaosRate = f
			args = args[2:]
		case "--mcp":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				exit(1)
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[1])
				exit(1)
			}
			mcpAddr = "127.0.0.1:" + args[1]
			args = args[2:]
		case "--mcp-token":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --mcp-token requires a value\n")
				exit(1)
			}
			mcpToken = args[1]
			args = args[2:]
		case "--web":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				exit(1)
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[1])
				exit(1)
			}
			webPort = args[1]
			args = args[2:]
		case "--insecure-web":
			insecureWeb = true
			args = args[1:]
		case "--color":
			_ = env.Set("ze.log.color", "true")
			args = args[1:]
		case "--no-color":
			_ = env.Set("ze.log.color", "false")
			args = args[1:]
		case "--plugins":
			// Handle here to avoid breaking the loop — this is a standalone flag
			args = args[0:] // Keep it for dispatch below
			goto dispatch
		case "--version", "-V":
			printVersion(false)
			exit(0)
		case "--extended-version":
			printVersion(true)
			exit(0)
		case "--help", "-h": //nolint:goconst // consistent pattern across cmd files
			args = args[0:]
			goto dispatch
		default:
			// Unknown flag — stop parsing, let dispatch handle it
			goto dispatch
		}
	}
dispatch:

	if pprofAddr != "" {
		startPprof(pprofAddr)
	}

	// Handle -f flag: use filesystem storage with the override path
	if fileOverride != "" {
		store := storage.NewFilesystem()
		fileOverride = config.ResolveConfigPath(fileOverride)
		switch detectConfigType(store, fileOverride) {
		case config.ConfigTypeBGP, config.ConfigTypeHub, config.ConfigTypeUnknown:
			exit(withPanicCapture(func() int {
				return hub.Run(store, fileOverride, plugins, chaosSeed, chaosRate, false, "", false, "", "")
			}))
		}
	}

	if len(args) < 1 {
		usage()
		exit(1)
	}

	arg := args[0]

	// Dispatch YANG verb commands (show, set, clear, request, delete, update, validate, monitor).
	// These go through the unified command tree, same path as the CLI editor.
	if isYANGVerb(arg) {
		// Check for help at any depth: "show help", "show bgp help", "show bgp decode help"
		if helpPath := extractHelpPath(args); helpPath != nil {
			yangTree := cli.YANGCommandTree()
			yangNode := command.FindNode(yangTree, helpPath)

			pathStr := strings.Join(helpPath, " ")
			fmt.Fprintf(os.Stderr, "Usage: ze %s <command> [options]\n\n", pathStr)
			if yangNode != nil && yangNode.Description != "" {
				label := strings.ToUpper(helpPath[len(helpPath)-1][:1]) + helpPath[len(helpPath)-1][1:]
				fmt.Fprintf(os.Stderr, "%s (%s).\n\n", label, yangNode.Description)
			}
			fmt.Fprintf(os.Stderr, "Available commands:\n")
			if yangNode != nil && len(yangNode.Children) > 0 {
				command.WriteHelp(os.Stderr, yangNode, nil)
			} else {
				fmt.Fprintf(os.Stderr, "  (no commands registered)\n")
			}
			fmt.Fprintln(os.Stderr)
			exit(0)
		}
		// ReadOnly is determined by the verb, not a flag on the registration.
		readOnly := command.IsReadOnlyVerb(arg)
		code := cmdutil.RunCommand(args, readOnly, arg)
		if code == -1 {
			fmt.Fprintf(os.Stderr, "unknown %s command: %s\n", arg, strings.Join(args[1:], " "))
			fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", arg)
			exit(1)
		}
		exit(code)
	}

	// Normalize flag-style aliases to their registered root command names
	// so the registry lookup handles them uniformly.
	switch arg {
	case "-h", "--help":
		arg = "help"
	}

	// Owner-backed root commands: every root command registers a handler
	// from init() and is dispatched here. No static switch needed.
	rctx := newRuntimeContext(plugins, fileOverride, webPort, insecureWeb, mcpAddr, mcpToken, chaosSeed, chaosRate)
	if code, handled := dispatchRegisteredRoot(arg, rctx, args[1:]); handled {
		exit(code)
	}

	// Derive web settings from global flags.
	webEnabled := webPort != ""
	webListenAddr := ""
	if webEnabled {
		webListenAddr = "0.0.0.0:" + webPort
		if insecureWeb {
			webListenAddr = "127.0.0.1:" + webPort
		}
	}
	if insecureWeb && !webEnabled {
		fmt.Fprintf(os.Stderr, "error: --insecure-web requires --web <port>\n")
		exit(1)
	}

	// If arg looks like a config file, dispatch based on content
	if looksLikeConfig(arg) {
		// For stdin, config data comes from stdin but we still need blob
		// storage for TLS certs, SSH host keys, and other persistent state.
		if arg == "-" {
			exit(withPanicCapture(func() int {
				return hub.Run(resolveStorage(), arg, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken)
			}))
		}
		store := resolveStorage()
		// Search XDG config paths if not found locally
		arg = config.ResolveConfigPath(arg)
		// When the config file lives on the filesystem (e.g., gokrazy's
		// read-only /etc/ze/ze.conf) but blob storage is available for
		// TLS certs, SSH keys, and other persistent state, keep the blob
		// store and let hub.Run read the config from the filesystem.
		// Only fall back to filesystem storage when blob is unavailable.
		if storage.IsBlobStorage(store) && !store.Exists(arg) {
			if _, statErr := os.Stat(arg); statErr != nil {
				store.Close() //nolint:errcheck // closing blob before filesystem fallback
				store = storage.NewFilesystem()
			}
		}
		switch detectConfigType(store, arg) {
		case config.ConfigTypeBGP, config.ConfigTypeHub, config.ConfigTypeUnknown:
			exit(withPanicCapture(func() int {
				return hub.Run(store, arg, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken)
			}))
		}
	}

	// Registry fallback: root-level commands registered via registry
	// (ping, generate wireguard keypair, ...) whose init()
	// wired a handler. Longest-prefix match on the raw argv so that
	// multi-word commands ("generate wireguard keypair") win over
	// shorter prefixes.
	if handler, remaining := registry.LookupLocal(args); handler != nil {
		exit(handler(remaining))
	}

	// Unknown command: suggest the closest match but never auto-dispatch.
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	known := knownCommands()
	if suggestion := suggest.Command(arg, known); suggestion != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
	}
	usage()
	exit(1)
}

// knownCommands returns all valid top-level command names, derived from
// the YANG verb map and the root command registry.
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

// yangVerbs are the top-level verbs dispatched through the unified YANG command tree.
var yangVerbs = map[string]bool{
	"show": true, "set": true, "clear": true, "request": true,
	"delete": true, "update": true, "validate": true, "monitor": true,
}

// isYANGVerb returns true if the argument is a YANG verb that should be
// dispatched through the unified command tree rather than the static switch.
func isYANGVerb(arg string) bool {
	return yangVerbs[arg]
}

// extractHelpPath checks if args end with help/-h/--help or have no subcommand,
// and returns the path to show help for. Returns nil if not a help request.
// Examples:
//
//	["show"] -> ["show"]
//	["show", "help"] -> ["show"]
//	["show", "bgp", "help"] -> ["show", "bgp"]
//	["show", "bgp", "--help"] -> ["show", "bgp"]
//	["show", "bgp", "decode", "hex"] -> nil (not a help request)
//
// isHelpArg returns true if the argument is a help flag.
func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func startUsage() {
	p := helpfmt.Page{
		Command: "ze start",
		Summary: "Start the Ze daemon from blob storage",
		Usage:   []string{"ze start [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--cli", Desc: "Attach interactive CLI after startup"},
				{Name: "--web <port>", Desc: "Enable web UI on given port"},
				{Name: "--insecure-web", Desc: "Disable web auth (binds to localhost only)"},
				{Name: "--mcp <port>", Desc: "Enable MCP server on given port"},
				{Name: "--mcp-token <token>", Desc: "Bearer token for MCP authentication"},
			}},
			{Title: "Prerequisites", Entries: []helpfmt.HelpEntry{
				{Name: "ze init", Desc: "Bootstrap database (required before first start)"},
				{Name: "ze config edit", Desc: "Create or edit configuration"},
			}},
		},
		Examples: []string{
			"ze start                           Start daemon with default config",
			"ze start --cli                     Start daemon and attach interactive CLI",
			"ze start --web 3443                Start with web UI on port 3443",
			"ze start --web 3443 --insecure-web Start with web UI, no auth (localhost)",
		},
	}
	p.Write()
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
		usage()
	}
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

// looksLikeConfig returns true if the argument looks like a config file path.
func looksLikeConfig(arg string) bool {
	// "-" means stdin
	if arg == "-" {
		return true
	}

	// Check for common config extensions
	if strings.HasSuffix(arg, ".conf") ||
		strings.HasSuffix(arg, ".cfg") ||
		strings.HasSuffix(arg, ".yaml") ||
		strings.HasSuffix(arg, ".yml") ||
		strings.HasSuffix(arg, ".json") {
		return true
	}

	// Check if it's a path (contains / or starts with .)
	if strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") {
		// Check if file exists
		if _, err := os.Stat(arg); err == nil {
			return true
		}
	}

	return false
}

// detectConfigType probes a config file to determine what daemon to start.
// Returns ConfigTypeBGP for bgp {} block, ConfigTypeHub for plugin { external },
// ConfigTypeUnknown otherwise. BGP takes precedence if both blocks are present.
func detectConfigType(store storage.Storage, path string) config.ConfigType {
	data, err := store.ReadFile(path)
	if err != nil {
		return config.ConfigTypeUnknown
	}
	return config.ProbeConfigType(string(data))
}

// resolveStorage creates the appropriate storage backend.
// Default: blob storage at {configDir}/database.zefs.
// Fallback: filesystem if blob cannot be created or ZE_STORAGE_BLOB=false.
func resolveStorage() storage.Storage {
	s, err := internalresolve.Storage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: blob storage unavailable (%v), using filesystem\n", err)
	}
	return s
}

// cmdStart resolves the default config from zefs and starts the daemon.
// For managed clients (meta/instance/managed=true), connects to hub to fetch config
// before starting, falling back to cached config if hub is unreachable.
// When --web is set and no config exists, starts the web server standalone.
// validPort checks a string is a numeric port in range 1-65535.
func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

func cmdStart(args, plugins []string, chaosSeed int64, chaosRate float64, globalMCPAddr, globalMCPToken, globalWebPort string, globalInsecureWeb bool) int {
	// Start with global flag values, allow local flags to override.
	mcpAddr := globalMCPAddr
	mcpToken := globalMCPToken
	webPort := globalWebPort
	insecureWeb := globalInsecureWeb
	cliEnabled := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cli":
			cliEnabled = true
		case "--web":
			if i+1 < len(args) {
				i++
				if !validPort(args[i]) {
					fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[i])
					return 1
				}
				webPort = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				return 1
			}
		case "--insecure-web":
			insecureWeb = true
		case "--mcp":
			if i+1 < len(args) {
				i++
				if !validPort(args[i]) {
					fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[i])
					return 1
				}
				mcpAddr = "127.0.0.1:" + args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				return 1
			}
		case "--mcp-token":
			if i+1 < len(args) {
				i++
				mcpToken = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: --mcp-token requires a value\n")
				return 1
			}
		}
	}

	webEnabled := webPort != ""
	webListenAddr := ""
	if webEnabled {
		webListenAddr = "0.0.0.0:" + webPort
		if insecureWeb {
			webListenAddr = "127.0.0.1:" + webPort
		}
	}
	if insecureWeb && !webEnabled {
		fmt.Fprintf(os.Stderr, "error: --insecure-web requires --web <port>\n")
		return 1
	}

	store := resolveStorage()
	defer store.Close() //nolint:errcheck // best-effort cleanup

	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "error: ze start requires blob storage (run ze init first)\n")
		return 1
	}

	// Check managed mode: meta/instance/managed=true in blob.
	if isManaged(store) {
		return cmdStartManaged(store, plugins, chaosSeed, chaosRate)
	}

	configName := internalresolve.DefaultConfig(store)
	if !store.Exists(configName) {
		// Config does not exist at all: try first-boot bootstrap.
		switch {
		case bootstrapConfigFromTemplate(store, configName):
			fmt.Fprintf(os.Stderr, "bootstrap: created config from template + discovery\n")
		case bootstrapFromDiscovery(store, configName):
			fmt.Fprintf(os.Stderr, "bootstrap: created config from interface discovery (DHCP + SSH)\n")
		case webEnabled:
			return withPanicCapture(func() int { return hub.RunWebOnly(store, webListenAddr, insecureWeb) })
		default:
			fmt.Fprintf(os.Stderr, "error: no config found in database (run ze config edit first)\n")
			return 1
		}
	}

	applied, preChange := checkPushedConfig(store, configName)
	writeConfigActiveHash(store, configName)

	if applied {
		hr := NewHealthRevert(store, configName)
		hr.Start(preChange)
		hub.PeerLifecycleCallback = hr
	}

	ct := detectConfigType(store, configName)
	if ct == config.ConfigTypeUnknown && webEnabled {
		return withPanicCapture(func() int { return hub.RunWebOnly(store, webListenAddr, insecureWeb) })
	}

	return withPanicCapture(func() int {
		return hub.Run(store, configName, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr, mcpToken, cliEnabled)
	})
}

// isManaged returns true if the blob has meta/instance/managed=true.
func isManaged(store storage.Storage) bool {
	data, err := store.ReadFile(zefs.KeyInstanceManaged.Pattern)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "true"
}

// cmdStartManaged handles ze start for managed clients.
// With cached config: starts BGP immediately, connects to hub in background for updates.
// Without cached config (first boot): requires hub connection to fetch initial config.
func cmdStartManaged(store storage.Storage, plugins []string, chaosSeed int64, chaosRate float64) int {
	configName := internalresolve.DefaultConfig(store)

	if store.Exists(configName) {
		// The hub starts the managed client after the runtime commit hook is wired.
		clientCfg := extractManagedClientConfig(store, configName)

		return withPanicCapture(func() int {
			return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, clientCfg)
		})
	}

	// No cached config: first boot after ze init --managed.
	server := env.Get("ze.managed.server")
	name := env.Get("ze.managed.name")
	token := env.Get("ze.managed.token")

	if server == "" || name == "" {
		fmt.Fprintf(os.Stderr, "error: managed mode with no cached config\n")
		fmt.Fprintf(os.Stderr, "hint: set ze.managed.server and ze.managed.name to bootstrap from hub\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_SERVER=hub-host:1791\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_NAME=edge-01\n")
		fmt.Fprintf(os.Stderr, "  export ZE_MANAGED_TOKEN=secret\n")
		return 1
	}
	if token == "" {
		fmt.Fprintf(os.Stderr, "error: ze.managed.token is required for first boot\n")
		return 1
	}

	// First boot: connect to hub, fetch config, validate, cache, then start.
	fmt.Fprintf(os.Stderr, "managed: first boot, connecting to hub %s as %s\n", server, name)
	configData, err := fetchInitialConfig(server, name, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fetch config from hub: %v\n", err)
		return 1
	}

	// Validate fetched config before caching. Reject invalid remote config
	// to prevent poisoning bootstrap state.
	if _, parseErr := config.LoadConfig(string(configData), "", nil); parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: hub config failed validation: %v\n", parseErr)
		return 1
	}

	// Cache validated config in blob.
	if writeErr := store.WriteFile(configName, configData, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: cache config: %v\n", writeErr)
		return 1
	}

	// The hub starts the managed client for first boot too, after the runtime
	// commit hook is available.
	clientCfg := extractManagedClientConfig(store, configName)

	return withPanicCapture(func() int {
		return hub.RunWithManagedClient(store, configName, plugins, chaosSeed, chaosRate, clientCfg)
	})
}

// extractManagedClientConfig reads config from blob and extracts the hub client block.
// Returns nil if no client block is found (standalone mode). Logs warnings on failures.
func extractManagedClientConfig(store storage.Storage, configName string) *managed.ClientConfig {
	data, err := store.ReadFile(configName)
	if err != nil {
		slog.Warn("managed: cannot read config for hub extraction", "config", configName, "error", err)
		return nil
	}

	loadResult, err := config.LoadConfig(string(data), "", nil)
	if err != nil {
		slog.Warn("managed: cannot parse config for hub extraction", "config", configName, "error", err)
		return nil
	}

	hubCfg, err := config.ExtractHubConfig(loadResult.Tree)
	if err != nil {
		slog.Warn("managed: cannot extract hub config", "error", err)
		return nil
	}
	if len(hubCfg.Clients) == 0 {
		return nil
	}

	cli := hubCfg.Clients[0]

	return &managed.ClientConfig{
		Name:        cli.Name,
		Server:      cli.Address(),
		Token:       cli.Secret,
		TLSInsecure: env.GetBool("ze.managed.tls.insecure", false),
		Version:     fleet.VersionHash(data),
		Handler: &managed.Handler{
			Validate: func(cfgData []byte) error {
				_, parseErr := config.LoadConfig(string(cfgData), "", nil)
				return parseErr
			},
		},
		CheckManaged: func() bool {
			return isManaged(store)
		},
	}
}

// fetchInitialConfig connects to the hub, authenticates, and fetches the initial config.
func fetchInitialConfig(server, name, token string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), env.GetDuration("ze.managed.connect.timeout", 5*time.Second))
	defer cancel()

	tlsInsecure := env.GetBool("ze.managed.tls.insecure", false)
	tlsConf := &tls.Config{
		InsecureSkipVerify: tlsInsecure, //nolint:gosec // opt-in via explicit env var
		MinVersion:         tls.VersionTLS13,
	}
	if tlsInsecure {
		slog.Warn("managed TLS: certificate verification disabled (insecure)")
	}

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", server, err)
	}
	defer conn.Close() //nolint:errcheck // cleanup

	if err := pluginipc.SendAuth(ctx, conn, token, name); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	// Read auth response line (newline-terminated) before wrapping in MuxConn.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	authLine, readErr := readAuthLine(conn, 512)
	if readErr != nil {
		return nil, fmt.Errorf("read auth response: %w", readErr)
	}
	_ = conn.SetReadDeadline(time.Time{})

	// Parse: #<id> <verb> [payload]. Verb must be "ok".
	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil || verb != "ok" {
		return nil, errAuthRejected
	}

	rc := rpc.NewConn(conn, conn)
	mc := rpc.NewMuxConn(rc)
	defer mc.Close() //nolint:errcheck // cleanup

	resp, err := managed.FetchConfig(ctx, mc, "")
	if err != nil {
		return nil, err
	}

	if resp.Config == "" {
		return nil, errHubReturnedEmptyConfig
	}

	data, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return data, nil
}

// readAuthLine reads from conn byte-by-byte until newline or maxSize.
func readAuthLine(conn net.Conn, maxSize int) ([]byte, error) {
	buf := make([]byte, 0, 128)
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == '\n' {
			// Strip trailing \r for CRLF compatibility.
			if len(buf) > 0 && buf[len(buf)-1] == '\r' {
				buf = buf[:len(buf)-1]
			}
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) >= maxSize {
			return nil, fmt.Errorf("auth response exceeds %d bytes", maxSize)
		}
	}
}

// bootstrapConfigFromTemplate reads file/template/ze.conf from zefs,
// runs interface discovery, merges them, and writes the result to the
// active config. Returns true on success.
func bootstrapConfigFromTemplate(store storage.Storage, configName string) bool {
	templateKey := zefs.KeyFileTemplate.Key("ze.conf")
	tmpl, err := store.ReadFile(templateKey)
	if err != nil {
		return false
	}

	var merged []byte
	if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: netlink backend unavailable: %v\n", loadErr)
		merged = tmpl
	} else {
		discovered, discErr := iface.DiscoverInterfaces()
		if closeErr := iface.CloseBackend(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: bootstrap: close backend: %v\n", closeErr)
		}
		if discErr != nil || len(discovered) == 0 {
			merged = tmpl
		} else {
			ifaceCfg := iface.EmitSetConfig(discovered)
			merged = make([]byte, 0, len(tmpl)+1+len(ifaceCfg))
			merged = append(merged, tmpl...)
			merged = append(merged, '\n')
			merged = append(merged, []byte(ifaceCfg)...)
		}
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	if writeErr := store.WriteFile(activeKey, merged, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: write config: %v\n", writeErr)
		return false
	}
	return true
}

// bootstrapFromDiscovery generates a minimal config from interface discovery
// when no config and no template exist. Enables DHCP client on every ethernet
// interface and SSH for operator access. Returns true on success.
func bootstrapFromDiscovery(store storage.Storage, configName string) bool {
	if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
		return false
	}
	discovered, discErr := iface.DiscoverInterfaces()
	if closeErr := iface.CloseBackend(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: close backend: %v\n", closeErr)
	}
	if discErr != nil {
		return false
	}

	cfg := iface.EmitBootstrapConfig(discovered)
	if cfg == "" {
		return false
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	if writeErr := store.WriteFile(activeKey, []byte(cfg), 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrap: write config: %v\n", writeErr)
		return false
	}
	return true
}

func usage() {
	// Dynamic verb list from YANG tree goes into the operations section.
	verbTree := cli.BuildCommandTree(false)
	cmdEntries := command.HelpEntries(verbTree, nil)
	verbEntries := make([]helpfmt.HelpEntry, len(cmdEntries))
	for i, e := range cmdEntries {
		verbEntries[i] = helpfmt.HelpEntry{Name: e.Name, Desc: e.Desc}
	}

	// Build command sections from registered metadata.
	// Operations section: root commands (cli) first, then YANG verbs.
	sections := []helpfmt.HelpSection{
		{Title: registry.SectionTitle(registry.SectionOperations)},
	}
	for _, se := range registry.ListRootBySection() {
		entries := make([]helpfmt.HelpEntry, len(se.Commands))
		for i, rc := range se.Commands {
			entries[i] = helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description}
		}
		title := registry.SectionTitle(se.Section)
		if title == "" {
			title = se.Section
		}
		if se.Section == registry.SectionOperations {
			sections[0].Entries = append(entries, sections[0].Entries...)
		} else {
			sections = append(sections, helpfmt.HelpSection{Title: title, Entries: entries})
		}
	}
	sections[0].Entries = append(sections[0].Entries, verbEntries...)

	sections = append(sections, helpfmt.HelpSection{
		Title: "Options",
		Entries: []helpfmt.HelpEntry{
			{Name: "-d, --debug", Desc: "Enable debug logging (sets ze.log=debug for all subsystems)"},
			{Name: "-f <file>", Desc: "Use filesystem directly, bypass blob store"},
			{Name: "--plugin <name>", Desc: "Load plugin before starting (repeatable)"},
			{Name: "--plugins", Desc: "List available internal plugins"},
			{Name: "--web <port>", Desc: "Start web server on given port"},
			{Name: "--insecure-web", Desc: "Disable web auth (binds to localhost only)"},
			{Name: "--mcp <port>", Desc: "Start MCP server on 127.0.0.1:<port>"},
			{Name: "--mcp-token <token>", Desc: "Bearer token for MCP authentication"},
			{Name: "--pprof <addr:port>", Desc: "Start pprof HTTP server (e.g. :6060)"},
			{Name: "--color", Desc: "Force colored output (even when not a TTY)"},
			{Name: "--no-color", Desc: "Disable colored output (also: NO_COLOR env var, TERM=dumb)"},
			{Name: "-V, --version", Desc: "Show version and exit"},
			{Name: "--extended-version", Desc: "Show extended version (commit, go, os/arch)"},
		},
	})

	p := helpfmt.Page{
		Command:  "ze",
		Software: "ze Software",
		Usage: []string{
			"ze [--plugin <name>]... <config>   Start with config file",
			"ze <verb> <command> [options]      Execute command (same grammar as ze cli)",
		},
		Sections: sections,
		Examples: []string{
			"ze config.conf                       Start with config",
			"ze --plugin ze.hostname config.conf  Start with hostname plugin",
			"ze --plugins                         List available plugins",
			"ze cli                               Interactive CLI",
			"ze show bgp peer list                Show peer list",
			"ze show help                         List available show commands",
			"ze delete bgp peer 10.0.0.1        Remove a peer",
			"ze bgp decode <hex>                  Decode BGP message",
		},
	}
	p.Write()
}

// printPlugins outputs available plugins in table or JSON format.
func printPlugins(jsonOutput bool) {
	plugins := plugin.InternalPluginInfo()

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(plugins)
		return
	}

	// Tabulated output
	// Header
	fmt.Printf("%-12s  %-35s  %-20s  %-15s  %s\n",
		"NAME", "DESCRIPTION", "RFC", "CAPABILITY", "FAMILY")
	fmt.Printf("%-12s  %-35s  %-20s  %-15s  %s\n",
		"----", "-----------", "---", "----------", "------")

	for _, info := range plugins {
		rfcs := strings.Join(info.RFCs, ", ")
		caps := ""
		if len(info.Capabilities) > 0 {
			capStrs := make([]string, len(info.Capabilities))
			for i, c := range info.Capabilities {
				capStrs[i] = strconv.Itoa(c)
			}
			caps = strings.Join(capStrs, ", ")
		}
		families := strings.Join(info.Families, ", ")

		fmt.Printf("%-12s  %-35s  %-20s  %-15s  %s\n",
			info.Name, info.Description, rfcs, caps, families)
	}
}
