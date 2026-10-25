// Design: docs/architecture/testing/interop.md -- lab peer pins and their upstream
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package pin

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// Rung 1 of the group ladder (internal/le/le/root/group.go): the pins are
	// committed lines in the lab Dockerfiles and Go sources, and `bump`
	// rewrites them, so the area owns a committed artifact. A report never
	// writes (TestReportAreasWriteNothing).
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		ShortHelp: "every pinned interop and integration lab peer compared with its upstream latest stable release (network)",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)
	// Each answer carries one row set, the pins, so the row operators act on
	// them rather than being refused.
	leroot.RegisterShape(area, command.ShapeMap)
}
