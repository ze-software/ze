// Design: docs/architecture/core-design.md -- le composition and parity claims
package scratch

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Keep disposable scratch and durable build caches outside the checkout, behind the tmp/ and cache/ links, and trim or clean them without touching another session's work.",
		ShortHelp:   "keep scratch and caches out of the checkout",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
	leroot.RegisterBeforeDispatch(area, storeTrimBeforeDispatch)
}
