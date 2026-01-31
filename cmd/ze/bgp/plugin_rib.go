package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
//
// CLI Mode:
//
//	ze bgp plugin rib --features                   # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginRib(args []string) int {
	fs := flag.NewFlagSet("plugin rib", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	capaHex := fs.String("capa", "", "Decode capability hex (not supported by this plugin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex (not supported by this plugin)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	// RIB plugin has no decode features - empty output (no newline)
	if *showFeatures {
		return 0
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(ribYANG)
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	rib.SetLogger(slogutil.PluginLogger("rib", *logLevel))

	// Unsupported feature: --nlri
	if *nlriHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'rib' does not support --nlri (available: none)")
		return 1
	}

	// Unsupported feature: --capa
	if *capaHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'rib' does not support --capa (available: none)")
		return 1
	}

	// Engine Mode: full plugin with startup protocol
	manager := rib.NewRIBManager(os.Stdin, os.Stdout)
	return manager.Run()
}

// ribYANG is the YANG schema for the RIB plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const ribYANG = ""
