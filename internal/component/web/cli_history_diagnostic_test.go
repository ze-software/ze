// Design: docs/architecture/doctor-and-health-checks.md -- browser recall boundary failures.
// Related: cli_history.go -- actual request admission and JSON response producers.
package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestBrowserHistoryRefusalReportsBoundedDoctorEvent admits a valid submission,
// then refuses invalid receipts without preventing the command's execution.
func TestBrowserHistoryRefusalReportsBoundedDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	mgr := newTestEditorManager(t)
	submit := func(receipt string) *http.Request {
		form := url.Values{"command": {"show private-input-never-report"}, "submission": {receipt}}
		request := httptest.NewRequest(http.MethodPost, "/cli", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request = request.WithContext(withUsername(request.Context(), "private-user-never-report"))
		return recordCLIHistory(httptest.NewRecorder(), request, mgr, terminalModeOperational, nil, form.Get("command"), nil)
	}
	require.Empty(t, cliHistoryWarning(submit("valid-one")))
	require.Empty(t, report.Errors(0), "successful recall must not raise a problem")
	for range 8 {
		require.Contains(t, cliHistoryWarning(submit("invalid receipt")), "identifier is invalid")
	}
	requireWebRecallDoctorEvent(t, "doctor-web-history-unavailable", "command-recall", "Command recall was unavailable")
}

type failingHistoryResponse struct {
	*httptest.ResponseRecorder
}

func (*failingHistoryResponse) Write([]byte) (int, error) {
	return 0, errors.New("private-write-error-never-report")
}

// TestBrowserHistoryResponseFailureReportsBoundedDoctorEvent writes actual JSON
// successfully first, then fails at the client writer rather than the store.
func TestBrowserHistoryResponseFailureReportsBoundedDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	mgr := newTestEditorManager(t)
	request := httptest.NewRequest(http.MethodGet, "/cli?history=1&mode=operational", nil)
	request = request.WithContext(withUsername(request.Context(), "private-user-never-report"))
	response := httptest.NewRecorder()
	writeCLIHistory(response, request, mgr)
	require.JSONEq(t, `{"entries":[]}`, response.Body.String())
	require.Empty(t, report.Errors(0), "successful JSON delivery must not raise a problem")
	for range 8 {
		writeCLIHistory(&failingHistoryResponse{httptest.NewRecorder()}, request, mgr)
	}
	requireWebRecallDoctorEvent(t, "doctor-web-history-response", "command-recall-response", "Command recall could not be sent")
}

// requireWebRecallDoctorEvent reads the real report bus and doctor registration,
// without injecting an event or retaining request-controlled report keys.
func requireWebRecallDoctorEvent(t *testing.T, code, subject, message string) {
	t.Helper()
	events := report.Errors(0)
	require.Len(t, events, 1)
	event := events[0]
	require.Equal(t, "web-history", event.Source)
	require.Equal(t, code, event.Code)
	require.Equal(t, subject, event.Subject)
	require.Contains(t, event.Message, message)
	require.NotContains(t, event.Message, "private-")
	require.Equal(t, "Open /cli and reconnect to restore command recall", event.Fix)
	require.Empty(t, event.Detail)
	diagnostic.RegisterBuiltinCodes()
	require.NotNil(t, diagnostic.Lookup(code))
	require.Contains(t, diagnostic.EventCodes(), code)
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "recent-events" {
			continue
		}
		require.Contains(t, check.Codes, code)
		rows := check.Check(diagnostic.DoctorCheckContext{})
		require.Len(t, rows, 1)
		require.Equal(t, code, rows[0].Code)
		require.Equal(t, event.Fix, rows[0].Fix)
		return
	}
	t.Fatal("the daemon recent-events doctor producer was not registered")
}
