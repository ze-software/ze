package verifystatus

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "read and write the verification certificate for the current checkout",
		Mode:        "offline",
		Section:     registry.SectionTest,
		Subs:        "write check show tree-hash",
	})
	leroot.RegisterShape(name, command.ShapeDoc)
}
