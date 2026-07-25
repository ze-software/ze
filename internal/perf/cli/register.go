// Design: docs/architecture/system-architecture.md -- ze-perf root handler registration

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

	registry.MustRegisterRootHandler("perf", func(_ *registry.RuntimeContext, args []string) int {
		if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
			fmt.Println(zeversion.Short())
			return 0
		}
		return Dispatch(args)
	}, registry.Meta{
		Description: "BGP propagation latency benchmark tool",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subcommands,
	})
}
