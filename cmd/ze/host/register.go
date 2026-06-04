// Register the `ze host` entry point with the cmd/ze dispatcher.
// Imported by cmd/ze/main.go for its side effects.

package host

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	// The section list is derived from validSections (the single source
	// of truth in host.go), not hardcoded here — adding a new section
	// only requires editing the map, not the help metadata.
	registry.RegisterRoot("host", registry.Meta{
		Description: "Show hardware inventory for this box (offline)",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "show [" + strings.ReplaceAll(sectionList(), ", ", "|") + "] [--text]",
	})
	registry.MustRegisterLocalMeta("host show", RunShow, registry.Meta{
		Description: "Show hardware details by section (cpu, nic, dmi, memory, thermal, storage, kernel). JSON by default, --text for human-readable.",
	})
	registry.MustRegisterLocalMeta("host", RunHint, registry.Meta{
		Description: "Show hardware inventory for this box (offline)",
	})
}
