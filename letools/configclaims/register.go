// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package configclaims

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "every config subtree an operator can write is delivered to a plugin, a hub handler, or a recorded exception",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer is ONE document holding two row sets, the exceptions and the
	// findings, so the row operators are refused by name while `| json`,
	// `| yaml` and `| table` render it (internal/component/command/answer_shape.go).
	command.RegisterShape([]string{name}, command.ShapeDoc)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.
	parity.Claim(name, "ze-config-claims-check")
}
