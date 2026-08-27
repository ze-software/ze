// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the three actions this registration reaches
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package docvalid

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the documentation gates: the YANG command contract, the doc drift check, and the generated operator table",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc, because ONE of the three answers is a document rather than a
	// row set: the contract result carries the YANG commands, the handlers, the
	// local handlers and three orphan lists, and rowsInKeyed refuses to choose
	// between them (internal/component/command/answer_shape.go). Declaring the
	// shape lets the engine refuse `| count` BY NAME instead of after the gate
	// has walked the tree. `| json`, `| yaml` and `| table` are anyShape
	// operators and render every one of the three answers.
	//
	// The shape is per ROOT rather than per action: leroot.Run hands the engine
	// the command NAME, so a per-action declaration would never be looked up
	// (internal/le/leroot, Run).
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census counts all three gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.
	parity.Claim(area, "ze-command-contract-check", "ze-doc-drift-check", "ze-docs-pipe-operators-update")
}
