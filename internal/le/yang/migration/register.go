// Design: docs/architecture/core-design.md -- one native action package, composed by internal/le/register.go
package yangmigration

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "Repository-wide YANG migrations: move owned command modules to their plugins, rewrite one path across every syntax, and rename schema directories.",
		ShortHelp:   "run repository-wide YANG migrations",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
