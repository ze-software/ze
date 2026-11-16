// Design: docs/architecture/web-interface.md -- schema key path transport.

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestURLPathKeepsEncodedKeyBoundaries proves URL parsing separates escaped
// segments before decoding key bytes, including a real policy prefix key.
func TestURLPathKeepsEncodedKeyBoundaries(t *testing.T) {
	for _, key := range []string{"192.0.2.0/24", "two words?/%雪", `quoted"key`} {
		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
				"/show/policy/prefix-list/edge/entry/"+url.PathEscape(key)+"/", http.NoBody)
			parsed, err := ParseURL(req)
			require.NoError(t, err)
			assert.Equal(t, []string{"policy", "prefix-list", "edge", "entry", key}, parsed.Path)
		})
	}
}

// TestBrowserCLIContextKeepsQuotedKey proves an entered keyed context survives
// its JSON reply and a subsequent request before another mutation is applied.
func TestBrowserCLIContextKeepsQuotedKey(t *testing.T) {
	mgr, handler := newYANGTerminal(t)
	key := "two words?/%雪"
	request := func(command, path string) terminalResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		handler(rec, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{
			"command": {command}, "path": {path}, "mode": {terminalModeConfig},
		}))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var response terminalResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.False(t, strings.HasPrefix(response.Output, "error:"), response.Output)
		return response
	}
	request(`set bgp group "`+key+`" description first`, "")
	context := request(`edit bgp group "`+key+`"`, "")
	assert.Equal(t, "bgp/group/"+url.PathEscape(key), context.Path)
	request("set description second", context.Path)
	value, present := walkTree(mgr.Tree("testuser"), mgr.schema, []string{"bgp", "group", key}).Get("description")
	assert.True(t, present)
	assert.Equal(t, "second", value)
}

// TestConfigPathTransportSeparatesEncodingLayers protects literal percent
// sequences from double decoding and refuses malformed or unsafe transport.
func TestConfigPathTransportSeparatesEncodingLayers(t *testing.T) {
	for _, token := range []string{"192.0.2.0/24", "two words?/%雪", "%2F", "%zz", "a+b@c&d=e$~", `a"b`} {
		path := []string{"bgp", "group", token}
		encoded := encodeConfigPath(path)
		decoded, err := decodeConfigPath(encoded)
		require.NoError(t, err)
		assert.Equal(t, path, decoded)
		for _, target := range []string{showPathPrefix + encoded + "/", fragmentURL(encoded), configEditPath + encoded + "/"} {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
			got, err := extractPath(r)
			require.NoError(t, err, target)
			assert.Equal(t, path, got)
		}
	}
	for _, malformed := range []string{"bgp/group/%", "bgp/group/%2", "bgp/group/%zz", "bgp//group", "bgp/%2e%2e", "bgp/%00", "bgp/%0a", "bgp/%ff"} {
		_, err := decodeConfigPath(malformed)
		assert.Error(t, err, malformed)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/bgp/", http.NoBody)
	r.URL.RawPath = "/show/%zz/"
	_, err := ParseURL(r)
	assert.Error(t, err)
	r = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/fragment/detail?path=bgp%zz", http.NoBody)
	_, err = extractPath(r)
	assert.Error(t, err)
}

// TestConfigKeyDOMIDsRemainDistinct checks all URI pchars and escaped bytes
// remain usable by HTMX's CSS selectors without changing legacy safe IDs.
func TestConfigKeyDOMIDsRemainDistinct(t *testing.T) {
	assert.Equal(t, "input-bgp-speer-s192-d0-d2-d1-sdescription", fieldInputID("bgp/peer/192.0.2.1", "description"))
	seen := make(map[string]string)
	for _, key := range []string{"a/b", "a%2Fb", "a b", "a+b", "a@b", "a&b", "a=b", "a$b", "a~b", "a-x25b", "a%25b", "a?b", `a"b`, "雪"} {
		id := fieldInputID(encodeConfigPath([]string{"bgp", "group", key}), "description")
		assert.NotRegexp(t, `[^A-Za-z0-9_-]`, id)
		if previous, exists := seen[id]; exists {
			t.Fatalf("keys %q and %q share ID %q", previous, key, id)
		}
		seen[id] = key
	}
}

// TestGraphicalPrefixKeyURLSaveReadback uses the real prefix-key schema and
// producer-built links, then reads both the candidate and the rendered field.
func TestGraphicalPrefixKeyURLSaveReadback(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", `set policy prefix-list edge entry 192.0.2.0/24 action accept`)
	path := []string{"policy", "prefix-list", "edge", "entry", "192.0.2.0/24"}
	renderer, err := NewRenderer()
	require.NoError(t, err)
	list := buildFragmentData(mgr.schema, mgr.Tree("testuser"), path[:len(path)-1])
	require.NotEmpty(t, list.Columns)
	var entryURL string
	for _, column := range list.Columns {
		for _, item := range column.NamedItems {
			if item.Name == "192.0.2.0/24" {
				entryURL = item.URL
			}
		}
	}
	require.Equal(t, "/show/policy/prefix-list/edge/entry/192.0.2.0%2F24/", entryURL)
	parsed, err := ParseURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, entryURL, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, path, parsed.Path)
	data := buildFragmentData(mgr.schema, mgr.Tree("testuser"), parsed.Path)
	require.NotEmpty(t, data.Fields)
	var fieldPath string
	for _, field := range data.Fields {
		if field.Leaf == "action" {
			fieldPath = field.Path
		}
	}
	require.Equal(t, encodeConfigPath(path), fieldPath)
	target := configSetURL(fieldPath)
	form := url.Values{"leaf": {"action"}, "value": {"reject"}}
	req := authedRequest(t, http.MethodPost, target, form)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	HandleConfigSetWithAuthorizer(mgr, mgr.schema, renderer, nil)(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	value, exists := walkTree(mgr.Tree("testuser"), mgr.schema, path).Get("action")
	require.True(t, exists)
	assert.Equal(t, "reject", value)
	reload := authedRequest(t, http.MethodGet, fragmentURL(fieldPath), nil)
	shown := httptest.NewRecorder()
	HandleFragment(renderer, mgr.schema, mgr.Tree("testuser"), mgr, false, nil)(shown, reload)
	require.Equal(t, http.StatusOK, shown.Code, shown.Body.String())
	assert.Contains(t, shown.Body.String(), `value="reject" selected`)
	// Read-only denial and schema rejection keep the same literal-key candidate.
	denied := httptest.NewRecorder()
	HandleConfigSetWithAuthorizer(mgr, mgr.schema, renderer, fakeAuthorizer{allowEdit: false})(denied, authedRequest(t, http.MethodPost, target, url.Values{"leaf": {"action"}, "value": {"accept"}}))
	assert.Equal(t, http.StatusForbidden, denied.Code)
	invalid := httptest.NewRecorder()
	HandleConfigSetWithAuthorizer(mgr, mgr.schema, renderer, nil)(invalid, authedRequest(t, http.MethodPost, target, url.Values{"leaf": {"action"}, "value": {"invented"}}))
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	value, _ = walkTree(mgr.Tree("testuser"), mgr.schema, path).Get("action")
	assert.Equal(t, "reject", value)
}

// TestRelatedToolKeyIDsKeepTokenBoundaries protects distinct pinned overlays
// when legal key punctuation or an apparent path separator is present.
func TestRelatedToolKeyIDsKeepTokenBoundaries(t *testing.T) {
	seen := make(map[string]bool)
	for _, path := range [][]string{{"bgp", "group", "a.b"}, {"bgp", "group", "a:b"}, {"bgp", "group", "a/b"}, {"bgp", "group", "a", "b"}, {"bgp", "group", "a-b"}} {
		id := overlayID("peer-detail", path)
		assert.NotRegexp(t, `[^A-Za-z0-9_-]`, id)
		assert.False(t, seen[id], "duplicate overlay id %q", id)
		seen[id] = true
	}
}

// TestGraphicalQuotedKeyAddRenameKeepsLiteralBytes covers creation and rename
// form values separately from already-encoded navigation paths.
func TestGraphicalQuotedKeyAddRenameKeepsLiteralBytes(t *testing.T) {
	mgr, _ := newYANGTerminal(t)
	key := " edge ?/%雪 "
	newKey := ` renamed "key" /%?雪 `
	add := httptest.NewRecorder()
	HandleConfigAddWithAuthorizer(mgr, mgr.schema, nil, nil)(add, authedRequest(t, http.MethodPost, "/config/add/bgp/group/", url.Values{"name": {key}}))
	require.Equal(t, http.StatusOK, add.Code, add.Body.String())
	require.NotNil(t, walkTree(mgr.Tree("testuser"), mgr.schema, []string{"bgp", "group", key}))
	rename := httptest.NewRecorder()
	HandleConfigRenameWithAuthorizer(mgr, mgr.schema, nil)(rename, authedRequest(t, http.MethodPost, "/config/rename/"+encodeConfigPath([]string{"bgp", "group", key}), url.Values{"new-key": {newKey}}))
	require.Equal(t, http.StatusSeeOther, rename.Code, rename.Body.String())
	assert.Nil(t, walkTree(mgr.Tree("testuser"), mgr.schema, []string{"bgp", "group", key}))
	assert.NotNil(t, walkTree(mgr.Tree("testuser"), mgr.schema, []string{"bgp", "group", newKey}))
}

// TestPendingRowsKeepEditorQuotedKey converts the editor's own path grammar
// once at the web boundary before matching a browser row's address.
func TestPendingRowsKeepEditorQuotedKey(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	key := "two words?/%雪"
	webTerminalLine(t, terminal, "", `set bgp group "`+key+`" description changed`)
	listPath := []string{"bgp", "group"}
	listNode := lookupListNode(mgr.schema, listPath)
	require.NotNil(t, listNode)
	assert.True(t, entryHasPendingChange(mgr.pendingChangePaths("testuser"), listPath, listNode, key))
	assert.False(t, entryHasPendingChange(mgr.pendingChangePaths("testuser"), listPath, listNode, "two words?"))
}

// TestQuotedKeyCommitReviewMasksSecret keeps schema identity intact when the
// editor's pending-change path carries a quoted key before a secret leaf.
func TestQuotedKeyCommitReviewMasksSecret(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", `set bgp group "secret key?/%雪" connection md5 password browser-private-sentinel`)
	review := mgr.Diff("testuser")
	assert.Contains(t, review, config.SecretDataPlaceholder)
	assert.NotContains(t, review, "browser-private-sentinel")
}
