// Design: docs/guide/mrt-analysis.md -- the MRT analysis commands
// Overview: mrt.go -- the dispatch this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package mrt

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "analyze, filter, convert, replay and serve MRT files: `le mrt <subcommand> [options]`",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  subcommands,
	})

	// A bare `le mrt` answers one row per subcommand, so the table shape
	// renders it and every pipe operator reads the same rows.
	leroot.RegisterShape(name, command.ShapeTab)
}
