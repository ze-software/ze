// Design: docs/architecture/core-design.md -- le's composition, one import per tool

package sitefleetdemo

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The fleet interface's recorded demonstration: record the fleet pages over the sample estate, for the owner to review from the website output. Never published.",
		ShortHelp:   "record the fleet interface demonstration",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
