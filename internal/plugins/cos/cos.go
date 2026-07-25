// Design: plan/learned/884-cos-plugin.md -- class-of-service plugin

package cos

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
)

const Name = "cos"

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
