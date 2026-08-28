// Design: plan/spec-le-is-a-ze-binary.md -- native module migration workflows
package module

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

// Both reports render themselves. Text has a pointer receiver, so each action
// MUST answer a pointer; a value would render as a bare JSON dump.
var (
	_ leroot.Prose = (*MoveReport)(nil)
	_ leroot.Prose = (*RenameReport)(nil)
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "preview or apply package-tree moves and repository Go module-path renames",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)

}
