// Design: docs/architecture/api/commands.md — bgp command ownership
//
// Register the `bgp` root command and its `show bgp decode` / `show bgp encode`
// offline shortcuts with the importable command registry. This is the owner
// package: the offline BGP tools CLI lives with internal/component/bgp, not
// under cmd/ze.
package cli

import (
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"

	// init() registers the show bgp decode/encode YANG module (ze-bgp-tools-cmd).
	_ "github.com/ze-software/ze/internal/component/bgp/cli/yang"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Subcommand tokens accepted by Run (main.go). One name for each token, so the
// switch, the suggestion list, and the hint map cannot drift apart.
const (
	bgpCmdDecode = "decode"
	bgpCmdEncode = "encode"
	bgpCmdPlugin = "plugin"
	bgpCmdHelp   = "help"
)

// bgpCommands lists the bare user-facing subcommand names, kept in sync with
// the switch cases in Run (main.go). Used for suggestion matching and as the
// basis for the Subs string.
var bgpCommands = []string{bgpCmdDecode, bgpCmdEncode, bgpCmdPlugin}

// bgpSubHints maps each command to its argument hint for display. Commands with
// no arguments are omitted.
var bgpSubHints = map[string]string{
	bgpCmdDecode: "decode <hex>",
	bgpCmdEncode: "encode <route>",
}

// subcommands returns the user-facing subcommand list as a comma-separated
// string. Each entry includes its argument hint where applicable.
func subcommands() string {
	display := make([]string, len(bgpCommands))
	for i, cmd := range bgpCommands {
		if hint, ok := bgpSubHints[cmd]; ok {
			display[i] = hint
		} else {
			display[i] = cmd
		}
	}
	return textbuf.Join(display, ", ")
}

func init() {
	// Publish the hex-packet decoder through the leaf registry seam so the web
	// tool page can decode in-process without cmd/ze/hub importing this
	// package (which would pin internal/component/bgp into every binary).
	pluginreg.SetPacketDecoder(decodeHexPacket)

	registry.MustRegisterRootHandler("bgp", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "BGP protocol tools",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        subcommands(),
	})
	registry.MustRegisterLocal("show bgp decode", func(args []string) int {
		return Run(append([]string{bgpCmdDecode}, args...))
	})
	registry.MustRegisterLocal("show bgp encode", func(args []string) int {
		return Run(append([]string{bgpCmdEncode}, args...))
	})
}
