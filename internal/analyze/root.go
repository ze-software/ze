// Design: docs/architecture/system-architecture.md -- ze-analyze root handler registration
// Related: register.go -- the subcommands the root dispatches

// codegen:skip -- the ze_analyze personality wires this, from cmd/ze/ze_analyze_register.go.
// The build tag keeps the `analyze` root out of every binary that links this package for
// its subcommands alone: le links it for `le mrt`, and without the tag the ze binary built
// with ze_le would answer `ze analyze` too.

//go:build ze_analyze

package analyze

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/command/registry"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

func init() {
	registry.MustRegisterRootHandler("analyze", func(_ *registry.RuntimeContext, args []string) int {
		if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
			fmt.Println(zeversion.Short())
			return 0
		}
		return Dispatch(args)
	}, registry.Meta{
		ShortHelp: "BGP MRT analysis tools",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subcommands,
	})
}
