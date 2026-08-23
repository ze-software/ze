// Design: docs/architecture/api/commands.md — data (ZeFS) command ownership
//
// Register the `data` root command and its `show data *` offline shortcuts with
// the importable command registry. This is the owner package: the offline ZeFS
// blob-store management CLI lives with internal/component/config/storage, not
// under cmd/ze.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("data", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "ZeFS blob store management",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "import, rm, ls, cat",
	})
	// ls and registered answer with DATA, so their answers reach the pipe
	// layer. They printed a table and returned an exit code, while YANG
	// declared a wire method for each that no daemon handler implements.
	registry.MustRegisterLocalData("show data ls", dataLs, registry.Meta{
		Description: "The keys the ZeFS blob store holds. Narrow them with a prefix.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show data registered", dataRegistered, registry.Meta{
		Description: "The key patterns the code declares, and what each one holds.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)

	// `show data cat` answers the BYTES of one stored file, which may be YAML,
	// JSON, a certificate or a binary blob. Those bytes are the answer: wrapping
	// them in a record would corrupt the one use the command has, and no pipe
	// operator has anything to do with them. It keeps its plain handler, and the
	// published page says it reaches no pipe layer, which is the truth.
	registry.MustRegisterLocal("show data cat", func(args []string) int {
		return Run(append([]string{"cat"}, args...))
	})

	command.RegisterShape([]string{"show data ls", "show data registered"}, command.ShapeTab)
	command.RegisterColumns([]string{"show data ls"}, command.ColumnOrder{"key"})
	command.RegisterColumns([]string{"show data registered"},
		command.ColumnOrder{"pattern", "description"})
}
