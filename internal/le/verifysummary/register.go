package verifysummary

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "append one stage failure block to the verification failure index",
		Mode:        "offline",
		Section:     registry.SectionTest,
		Subs:        "append failures <failures-log> stage <stage> log <stage-log>",
	})
	leroot.RegisterShape(name, command.ShapeDoc)
}
