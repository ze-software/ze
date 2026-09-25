// Design: docs/architecture/system-architecture.md -- le perf root handler registration

// codegen:skip -- the ze-perf personality wires this, from cmd/ze/ze_perf_register.go under
// //go:build ze_perf. The universal composition root carries no tag, so naming it there
// would link the benchmark commands into the ze daemon as well.
//
// The root is registered by RegisterRoot rather than by init: le links this
// package for `le perf send|report|track` (internal/le/perf), and le MUST
// register one root and no tool root.

package cli

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/subdispatch"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

func init() {
	Register("run", cmdRun, subdispatch.SubMeta{Desc: "Run benchmark against a BGP DUT"})
	Register("report", cmdReport, subdispatch.SubMeta{Desc: "Generate comparison report from result files"})
	Register("track", cmdTrack, subdispatch.SubMeta{Desc: "Track performance history and detect regressions"})
}

// RegisterRoot registers the `perf` root of the ze-perf personality. It MUST be
// called once, from that personality's own register file.
func RegisterRoot() {
	registry.MustRegisterRootHandler("perf", func(_ *registry.RuntimeContext, args []string) int {
		if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
			fmt.Println(zeversion.Short())
			return 0
		}
		return Dispatch(args)
	}, registry.Meta{
		ShortHelp: "BGP propagation latency benchmark tool",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subcommands,
	})
}
