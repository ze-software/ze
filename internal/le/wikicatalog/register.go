// Design: docs/architecture/core-design.md -- le's composition, one import per tool
package wikicatalog

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the generated command-catalog Markdown: check it against live registries, or rewrite it",
		Mode:        modeOffline,
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
