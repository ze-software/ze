// Design: docs/architecture/system-architecture.md — ze main entry point
//
// Package main provides the ze command entry point.
package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	zecompletion "codeberg.org/thomas-mangin/ze/cmd/ze/completion"
	zeconfig "codeberg.org/thomas-mangin/ze/cmd/ze/config"
	zedata "codeberg.org/thomas-mangin/ze/cmd/ze/data"
	zeenv "codeberg.org/thomas-mangin/ze/cmd/ze/environ"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	zeiface "codeberg.org/thomas-mangin/ze/cmd/ze/iface"
	zeinit "codeberg.org/thomas-mangin/ze/cmd/ze/init"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	zeplugin "codeberg.org/thomas-mangin/ze/cmd/ze/plugin"
	zerun "codeberg.org/thomas-mangin/ze/cmd/ze/run"
	"codeberg.org/thomas-mangin/ze/cmd/ze/schema"
	zesignal "codeberg.org/thomas-mangin/ze/cmd/ze/signal"
	zeyang "codeberg.org/thomas-mangin/ze/cmd/ze/yang"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/managed"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"

	// Import all plugins to trigger init() registration.
	// Must happen at the binary entry point (not in internal/plugin)
	// to avoid import cycles: format → plugin → all → bgp-rs → format.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
)

// Env var registrations for storage and config.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.storage.blob", Type: "bool", Default: "true", Description: "Use blob storage (false = filesystem)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.server", Type: "string", Description: "Override hub address (host:port) for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.name", Type: "string", Description: "Override client name for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.token", Type: "string", Description: "Override auth token for managed mode"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.managed.connect.timeout", Type: "duration", Default: "5s", Description: "Connection timeout for managed hub"})
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
	var mcpAddr string      // --mcp <port>: start MCP server on 127.0.0.1:<port>
	var webPort string      // --web <port>: start web server
	var insecureWeb bool    // --insecure-web: disable web auth
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
		case "--server":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --server requires host:port\n")
				os.Exit(1)
			}
			_ = env.Set("ze.managed.server", args[1])
			args = args[2:]
		case "--name":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --name requires client name\n")
				os.Exit(1)
			}
			_ = env.Set("ze.managed.name", args[1])
			args = args[2:]
		case "--token":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --token requires auth token\n")
				os.Exit(1)
			}
			_ = env.Set("ze.managed.token", args[1])
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
		case "--mcp":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --mcp requires a port\n")
				os.Exit(1)
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --mcp port must be 1-65535, got %q\n", args[1])
				os.Exit(1)
			}
			mcpAddr = "127.0.0.1:" + args[1]
			args = args[2:]
		case "--web":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "error: --web requires a port\n")
				os.Exit(1)
			}
			if !validPort(args[1]) {
				fmt.Fprintf(os.Stderr, "error: --web port must be 1-65535, got %q\n", args[1])
				os.Exit(1)
			}
			webPort = args[1]
			args = args[2:]
		case "--insecure-web":
			insecureWeb = true
			args = args[1:]
		case "--plugins":
			// Handle here to avoid breaking the loop — this is a standalone flag
			args = args[0:] // Keep it for dispatch below
			goto dispatch
		case "--version", "-V":
			printVersion()
			os.Exit(0)
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
		case config.ConfigTypeBGP, config.ConfigTypeHub:
			os.Exit(hub.Run(store, fileOverride, plugins, chaosSeed, chaosRate, false, "", false, ""))
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

	// Dispatch YANG verb commands (show, set, del, update, validate, monitor).
	// These go through the unified command tree, same path as the CLI editor.
	if isYANGVerb(arg) {
		if len(args) < 2 || args[1] == "help" || args[1] == "-h" || args[1] == "--help" {
			// Use both trees: YANG tree for descriptions, RPC tree for command list.
			yangTree := cli.YANGCommandTree()
			yangNode := command.FindNode(yangTree, []string{arg})
			rpcTree := cli.BuildCommandTree(false)
			rpcNode := command.FindNode(rpcTree, []string{arg})

			desc := ""
			if yangNode != nil {
				desc = yangNode.Description
			}
			fmt.Fprintf(os.Stderr, "Usage: ze %s <command> [options]\n\n", arg)
			if desc != "" {
				fmt.Fprintf(os.Stderr, "%s (%s).\n\n", strings.ToUpper(arg[:1])+arg[1:], desc)
			}
			fmt.Fprintf(os.Stderr, "Available commands:\n")
			if rpcNode != nil {
				command.WriteHelp(os.Stderr, rpcNode, nil)
			}
			fmt.Fprintln(os.Stderr)
			os.Exit(0)
		}
		// ReadOnly is determined by the verb, not a flag on the registration.
		readOnly := command.IsReadOnlyVerb(arg)
		code := cmdutil.RunCommand(args, readOnly, arg)
		if code == -1 {
			fmt.Fprintf(os.Stderr, "unknown %s command: %s\n", arg, strings.Join(args[1:], " "))
			fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", arg)
			os.Exit(1)
		}
		os.Exit(code)
	}

	// Static dispatch for commands not yet migrated to YANG verb registration.
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
	case "data":
		os.Exit(zedata.Run(args[1:]))
	case "schema":
		os.Exit(schema.Run(args[1:], plugins))
	case "yang":
		os.Exit(zeyang.Run(args[1:]))
	case "interface":
		os.Exit(zeiface.Run(args[1:]))
	case "exabgp":
		os.Exit(exabgp.Run(args[1:]))
	case "signal":
		os.Exit(zesignal.Run(args[1:]))
	case "status":
		os.Exit(zesignal.RunStatus(args[1:]))
	case "env":
		os.Exit(zeenv.Run(args[1:]))
	case "run":
		// Fallback: ze run delegates to the run package until migration is complete.
		os.Exit(zerun.Run(args[1:]))
	case "completion":
		os.Exit(zecompletion.Run(args[1:]))
	case "version":
		printVersion()
		os.Exit(0)
	case "start":
		os.Exit(cmdStart(args[1:], plugins, chaosSeed, chaosRate, mcpAddr, webPort, insecureWeb))
	case "help", "-h", "--help":
		subArgs := args[1:]
		switch {
		case aiHelpRequested(subArgs):
			printAIHelp(subArgs)
		case slices.Contains(subArgs, "--help") || slices.Contains(subArgs, "-h"):
			helpUsage()
		default:
			usage()
		}
		os.Exit(0)
	case "--plugins":
		// Check for --json flag
		jsonOutput := len(os.Args) > 2 && os.Args[2] == "--json"
		printPlugins(jsonOutput)
		os.Exit(0)
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
		os.Exit(1)
	}

	// If arg looks like a config file, dispatch based on content
	if looksLikeConfig(arg) {
		// For stdin, skip blob - hub.Run reads and probes internally
		if arg == "-" {
			os.Exit(hub.Run(storage.NewFilesystem(), arg, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr))
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
			os.Exit(hub.Run(store, arg, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr))
		case config.ConfigTypeHub:
			// Start hub orchestrator (forks external plugins)
			os.Exit(hub.Run(store, arg, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr))
		case config.ConfigTypeUnknown:
			fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
			os.Exit(1)
		}
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	commands := []string{
		"show", "set", "del", "update", "validate", "monitor",
		"bgp", "plugin", "cli", "config", "data", "env", "init", "interface", "start", "schema",
		"yang", "exabgp", "signal", "status", "completion", "version", "help",
	}
	if suggestion := suggest.Command(arg, commands); suggestion != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
	}
	usage()
	os.Exit(1)
}

// yangVerbs are the top-level verbs dispatched through the unified YANG command tree.
var yangVerbs = map[string]bool{
	"show": true, "set": true, "del": true,
	"update": true, "validate": true, "monitor": true,
}

// isYANGVerb returns true if the argument is a YANG verb that should be
// dispatched through the unified command tree rather than the static switch.
func isYANGVerb(arg string) bool {
	return yangVerbs[arg]
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

// cmdStart resolves the default config from zefs and starts the daemon.
// For managed clients (meta/instance/managed=true), connects to hub to fetch config
// before starting, falling back to cached config if hub is unreachable.
// When --web is set and no config exists, starts the web server standalone.
// validPort checks a string is a numeric port in range 1-65535.
func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

func cmdStart(args, plugins []string, chaosSeed int64, chaosRate float64, globalMCPAddr, globalWebPort string, globalInsecureWeb bool) int {
	// Start with global flag values, allow local flags to override.
	mcpAddr := globalMCPAddr
	webPort := globalWebPort
	insecureWeb := globalInsecureWeb

	for i := 0; i < len(args); i++ {
		switch args[i] {
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

	configName := resolveDefaultConfig(store)
	if !store.Exists(configName) {
		if !webEnabled {
			fmt.Fprintf(os.Stderr, "error: no config found in database (run ze config edit first)\n")
			return 1
		}
		return hub.RunWebOnly(store, webListenAddr, insecureWeb)
	}

	ct := detectConfigType(store, configName)
	if ct == config.ConfigTypeUnknown {
		if webEnabled {
			return hub.RunWebOnly(store, webListenAddr, insecureWeb)
		}
		fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
		return 1
	}

	return hub.Run(store, configName, plugins, chaosSeed, chaosRate, webEnabled, webListenAddr, insecureWeb, mcpAddr)
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
	configName := resolveDefaultConfig(store)

	if store.Exists(configName) {
		ct := detectConfigType(store, configName)
		if ct == config.ConfigTypeUnknown {
			fmt.Fprintf(os.Stderr, "error: cached config has no recognized block (bgp, plugin)\n")
			return 1
		}

		// Start background hub connection if client block found.
		clientCfg := extractManagedClientConfig(store, configName)
		if clientCfg != nil {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go managed.RunManagedClient(ctx, *clientCfg)
		}

		return hub.Run(store, configName, plugins, chaosSeed, chaosRate, false, "", false, "")
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

	// First boot: connect to hub, fetch config, cache it, then start.
	fmt.Fprintf(os.Stderr, "managed: first boot, connecting to hub %s as %s\n", server, name)
	configData, err := fetchInitialConfig(server, name, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fetch config from hub: %v\n", err)
		return 1
	}

	// Cache fetched config in blob.
	if writeErr := store.WriteFile(configName, configData, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: cache config: %v\n", writeErr)
		return 1
	}

	ct := detectConfigType(store, configName)
	if ct == config.ConfigTypeUnknown {
		fmt.Fprintf(os.Stderr, "error: fetched config has no recognized block (bgp, plugin)\n")
		return 1
	}

	// Start background hub connection for first-boot too.
	clientCfg := extractManagedClientConfig(store, configName)
	if clientCfg != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go managed.RunManagedClient(ctx, *clientCfg)
	}

	return hub.Run(store, configName, plugins, chaosSeed, chaosRate, false, "", false, "")
}

// extractManagedClientConfig reads config from blob and extracts the hub client block.
// Returns nil if no client block is found (standalone mode). Logs warnings on failures.
func extractManagedClientConfig(store storage.Storage, configName string) *managed.ClientConfig {
	data, err := store.ReadFile(configName)
	if err != nil {
		slog.Warn("managed: cannot read config for hub extraction", "config", configName, "error", err)
		return nil
	}

	loadResult, err := bgpconfig.LoadConfig(string(data), "", nil)
	if err != nil {
		slog.Warn("managed: cannot parse config for hub extraction", "config", configName, "error", err)
		return nil
	}

	hubCfg, err := bgpconfig.ExtractHubConfig(loadResult.Tree)
	if err != nil {
		slog.Warn("managed: cannot extract hub config", "error", err)
		return nil
	}
	if len(hubCfg.Clients) == 0 {
		return nil
	}

	cli := hubCfg.Clients[0]

	return &managed.ClientConfig{
		Name:    cli.Name,
		Server:  cli.Address(),
		Token:   cli.Secret,
		Version: fleet.VersionHash(data),
		Handler: &managed.Handler{
			Validate: func(cfgData []byte) error {
				_, parseErr := bgpconfig.LoadConfig(string(cfgData), "", nil)
				return parseErr
			},
			Cache: func(cfgData []byte) error {
				return store.WriteFile(configName, cfgData, 0)
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

	tlsConf := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // hub uses self-signed certs; cert pinning is a future spec
		MinVersion:         tls.VersionTLS13,
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
		return nil, fmt.Errorf("auth rejected")
	}

	rc := rpc.NewConn(conn, conn)
	mc := rpc.NewMuxConn(rc)
	defer mc.Close() //nolint:errcheck // cleanup

	resp, err := managed.FetchConfig(ctx, mc, "")
	if err != nil {
		return nil, err
	}

	if resp.Config == "" {
		return nil, fmt.Errorf("hub returned empty config")
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

// validInstanceName matches alphanumeric names with hyphens, max 64 chars.
// Same validation as plugin names -- prevents path traversal in blob keys.
var validInstanceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

// resolveDefaultConfig returns the config name from meta/instance/name or the fallback.
// Validates the name to prevent path traversal via blob key injection.
func resolveDefaultConfig(store storage.Storage) string {
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err != nil || len(data) == 0 {
		return "ze.conf"
	}
	name := strings.TrimSpace(string(data))
	if name == "" || !validInstanceName.MatchString(name) {
		return "ze.conf"
	}
	return name + ".conf"
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze - Ze toolchain

Usage:
  ze [--plugin <name>]... <config>   Start with config file
  ze <verb> <command> [options]      Execute command (same grammar as ze cli)

Verbs (dispatched via YANG command tree):
`)
	// Dynamic verb list from YANG tree.
	verbTree := cli.BuildCommandTree(false)
	command.WriteHelp(os.Stderr, verbTree, nil)

	fmt.Fprintf(os.Stderr, `
Tools:
  start        Start daemon (--web <port>, --insecure-web, --mcp <port>)
  init         Bootstrap database with SSH credentials
  config       Configuration management (validate, edit, migrate, ...)
  data         Blob store management
  schema       Schema discovery
  yang         YANG tree analysis and command docs
  cli          Interactive CLI for running daemons
  status       Check if daemon is running
  bgp          BGP protocol tools (decode, encode)
  plugin       Plugin system (rib, rr, gr, etc.)
  signal       Send signals to running daemon (reload, stop, quit)
  interface    Manage OS network interfaces
  exabgp       ExaBGP bridge tools
  completion   Generate shell completion scripts
  version      Show version
  help         Show this help (--ai for machine-readable reference)

Options:
  -d, --debug           Enable debug logging (sets ze.log=debug for all subsystems)
  -f <file>             Use filesystem directly, bypass blob store
  --plugin <name>       Load plugin before starting (repeatable)
  --plugins             List available internal plugins
  --pprof <addr:port>   Start pprof HTTP server (e.g. :6060)
  -V, --version         Show version and exit

Examples:
  ze config.conf                       Start with config
  ze --plugin ze.hostname config.conf  Start with hostname plugin
  ze --plugins                         List available plugins
  ze cli                               Interactive CLI
  ze show peer list                    Show peer list
  ze show help                         List available show commands
  ze del bgp peer 10.0.0.1            Remove a peer
  ze bgp decode <hex>                  Decode BGP message
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
