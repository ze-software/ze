// Register the completion root command with the command registry.

package completion

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

// shells lists the user-facing shell names for completion generation.
// "nu" is an alias for "nushell" and not listed separately.
// "words" and "peers" are internal helpers, not user-facing.
var shells = []string{"bash", "zsh", "fish", "nushell"}

// subcommands returns the shell list as a comma-separated string,
// derived from shells (the single source of truth).
func subcommands() string {
	return strings.Join(shells, ", ")
}

func init() {
	registry.MustRegisterRootHandler("completion", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Shell completion scripts",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        subcommands(),
	})
}
