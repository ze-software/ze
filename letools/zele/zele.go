// Design: docs/architecture/core-design.md -- le's commands inside ze
// Detail: tools.go -- the tool set this package links
//
// Package zele is the ONE seam through which a ze build carries le's
// development commands. It is reached only from cmd/ze/ze_le_register.go,
// which is `//go:build ze_le`, so no shipped ze links it.
//
// This package tests the architectural claim that ze and le use one engine.
// A ze build must run le commands without a second grammar, help page, or pipe implementation.
// The package provides that tested crossing, but no default build includes it.
//
// The crossing claims one root, `le`, and dispatches every tool below it.
// Linking each tool still runs its init and registers its own root.
// Thus, `ze le lint` and `ze lint` reach the same handler in a ze_le build.
// The shared root provides a stable entry point and a help page containing only le commands.
//
// leroot.Dispatch (letools/leroot/dispatch.go) implements the loop for both the le binary and this root.
//
// ze_le is intentionally absent from feature-gates.txt.
// That manifest lists product features and supplies tag sets for builds, lint, tests, and installation.
// Adding ze_le would include the crossing in shipped tag sets, which must never occur.
// ze_perf, ze_test, ze_chaos, and ze_analyze are absent for the same reason.
// ze_le still needs a lint flavor because otherwise no lint pass compiles its files
// (scripts/dev/lint_flavors.py, the capability row).
//
// Registration belongs here instead of cmd/ze.
// The command-ownership gate requires a root handler to live in the package that owns its behavior
// (letools/commandownership, checkRootHandlersAreInternal).
package zele

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

// name is the single root ze claims for the whole development command set.
const name = "le"

// program is the command name used by help and refusals.
// A reader can type a command line copied from either output.
const program = "ze le"

func init() {
	registry.MustRegisterRootHandler(name, run, registry.Meta{
		Description: "the Ze repository and development commands, in-process",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})
}

// run dispatches the words after `ze le` through le's commands.
// It returns the tool's exit code unchanged.
//
// No le tool reads RuntimeContext because each tool examines a checkout, not a running daemon.
// leroot.Register drops the context for the same reason.
func run(_ *registry.RuntimeContext, args []string) int {
	return leroot.Dispatch(program, args)
}
