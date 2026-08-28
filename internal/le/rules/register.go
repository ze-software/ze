// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package rules

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the rule corpus in ai/rules/: lint and render it, map hook enforcement, " +
			"and report matched rules unread in a session transcript",
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer contains one row set: actions, gate-map sets, or a transcript
	// coverage report. Thus, row operators apply to each answer.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts these gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.

}
