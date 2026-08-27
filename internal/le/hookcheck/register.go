// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// The unit action is intentionally gateless. ze-unit-hook-test was never a
// Python le registry row, so claiming it would invent a parity denominator row.
package hookcheck

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "native hook dispatcher golden and behavioral fixture selftests",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
