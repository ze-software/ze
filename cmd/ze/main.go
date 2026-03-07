// Design: docs/architecture/system-architecture.md — ze main entry point
//
// Package main provides the ze command entry point.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when --pprof flag is set
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	zecompletion "codeberg.org/thomas-mangin/ze/cmd/ze/completion"
	zeconfig "codeberg.org/thomas-mangin/ze/cmd/ze/config"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	zeplugin "codeberg.org/thomas-mangin/ze/cmd/ze/plugin"
	zerun "codeberg.org/thomas-mangin/ze/cmd/ze/run"
	"codeberg.org/thomas-mangin/ze/cmd/ze/schema"
	"codeberg.org/thomas-mangin/ze/cmd/ze/show"
	zesignal "codeberg.org/thomas-mangin/ze/cmd/ze/signal"
	"codeberg.org/thomas-mangin/ze/cmd/ze/validate"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	// Import all plugins to trigger init() registration.
	// Must happen at the binary entry point (not in internal/plugin)
	// to avoid import cycles: format → plugin → all → bgp-rs → format.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
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
	args := os.Args[1:]
	for len(args) > 0 && (strings.HasPrefix(args[0], "--") || args[0] == "-d" || args[0] == "-V") {
		switch args[0] {
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

	// Start pprof HTTP server if --pprof was set.
	// Uses DefaultServeMux which net/http/pprof registers handlers on.
	if pprofAddr != "" {
		if !isLocalhostPprof(pprofAddr) {
			fmt.Fprintf(os.Stderr, "error: --pprof must bind to localhost (e.g. 127.0.0.1:6060 or [::1]:6060), got %q\n", pprofAddr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "pprof server listening on %s\n", pprofAddr) //nolint:gosec // stderr, not HTTP response
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil { //nolint:gosec // pprof is bound to localhost only
				fmt.Fprintf(os.Stderr, "error: pprof server: %v\n", err)
			}
		}()
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
		os.Exit(zeconfig.Run(args[1:]))
	case "validate":
		os.Exit(validate.Run(args[1:]))
	case "schema":
		os.Exit(schema.Run(args[1:], plugins))
	case "exabgp":
		os.Exit(exabgp.Run(args[1:]))
	case "signal":
		os.Exit(zesignal.Run(args[1:]))
	case "show":
		os.Exit(show.Run(args[1:]))
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
		// For stdin, skip detection - hub.Run reads and probes internally
		if arg == "-" {
			os.Exit(hub.Run(arg, plugins, chaosSeed, chaosRate))
		}
		// Search XDG config paths if not found locally
		arg = config.ResolveConfigPath(arg)
		switch detectConfigType(arg) {
		case config.ConfigTypeBGP:
			// Start BGP daemon in-process via hub
			os.Exit(hub.Run(arg, plugins, chaosSeed, chaosRate))
		case config.ConfigTypeHub:
			// Start hub orchestrator (forks external plugins)
			os.Exit(hub.Run(arg, plugins, chaosSeed, chaosRate))
		case config.ConfigTypeUnknown:
			fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
			os.Exit(1)
		}
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	commands := []string{
		"bgp", "plugin", "cli", "config", "validate", "schema",
		"exabgp", "signal", "show", "run", "completion", "version", "help",
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
func detectConfigType(path string) config.ConfigType {
	data, err := os.ReadFile(path) //nolint:gosec // Config file path from user
	if err != nil {
		return config.ConfigTypeUnknown
	}
	return config.ProbeConfigType(string(data))
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze - Ze toolchain

Usage:
  ze [--plugin <name>]... <config>   Start with config file
  ze <command> [options]             Execute command

Options:
  -d, --debug           Enable debug logging (sets ze.log=debug for all subsystems)
  --plugin <name>       Load plugin before starting (repeatable)
  --plugins             List available internal plugins
  --pprof <addr:port>   Start pprof HTTP server (e.g. :6060)
  -V, --version         Show version and exit
  --chaos-seed <N>      Enable chaos self-test mode with PRNG seed N (-1 = time-based)
  --chaos-rate <0-1>    Fault probability per operation (default: 0.1)

Commands:
  validate  Validate configuration file
  config    Configuration management
  schema    Schema discovery
  cli      Interactive CLI for running daemons
  show     Show daemon state (read-only commands)
  run      Execute daemon command (all commands)
  bgp      BGP protocol tools (decode, encode)
  plugin   Plugin system (rib, rr, gr, etc.)
  signal   Send signals to running daemon (reload, stop, status)
  exabgp       ExaBGP bridge tools
  completion   Generate shell completion scripts
  version      Show version
  help         Show this help

Examples:
  ze config.conf                      Start with config
  ze --plugin ze.hostname config.conf Start with hostname plugin
  ze --plugins                        List available plugins
  ze cli                              Interactive CLI
  ze cli --run "peer list"            Execute CLI command
  ze show help                         List read-only commands
  ze show <command>                    Show daemon state
  ze run help                          List all commands
  ze run <command>                     Execute daemon command
  ze bgp help                         Show BGP commands
`)
}

// isLocalhostPprof returns true if addr binds to a loopback address only.
// Rejects empty host (binds all interfaces), 0.0.0.0, and non-loopback addresses.
func isLocalhostPprof(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
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
