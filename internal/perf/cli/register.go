// Design: docs/architecture/system-architecture.md -- le perf subcommand registration

// codegen:skip -- le links this package for `le perf send|report|track`
// (internal/le/perf). The universal composition root carries no tag, so naming it
// there would link the benchmark commands into the ze daemon as well.

package cli

import (
	"github.com/ze-software/ze/internal/core/subdispatch"
)

func init() {
	Register("run", cmdRun, subdispatch.SubMeta{Desc: "Run benchmark against a BGP DUT"})
	Register("report", cmdReport, subdispatch.SubMeta{Desc: "Generate comparison report from result files"})
	Register("track", cmdTrack, subdispatch.SubMeta{Desc: "Track performance history and detect regressions"})
}
