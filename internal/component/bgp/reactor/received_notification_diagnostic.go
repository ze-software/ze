// Design: docs/architecture/behavior/peer-lifecycle.md -- terminal observer delivery follows received UPDATE publication.
// Related: reactor_notify.go -- only a skipped received NOTIFICATION raises this event.
// Related: docs/architecture/doctor-and-health-checks.md -- bounded historical events.
package reactor

import (
	"context"
	"log/slog"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeReceivedNotificationSkipped = "doctor-bgp-received-notification-skipped"
	receivedNotificationSkippedFix  = "ze show errors"
	receivedNotificationSkippedWhy  = "Ze could not confirm the order of earlier UPDATE events before this wait ended. Observers miss this NOTIFICATION event; the session still processes the NOTIFICATION and closes."
)

// init registers a historical observer gap, not a condition that remains
// active after the session closes. The event retains the actual wait cause.
func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeReceivedNotificationSkipped,
		Title:       "Received BGP NOTIFICATION observer event was skipped",
		Description: receivedNotificationSkippedWhy,
		Examples:    []string{receivedNotificationSkippedFix, "ze show diagnostic " + codeReceivedNotificationSkipped},
	}); err != nil {
		panic("BUG: bgp reactor: received notification diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeReceivedNotificationSkipped}); err != nil {
		panic("BUG: bgp reactor: received notification event registration failed: " + err.Error())
	}
}

// reportReceivedNotificationSkipped records one actual skipped observer event.
// The caller MUST still process the received NOTIFICATION and MUST pass the
// non-nil wait error as cause. Safe for concurrent use; cancellation cannot
// suppress the log or the context-independent bounded report-bus event.
func reportReceivedNotificationSkipped(ctx context.Context, peer string, cause error) {
	err := opererr.Wrap(cause, "BGP NOTIFICATION event was skipped for peer "+peer,
		receivedNotificationSkippedWhy, receivedNotificationSkippedFix)
	reactorLogger().LogAttrs(context.WithoutCancel(ctx), slog.LevelWarn, err.Problem(),
		err.LogAttrs(slog.String("code", codeReceivedNotificationSkipped), slog.String("peer", peer))...)
	report.RaiseErrorFix(reportSourceBGP, codeReceivedNotificationSkipped, peer,
		report.FitMessage(err.Problem()+". "+err.Why()+": "+cause.Error()), err.Fix(), nil)
}
