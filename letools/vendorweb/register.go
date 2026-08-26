// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package vendorweb

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the vendored web assets: check every consumer copy against third_party/web/, sync them, or ask npm what is newer",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set -- the actions,
	// the problems, or the files the sync acted on -- so the row operators act
	// on them rather than being refused. Declaring the shape lets the engine
	// refuse what the shape cannot support BEFORE the tool walks the tree.
	command.RegisterShape([]string{area}, command.ShapeMap)

	// The census counts all three gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.
	parity.Claim(area, "ze-vendor-web-check", "ze-vendor-web-sync", "ze-vendor-web-update-report")
}
