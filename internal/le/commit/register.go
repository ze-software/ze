// Design: docs/architecture/core-design.md -- le's composition, one import per tool
package commit

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// The prepared-commit payload renders itself. Text has a pointer receiver, so
// the action MUST answer &Prepared; a value would render as a bare JSON dump.
var _ leroot.Prose = (*Prepared)(nil)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "prepare explicit commits without touching the shared staging index",
		Mode:      "offline",
		Section:   registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what each verb takes (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
