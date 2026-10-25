// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the gate table and dispatch this registration exposes
//
// One package, one register.go, one init(). The later composition cut imports
// this package into both le binaries.
package chaosselftest

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(Area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "The chaos simulator's unit tests, the orchestrator's CLI tests under its reduced tags, and its lint, each runnable alone or all three in order.",
		ShortHelp:   "test, CLI-test and lint the chaos tooling",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(Area, Actions)

	// The listing and a sweep contain different row sets. The document shape
	// keeps every pipe operator honest instead of selecting one set by accident.
	leroot.RegisterShape(Area, command.ShapeDoc)

	// The claims come from the same table that Answer dispatches.

}
