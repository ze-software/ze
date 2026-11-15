// Design: docs/architecture/behavior/peer-lifecycle.md -- historical received NOTIFICATION observer gaps.
// Related: received_notification_diagnostic.go -- the warning and bounded error event.
package reactor

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// notificationDiagnosticLog captures the actual reactor logger at WARN.
// Call before starting peers, and join them before cleanup restores the logger.
// Tests using the package logger and report store MUST NOT run in parallel.
func notificationDiagnosticLog(t *testing.T) *syncBuffer {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := &syncBuffer{}
	previous := reactorLogger
	reactorLogger = func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { reactorLogger = previous })
	return sink
}

func notificationDiagnosticEvents() []report.Issue {
	var events []report.Issue
	for _, event := range report.Errors(0) {
		if event.Code == codeReceivedNotificationSkipped {
			events = append(events, event)
		}
	}
	return events
}

// TestReceivedNotificationSkippedDiagnostic proves cancellation and sticky
// publication failure retain their real cause in one WARN and one report event.
// A canceled context must not suppress either operator-visible record.
func TestReceivedNotificationSkippedDiagnostic(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "canceled wait"
		if failed {
			name = "failed publication"
		}
		t.Run(name, func(t *testing.T) {
			sink := notificationDiagnosticLog(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			publication := new(receivePublication)
			publication.accepted.Store(1)
			if failed {
				require.True(t, publication.fail())
				publication.complete(1)
			} else {
				cancel()
			}
			cause := (receivePublicationCut{publication: publication, messageID: 1}).wait(ctx)
			require.Error(t, cause)
			if failed {
				require.ErrorIs(t, cause, errReceivePublicationFailed)
			} else {
				require.ErrorIs(t, cause, context.Canceled)
			}
			reportReceivedNotificationSkipped(ctx, "192.0.2.1", cause)
			notificationDiagnosticRequire(t, sink, "192.0.2.1", cause)
		})
	}
}

// TestReceivedNotificationCanceledCompletedCut preserves accurate wording when
// cancellation races completed publication: the skipped event is historical,
// but neither unfinished UPDATE delivery nor a stuck peer is established.
func TestReceivedNotificationCanceledCompletedCut(t *testing.T) {
	sink := notificationDiagnosticLog(t)
	publication := new(receivePublication)
	publication.accepted.Store(1)
	publication.complete(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cause := (receivePublicationCut{publication: publication, messageID: 1}).wait(ctx)
	require.ErrorIs(t, cause, context.Canceled)
	require.False(t, publication.failed.Load())
	reportReceivedNotificationSkipped(ctx, "192.0.2.2", cause)
	notificationDiagnosticRequire(t, sink, "192.0.2.2", cause)
	record := closeRecordsWith(closeLogRecords(t, sink), "code", codeReceivedNotificationSkipped)[0]
	require.Equal(t, "Ze could not confirm the order of earlier UPDATE events before this wait ended. Observers miss this NOTIFICATION event; the session still processes the NOTIFICATION and closes.", record["why"])
}

func notificationDiagnosticRequire(t *testing.T, sink *syncBuffer, peer string, cause error) {
	t.Helper()
	records := closeRecordsWith(closeLogRecords(t, sink), "code", codeReceivedNotificationSkipped)
	require.Len(t, records, 1)
	require.Equal(t, "WARN", records[0]["level"])
	require.Equal(t, "BGP NOTIFICATION event was skipped for peer "+peer, records[0]["msg"])
	require.Equal(t, receivedNotificationSkippedWhy, records[0]["why"])
	require.Equal(t, receivedNotificationSkippedFix, records[0]["fix"])
	require.Equal(t, peer, records[0]["peer"])
	require.Equal(t, cause.Error(), records[0]["error"])
	events := notificationDiagnosticEvents()
	require.Len(t, events, 1)
	require.Equal(t, reportSourceBGP, events[0].Source)
	require.Equal(t, peer, events[0].Subject)
	require.Equal(t, report.SeverityError, events[0].Severity)
	require.Equal(t, receivedNotificationSkippedFix, events[0].Fix)
	require.Contains(t, events[0].Message, "BGP NOTIFICATION event was skipped for peer "+peer)
	require.Contains(t, events[0].Message, receivedNotificationSkippedWhy)
	require.Contains(t, events[0].Message, cause.Error())
	require.LessOrEqual(t, len(events[0].Message), report.MessageMax)
}

// TestReceivedNotificationDiagnosticRegistration resolves the inspection fix
// against the real show-errors YANG command, including its required arguments.
func TestReceivedNotificationDiagnosticRegistration(t *testing.T) {
	metadata := diagnostic.Lookup(codeReceivedNotificationSkipped)
	require.NotNil(t, metadata)
	require.NotEmpty(t, metadata.Title)
	require.NotEmpty(t, metadata.Description)
	require.Contains(t, diagnostic.EventCodes(), codeReceivedNotificationSkipped)
	require.Contains(t, metadata.Examples, "ze show diagnostic "+codeReceivedNotificationSkipped)
	schema, err := yang.DefaultLoader()
	require.NoError(t, err)
	path := strings.TrimPrefix(receivedNotificationSkippedFix, "ze ")
	require.Contains(t, yang.WireMethodToPaths(schema)["ze-cmd:show-errors"], path)
	require.Empty(t, yang.PathToArgDefs(schema)[path], "the inspection command must need no omitted argument")
}
