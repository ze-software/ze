// Design: docs/architecture/system-architecture.md — ze main entry point
//
// Package main provides the ze command entry point.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	zecompletion "codeberg.org/thomas-mangin/ze/cmd/ze/completion"
	zeconfig "codeberg.org/thomas-mangin/ze/cmd/ze/config"
	zedb "codeberg.org/thomas-mangin/ze/cmd/ze/db"
	zeenv "codeberg.org/thomas-mangin/ze/cmd/ze/environ"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	zeinit "codeberg.org/thomas-mangin/ze/cmd/ze/init"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	zeplugin "codeberg.org/thomas-mangin/ze/cmd/ze/plugin"
	zerun "codeberg.org/thomas-mangin/ze/cmd/ze/run"
	"codeberg.org/thomas-mangin/ze/cmd/ze/schema"
	"codeberg.org/thomas-mangin/ze/cmd/ze/show"
	zesignal "codeberg.org/thomas-mangin/ze/cmd/ze/signal"
	"codeberg.org/thomas-mangin/ze/cmd/ze/validate"
	zeyang "codeberg.org/thomas-mangin/ze/cmd/ze/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"

	// Import all plugins to trigger init() registration.
	// Must happen at the binary entry point (not in internal/plugin)
	// to avoid import cycles: format → plugin → all → bgp-rs → format.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
)

// Env var registrations for storage and config.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.storage.blob", Type: "bool", Default: "true", Description: "Use blob storage (false = filesystem)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})
)

// version and buildDate are set via ldflags at build time.
// Format: -ldflags "-X main.version=YY.MM.DD -X main.buildDate=YYYY-MM-DD".
var (
	version   = "dev"
	buildDate = "unknown"
)

func printVersion() {
	fmt.Printf("ze %s (built %s)\n", version, buildDate)
}

func main() {
	pluginserver.SetVersion(version, buildDate)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// Parse global flags before command dispatch
	var plugins []string
	var chaosSeed int64
	var chaosRate float64 = -1 // -1 means "not set by CLI"
	var pprofAddr string
	var fileOverride string // -f flag: bypass blob, use filesystem directly
	args := os.Args[1:]
	for len(args) > 0 && (strings.HasPrefix(args[0], "--") || args[0] == "-d" || args[0] == "-V" || args[0] == "-f") {
		switch args[0] {
		case "-f":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: -f requires a file path\n")
				os.Exit(1)
			}
			fileOverride = args[1]
			args = args[2:]
		case "-d", "--debug":
			_ = os.Setenv("ze.log", "debug")
			_ = os.Setenv("ze.log.relay", "debug")
			args = args[1:]
		case "--plugin":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --plugin requires an argument\n")
				os.Exit(1)
			}
			plugins = append(plugins, args[1])
			args = args[2:]
		case "--pprof":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --pprof requires an address (e.g. :6060)\n")
				os.Exit(1)
			}
			pprofAddr = args[1]
			args = args[2:]
		case "--chaos-seed":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-seed requires an argument\n")
				os.Exit(1)
			}
			n, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-seed: %v\n", err)
				os.Exit(1)
			}
			chaosSeed = n
			args = args[2:]
		case "--chaos-rate":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate requires an argument\n")
				os.Exit(1)
			}
			f, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --chaos-rate: %v\n", err)
				os.Exit(1)
			}
			if f < 0 || f > 1.0 {
				fmt.Fprintf(os.Stderr, "error: --chaos-rate must be 0.0-1.0, got %.2f\n", f)
				os.Exit(1)
			}
			chaosRate = f
			args = args[2:]
		case "--plugins":
			// Handle here to avoid breaking the loop — this is a standalone flag
			args = args[0:] // Keep it for dispatch below
			goto dispatch
		case "--version", "-V":
			printVersion()
			os.Exit(0)
		case "--help", "-h":
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
		case config.ConfigTypeBGP, config.ConfigTypeHub:
			os.Exit(hub.Run(store, fileOverride, plugins, chaosSeed, chaosRate))
		case config.ConfigTypeUnknown:
			fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
			os.Exit(1)
		}
	}

	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	arg := args[0]

	// Check for known commands first
	switch arg {
	case "bgp":
		os.Exit(bgp.Run(args[1:]))
	case "plugin":
		os.Exit(zeplugin.Run(args[1:]))
	case "cli":
		os.Exit(cli.Run(args[1:]))
	case "config":
		store := resolveStorage()
		code := zeconfig.RunWithStorage(store, args[1:])
		store.Close() //nolint:errcheck // best-effort cleanup before exit
		os.Exit(code)
	case "init":
		os.Exit(zeinit.Run(args[1:]))
	case "db":
		os.Exit(zedb.Run(args[1:]))
	case "validate":
		os.Exit(validate.Run(args[1:]))
	case "schema":
		os.Exit(schema.Run(args[1:], plugins))
	case "yang":
		os.Exit(zeyang.Run(args[1:]))
	case "exabgp":
		os.Exit(exabgp.Run(args[1:]))
	case "signal":
		os.Exit(zesignal.Run(args[1:]))
	case "status":
		os.Exit(zesignal.RunStatus(args[1:]))
	case "show":
		os.Exit(show.Run(args[1:]))
	case "env":
		os.Exit(zeenv.Run(args[1:]))
	case "run":
		os.Exit(zerun.Run(args[1:]))
	case "completion":
		os.Exit(zecompletion.Run(args[1:]))
	case "version":
		printVersion()
		os.Exit(0)
	case "help", "-h", "--help":
		usage()
		os.Exit(0)
	case "--plugins":
		// Check for --json flag
		jsonOutput := len(os.Args) > 2 && os.Args[2] == "--json"
		printPlugins(jsonOutput)
		os.Exit(0)
	}

	// If arg looks like a config file, dispatch based on content
	if looksLikeConfig(arg) {
		// For stdin, skip blob - hub.Run reads and probes internally
		if arg == "-" {
			os.Exit(hub.Run(storage.NewFilesystem(), arg, plugins, chaosSeed, chaosRate))
		}
		store := resolveStorage()
		// Search XDG config paths if not found locally
		arg = config.ResolveConfigPath(arg)
		// If blob storage doesn't have the file, fall back to filesystem
		// (config may not be imported into blob yet)
		if storage.IsBlobStorage(store) && !store.Exists(arg) {
			store.Close() //nolint:errcheck // closing blob before filesystem fallback
			store = storage.NewFilesystem()
		}
		switch detectConfigType(store, arg) {
		case config.ConfigTypeBGP:
			// Start BGP daemon in-process via hub
			os.Exit(hub.Run(store, arg, plugins, chaosSeed, chaosRate))
		case config.ConfigTypeHub:
			// Start hub orchestrator (forks external plugins)
			os.Exit(hub.Run(store, arg, plugins, chaosSeed, chaosRate))
		case config.ConfigTypeUnknown:
			fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
			os.Exit(1)
		}
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	commands := []string{
		"bgp", "plugin", "cli", "config", "db", "init", "validate", "schema", "yang",
		"exabgp", "signal", "status", "show", "run", "completion", "version", "help",
	}
	if suggestion := suggest.Command(arg, commands); suggestion != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
	}
	usage()
	os.Exit(1)
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
	if v := env.Get("ze.storage.blob"); strings.EqualFold(v, "false") {
		return storage.NewFilesystem()
	}
	configDir := env.Get("ze.config.dir")
	if configDir == "" {
		configDir = paths.DefaultConfigDir()
	}
	if configDir == "" {
		return storage.NewFilesystem()
	}
	blobPath := filepath.Join(configDir, "database.zefs")
	store, err := storage.NewBlob(blobPath, configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: blob storage unavailable (%v), using filesystem\n", err)
		return storage.NewFilesystem()
	}
	return store
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze - Ze toolchain

Usage:
  ze [--plugin <name>]... <config>   Start with config file
  ze <command> [options]             Execute command

Options:
  -d, --debug           Enable debug logging (sets ze.log=debug for all subsystems)
  -f <file>             Use filesystem directly, bypass blob store
  --plugin <name>       Load plugin before starting (repeatable)
  --plugins             List available internal plugins
  --pprof <addr:port>   Start pprof HTTP server (e.g. :6060)
  -V, --version         Show version and exit
  --chaos-seed <N>      Enable chaos self-test mode with PRNG seed N (-1 = time-based)
  --chaos-rate <0-1>    Fault probability per operation (default: 0.1)

Commands:
  init         Bootstrap database with SSH credentials
  validate     Validate configuration file
  config       Configuration management
  db           Blob store management
  schema       Schema discovery
  yang         YANG tree analysis and command docs
  cli          Interactive CLI for running daemons
  show         Show daemon state (read-only commands)
  run          Execute daemon command (all commands)
  status       Check if daemon is running
  bgp          BGP protocol tools (decode, encode)
  plugin       Plugin system (rib, rr, gr, etc.)
  signal       Send signals to running daemon (reload, stop, quit)
  exabgp       ExaBGP bridge tools
  completion   Generate shell completion scripts
  version      Show version
  help         Show this help

Examples:
  ze config.conf                       Start with config
  ze --plugin ze.hostname config.conf  Start with hostname plugin
  ze --plugins                         List available plugins
  ze cli                               Interactive CLI
  ze cli --run "peer list"             Execute CLI command
  ze show help                         List read-only commands
  ze show <command>                    Show daemon state
  ze run help                          List all commands
  ze run <command>                     Execute daemon command
  ze bgp help                          Show BGP commands
`)
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
				capStrs[i] = fmt.Sprintf("%d", c)
			}
			caps = strings.Join(capStrs, ", ")
		}
		families := strings.Join(info.Families, ", ")

		fmt.Printf("%-12s  %-35s  %-20s  %-15s  %s\n",
			info.Name, info.Description, rfcs, caps, families)
	}
}
