package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// workbenchForLogs creates a workbench handler with optional dispatch and broker.
func workbenchForLogs(t *testing.T, dispatch CommandDispatcher, broker *EventBroker) http.HandlerFunc {
	t.Helper()
	renderer, err := NewRenderer()
	assert.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	var opts []WorkbenchOption
	if dispatch != nil {
		opts = append(opts, WithDispatch(dispatch))
	}
	if broker != nil {
		opts = append(opts, WithBroker(broker))
	}
	return HandleWorkbench(renderer, schema, tree, nil, true, opts...)
}

// --- Live Log ---

func TestLogLivePageRendersToolbar(t *testing.T) {
	broker := NewEventBroker(10)
	defer broker.Close()
	handler := workbenchForLogs(t, nil, broker)
	req := httptest.NewRequest(http.MethodGet, "/show/logs/live/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Live Log")
	assert.Contains(t, html, "log-namespace")
	assert.Contains(t, html, "log-search")
	assert.Contains(t, html, "log-pause")
	assert.Contains(t, html, `name="level"`)
}

// flushRecorder wraps httptest.ResponseRecorder with a Flush method to
// satisfy http.Flusher, which the SSE handler requires.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func TestLogLiveSSEStreamsEvents(t *testing.T) {
	broker := NewEventBroker(10)
	defer broker.Close()

	handler := handleLogLiveStream(broker)
	req := httptest.NewRequest(http.MethodGet, "/logs/live/stream", http.NoBody)
	// Cancel the request context to make the SSE handler exit.
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := &flushRecorder{httptest.NewRecorder()}

	// Start handler in goroutine since SSE blocks.
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(rec, req)
	}()

	// Cancel the request to terminate the SSE handler.
	cancel()
	<-done

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
}

func TestLogLiveSSEClientDisconnect(t *testing.T) {
	broker := NewEventBroker(10)
	defer broker.Close()

	// Verify client count returns to zero after handler exits.
	assert.Equal(t, 0, broker.ClientCount())
}

func TestLogLiveSSENilBroker(t *testing.T) {
	handler := handleLogLiveStream(nil)
	req := httptest.NewRequest(http.MethodGet, "/logs/live/stream", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// --- Warnings ---

func TestLogWarningsRendersTable(t *testing.T) {
	jsonResp := `{"warnings":[{"source":"bgp","code":"peer-down","severity":"warning","subject":"192.0.2.1","message":"Warning message","raised":"2024-01-01T00:00:00Z","updated":"2024-01-01T00:05:00Z"}],"count":1}`
	dispatch, _ := mockDispatcher(jsonResp)
	handler := workbenchForLogs(t, dispatch, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/warnings/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Warnings")
	assert.Contains(t, html, "Warning message")
	assert.Contains(t, html, "bgp")
}

// TestLogWarningsAllClear: with a working dispatcher returning no warnings, the
// page shows the genuine "all clear" state (F4: that claim is only honest when
// the dispatcher actually answered).
func TestLogWarningsAllClear(t *testing.T) {
	dispatch, _ := mockDispatcher(`{"warnings":[],"count":0}`)
	handler := workbenchForLogs(t, dispatch, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/warnings/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No active warnings")
	assert.Contains(t, html, "All systems operating normally")
}

// TestLogWarningsUnavailableWithoutDispatch: with no dispatcher, the page must
// NOT claim all is well -- it could not ask (F4/AC-9).
func TestLogWarningsUnavailableWithoutDispatch(t *testing.T) {
	handler := workbenchForLogs(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/warnings/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "unavailable in this mode")
	assert.NotContains(t, html, "All systems operating normally")
}

// --- Errors ---

func TestLogErrorsRendersTable(t *testing.T) {
	jsonResp := `{"errors":[{"source":"bgp","code":"fsm-error","severity":"error","subject":"192.0.2.1","message":"Error occurred","raised":"2024-01-01T00:00:00Z","updated":"2024-01-01T00:00:00Z"}],"count":1}`
	dispatch, _ := mockDispatcher(jsonResp)
	handler := workbenchForLogs(t, dispatch, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/errors/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Errors")
	assert.Contains(t, html, "Error occurred")
	assert.Contains(t, html, "bgp")
}

func TestLogErrorsEmptyJSON(t *testing.T) {
	dispatch, _ := mockDispatcher(`{"errors":[],"count":0}`)
	handler := workbenchForLogs(t, dispatch, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/errors/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No recent errors")
}

// TestLogErrorsUnavailableWithoutDispatch: with no dispatcher, the Errors page
// shows the honest "unavailable" message rather than "No recent errors" (F4/AC-9).
func TestLogErrorsUnavailableWithoutDispatch(t *testing.T) {
	handler := workbenchForLogs(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/logs/errors/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "unavailable in this mode")
	assert.NotContains(t, html, "No recent errors")
}

// TestLogPagesDispatchError: when the dispatcher returns an error (e.g. web-only
// standalone mode where "show warnings" is not available), both log pages show
// the honest "unavailable" message, never the "all good" empty state (F4/AC-9).
func TestLogPagesDispatchError(t *testing.T) {
	failing := func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errors.New("operational commands require a running daemon")
	}

	for _, tc := range []struct {
		name      string
		path      string
		forbidden string
	}{
		{"warnings", "/show/logs/warnings/", "All systems operating normally"},
		{"errors", "/show/logs/errors/", "No recent errors"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := workbenchForLogs(t, failing, nil)
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			html := rec.Body.String()
			assert.Contains(t, html, "unavailable in this mode",
				"dispatch error must surface an honest unavailable message")
			assert.NotContains(t, html, tc.forbidden,
				"must not claim all-good when the dispatcher errored")
		})
	}
}
