// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, and one init. Composition imports this package
// after the independent implementation slices land.
package govetplatforms

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "The shipped tree type-checked under GOOS darwin, freebsd and openbsd, so code that only a Linux host build selects cannot hide a break on the owner's Mac or a BSD.",
		ShortHelp:   "type-check the tree under other GOOSes",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The listing, each platform report, and a multi-platform sweep each contain
	// row-shaped data. Pipe operators can render those answers directly.
	leroot.RegisterShape(area, command.ShapeMap)

	// Claims come from the same action table that dispatches every platform.

}
