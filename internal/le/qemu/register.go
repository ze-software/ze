// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package qemu

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "proofs that boot a real appliance image in a virtual machine and ask it what it did",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc is required because one answer is a document and the other is a
	// row set. The listing is rows. A proof report is one verdict with a kernel
	// command line and a console tail. rowsInKeyed cannot choose between them.
	//
	// The shape therefore declares no rows. The engine refuses `| count` by name
	// before a virtual machine starts. `| json`, `| yaml`, and `| table` render
	// both shapes.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census counts each gate as ported from here, read out of the action
	// table rather than from a second hand-typed list.

}
