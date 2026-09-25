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
		Subs:      "append failures <failures-log> stage <stage> log <stage-log>",
	})
	leroot.RegisterShape(name, command.ShapeDoc)
}
