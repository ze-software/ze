// Design: docs/architecture/api/commands.md — yang command ownership
//
// Register the `yang` root command and its `show yang *` offline shortcuts with
// the importable command registry. This is the owner package: the offline YANG
// tree-analysis CLI lives with internal/component/config/yang, not under cmd/ze.
package cli

import (
	"sort"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// yangCommands lists the user-facing subcommand names, kept in sync with the
// switch cases in Run (main.go). "help" is excluded.
var yangCommands = []string{"tree", "completion", "doc"}

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands (single source of truth for Meta.Subs and error messages).
func subcommands() string {
	sorted := make([]string, len(yangCommands))
	copy(sorted, yangCommands)
	sort.Strings(sorted)
	return textbuf.Join(sorted, ", ")
}

func init() {
	registry.MustRegisterRootHandler("yang", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "YANG tree analysis",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        subcommands(),
	})
	registry.MustRegisterLocal("show yang tree", func(args []string) int {
		return Run(append([]string{"tree"}, args...))
	})
	registry.MustRegisterLocal("show yang completion", func(args []string) int {
		return Run(append([]string{"completion"}, args...))
	})
	registry.MustRegisterLocal("show yang doc", func(args []string) int {
		return Run(append([]string{"doc"}, args...))
	})
}
