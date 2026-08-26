// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package goextract

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

func init() {
	leroot.Register("go-extract", Answer, registry.Meta{
		Description: "move named declarations from one Go file to another, comments and formatting intact",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		Subs:    "source, dest, symbol",
	})

	// The answer carries one row per declaration that moved, so the row
	// operators act on those rather than being refused. Declaring it lets the
	// engine refuse what the shape cannot support BEFORE a file is rewritten
	// (ai/rules/cli.md).
	command.RegisterShape([]string{"go-extract"}, command.ShapeMap)

	// No parity.Claim here, and that absence is a fact rather than an
	// oversight: `./le gates --json` declares 156 gates, 51 of them naming a
	// file under scripts/dev, and every one of those 51 runs a PYTHON tool.
	// Nothing reaches this one but a developer and the ze-extract skill, so the
	// census counts it under script-files and has no gate to move
	// (letools/parity, Take).
}
