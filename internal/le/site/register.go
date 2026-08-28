// Design: docs/architecture/core-design.md -- le composition is one registration per native tool
package site

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "build, check, and render the public website and presentation artifacts without an interpreter",
		Mode:        "offline", Section: registry.SectionTest, SubsFunc: Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
