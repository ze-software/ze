// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package repository

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the post-verify repository checks: source anchors resolve, exported symbols have a cross-package caller, CLI commands have a .ci test, and an in-progress spec's acceptance criteria say how they are demonstrated",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// The answer carries the findings beside the changed set and the two
	// counts, so the any-shape operators render the whole document.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts both gates as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.
	parity.Claim(area, actions.Gates()...)
}
