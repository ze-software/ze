// Design: docs/contributing/patching-the-standard-library.md -- the review stamp, the check ./le verify runs
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package patchstamps

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(stampsArea, leroot.GroupGate, Answer, registry.Meta{
		Description: "Check every standard-library patch's review stamp against go.mod's pinned release and the activated toolchain, so a patch is never applied to a release nobody reviewed it against.",
		ShortHelp:   "check the stdlib patch review stamps",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section:  registry.SectionTest,
		SubsFunc: Subs,
	})
	leroot.RegisterActions(stampsArea, Actions)
	leroot.RegisterShape(stampsArea, command.ShapeDoc)
}
