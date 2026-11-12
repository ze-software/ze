// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a stop of the receiver's raw ESP reader, raised on the report bus
// Related: doctor_esp_form_fast_path.go -- the check that reports the receiver's failures to open
// Related: register.go -- runEngine, which registers the sink once its plugin exists

package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The receiver's reader stop is transient: the receiver opens the reader again by
// itself, often at once, and then no state is left for a doctor check to read. So it is
// raised on the report bus under its own event code, which the recent-events check
// reports (ai/rules/user-facing-errors.md).
func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorIPsecESPFormReaderStopped}); err != nil {
		panic("BUG: ike: the ESP form reader's event code registration was refused: " + err.Error())
	}
}

// espFormReaderStoppedRaiseTimeout bounds one report-issue request for a reader stop, so
// a daemon that does not answer holds the receiver's revive worker for no longer.
const espFormReaderStoppedRaiseTimeout = 5 * time.Second

// espFormIssueRaiser is the part of *sdk.Plugin the sink uses; a test records into one.
type espFormIssueRaiser interface {
	RaiseIssue(ctx context.Context, issue sdk.Issue) error
}

// espFormReaderStoppedSink answers the sink runEngine registers with the dataplane: each
// reader stop is raised once on the daemon's report bus through p, as an error event the
// error ring keeps, so show errors and show doctor report it after the reader opened
// again. A raise the daemon refuses, or one sent before the plugin's handshake ended, is
// logged at Warn under the same code, so the log still records the stop. It runs on the
// receiver's revive worker, with no receiver lock held.
//
// Safe alongside other plugin RPCs: RaiseIssue builds its request locally, the SDK
// multiplexes each pipe request separately, and its direct bridge admits concurrent
// dispatches. The daemon serializes report admission and the error ring's append.
func espFormReaderStoppedSink(p espFormIssueRaiser, log *slog.Logger) dataplane.ESPFormReaderStoppedSink {
	return func(stopped dataplane.ESPFormReaderStopped) {
		ctx, cancel := context.WithTimeout(context.Background(), espFormReaderStoppedRaiseTimeout)
		defer cancel()
		if err := p.RaiseIssue(ctx, espFormReaderStoppedIssue(stopped)); err != nil {
			log.Warn("ike: Ze could not raise the stop of its own receiver for bare ESP on the report bus, "+
				"so only the log records it", "error", err, "fix", dataplane.ESPFormFixLog,
				"code", diagnostic.CodeDoctorIPsecESPFormReaderStopped)
		}
	}
}

// espFormReaderStoppedIssue is the report bus event of one reader stop. Its subject is the
// ze-espform interface the receiver writes into, so every stop refreshes one subject.
func espFormReaderStoppedIssue(stopped dataplane.ESPFormReaderStopped) sdk.Issue {
	var tb textbuf.Buffer
	tb.Str("Ze's own receiver for bare ESP stopped reading its raw ESP socket at ").
		Str(stopped.At.UTC().Format(time.RFC3339)).Str(" (").Err(stopped.Err).
		Str("), so bare ESP for every Child SA that must receive both ESP forms was received only where " +
			"the fast path rewrites it until Ze opened the receiver again. Ze opens it again by itself; the " +
			"warnings the command below lists show the stop and each attempt to open it again.")
	return sdk.Issue{
		Severity: rpc.ReportError,
		Code:     diagnostic.CodeDoctorIPsecESPFormReaderStopped,
		Subject:  espFormDevice,
		Message:  tb.String(),
		Fix:      dataplane.ESPFormFixLog,
	}
}
