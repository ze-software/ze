// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, and one init() are required. To add a tool to
// le, add this file and a blank import in internal/le/register.go. No other change
// is necessary.
//
// No parity claim applies. Admission is not one of the 156 gates declared by
// the Python le, so this command cannot reduce that census. It reduces the
// OTHER census by one shell file. That count decreases when
// internal/le/job/answer.go is deleted at the swap, not when this command appears.

package job

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Admit a heavy job before it runs, so the sessions sharing this machine do not oversubscribe it, then run it and answer its exit code. Report the job slots, who holds and who waits for them, and the machine's load.",
		ShortHelp:   "admit a heavy job, or report the job slots",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what `run` and `report` take (answer.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(name, Actions)

	// The dispatcher admits every heavy line that declares no slot handling of
	// its own (leroot.SlotOfLine). It cannot import this package, which
	// imports it, so the admitter is handed to it here (central.go).
	leroot.RegisterAdmitter(admitCentral)

	// `run` answers one document, the Report of the job it ran (report.go). It
	// holds no rows, so row operators have no data to process. They are
	// rejected by name instead of producing a plausible answer.
	leroot.RegisterShape(name, command.ShapeDoc)
	// `report` answers one document too, the Overview (overview.go), and that
	// document holds TWO row sets, the holders and the waiters. An answer with
	// several row sets has no single set a row operator could act on, so the
	// shape is declared for the action by name rather than left to the area's:
	// `| json`, `| yaml` and `| table` render it, and a row operator is
	// refused by name (internal/component/command/answer_shape.go, rowsIn).
	leroot.RegisterShape(name+" "+reportVerb, command.ShapeDoc)
}
