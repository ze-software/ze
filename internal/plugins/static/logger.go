package static

import (
	"log/slog"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}
