// Design: docs/architecture/core-design.md -- the rules area's measurements, as one command
// Related: ../actions.go -- the `ai rules report` action table this registers
//
// Package airulesreport registers `le ai rules report`, the sub-area holding
// the rule corpus's four measurements: gate-map, payload, router and coverage
// (spec-le-cli-usability, D-P4-positional and D-P4-depth3). It sits at the
// path its name predicts. The table and the answers live in airules, beside
// the corpus they read; this package only registers them.
// One register.go, one init().

package airulesreport

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	airules "github.com/ze-software/ze/internal/le/ai/rules"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// area is the command this package registers.
const area = "ai rules report"

func init() {
	leroot.Register(area, leroot.GroupGenerate, airules.ReportAnswer, registry.Meta{
		Description: "Measure the rule corpus in ai/rules/: which hook check enforces each point, what a session loads, which rules no past task surfaces, and which matched rules a session never read.",
		ShortHelp:   "measure the rule corpus and its enforcement",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command,
		// as the parent `ai rules` area does.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES.
		SubsFunc: airules.ReportSubs,
	})
	leroot.RegisterActions(area, airules.ReportActions)

	// Every answer contains one row set: gate-map sets, a payload, a router
	// measurement or a transcript coverage report.
	leroot.RegisterShape(area, command.ShapeMap)
}
