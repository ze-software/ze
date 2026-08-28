// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the gate table and dispatch this registration exposes
//
// One package, one register.go, one init(). The later composition cut imports
// this package into both le binaries.
package testchaos

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "chaos simulator tests, reduced-tag CLI tests, and lint",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})

	// The listing and a sweep contain different row sets. The document shape
	// keeps every pipe operator honest instead of selecting one set by accident.
	leroot.RegisterShape(Area, command.ShapeDoc)

	// The claims come from the same table that Answer dispatches.

}
