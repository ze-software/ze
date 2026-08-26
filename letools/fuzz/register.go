// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package fuzz

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "Go fuzzing: every `func Fuzz` under internal/, discovered at run time",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action writes.
		SubsFunc: Subs,
	})

	// Each answer has one row set: actions, planned runs, or completed runs.
	// The row operators therefore act on that set instead of being refused.
	command.RegisterShape([]string{area}, command.ShapeMap)

	// No parity.Claim, and the absence is a fact rather than an oversight:
	// `./le gates --json` declares 156 gates and neither ze-fuzz-test nor
	// ze-fuzz-test-one is one of them. The census counts this directory under
	// script-files, which falls when the swap deletes the Python.
}
