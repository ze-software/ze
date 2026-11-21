// Design: docs/architecture/host/observability.md -- the host trend monitor
// Related: register.go -- the offline fallbacks of show host

package host

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// trendOfflineRefusal is the answer of show host trend when no daemon answers.
// The trend monitor keeps its history inside the daemon, so no process but
// the daemon has a trend to show.
func trendOfflineRefusal() *opererr.Error {
	return opererr.New("show host trend needs the daemon, and no daemon answered",
		"the host trend monitor runs inside the daemon and keeps the history of each host resource there, "+
			"so a command run without the daemon has no trend to show",
		"ze start")
}

// runShowTrendOffline answers show host trend when the daemon is unreachable.
// It exists because the offline fallback of show host matches the longest
// prefix: without it, show host trend would reach RunShow, which would refuse
// trend as an unknown inventory section and list the sections instead.
func runShowTrendOffline(command.ValidatedArgs) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(trendOfflineRefusal().Error()).Byte('\n').StdErr() //nolint:errcheck // CLI error
	return 1
}
