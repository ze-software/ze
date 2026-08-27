// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package specstatus

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register("spec-status", Answer, registry.Meta{
		Description: "the spec inventory: status, bucket and stale-skeleton flag for every plan/spec-*.md",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer IS the rows, so the row operators act on the specs. Declaring
	// the shape lets the engine refuse an operator the answer cannot support
	// before the tool reads the tree (ai/rules/cli.md).
	leroot.RegisterShape("spec-status", command.ShapeMap)

	// NO parity.Claim. `le gates --json` declares 156 gates and none of them is
	// this tool: `ze-spec-status` and `ze-spec-status-json` are Make targets in
	// mk/report-inventory.mk that run `go run scripts/status/spec_status.go`
	// directly, never through the Python `le`. The census therefore cannot move
	// for this port, and claiming a gate the census does not hold is red.
	// internal/le/weekly carries the same note for the same reason.
}
