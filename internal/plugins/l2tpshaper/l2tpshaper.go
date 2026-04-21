// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper plugin

package l2tpshaper

import (
	"log/slog"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Name is the plugin name used in registration and logging.
const Name = "l2tp-shaper"

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
