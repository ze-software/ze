// Register the interface root command and `show interface` offline
// shortcut with the cmd/ze dispatcher.

package iface

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands, derived from ifaceCommands (the single source of truth
// shared with the known-subcommand gate in Run).
func subcommands() string {
	sorted := make([]string, len(ifaceCommands))
	copy(sorted, ifaceCommands)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func init() {
	cmdregistry.RegisterRoot("interface", cmdregistry.Meta{
		Description: "Manage OS network interfaces",
		Mode:        "offline",
		Section:     cmdregistry.SectionConfiguration,
		Subs:        subcommands(),
	})
	cmdregistry.MustRegisterLocal("show interface", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	})
}
