package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// requireStatus asserts the recorder's status code and, on mismatch, dumps
// the response body and headers so the failure message is actionable.
func requireStatus(t *testing.T, want int, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code == want {
		return
	}
	body := rec.Body.String()
	if len(body) > 1024 {
		body = body[:1024] + fmt.Sprintf("... (%d bytes truncated)", len(body)-1024)
	}
	t.Fatalf("status %d, want %d\nHeaders: %v\nBody:\n%s", rec.Code, want, rec.Header(), body)
}

// requireContains is requireStatus + body-contains in one call.
func requireContains(t *testing.T, rec *httptest.ResponseRecorder, substr string) {
	t.Helper()
	requireStatus(t, http.StatusOK, rec)
	body := rec.Body.String()
	require.Contains(t, body, substr)
}

// webRequest builds an httptest request with common web-test options.
type webRequest struct {
	method  string
	path    string
	body    string
	headers map[string]string
}

func newGET(path string) *webRequest {
	return &webRequest{method: http.MethodGet, path: path}
}

func newPOST(path string) *webRequest {
	return &webRequest{method: http.MethodPost, path: path}
}

func (r *webRequest) htmx() *webRequest {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers["HX-Request"] = "true"
	return r
}

func (r *webRequest) form(values url.Values) *webRequest {
	r.body = values.Encode()
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers["Content-Type"] = "application/x-www-form-urlencoded"
	return r
}

// build returns the httptest request and a fresh recorder.
func (r *webRequest) build(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var bodyReader *strings.Reader
	if r.body != "" {
		bodyReader = strings.NewReader(r.body)
	}
	var req *http.Request
	if bodyReader != nil {
		req = httptest.NewRequestWithContext(t.Context(), r.method, r.path, bodyReader)
	} else {
		req = httptest.NewRequestWithContext(t.Context(), r.method, r.path, http.NoBody)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	return req, httptest.NewRecorder()
}

// serve calls handler.ServeHTTP and returns the recorder for assertion.
func (r *webRequest) serve(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req, rec := r.build(t)
	handler.ServeHTTP(rec, req)
	return rec
}
