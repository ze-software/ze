// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package docstocode

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the two generated doc indexes, ai/DOCS-TO-CODE.md and its reverse ai/CODE-TO-DOCS.md: check either against the tree, or rewrite it",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set, the actions or
	// the references, so the row operators act on them.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts each gate as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.

	// Both indexes are DERIVED and untracked since c03dbe18a8. Until the
	// registry existed, the session-start hook named them in two hardcoded
	// os.Stat blocks and rebuilt each only when it was ABSENT, so an index that
	// existed and no longer matched the tree was never rebuilt.
	// Both indexes are built at session start, for the reason
	// derived.SessionStartPolicy states: an unnamed grep is how a reader
	// reaches them, and the two render in about a second between them.
	derived.Register(derived.Artifact{
		Path:         OutputRel,
		Feeds:        func(_, path string) bool { return IsDesignSource(path) },
		Rebuild:      func(root string) error { _, err := Update(root); return err },
		SessionStart: derived.SessionStartBuild,
	})
	derived.Register(derived.Artifact{
		Path:         CodeOutputRel,
		Feeds:        func(_, path string) bool { return IsAnchorSource(path) },
		Rebuild:      func(root string) error { _, err := UpdateCodeIndex(root); return err },
		SessionStart: derived.SessionStartBuild,
	})
}
