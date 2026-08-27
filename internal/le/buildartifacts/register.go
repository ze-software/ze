// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package owns one root action area. Composition imports this package in the
// later integration cut.

package buildartifacts

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "build the host appliance driver and the amd64 or arm64 installer initrd",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})

	leroot.RegisterShape(area, command.ShapeMap)
	parity.Claim(area, actions.Gates()...)
}
