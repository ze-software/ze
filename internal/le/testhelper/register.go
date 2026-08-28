// Design: docs/architecture/core-design.md -- le's composition, one import per tool
package testhelper

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "long-running native protocol test fixture producers",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(Area, command.ShapeDoc)
}

// Subs returns the action names shown in command help.
func Subs() string { return "dynamic watchdog" }
