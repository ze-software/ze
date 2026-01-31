package bgp

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginGR runs the Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin gr --capa 0078000101...            # JSON output (default)
//	ze bgp plugin gr --capa 0078000101... --text     # text output
//	ze bgp plugin gr --capa -                        # read hex from stdin
//	ze bgp plugin gr --features                      # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginGR(args []string) int {
	fs := flag.NewFlagSet("plugin gr", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	capaHex := fs.String("capa", "", "Decode capability hex and output JSON (use - for stdin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex (not supported by this plugin)")
	textOutput := fs.Bool("text", false, "Output human-readable text instead of JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	if *showFeatures {
		fmt.Println("capa")
		return 0
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(grYANG)
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	gr.SetLogger(slogutil.PluginLogger("gr", *logLevel))

	// Unsupported feature: --nlri
	if *nlriHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'gr' does not support --nlri (available: --capa)")
		return 1
	}

	// CLI Mode: --capa <hex> [--text]
	if *capaHex != "" {
		hex := *capaHex
		if hex == "-" {
			// Read single line from stdin
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				hex = strings.TrimSpace(scanner.Text())
			} else {
				_, _ = fmt.Fprintln(os.Stderr, "error: no input on stdin")
				return 1
			}
		}
		return gr.RunCLIDecode(hex, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Mode: full plugin with startup protocol
	plugin := gr.NewGRPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}

// grYANG is the YANG schema for the GR plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const grYANG = ""
