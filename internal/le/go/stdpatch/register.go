// Design: docs/contributing/patching-the-standard-library.md -- the le commands over the patched GOROOT
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package stdpatch

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "The patched GOROOT every Go command compiles Ze with: printed for plain go commands, and built on first use from the pinned toolchain and the patches under the repository.",
		ShortHelp:   "print or build the patched GOROOT",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section:  registry.SectionTest,
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
