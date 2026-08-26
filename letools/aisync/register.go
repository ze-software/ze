// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.
//
// No parity claim exists. ze-ai-skills-sync and ze-ai-sync-check are Make
// targets that run a shell script. They are not among the 156 Python gates that
// the Python le declares, so these verbs cannot leave that census. They do
// remove a shell file from the OTHER census. That count falls when the swap
// deletes the script, not when this command appears.

package aisync

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the generated agent files: sync every tool's copy of the skills and instructions, or check them",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc, because the answers are not one row set: the listing is rows,
	// and a sync or a check is one verdict carrying three lists. rowsInKeyed
	// cannot choose between them, so the shape says there are no rows and the
	// engine refuses `| count` by name rather than counting something
	// plausible. `| json`, `| yaml` and `| table` render both.
	command.RegisterShape([]string{area}, command.ShapeDoc)
}
