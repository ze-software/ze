package bgp

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginHostname runs the hostname (FQDN) capability plugin.
// It receives per-peer hostname/domain config and registers FQDN capabilities.
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin hostname --capa 07726f7574657231...        # JSON output (default)
//	ze bgp plugin hostname --capa 07726f7574657231... --text # text output
//	ze bgp plugin hostname --capa -                          # read hex from stdin
//	ze bgp plugin hostname --features                        # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin hostname --decode                          # reads "decode capability ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginHostname(args []string) int {
	fs := flag.NewFlagSet("plugin hostname", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	decodeMode := fs.Bool("decode", false, "Engine decode protocol mode (reads commands from stdin)")
	capaHex := fs.String("capa", "", "Decode capability hex and output JSON (use - for stdin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex (not supported by this plugin)")
	textOutput := fs.Bool("text", false, "Output human-readable text instead of JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	if *showFeatures {
		fmt.Println("capa yang")
		return 0
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(hostname.GetYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	hostname.ConfigureLogger(slogutil.PluginLogger("hostname", *logLevel))

	// Unsupported feature: --nlri
	if *nlriHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'hostname' does not support --nlri (available: --capa)")
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
		return hostname.RunCLIDecode(hex, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode: protocol commands on stdin (used by ze bgp decode)
	if *decodeMode {
		return hostname.RunDecodeMode(os.Stdin, os.Stdout)
	}

	// Engine Mode: full plugin with startup protocol
	plugin := hostname.NewHostnamePlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
