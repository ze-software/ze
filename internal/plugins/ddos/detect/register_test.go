package detect

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: warnIfExternal logs a warning exactly when ddos-detect is not
// running in-process -- both its subscribe paths (trafficstat.EnsureGlobal/
// SubscribeRates, and the iface.SubscribeCollectNotify fallback when
// trafficstat is unavailable) register a callback into a package-level
// subscriber list as a plain Go function call, which only reaches the real
// background tracker when this plugin shares process memory with it. An
// external ddos-detect would silently never receive a rate signal on either
// path, with no error anywhere until this warning.
// PREVENTS: an operator running `plugin { external ddos-detect { ... } }`
// getting no indication that detection is permanently starved of input.
func TestWarnIfExternal(t *testing.T) {
	t.Cleanup(func() { setLogger(slogutil.DiscardLogger()) })

	var buf bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(false)
	assert.Contains(t, buf.String(), "rate signal")

	buf.Reset()
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(true)
	assert.Empty(t, buf.String(), "internal ddos-detect must not log the external-mode warning")
}
