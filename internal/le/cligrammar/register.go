// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package cligrammar

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

// name is the word this command is typed as. The gate it is has one action, so
// it takes no verb and no argument.
const name = "cli-grammar"

// gate is the Make target this command still is.
const gate = "ze-cli-grammar-check"

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "every built-in command, every registered root and every demo call site still obeys the CLI grammar: keyword before value, no flag in the command model, no dead launch form",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer is ONE document holding three row sets, the grammar findings,
	// the flag spellings and the dead launch forms, so the row operators are
	// refused by name while `| json`, `| yaml` and `| table` render it
	// (internal/component/command/answer_shape.go).
	leroot.RegisterShape(name, command.ShapeDoc)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.
	parity.Claim(name, gate)
}
