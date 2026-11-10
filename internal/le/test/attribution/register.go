// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package testattribution

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "Tell, for each test a package run reported failed, whether your own files cause the red: each failing test re-runs as on disk and with the files you name put back to HEAD through a Go overlay, and the tree is never touched.",
		ShortHelp:   "tell which failing tests your own files cause",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	// One key holds rows; the others name the log, the package, the tags, the
	// files and what the two runs compared.
	leroot.RegisterShape(area, command.ShapeMap)
}
