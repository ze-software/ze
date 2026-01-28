package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
)

// cmdPluginRR runs the Route Server plugin.
func cmdPluginRR(args []string) int {
	fs := flag.NewFlagSet("plugin rr", flag.ExitOnError)
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	_ = fs.Parse(args)

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(rrYANG)
		return 0
	}

	server := rr.NewRouteServer(os.Stdin, os.Stdout)
	return server.Run()
}

// rrYANG is the YANG schema for the RR plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const rrYANG = ""
