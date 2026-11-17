//go:build ze_web

// Design: docs/architecture/web-interface.md -- route cleanup retains its own diagnostic class.
package hub

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selfcert"
)

type failedRouteCleanup struct{ failure error }

func (owner failedRouteCleanup) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (owner failedRouteCleanup) Close() error { return owner.failure }

// TestOwnedRouteCleanupIsNotANetworkFailure seeds an already reported storage
// event and drives a route-only Close failure through the real service join.
// Classification must preserve that event without inventing a network failure.
// The storage producer itself is covered by TestBackupHTTPStorageFailureReachesDoctor.
func TestOwnedRouteCleanupIsNotANetworkFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	report.RaiseErrorFixOnce("backup-workspace", diagnostic.CodeDoctorBackupWorkspaceStorage, "private-artifact",
		"Private backup cleanup did not finish.", "Run ze show doctor", time.Minute)
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	server, err := zeweb.NewWebServer(zeweb.WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
	require.NoError(t, err)
	failure := errors.New("owned backup resource cleanup failed")
	server.OwnHandler(failedRouteCleanup{failure: failure})
	service := newWebService(server, nil, nil)
	err = service.Shutdown(context.Background())
	require.ErrorIs(t, err, failure)
	assert.NoError(t, zeweb.ShutdownNetworkError(err), "a route close failure is not a socket failure")
	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, diagnostic.CodeDoctorBackupWorkspaceStorage, events[0].Code)
	assert.NotEqual(t, diagnostic.CodeDoctorWebNetworkCleanup, events[0].Code)
	require.ErrorIs(t, service.Shutdown(context.Background()), failure)
}
