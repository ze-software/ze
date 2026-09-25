// Design: docs/architecture/chaos-web-dashboard.md -- chaos root handler registration
//
// The build tag keeps the `chaos` root out of every binary that links this
// package for CLIRun alone: le links it for `le chaos run`, and without the tag
// the ze binary built with ze_le would answer `ze chaos` too. The le chaos run
// program sets the tag (cmd/ze/ze_chaos_run.go).

//go:build ze_chaos

package orchestrator

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("chaos", func(_ *registry.RuntimeContext, args []string) int {
		return CLIRun(args)
	}, registry.Meta{
		ShortHelp: "Chaos monkey for BGP testing",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})
}
