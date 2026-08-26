// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package docwiring

import (
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "the changed-file wiring, documentation, command and inventory gate",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// The keywords this command takes, so help names them where a developer
		// reads what to type.
		SubsFunc: Subs,
	})

	// The answer carries several lists -- the changed files, the selected
	// targets, the per-check results and the failure groups -- so no one key
	// holds the rows. A document shape refuses the row operators by name rather
	// than letting one of the lists stand in for the answer.
	command.RegisterShape([]string{name}, command.ShapeDoc)

	// The census counts the gate as claimed from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.
	parity.Claim(name, gateTarget)

	// Claimed is not converted. This gate still starts the unported
	// scripts/dev/check_doc_links.py, so the census reports it separately from
	// ported work. The fact comes from the argv list in forks.go and uses the
	// same predicate as the area census. Porting the final script empties the
	// list and marks the gate converted without an edit here.
	if slices.ContainsFunc(Forks(), leaction.ForksAScript) {
		parity.ClaimForked(name, gateTarget)
	}
}
