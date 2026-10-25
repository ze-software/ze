package verifystatus

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "The verification certificate for this checkout: record a run's verdict, check that a recorded PASS still covers the tree, print it, or print the tracked tree's hash.",
		ShortHelp:   "read or write the verification certificate",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(name, Actions)
	leroot.RegisterShape(name, command.ShapeDoc)
}
