// Design: docs/architecture/host/observability.md -- the host trend monitor
// Related: register.go -- the offline fallbacks of show host

package host

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// trendOfflineRefusal is the answer of show host trend when the daemon did not
// answer. The trend monitor keeps its history inside the daemon, so no process
// but the daemon has a trend to show.
//
// The refusal names why the daemon did not answer and ends on the command
// that gets its answer, both as the route that asked the daemon learned them
// (unanswered). It names no cause of its own: a daemon that is not running, a
// daemon this account cannot log in to and a connection that failed each have
// another fix, and only the route knows which one it met.
func trendOfflineRefusal(unanswered command.DaemonUnanswered) *opererr.Error {
	var problem textbuf.Buffer
	problem.Str("show host trend needs the daemon, and ").Str(unanswered.Why())
	return opererr.New(problem.String(),
		"the host trend monitor runs inside the daemon and keeps the history of each host resource there, "+
			"so a command the daemon did not answer has no trend to show",
		unanswered.Fix())
}

// runShowTrendOffline answers show host trend when the daemon did not answer.
// It exists because the offline fallback of show host matches the longest
// prefix: without it, show host trend would reach RunShow, which would refuse
// trend as an unknown inventory section and list the sections instead.
func runShowTrendOffline(_ command.ValidatedArgs, unanswered command.DaemonUnanswered) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(trendOfflineRefusal(unanswered).Error()).Byte('\n').StdErr() //nolint:errcheck // CLI error
	return 1
}
