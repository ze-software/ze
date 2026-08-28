// Design: plan/spec-le-is-a-ze-binary.md -- native module migration workflows
package modulemigration

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "preview or apply package-tree moves and repository Go module-path renames",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)

}
