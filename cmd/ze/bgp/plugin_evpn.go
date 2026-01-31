package bgp

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginEVPN runs the EVPN family plugin.
// It handles decoding of EVPN NLRI (RFC 7432, 9136).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin evpn --nlri 02210001252C...        # JSON output (default)
//	ze bgp plugin evpn --nlri 02210001252C... --text # text output
//	ze bgp plugin evpn --nlri -                      # read hex from stdin
//	ze bgp plugin evpn --features                    # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin evpn --decode                      # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginEVPN(args []string) int {
	fs := flag.NewFlagSet("plugin evpn", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	decodeMode := fs.Bool("decode", false, "Engine decode protocol mode (reads commands from stdin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex and output JSON (use - for stdin)")
	capaHex := fs.String("capa", "", "Decode capability hex (not supported by this plugin)")
	textOutput := fs.Bool("text", false, "Output human-readable text instead of JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output features if requested
	if *showFeatures {
		fmt.Println("nlri")
		return 0
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(evpn.GetEVPNYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	evpn.SetEVPNLogger(slogutil.PluginLogger("evpn", *logLevel))

	// Unsupported feature: --capa
	if *capaHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'evpn' does not support --capa (available: --nlri)")
		return 1
	}

	// CLI Mode: --nlri <hex> [--text]
	if *nlriHex != "" {
		hex := *nlriHex
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
		return evpn.RunCLIDecode(hex, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode: protocol commands on stdin (used by ze bgp decode)
	if *decodeMode {
		return evpn.RunEVPNDecode(os.Stdin, os.Stdout)
	}

	// Engine Mode: full plugin with startup protocol
	plugin := evpn.NewEVPNPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
