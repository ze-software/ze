// Design: docs/architecture/web-interface.md -- URL Scheme and view routing.
// Related: fragment.go -- IsViewRoutePath and extractPath.

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsViewRoutePathClassifiesViewPrefixes verifies which URL paths route to
// the active UI shell (Finder or Workbench) versus the Finder fragment handler.
//
// VALIDATES: /show/, /monitor/, /config/edit/ and /config/edit route through
// the active UI handler while /fragment/detail and non-view paths do not.
// PREVENTS: emitted /config/edit/... URLs falling through to fragmentHandler
// and stranding Workbench users in Finder.
func TestIsViewRoutePathClassifiesViewPrefixes(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{path: "/show/", want: true},
		{path: "/show/system/host/", want: true},
		{path: "/monitor/", want: true},
		{path: "/monitor/bgp/", want: true},
		{path: "/config/edit/", want: true},
		{path: "/config/edit/bgp/peer/192.0.2.1/", want: true},
		{path: "/config/edit", want: true},
		{path: "/config/edit-other", want: false},
		{path: "/fragment/detail", want: false},
		{path: "/fragment/detail/bgp/", want: false},
		{path: "/nothing-here", want: false},
	} {
		assert.Equal(t, tc.want, IsViewRoutePath(tc.path), tc.path)
	}
}

// TestExtractPathHandlesConfigEditRootAndSubpaths verifies extractPath resolves
// both /config/edit and /config/edit/ to root and /config/edit/<path>/ to its
// decoded YANG tokens.
//
// VALIDATES: emitted /config/edit URLs extract identical paths to /show/ URLs.
// PREVENTS: /config/edit (no trailing slash) decoding as ["config", "edit"]
// and triggering an invalid-path redirect.
func TestExtractPathHandlesConfigEditRootAndSubpaths(t *testing.T) {
	for _, tc := range []struct {
		target string
		want   []string
	}{
		{target: "/config/edit", want: nil},
		{target: "/config/edit/", want: nil},
		{target: "/config/edit/system/", want: []string{"system"}},
		{target: "/config/edit/bgp/peer/192.0.2.1/", want: []string{"bgp", "peer", "192.0.2.1"}},
		{target: "/show", want: nil},
		{target: "/monitor", want: nil},
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)
		got, err := extractPath(req)
		require.NoError(t, err, tc.target)
		assert.Equal(t, tc.want, got, tc.target)
	}
}
