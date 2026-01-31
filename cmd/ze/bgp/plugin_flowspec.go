package bgp

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginFlowSpec runs the FlowSpec family plugin.
// It handles decoding of FlowSpec NLRI (RFC 8955, 8956).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin flowspec --nlri 0718...              # JSON output (default)
//	ze bgp plugin flowspec --nlri 0718... --text       # text output
//	ze bgp plugin flowspec --nlri - --family ipv4/flow # read hex from stdin
//	ze bgp plugin flowspec --features                  # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin flowspec --decode                    # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginFlowSpec(args []string) int {
	fs := flag.NewFlagSet("plugin flowspec", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	showFeatures := fs.Bool("features", false, "List supported decode features")
	decodeMode := fs.Bool("decode", false, "Engine decode protocol mode (reads commands from stdin)")
	nlriHex := fs.String("nlri", "", "Decode NLRI hex and output JSON (use - for stdin)")
	capaHex := fs.String("capa", "", "Decode capability hex (not supported by this plugin)")
	textOutput := fs.Bool("text", false, "Output human-readable text instead of JSON")
	family := fs.String("family", "ipv4/flow", "Address family (ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn)")
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
		fmt.Print(flowspec.GetFlowSpecYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	flowspec.SetFlowSpecLogger(slogutil.PluginLogger("flowspec", *logLevel))

	// Unsupported feature: --capa
	if *capaHex != "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: plugin 'flowspec' does not support --capa (available: --nlri)")
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
		return flowspec.RunCLIDecode(hex, *family, *textOutput, os.Stdout, os.Stderr)
	}

	// Engine Decode Mode: protocol commands on stdin (used by ze bgp decode)
	if *decodeMode {
		return flowspec.RunFlowSpecDecode(os.Stdin, os.Stdout)
	}

	// Engine Mode: full plugin with startup protocol
	plugin := flowspec.NewFlowSpecPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
