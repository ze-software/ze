// Design: plan/learned/710-gap-2-static-route-enhancements.md -- routing-table plugin logger

package routingtable

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}
