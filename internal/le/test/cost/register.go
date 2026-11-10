// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package testcost

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "Rank the tests that take the largest share of their suite or package run, as candidates to read against the testing rule's one-question-per-test point. A report, never a gate; its times are host-dependent triage, never a performance figure.",
		ShortHelp:   "rank the tests that cost the most of their run",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	// One key holds rows; the others name the source, the host and the
	// thresholds the rows were read against.
	leroot.RegisterShape(area, command.ShapeMap)
}
