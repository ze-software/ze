// Design: docs/architecture/api/commands.md — cli command ownership
//
// Register the `cli` root command with the importable command registry. This is
// the owner package: the interactive CLI client (and the command-tree builders
// it exposes for cmd/ze) lives alongside the CLI implementation under
// internal/component/cli, not under cmd/ze. cmd/ze/main.go dispatches `ze cli`
// through the registry handler registered here.
package client

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("cli", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Interactive CLI for the running daemon",
		Mode:        "daemon",
		Section:     registry.SectionOperations,
		Subs:        "-c <cmd> for single command",
	})
}
