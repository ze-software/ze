//go:build ze_web

// Design: docs/architecture/doctor-and-health-checks.md -- overdue web cleanup events.
// Related: web_history_lifetime_test.go -- actual blocked history publication store.
package hub

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// TestWebServiceOverdueHistoryReportsOneDoctorEvent crosses the actual service
// allowance with a blocked shared History publication. Concurrent shutdown
// callers must publish one event while retaining the store until both modes drain.
func TestWebServiceOverdueHistoryReportsOneDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	underlying, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, underlying.Close()) })
	synctest.Test(t, func(t *testing.T) {
		store := &webHistoryStore{Storage: underlying, entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
		manager := zeweb.NewEditorManager(store, "config.conf", nil, nil, nil)
		server, err := zeweb.NewWebServer(zeweb.WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
		require.NoError(t, err)
		service := newWebService(server, nil, manager)
		service.drainLimit = time.Second
		require.NoError(t, manager.RecordHistory("alice", cli.ModeConfig, nil, "show private-input-never-report", "one"))
		<-store.entered
		require.NoError(t, manager.RecordHistory("alice", cli.ModeOperational, nil, "show queued", "two"))
		ended := make(chan error, 2)
		go func() { ended <- service.Shutdown(context.Background()) }()
		go func() { ended <- service.Shutdown(context.Background()) }()
		synctest.Wait()
		<-time.After(2 * time.Second)
		synctest.Wait()
		// Capture before releasing the write: a later event would hide a hang.
		events := report.Errors(0)
		close(store.release)
		require.ErrorIs(t, <-ended, errWebHistoryDrainOverdue)
		require.ErrorIs(t, <-ended, errWebHistoryDrainOverdue)
		require.NoError(t, manager.Close())
		require.NoError(t, store.Close())
		require.Equal(t, map[string]string{"config": "show private-input-never-report", "operational": "show queued"}, store.atClose)
		require.Len(t, events, 1, "concurrent callers must publish one event before blocked cleanup returns")
		event := events[0]
		require.Equal(t, "web-server", event.Source)
		require.Equal(t, "web-shutdown", event.Subject)
		require.Equal(t, "doctor-web-cleanup-overdue", event.Code)
		require.Equal(t, report.SeverityError, event.Severity)
		require.Equal(t, "ze show log recent level err component web.server", event.Fix)
		require.Contains(t, event.Message, "configuration store")
		require.NotContains(t, event.Message, "private-input-never-report")
		require.LessOrEqual(t, len(event.Message), report.MessageMax)
		require.Empty(t, event.Detail)
		diagnostic.RegisterBuiltinCodes()
		require.NotNil(t, diagnostic.Lookup(event.Code))
		require.Contains(t, diagnostic.EventCodes(), event.Code)
		for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
			if check.Name != "recent-events" {
				continue
			}
			require.Contains(t, check.Codes, event.Code)
			rows := check.Check(diagnostic.DoctorCheckContext{})
			require.Len(t, rows, 1)
			require.Equal(t, event.Code, rows[0].Code)
			require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
			require.Equal(t, event.Fix, rows[0].Fix)
			return
		}
		t.Fatal("the daemon recent-events doctor producer was not registered")
	})
}
