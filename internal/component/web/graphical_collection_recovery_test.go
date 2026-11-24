// Design: docs/architecture/web-components.md -- graphical collection refusal recovery.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphicalCollectionRefusalAllowsCorrection exercises the authenticated
// save route: a refusal preserves the current form and candidate, then a
// corrected request adds the selected member without replacing its sibling.
func TestGraphicalCollectionRefusalAllowsCorrection(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	path := []string{"bgp", "peer", "upstream", "session", "community"}
	require.NoError(t, mgr.SetValue("testuser", path, "send", "standard"))
	sibling := []string{"bgp", "peer", "upstream", "session"}
	require.NoError(t, mgr.SetValue("testuser", sibling, "router-id", "10.0.0.77"))
	changes := mgr.ChangeCount("testuser")
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil)
	form := url.Values{"leaf": {"send"}, "collection": {"true"}, "operation": {"add"}, "member": {"standard"}, "new-member": {""}}
	req := authedRequest(t, http.MethodPost, "/config/set/"+strings.Join(path, "/"), form)
	req.Header.Set("HX-Request", htmxRequestTrue)
	rec := httptest.NewRecorder()
	handler(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "none", rec.Header().Get("HX-Reswap"), "the error must not replace the collection form")
	assert.Contains(t, rec.Body.String(), "Choose or enter the new member")
	assert.Contains(t, rec.Body.String(), `id="error-list"`)
	assert.Contains(t, rec.Body.String(), `hx-swap-oob="beforeend"`)
	assert.Equal(t, []string{"standard"}, walkTree(mgr.snapshotTree("testuser"), schema, path).GetSlice("send"))
	assert.Equal(t, changes, mgr.ChangeCount("testuser"))

	form.Set("new-member", "large")
	req = authedRequest(t, http.MethodPost, "/config/set/"+strings.Join(path, "/"), form)
	req.Header.Set("HX-Request", htmxRequestTrue)
	rec = httptest.NewRecorder()
	handler(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Get("HX-Reswap"))
	assert.Contains(t, rec.Body.String(), `id="collection-add-send"`)
	assert.Equal(t, []string{"standard", "large"}, walkTree(mgr.snapshotTree("testuser"), schema, path).GetSlice("send"))
	value, present := walkTree(mgr.snapshotTree("testuser"), schema, sibling).Get("router-id")
	assert.True(t, present)
	assert.Equal(t, "10.0.0.77", value)
}
