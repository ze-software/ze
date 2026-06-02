// Register the bgp root command and its `show bgp decode` / `show bgp
// encode` offline shortcuts with the cmd/ze dispatcher. Imported by
// cmd/ze/main.go for its side effects.

package bgp

import (
	"strings"

	_ "codeberg.org/thomas-mangin/ze/cmd/ze/bgp/schema" // init() registers the show bgp decode/encode YANG module
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

// bgpCommands lists the bare user-facing subcommand names, kept in
// sync with the switch cases in Run (main.go). Used for suggestion
// matching and as the basis for the Subs string.
var bgpCommands = []string{"decode", "encode", "plugin"}

// bgpSubHints maps each command to its argument hint for display.
// Commands with no arguments are omitted.
var bgpSubHints = map[string]string{
	"decode": "decode <hex>",
	"encode": "encode <route>",
}

// subcommands returns the user-facing subcommand list as a
// comma-separated string. Each entry includes its argument hint
// where applicable.
func subcommands() string {
	display := make([]string, len(bgpCommands))
	for i, cmd := range bgpCommands {
		if hint, ok := bgpSubHints[cmd]; ok {
			display[i] = hint
		} else {
			display[i] = cmd
		}
	}
	return strings.Join(display, ", ")
}

func init() {
	cmdregistry.RegisterRoot("bgp", cmdregistry.Meta{
		Description: "BGP protocol tools",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        subcommands(),
	})
	cmdregistry.MustRegisterLocal("show bgp decode", func(args []string) int {
		return Run(append([]string{"decode"}, args...))
	})
	cmdregistry.MustRegisterLocal("show bgp encode", func(args []string) int {
		return Run(append([]string{"encode"}, args...))
	})
}
