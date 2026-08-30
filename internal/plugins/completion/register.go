// Register the completion root command with the command registry.
// codegen:skip -- CLI command wired via cmd/ze/main.go; imports cli/client which cycles with plugin/all.

package completion

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The shell names ze generates a completion script for. "nu" is an alias for
// nushell, accepted on the command line and not listed as a shell of its own.
const (
	shellBash    = "bash"
	shellZsh     = "zsh"
	shellFish    = "fish"
	shellNushell = "nushell"
)

// shells lists the user-facing shell names for completion generation.
// "words" and "peers" are internal helpers, not user-facing.
var shells = []string{shellBash, shellZsh, shellFish, shellNushell}

// subcommands returns the shell list as a comma-separated string,
// derived from shells (the single source of truth).
func subcommands() string {
	return textbuf.Join(shells, ", ")
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
