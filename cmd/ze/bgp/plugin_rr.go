package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
)

// cmdPluginRR runs the Route Server plugin.
//
// CLI Mode:
//
//	ze bgp plugin rr --features                    # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginRR(args []string) int {
	fs := flag.NewFlagSet("plugin rr", flag.ExitOnError)
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	capaHex := fs.String("capa", "", "Decode capability hex (not supported by this plugin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex (not supported by this plugin)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	// RR plugin has no decode features - empty output (no newline)
	if *showFeatures {
		return 0
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(rrYANG)
		return 0
	}

	// Unsupported feature: --nlri
	if *nlriHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'rr' does not support --nlri (available: none)")
		return 1
	}

	// Unsupported feature: --capa
	if *capaHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'rr' does not support --capa (available: none)")
		return 1
	}

	// Engine Mode: full plugin with startup protocol
	server := rr.NewRouteServer(os.Stdin, os.Stdout)
	return server.Run()
}

// rrYANG is the YANG schema for the RR plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const rrYANG = ""
