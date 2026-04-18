// Register the `ze host` entry point with the cmd/ze dispatcher.
// Imported by cmd/ze/main.go for its side effects.

package host

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	// The section list is derived from validSections (the single source
	// of truth in host.go), not hardcoded here — adding a new section
	// only requires editing the map, not the help metadata.
	cmdregistry.RegisterRoot("host", cmdregistry.Meta{
		Description: "Show hardware inventory (CPU, NICs, DMI, memory, thermal, storage, kernel)",
		Mode:        "offline",
		Subs:        "show [" + strings.ReplaceAll(sectionList(), ", ", "|") + "] [--text]",
	})
	cmdregistry.MustRegisterLocal("host show", RunShow)
	// Bare `ze host` (no subcommand) prints a one-line hint at the
	// intended subcommand rather than the generic "unknown command"
	// banner. Surfaces the available shape without forcing the
	// operator to consult --help.
	cmdregistry.MustRegisterLocal("host", RunHint)
}
