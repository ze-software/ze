// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, and one init. Composition imports this package
// after the independent implementation slices land.
package platformvet

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "vet the host and interface trees against their Darwin and FreeBSD implementations",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})

	// The listing, each platform report, and a multi-platform sweep each contain
	// row-shaped data. Pipe operators can render those answers directly.
	leroot.RegisterShape(area, command.ShapeMap)

	// Claims come from the same action table that dispatches both platforms.
	parity.Claim(area, Gates()...)
}
