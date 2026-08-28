// Design: docs/architecture/core-design.md -- le's composition, one import per tool
package commit

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

// The prepared-commit payload renders itself. Text has a pointer receiver, so
// the action MUST answer &Prepared; a value would render as a bare JSON dump.
var _ leroot.Prose = (*Prepared)(nil)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "prepare explicit commits without touching the shared staging index",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
