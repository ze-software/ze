// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package cligrammar

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

// name is the word this command is typed as.
const name = "cli-grammar"

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		Description: "every built-in command, every registered root, every demo call site and every offline flag still obeys the CLI grammar: keyword before value, no flag in the command model, no dead launch form, and each flag in its own register",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer is ONE document holding five row sets, the grammar findings,
	// the flag spellings, the dead launch forms, the flag-register findings and
	// the tracked flag debt, so the row operators are
	// refused by name while `| json`, `| yaml` and `| table` render it
	// (internal/component/command/answer_shape.go).
	leroot.RegisterShape(name, command.ShapeDoc)

}
