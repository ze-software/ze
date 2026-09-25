// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// The unit action is intentionally gateless. ze-unit-hook-test was never a
// Python le registry row, so claiming it would invent a parity denominator row.
package aihooks

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "native hook dispatcher golden and behavioral fixture selftests",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
