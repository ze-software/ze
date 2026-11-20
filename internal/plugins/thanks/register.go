// Design: docs/architecture/api/commands.md — local-data commands at an offline root, served by the daemon too
// Related: thanks.go — the two handlers and the credits they answer from
// Related: yang/ze-thanks-cmd.yang — the daemon paths the registrations name

// Package thanks owns `ze thanks` and `ze thanks licence`: the third-party
// works this binary carries, and their licence texts. Both are local-data
// commands at the offline root `thanks`, which is not a verb, so no YANG node
// sits at their own paths: the daemon serves the same handlers as
// `show thanks list` and `show thanks licence`, the paths ze-thanks-cmd.yang
// declares and each registration names (registry.Meta.DaemonPath). The data is
// internal/core/credits; this package only answers it.
package thanks

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// modeOffline is the registry mode of a command that needs no daemon.
const modeOffline = "offline"

func init() {
	registry.RegisterRoot("thanks", registry.Meta{
		ShortHelp: "List the third-party works Ze is built on, with their licences",
		Mode:      modeOffline,
		Section:   registry.SectionSystem,
		Subs:      "licence",
	})
	source := compiledCredits{}
	// Each path and each DaemonPath is a literal, not pathLicence, commandList
	// or commandLicence: the registry's coverage test and the YANG/handler
	// contract (internal/le/doc/yangcontract declaredDaemonPath) read them from
	// the source and refuse a name they cannot resolve.
	// TestThanksDaemonPathsMatchTheConstants compares the literals with the
	// constants the refusals and the shape declarations use.
	command.MustRegisterLocalData("thanks", listHandler(source), registry.Meta{
		ShortHelp: "List the third-party works this binary carries, with version, licence and copyright.",
		Description: "Each row names one work Ze is built on, the version this binary carries, its licence, " +
			"its copyright holder and why Ze uses it.",
		Mode:       modeOffline,
		DaemonPath: "show thanks list",
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("thanks licence", licenceHandler(source), registry.Meta{
		ShortHelp: "Print the licence texts of the third-party works this binary carries.",
		Description: "The document holds each licence text as its work ships it, after the works it covers " +
			"and their copyright lines. The name pipe operation narrows it to one work.",
		Mode:       modeOffline,
		DaemonPath: "show thanks licence",
	}, command.RenderLocalAnswer)

	// The shape and the column order belong to the answer, so each is declared
	// on both paths that return it: the local root and the daemon path.
	command.RegisterShape([]string{"thanks", commandList}, command.ShapeTab)
	command.RegisterColumns([]string{"thanks", commandList}, command.ColumnOrder{
		keyGroup, keyName, keyVersion, keyLicence, keyCopyright, keyWhy, keySource,
	})
	command.RegisterShape([]string{pathLicence, commandLicence}, command.ShapeBlob)
	// Column lookup takes the longest registered prefix, so without this empty
	// declaration the document at `thanks licence` would inherit the list's
	// columns from `thanks`. The daemon paths are siblings and need none.
	command.RegisterColumns([]string{pathLicence})
	// One work is a selector, and a selector is a pipe operation after the path
	// (docs/architecture/api/commands.md, "Design decisions"): the engine hands
	// it to licenceHandler in the declared list on every route.
	command.RegisterPipeFilters([]string{pathLicence, commandLicence}, command.PipeFilter{
		Name:        operationName,
		Description: "Print the licence of the one work with this exact name.",
		Argument:    command.ArgText,
	})
}
