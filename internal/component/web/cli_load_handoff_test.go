// Design: docs/architecture/web-interface.md -- browser-owned load content.
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: a legacy command request needing local content reaches the actual
// picker with its command and context, while leaving the candidate unchanged.
func TestCLIBarLoadHandoffKeepsCommandAndContext(t *testing.T) {
	mgr, _ := newYANGTerminal(t)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleCLICommand(mgr, mgr.schema, renderer)
	before := config.Serialize(mgr.Tree("testuser"), mgr.schema)
	for _, line := range []string{"load terminal relative merge", `top load file absolute replace "two words.conf"`} {
		req := authedRequest(t, http.MethodPost, "/cli", url.Values{"command": {line}, "path": {"bgp"}})
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		handler(rec, req)
		target, err := url.Parse(rec.Header().Get("HX-Redirect"))
		require.NoError(t, err)
		assert.Equal(t, "/cli", target.Path)
		assert.Equal(t, line, target.Query().Get("command"))
		assert.Equal(t, "bgp", target.Query().Get("path"))
		assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema))
	}
}

// VALIDATES: GET can open a content request, but cannot auto-run other verbs.
func TestCLIPageLoadHandoffIsContentOnly(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleCLIPageHTTP(renderer, false, newTestEditorManager(t))
	rec := httptest.NewRecorder()
	query := url.Values{"command": {"load terminal relative merge"}, "path": {"bgp"}}
	handler(rec, authedRequest(t, http.MethodGet, "/cli?"+query.Encode(), nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `data-initial-command="load terminal relative merge"`)
	assert.Contains(t, rec.Body.String(), `id="cli-context-path" class="is-hidden">bgp</span>`)
	assert.Contains(t, rec.Body.String(), `id="cli-load-file"`)
	for _, bad := range []url.Values{
		{"command": {"set bgp router-id 192.0.2.7"}},
		{"command": {"top set bgp router-id 192.0.2.7"}},
		{"command": {strings.Repeat("x", maxCommandLength+1)}},
		{"command": {"load terminal relative merge"}, "path": {"../bgp"}},
	} {
		rec = httptest.NewRecorder()
		handler(rec, authedRequest(t, http.MethodGet, "/cli?"+bad.Encode(), nil))
		assert.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
}
