// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package tokeneconomy

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "where this repository's sessions spend their tokens: API calls, the context carried at each one, the size histogram and a capped-context counterfactual, read from the machine-local transcript store",
		// Offline, and the word is exact for an unusual reason: the tool reads
		// no network AND no checkout. Its corpus is ~/.claude/projects, which
		// is written by another program on this machine.
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		Subs:    "root <path> | project <slug> | cap <n> | top <n> | session <prefix>",
	})

	// ShapeDoc applies because the answer contains EIGHT tables over one corpus.
	// No single row set represents the sessions, histograms, phases, agents,
	// tools, and result sizes. The engine refuses row operators by name.
	// `| json`, `| yaml`, and `| table` render the complete payload.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach. Claim rather than
	// ClaimForked: the report reads the store and renders it in Go, so nothing
	// here starts a script.

}
