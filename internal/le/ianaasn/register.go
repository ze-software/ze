// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package ianaasn

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the compiled RIR delegation seed: fetch the five registries' files and rewrite the ASN-to-RIR table",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Both answers carry one row set -- the actions, or nothing but scalars for
	// the write verdict -- so the row operators act on them rather than being
	// refused.
	leroot.RegisterShape(area, command.ShapeMap)

	// No Make target names this generator, so it claims no gate. The census
	// counts gates, and a tool with none moves it by zero: what moves the count
	// here are the four checks the other codegen tools carry.
	parity.Claim(area, actions.Gates()...)
}
