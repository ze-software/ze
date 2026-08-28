// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). A later composition change adds
// the package's blank import after every independent port has landed.
package journal

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "report committed journal classes and validate one edited shard",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})

	// The listing is one row set and the report has three problem-class views.
	// Any-shape operators render both payloads. Row-only operators are refused
	// because no one row set represents the full report.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census reads the same action table that dispatch and help read.

}
