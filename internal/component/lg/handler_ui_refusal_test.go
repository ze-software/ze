// Design: docs/architecture/api/birdwatcher-compat.md -- Section 3.1, a command the engine refuses
// Related: handler_ui.go -- the pages that refuse through ask
// Related: handler_graph.go -- handleGraph, which refuses through ask
// Related: handler_api_dispatch_test.go -- apiRequest, the same refusals on the API

package lg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// failingDispatch fails every command, the way a dispatcher fails a command
// no plugin serves.
func failingDispatch(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
	return nil, errTestEngineDown
}

// silentDispatch answers every command done and empty.
func silentDispatch(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
	return plugin.NewResponse(plugin.StatusDone, nil), nil
}

// TestUIPagesRefuseAFailedCommand proves that a page which cannot show what
// it exists to show answers the engine's failure, never a page built over it.
//
// VALIDATES: the peer page, its download, the route detail and the graph each
// answer 502 when the dispatch fails and 503 when the engine answers nothing,
// with a body naming the command and the failure class, and never the
// dispatcher's own error.
// PREVENTS: the peer page answering 404 "not found" for a peer the engine was
// never asked about, the graph drawing "No routes found" over a failure, the
// download answering 500, and a transport error (a socket path, a host
// detail) reaching an unauthenticated client.
func TestUIPagesRefuseAFailedCommand(t *testing.T) {
	pages := []struct {
		target  string
		command string
	}{
		{"/lg/peer/192.0.2.1", cmdBGPOverview},
		{"/lg/peer/192.0.2.1/download", "show bgp rib received peer 192.0.2.1"},
		{"/lg/route/detail?prefix=10.0.0.0/24&peer=192.0.2.1", "show bgp rib received prefix 10.0.0.0/24"},
		{"/lg/graph?prefix=10.0.0.0/24", "show bgp rib received prefix 10.0.0.0/24"},
	}
	failures := []struct {
		name     string
		dispatch CommandDispatcher
		status   int
		class    string
	}{
		{"dispatch fails", failingDispatch, http.StatusBadGateway, failureEngineError},
		{"engine silent", silentDispatch, http.StatusServiceUnavailable, failureEngineUnavailable},
	}

	for _, page := range pages {
		for _, failure := range failures {
			t.Run(page.target+"/"+failure.name, func(t *testing.T) {
				rec := apiRequest(t, failure.dispatch, page.target)

				if rec.Code != failure.status {
					t.Fatalf("status = %d, want %d; body %s", rec.Code, failure.status, rec.Body.String())
				}
				body := rec.Body.String()
				if want := page.command + ": " + failure.class; !strings.Contains(body, want) {
					t.Errorf("body %q does not carry %q", body, want)
				}
				if strings.Contains(body, errTestEngineDown.Error()) {
					t.Errorf("body %q carries the dispatcher's own error", body)
				}
			})
		}
	}
}

// TestUISearchBannerNamesTheFailure proves the search page reports a failed
// search in its banner, which is how that page reports every refusal.
//
// VALIDATES: a POST to /lg/search whose dispatch fails renders the error
// banner naming the command and "engine error", and not the dispatcher's
// error.
// PREVENTS: a failed search reading as a search that matched nothing.
func TestUISearchBannerNamesTheFailure(t *testing.T) {
	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    failingDispatch,
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	form := url.Values{"prefix": {"10.0.0.0/24"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/lg/search", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="error-banner"`) {
		t.Fatalf("no error banner; status %d, body %s", rec.Code, body)
	}
	if want := "show bgp rib received prefix 10.0.0.0/24: " + failureEngineError; !strings.Contains(body, want) {
		t.Errorf("banner does not carry %q: %s", want, body)
	}
	if strings.Contains(body, errTestEngineDown.Error()) {
		t.Errorf("banner carries the dispatcher's own error: %s", body)
	}
}
