// Package main provides the ze command entry point.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/bgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/cmd/ze/config"
	"codeberg.org/thomas-mangin/ze/cmd/ze/exabgp"
	"codeberg.org/thomas-mangin/ze/cmd/ze/hub"
	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// Parse --plugin flags before command dispatch
	var plugins []string
	args := os.Args[1:]
	for len(args) > 0 && args[0] == "--plugin" {
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "error: --plugin requires an argument\n")
			os.Exit(1)
		}
		plugins = append(plugins, args[1])
		args = args[2:]
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
	case "cli":
		os.Exit(cli.Run(args[1:]))
	case "config":
		os.Exit(zeconfig.Run(args[1:]))
	case "exabgp":
		os.Exit(exabgp.Run(args[1:]))
	case "version":
		fmt.Printf("ze %s\n", version)
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
			os.Exit(hub.Run(arg, plugins))
		}
		switch detectConfigType(arg) {
		case config.ConfigTypeBGP:
			// Start BGP daemon in-process via hub
			os.Exit(hub.Run(arg, plugins))
		case config.ConfigTypeHub:
			// Start hub orchestrator (forks external plugins)
			os.Exit(hub.Run(arg, plugins))
		case config.ConfigTypeUnknown:
			fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
			os.Exit(1)
		}
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
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
  --plugin <name>   Load plugin before starting (repeatable)
  --plugins         List available internal plugins

Commands:
  cli      Interactive CLI for running daemons
  bgp      BGP daemon and tools
  exabgp   ExaBGP tools
  version  Show version
  help     Show this help

Examples:
  ze config.conf                      Start with config
  ze --plugin ze.hostname config.conf Start with hostname plugin
  ze --plugins                        List available plugins
  ze cli                              Interactive CLI
  ze cli --run "peer list"            Execute CLI command
  ze bgp help                         Show BGP commands
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
