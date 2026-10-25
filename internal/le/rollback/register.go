// Design: docs/contributing/running-commands.md -- launcher builds, `./le rollback`
// Related: rollback.go -- the action table and the refusal

package rollback

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Set the newest le build aside so the previous one runs. The ./le launcher in the checkout performs it before le starts, so a build that misbehaves can always be stepped back from.",
		ShortHelp:   "set the newest le build aside",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The only answer is a refusal or the action listing, so no one key holds
	// rows: a document shape refuses the row operators by name.
	leroot.RegisterShape(area, command.ShapeDoc)
}
