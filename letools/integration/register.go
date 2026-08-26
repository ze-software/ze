// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package integration

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(Area, Answer, registry.Meta{
		Description: "integration, interop, stress, live and QEMU tests: the proofs that need Docker, root, a namespace or a VM",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the gate table, so help cannot disagree with the listing
		// about which gates exist (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc applies because these two answers do not share one row set.
	// The listing has rows, and a sweep has separate rows for each gate report.
	// rowsInKeyed cannot select between them, so the engine refuses `| count` before a container starts.
	// The `| json`, `| yaml`, and `| table` operators render both answers.
	command.RegisterShape([]string{Area}, command.ShapeDoc)

	// The census derives each claimed gate from the gate table.
	// It counts the three lab runners separately.
	// A test/interop/run.py gate has a Go command and Python driver, so it is claimed but not converted.
	parity.Claim(Area, Gates()...)
	parity.ClaimForked(Area, ForkedGates()...)
}
