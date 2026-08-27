// Design: docs/architecture/core-design.md -- le composition, one import per tool
// Overview: actions.go -- the gateless verifier action table
package verifydeps

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "the Go-tool and dependency stages used only by native pre-commit verification",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(Area, command.ShapeDoc)
}
