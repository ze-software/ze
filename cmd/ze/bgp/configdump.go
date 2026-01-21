package bgp

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

func cmdConfigDump(args []string) int {
	fs := flag.NewFlagSet("config-dump", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp config-dump [options] <config>

Dump parsed configuration in human-readable or JSON format.
Useful for debugging config parsing issues.

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Read config file
	data, err := os.ReadFile(configPath) //nolint:gosec // Path from CLI
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		return 1
	}

	// Parse with schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		return 1
	}

	// Show any warnings
	if warnings := p.Warnings(); len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Warnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  %s\n", w)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Convert to typed config
	cfg, err := config.TreeToConfig(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting config: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	// Human-readable output
	printConfig(cfg)
	return 0
}

func printConfig(cfg *config.BGPConfig) {
	fmt.Printf("router-id: %s\n", uint32ToIP(cfg.RouterID))
	fmt.Printf("local-as: %d\n", cfg.LocalAS)
	if cfg.Listen != "" {
		fmt.Printf("listen: %s\n", cfg.Listen)
	}
	fmt.Println()

	for _, n := range cfg.Peers {
		fmt.Printf("peer %s:\n", n.Address)
		if n.Description != "" {
			fmt.Printf("  description: %s\n", n.Description)
		}
		if n.RouterID != 0 {
			fmt.Printf("  router-id: %s\n", uint32ToIP(n.RouterID))
		}
		if n.LocalAddress.IsValid() {
			fmt.Printf("  local-address: %s\n", n.LocalAddress)
		}
		if n.LocalAS != 0 {
			fmt.Printf("  local-as: %d\n", n.LocalAS)
		}
		if n.PeerAS != 0 {
			fmt.Printf("  peer-as: %d\n", n.PeerAS)
		}
		if n.HoldTime != 0 {
			fmt.Printf("  hold-time: %d\n", n.HoldTime)
		}
		if n.Passive {
			fmt.Printf("  passive: true\n")
		}
		if n.Hostname != "" {
			fmt.Printf("  hostname: %s\n", n.Hostname)
		}

		// Families
		if len(n.Families) > 0 {
			fmt.Printf("  families:\n")
			for _, f := range n.Families {
				fmt.Printf("    - %s\n", f)
			}
		}

		// Capabilities
		cap := n.Capabilities
		if cap.ASN4 || cap.RouteRefresh || cap.GracefulRestart || cap.AddPathSend || cap.AddPathReceive || cap.SoftwareVersion {
			fmt.Printf("  capabilities:\n")
			if cap.ASN4 {
				fmt.Printf("    asn4: true\n")
			}
			if cap.RouteRefresh {
				fmt.Printf("    route-refresh: true\n")
			}
			if cap.GracefulRestart {
				fmt.Printf("    graceful-restart: true (restart-time: %d)\n", cap.RestartTime)
			}
			if cap.AddPathSend {
				fmt.Printf("    add-path-send: true\n")
			}
			if cap.AddPathReceive {
				fmt.Printf("    add-path-receive: true\n")
			}
			if cap.SoftwareVersion {
				fmt.Printf("    software-version: true\n")
			}
		}

		// Static routes
		if len(n.StaticRoutes) > 0 {
			fmt.Printf("  static-routes:\n")
			for _, sr := range n.StaticRoutes {
				fmt.Printf("    - prefix: %s\n", sr.Prefix)
				if sr.NextHop != "" {
					fmt.Printf("      next-hop: %s\n", sr.NextHop)
				}
				if sr.LocalPreference != 0 {
					fmt.Printf("      local-preference: %d\n", sr.LocalPreference)
				}
				if sr.MED != 0 {
					fmt.Printf("      med: %d\n", sr.MED)
				}
				if sr.Community != "" {
					fmt.Printf("      community: %s\n", sr.Community)
				}
				if sr.ASPath != "" {
					fmt.Printf("      as-path: %s\n", sr.ASPath)
				}
			}
		}

		fmt.Println()
	}

	// Plugins
	if len(cfg.Plugins) > 0 {
		fmt.Printf("plugins:\n")
		for _, p := range cfg.Plugins {
			fmt.Printf("  - name: %s\n", p.Name)
			if p.Run != "" {
				fmt.Printf("    run: %s\n", p.Run)
			}
			if p.Encoder != "" {
				fmt.Printf("    encoder: %s\n", p.Encoder)
			}
		}
	}
}
