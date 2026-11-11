// Design: docs/architecture/testing/ci-workflows.md -- native CodeQL build admission.
package codeql

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const area = "repo codeql"

func init() {
	// The database proof is the purpose; selection and preparation serve it.
	// It therefore reaches rung 2 of the maintained command-purpose ladder.
	leroot.Register(area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "Admit tracked Go build roots and prove authored inclusion and cache exclusion from CodeQL's database.",
		ShortHelp:   "admit and prove CodeQL's Go population",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
