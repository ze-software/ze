package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginHostname runs the hostname (FQDN) capability plugin.
// It receives per-peer hostname/domain config and registers FQDN capabilities.
func cmdPluginHostname(args []string) int {
	fs := flag.NewFlagSet("plugin hostname", flag.ExitOnError)
	logLevel := fs.String("log-level", "disabled", "Log level (disabled, debug, info, warn, err)")
	showYang := fs.Bool("yang", false, "Output YANG schema and exit")
	_ = fs.Parse(args)

	// Output YANG schema if requested
	if *showYang {
		fmt.Print(hostname.GetYANG())
		return 0
	}

	// Configure plugin logger (CLI flag takes precedence, then env var hierarchy)
	hostname.ConfigureLogger(slogutil.PluginLogger("hostname", *logLevel))

	plugin := hostname.NewHostnamePlugin(os.Stdin, os.Stdout)
	return plugin.Run()
}
