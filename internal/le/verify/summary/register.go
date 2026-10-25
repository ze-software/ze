package verifysummary

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "append one stage failure block to the verification failure index",
		Mode:      "offline",
		Section:   registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what `append` takes (answer.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(name, Actions)
	leroot.RegisterShape(name, command.ShapeDoc)
}
