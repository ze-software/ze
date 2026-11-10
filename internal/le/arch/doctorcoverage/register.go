// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package archdoctorcoverage

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Every Warn or Error log line and report-bus raise in the product names a doctor code that a registered doctor check declares, and every diagnostic.Diagnostic finding carries the Fix that clears it, so ze doctor answers for every problem an operator can hit.",
		ShortHelp:   "refuse a problem site with no doctor code or no fix",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer carries the findings AND the scope that produced them, so a
	// reader of an empty row set still sees what was judged.
	leroot.RegisterShape(area, command.ShapeDoc)

	// `./le commit create` judges the problem sites the commit adds.
	changecheck.Register(changecheck.Check{Name: area + " check", Judge: judgeChange})
}
