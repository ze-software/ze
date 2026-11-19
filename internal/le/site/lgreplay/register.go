// Design: docs/architecture/core-design.md -- le's composition, one import per tool

package sitelgreplay

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The website's looking glass replay: record a real looking glass serving the simulated network, as the answers the site replays.",
		ShortHelp:   "record the looking glass replay",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
