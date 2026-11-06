// Design: docs/architecture/testing/ci-workflows.md -- native CodeQL build admission.
package codeql

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const area = "repo codeql"

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "Derive tracked CodeQL package roots and refuse canonical cache dependencies before a traced build.",
		ShortHelp:   "select CodeQL's tracked Go population",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
