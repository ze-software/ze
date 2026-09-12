// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package discoveryindex

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the generated package map in ai/PACKAGE-MAP.md: rewrite it from the tree",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set, the actions or
	// the packages, so the row operators act on them.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts both gates as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

	// The map is DERIVED, so it is not tracked and no gate compares a
	// re-render against a committed copy. A write to a file it takes text from
	// deletes it, and a command that names it rebuilds it before that command
	// runs.
	// Built at session start: a reader greps the map without naming it
	// (`grep -rn ResolveBGPTree`), so an absent map answers "no match" for a
	// tree nobody could see was missing it (derived.SessionStartPolicy).
	derived.Register(derived.Artifact{
		Path:         OutputRel,
		Feeds:        func(_, path string) bool { return IsSourcePath(path) },
		Rebuild:      func(root string) error { _, err := Update(root); return err },
		SessionStart: derived.SessionStartBuild,
	})
}
