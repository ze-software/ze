// Design: docs/architecture/core-design.md -- le composition, one import per tool
// Overview: actions.go -- the action table this registration exposes
package testunit

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "the five race-instrumented component-group Go test suites",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})

	// A listing and a sweep have different row shapes. Document rendering keeps
	// both structured answers available without claiming one common row set.
	leroot.RegisterShape(Area, command.ShapeDoc)

	// Every claim comes from the table that dispatches the action.

}
