// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package goversion

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "every build carrier that copies this module in names the Go minor version go.mod declares",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set -- the actions,
	// the findings, or the selftest cases -- so the row operators act on them
	// rather than being refused.
	leroot.RegisterShape(area, command.ShapeMap)
}
