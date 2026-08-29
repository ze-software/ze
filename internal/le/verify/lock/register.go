package verifylock

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "run a verify-class command through the shared heavy-job admission",
		Mode:        "offline",
		Section:     registry.SectionTest,
		Subs:        usage,
	})
	leroot.RegisterShape(name, command.ShapeDoc)
}
