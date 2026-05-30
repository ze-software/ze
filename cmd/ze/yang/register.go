// Register the yang root command and its `show yang *` offline
// shortcuts with the cmd/ze dispatcher.

package yang

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

// yangCommands lists the user-facing subcommand names, kept in sync
// with the switch cases in Run (main.go). "help" is excluded.
var yangCommands = []string{"tree", "completion", "doc"}

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands (single source of truth for Meta.Subs and error messages).
func subcommands() string {
	sorted := make([]string, len(yangCommands))
	copy(sorted, yangCommands)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func init() {
	cmdregistry.RegisterRoot("yang", cmdregistry.Meta{
		Description: "YANG tree analysis",
		Mode:        "offline",
		Section:     cmdregistry.SectionConfiguration,
		Subs:        subcommands(),
	})
	cmdregistry.MustRegisterLocal("show yang tree", func(args []string) int {
		return Run(append([]string{"tree"}, args...))
	})
	cmdregistry.MustRegisterLocal("show yang completion", func(args []string) int {
		return Run(append([]string{"completion"}, args...))
	})
	cmdregistry.MustRegisterLocal("show yang doc", func(args []string) int {
		return Run(append([]string{"doc"}, args...))
	})
}
