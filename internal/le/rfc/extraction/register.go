// Design: docs/architecture/core-design.md -- the rfc area's extraction walk, as one command
// Related: ../actions.go -- the `rfc extraction` action table this registers
//
// Package rfcextraction registers `le rfc extraction`, the sub-area holding
// the three actions of an extraction walk: create derives a skeleton,
// classify applies an authored walk to it, and report prints the sign-off
// counts (spec-le-cli-usability, D-P4-positional and D-P4-depth3). It sits at
// the path its name predicts. The table and the answers live in rfc, beside
// the corpus they read; this package only registers them.
// One register.go, one init().

package rfcextraction

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/rfc"
)

// area is the command this package registers.
const area = "rfc extraction"

func init() {
	leroot.Register(area, leroot.GroupGate, rfc.ExtractionAnswer, registry.Meta{
		Description: "Walk an RFC's own text for the obligations its summary missed: derive the extraction skeleton, apply an authored walk to it, and count the sign-offs.",
		ShortHelp:   "walk an RFC's text for what its summary missed",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command,
		// as the parent `rfc` area does.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES.
		SubsFunc: rfc.ExtractionSubs,
	})
	leroot.RegisterActions(area, rfc.ExtractionActions)

	// Every answer contains one row set: a skeleton report, a classification
	// report or the sign-off envelope.
	leroot.RegisterShape(area, command.ShapeMap)
}
