// Design: docs/architecture/core-design.md -- le composition, one import per tool
// Overview: actions.go -- the gateless verifier action table
package verifydeps

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(Area, leroot.GroupGate, Answer, registry.Meta{
		Description: "The Go-tool and dependency stages that only native pre-commit verification runs: evidence vet, vulnerability scan, cached and changed unit passes, race groups and allocation ceilings.",
		ShortHelp:   "run the dependency stages of verification",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(Area, Actions)
	leroot.RegisterShape(Area, command.ShapeDoc)
}
