// Design: docs/architecture/core-design.md -- Policy Filter Chain
// Related: egress_inject_filter_test.go -- captureWarnPeers, the slog.Default form

package reactor

import (
	"log/slog"
	"testing"
)

// captureFwdWarnPeers points fwdLogger, through which the library chain
// writes its refusals (runLibraryChain), at a recorder for the test and
// restores it after. It is captureWarnPeers for the forward logger. Not safe
// for parallel tests: fwdLogger is package state.
func captureFwdWarnPeers(t *testing.T) *warnRecorder {
	t.Helper()
	rec := &warnRecorder{}
	prev := fwdLogger
	fwdLogger = func() *slog.Logger { return slog.New(rec) }
	t.Cleanup(func() { fwdLogger = prev })
	return rec
}
