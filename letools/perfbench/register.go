// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package perfbench

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "suggest a perf run when BGP data-plane code changed since the last one",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: Subs,
	})

	// ShapeDoc, because the two answers are not one row set: the listing is
	// rows and the nudge is one verdict carrying a file list. rowsInKeyed
	// cannot choose between them, so the shape says there are no rows and the
	// engine refuses `| count` by name. `| json`, `| yaml` and `| table`
	// render both.
	command.RegisterShape([]string{area}, command.ShapeDoc)

	// The census counts this gate as ported from here, read out of the action
	// table rather than from a second hand-typed list. No ClaimForked call: the
	// nudge reads git and answers in Go, so nothing here starts a script.
	parity.Claim(area, actions.Gates()...)
}
