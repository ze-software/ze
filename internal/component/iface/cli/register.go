// Design: docs/architecture/api/commands.md — interface command ownership
//
// Register the `interface` root command and its `show interface` offline
// shortcut with the importable command registry. This is the owner package:
// the interface command lives with internal/component/iface, not under cmd/ze.
// cmd/ze/main.go dispatches `ze interface ...` through the registry handler
// registered here, so no central static switch case is needed.
package cli

import (
	"sort"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands, derived from ifaceCommands (the single source of truth shared
// with the known-subcommand gate in Run).
func subcommands() string {
	sorted := make([]string, len(ifaceCommands))
	copy(sorted, ifaceCommands)
	sort.Strings(sorted)
	return textbuf.Join(sorted, ", ")
}

func init() {
	registry.MustRegisterRootHandler("interface", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Manage OS network interfaces",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        subcommands(),
	})
	registry.MustRegisterLocal("show interface", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	})
}
