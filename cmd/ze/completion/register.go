// Register the completion root command with the cmd/ze dispatcher.

package completion

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
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
	cmdregistry.RegisterRoot("completion", cmdregistry.Meta{
		Description: "Shell completion scripts",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        subcommands(),
	})
}
