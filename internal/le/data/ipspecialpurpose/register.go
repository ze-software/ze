// Design: docs/architecture/core-design.md -- registered le data refresh tools.
package dataipspecialpurpose

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		ShortHelp: "the shipped IANA special-purpose IP registries: deliberately fetch and validate both canonical XML snapshots",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
