// Design: docs/architecture/web-components.md -- graphical value editors

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

// TestGraphicalDirectLeafReadsCandidate proves a leaf URL displays its actual
// candidate value, rather than an empty input that can overwrite it.
func TestGraphicalDirectLeafReadsCandidate(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("name", config.Leaf(config.TypeString))
	tree := config.NewTree()
	tree.Set("name", "edge-one")
	data := buildFragmentData(schema, tree, []string{"name"})
	require.Len(t, data.Fields, 1)
	assert.Equal(t, "edge-one", data.Fields[0].Value)
}

// TestGraphicalDirectSecretLeafStaysMasked checks direct leaf navigation uses
// the same masking producer as the surrounding container's field list.
func TestGraphicalDirectSecretLeafStaysMasked(t *testing.T) {
	schema := config.NewSchema()
	leaf := config.Leaf(config.TypeString)
	leaf.Sensitive = true
	schema.Define("key", leaf)
	tree := config.NewTree()
	tree.Set("key", "private-key-value")
	data := buildFragmentData(schema, tree, []string{"key"})
	require.Len(t, data.Fields, 1)
	assert.Equal(t, config.SecretDataPlaceholder, data.Fields[0].Value)
}

// TestGraphicalLeafListsHaveMemberControls proves both real BGP collection
// kinds display current members and controls at their container and leaf URLs.
func TestGraphicalLeafListsHaveMemberControls(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(`bgp {
		filter { import { filters [ first second ]; final reject; } }
		peer upstream { session { community { send [ standard large ]; } } }
	}`)
	require.NoError(t, err)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	for _, path := range [][]string{
		{"bgp", "filter", "import"},
		{"bgp", "filter", "import", "filters"},
		{"bgp", "peer", "upstream", "session", "community"},
		{"bgp", "peer", "upstream", "session", "community", "send"},
	} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			data := buildFragmentData(schema, tree, path)
			html := string(renderer.renderComponent("graphical values", detail(data)))
			assert.Contains(t, html, `name="member"`)
			assert.Contains(t, html, `name="new-member"`)
			assert.Contains(t, html, `value="save"`)
			if strings.Contains(strings.Join(path, "/"), "community") {
				assert.Contains(t, html, "standard")
				assert.Contains(t, html, "large")
			} else {
				assert.Contains(t, html, `value="first"`)
				assert.Contains(t, html, `value="second"`)
			}
		})
	}
}

// TestGraphicalLeafListReplacementUsesCandidate proves a graphical save replaces
// and reorders the selected collection, preserving another field beside it.
func TestGraphicalLeafListReplacementUsesCandidate(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "filter", "import"}, "filters", "old"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "filter", "import"}, "final", "reject"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil)
	form := url.Values{"leaf": {"filters"}, "collection": {"true"}, "operation": {"save"}, "member": {"second", "first"}}
	req := authedRequest(t, http.MethodPost, "/config/set/bgp/filter/import", form)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	tree := walkTree(mgr.snapshotTree("testuser"), schema, []string{"bgp", "filter", "import"})
	require.NotNil(t, tree)
	assert.Equal(t, []string{"second", "first"}, tree.GetSlice("filters"))
	value, present := tree.Get("final")
	assert.True(t, present)
	assert.Equal(t, "reject", value)
}

// TestGraphicalTerminalKindsRenderRows checks every non-scalar terminal kind
// through the live detail component, including unset collections.
func TestGraphicalTerminalKindsRenderRows(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("words", &config.MultiLeafNode{Type: config.TypeString})
	schema.Define("brackets", &config.BracketLeafListNode{Type: config.TypeString, Ordered: true})
	schema.Define("freeform", config.Freeform())
	schema.Define("routes", config.InlineList(config.TypeString, config.Field("description", config.Leaf(config.TypeString))))
	tree := config.NewTree()
	tree.SetSlice("words", []string{"python3", "two words"})
	tree.SetSlice("brackets", []string{"65001", "65001"})
	entries := config.NewTree()
	entries.Set("ipv4 unicast", "true")
	tree.SetContainer("freeform", entries)
	route := config.NewTree()
	route.Set("description", "route description")
	tree.AddListEntry("routes", "lab", route)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	for _, tc := range []struct {
		path []string
		want string
	}{
		{[]string{"words"}, `value="two words"`},
		{[]string{"brackets"}, `value="65001"`},
		{[]string{"freeform"}, `value="ipv4 unicast"`},
		{[]string{"routes"}, "Add entry"},
		{[]string{"routes", "lab"}, `value="route description"`},
	} {
		t.Run(strings.Join(tc.path, "/"), func(t *testing.T) {
			data := buildFragmentData(schema, tree, tc.path)
			html := string(renderer.renderComponent("graphical kind", detail(data)))
			assert.Contains(t, html, tc.want)
			data.ReadOnly = true
			readOnly := string(renderer.renderComponent("read only kind", detail(data)))
			assert.NotContains(t, readOnly, `type="submit"`)
			if tc.path[0] != "routes" {
				assert.Contains(t, readOnly, "disabled")
			}
		})
	}
	unset := buildFragmentData(schema, config.NewTree(), []string{"brackets"})
	html := string(renderer.renderComponent("unset collection", detail(unset)))
	assert.Contains(t, html, `name="new-member"`)
	assert.NotContains(t, html, `name="member"`)
}

// TestGraphicalCollectionRefusalKeepsCandidate proves enum restrictions and
// duplicate restrictions refuse the entire save before changing another member.
func TestGraphicalCollectionRefusalKeepsCandidate(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	path := []string{"bgp", "peer", "upstream", "session", "community"}
	require.NoError(t, mgr.SetValue("testuser", path, "send", "standard"))
	handler := HandleConfigSetWithAuthorizer(mgr, schema, nil, nil)
	for _, members := range [][]string{{"large", "invalid"}, {"large", "large"}} {
		form := url.Values{"leaf": {"send"}, "collection": {"true"}, "operation": {"save"}, "member": members}
		req := authedRequest(t, http.MethodPost, "/config/set/"+strings.Join(path, "/"), form)
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, []string{"standard"}, walkTree(mgr.snapshotTree("testuser"), schema, path).GetSlice("send"))
	}
}

// TestGraphicalCollectionReadOnlySaveRefused exercises the set route gate,
// rather than relying on hidden controls to enforce authorization.
func TestGraphicalCollectionReadOnlySaveRefused(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	handler := HandleConfigSetWithAuthorizer(mgr, schema, nil, fakeAuthorizer{allowEdit: false})
	form := url.Values{"leaf": {"filters"}, "collection": {"true"}, "operation": {"save"}, "member": {"first"}}
	req := authedRequest(t, http.MethodPost, "/config/set/bgp/filter/import", form)
	rec := httptest.NewRecorder()
	handler(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Zero(t, mgr.ChangeCount("testuser"))
}

// TestGraphicalMemberActivationPreservesOrder uses the real save handler to
// toggle one member while retaining another member's state and its position.
func TestGraphicalMemberActivationPreservesOrder(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	path := []string{"bgp", "filter", "import"}
	require.NoError(t, mgr.SetValue("testuser", path, "filters", "first second third"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil)
	for _, tc := range []struct {
		operation string
		inactive  bool
	}{{"deactivate:1", true}, {"activate:1", false}} {
		form := url.Values{"leaf": {"filters"}, "collection": {"true"}, "operation": {tc.operation}, "member": {"first", "second", "third"}, "member-inactive": {"true", "false", "false"}}
		if !tc.inactive {
			form["member-inactive"][1] = "true"
		}
		req := authedRequest(t, http.MethodPost, "/config/set/bgp/filter/import", form)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		handler(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, []config.MemberState{{Value: "first", Inactive: true}, {Value: "second", Inactive: tc.inactive}, {Value: "third"}}, walkTree(mgr.snapshotTree("testuser"), schema, path).GetMultiValuesState("filters"))
		assert.Contains(t, rec.Body.String(), "collection-deactivate-filters-2")
	}
}
