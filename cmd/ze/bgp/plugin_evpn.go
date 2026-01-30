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
//	ze bgp plugin evpn --json 02210001252C...   # JSON output (default format)
//	ze bgp plugin evpn --text 02210001252C...   # text output
//	ze bgp plugin evpn --json -                 # read hex from stdin
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin evpn --decode                 # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginEVPN(args []string) int {
	fs := flag.NewFlagSet("plugin evpn", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	decodeMode := fs.Bool("decode", false, "Engine decode protocol mode (reads commands from stdin)")
	textHex := fs.String("text", "", "Decode hex and output human-readable text (use - for stdin)")
	jsonHex := fs.String("json", "", "Decode hex and output JSON (use - for stdin)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(evpn.GetEVPNYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	evpn.SetEVPNLogger(slogutil.PluginLogger("evpn", *logLevel))

	// CLI Mode: --text <hex> or --json <hex> (mutually exclusive)
	if *textHex != "" && *jsonHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: --json and --text are mutually exclusive")
		return 1
	}
	if *textHex != "" || *jsonHex != "" {
		hex := *jsonHex
		textOutput := false
		if *textHex != "" {
			hex = *textHex
			textOutput = true
		}
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
		return evpn.RunCLIDecode(hex, textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode: protocol commands on stdin (used by ze bgp decode)
	if *decodeMode {
		return evpn.RunEVPNDecode(os.Stdin, os.Stdout)
	}

	// Engine Mode: full plugin with startup protocol
	plugin := evpn.NewEVPNPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
