// Design: docs/architecture/web-components.md -- multi-word leaf controls.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestGraphicalMultiWordLeafHasWholeSettingActivation keeps word editing
// available without offering a member toggle the shared editor cannot apply.
func TestGraphicalMultiWordLeafHasWholeSettingActivation(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("words", &config.MultiLeafNode{Type: config.TypeString})
	tree := config.NewTree()
	tree.SetSlice("words", []string{"first", "second"})
	renderer, err := NewRenderer()
	require.NoError(t, err)
	data := buildFragmentData(schema, tree, []string{"words"})
	data.Actions = buildGraphicalActions(schema, tree, []string{"words"})
	html := string(renderer.renderComponent("multi-word leaf", detail(data)))
	assert.Contains(t, html, `value="first"`)
	assert.Contains(t, html, `value="second"`)
	assert.Contains(t, html, `id="collection-save-words"`)
	assert.Contains(t, html, `id="graphical-deactivate"`)
	assert.NotContains(t, html, `id="collection-deactivate-words-`)
	assert.NotContains(t, html, `id="collection-activate-words-`)
}

// TestGraphicalMultiWordLeafRefusesMemberActivation proves a forged row toggle
// cannot remove words, including an inactive marker carried in a Save request.
func TestGraphicalMultiWordLeafRefusesMemberActivation(t *testing.T) {
	module := `module ze-web-multi-word-conf {
  yang-version 1.1;
  namespace "urn:ze:web-multi-word";
  prefix probe;
  import ze-extensions { prefix ze; }
  leaf graphical-words { type string; ze:syntax "multi-leaf"; }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-web-multi-word-conf.yang", module))
	mgr, schema := newPromotingEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", nil, "graphical-words", "first second"))
	before := mgr.ContentAtPath("testuser", nil)
	changes := mgr.ChangeCount("testuser")
	handler := HandleConfigSetWithAuthorizer(mgr, schema, nil, nil)
	for _, tc := range []struct {
		operation string
		inactive  string
	}{{"deactivate:0", "false"}, {"activate:0", "false"}, {"save", "true"}} {
		t.Run(tc.operation, func(t *testing.T) {
			form := url.Values{
				"leaf": {"graphical-words"}, "collection": {"true"}, "operation": {tc.operation},
				"member": {"first", "second"}, "member-inactive": {tc.inactive, "false"},
			}
			req := authedRequest(t, http.MethodPost, "/config/set/", form)
			rec := httptest.NewRecorder()
			handler(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "whole setting")
			assert.Equal(t, before, mgr.ContentAtPath("testuser", nil))
			assert.Equal(t, changes, mgr.ChangeCount("testuser"))
			assert.Equal(t, []string{"first", "second"}, mgr.snapshotTree("testuser").GetSlice("graphical-words"))
		})
	}
}
