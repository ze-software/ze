// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package functional

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "the .ci functional suites: what each one runs, and the wall-clock budget it runs under",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc applies because these answers do not share one row set.
	// The suite table has rows, a gating run has one verdict, and a sweep has separate rows.
	// rowsInKeyed cannot select among them, so the engine refuses `| count` before a run starts.
	// The `| json`, `| yaml`, and `| table` operators render all three answers.
	command.RegisterShape([]string{Area}, command.ShapeDoc)

	// The census derives each claimed gate from the action table.
	// It counts the two docker-exec gates separately.
	// Their driver remains scripts/dev/docker_exec_checked.py, so Go owns the command but not the work.
	parity.Claim(Area, Gates()...)
	parity.ClaimForked(Area, ForkedGates()...)
}
