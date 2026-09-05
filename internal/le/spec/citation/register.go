// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// This file owns registration. Each composition root needs only a blank import
// of this package.

package speccitation

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		Description: "a spec citing a sibling spec absent on disk fails, unless the" +
			" target is grandfathered in plan/.citation-baseline; a path:line citation" +
			" whose backtick-quoted token drifted off that line warns",
		Mode:    "offline",
		Section: registry.SectionTest,
	})
	leroot.RegisterShape(name, command.ShapeDoc)

}
