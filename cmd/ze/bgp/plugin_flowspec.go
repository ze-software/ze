package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginFlowSpec runs the FlowSpec family plugin.
// It handles decoding of FlowSpec NLRI (RFC 8955, 8956).
func cmdPluginFlowSpec(args []string) int {
	fs := flag.NewFlagSet("plugin flowspec", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	decodeMode := fs.Bool("decode", false, "Run in decode mode (for ze bgp decode)")
	_ = fs.Parse(args)

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(flowspec.GetFlowSpecYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	flowspec.SetFlowSpecLogger(slogutil.PluginLogger("flowspec", *logLevel))

	// Decode mode: read NLRI decode requests, output JSON
	if *decodeMode {
		return flowspec.RunFlowSpecDecode(os.Stdin, os.Stdout)
	}

	plugin := flowspec.NewFlowSpecPlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
