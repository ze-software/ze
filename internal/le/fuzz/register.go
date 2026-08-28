// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package fuzz

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "Go fuzzing: every `func Fuzz` under internal/, discovered at run time",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action writes.
		SubsFunc: Subs,
	})

	// Each answer has one row set: actions, planned runs, or completed runs.
	// The row operators therefore act on that set instead of being refused.
	leroot.RegisterShape(area, command.ShapeMap)

	// No parity.Claim: these operator actions have no census gate.
}
