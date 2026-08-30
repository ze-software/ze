// Design: docs/architecture/api/commands.md — yang command ownership
//
// Register the `yang` root command and its `show yang *` offline shortcuts with
// the importable command registry. This is the owner package: the offline YANG
// tree-analysis CLI lives with internal/component/config/yang, not under cmd/ze.
package cli

import (
	"sort"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Subcommand names that appear in more than one place: the switch in Run, the
// help page, and the list below.
const (
	subTree       = "tree"
	subCompletion = "completion"
	subDoc        = "doc"
)

// modeOffline marks a command that answers from the compiled-in YANG modules
// without a running daemon. The help output groups commands by this tag.
const modeOffline = "offline"

// yangCommands lists the user-facing subcommand names, kept in sync with the
// switch cases in Run (main.go). "help" is excluded.
var yangCommands = []string{subTree, subCompletion, subDoc}

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
		Mode:        modeOffline,
		Section:     registry.SectionConfiguration,
		Subs:        subcommands(),
	})
	// tree and completion answer with DATA, so their answers reach the pipe
	// layer. Both printed text and returned an exit code, while YANG declared a
	// wire method for each that no daemon handler implements.
	registry.MustRegisterLocalData("show yang tree", dataTree, registry.Meta{
		Description: "The unified config and command tree. Narrow it with --commands or --config.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show yang completion", dataCompletion, registry.Meta{
		Description: "Prefix collisions in the config and command trees.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)

	// `show yang doc` renders documentation PROSE for a reader, and the same
	// facts already reach a machine through `ze help command --json`.
	// Inventing a second record for them would be a second surface to keep
	// true, so it keeps its plain handler.
	registry.MustRegisterLocal("show yang doc", func(args []string) int {
		return Run(append([]string{subDoc}, args...))
	})

	// Both answer ROWS. The tree's rows are its TOP-LEVEL nodes, each carrying
	// its own children, so `| first 1` answers one subtree and `| match` keeps
	// the roots that hold the text. It was declared `doc` first, on the reading
	// that a tree is one document; running it showed formatTreeJSON emits a
	// top-level array, and a declaration that disagrees with the answer would
	// publish a refusal the product does not make.
	command.RegisterShape([]string{"show yang tree", "show yang completion"}, command.ShapeTab)
	command.RegisterColumns([]string{"show yang tree"},
		command.ColumnOrder{"name", "kind", "source", "description"})
}
