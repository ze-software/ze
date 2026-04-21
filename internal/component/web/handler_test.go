package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutes verifies that RegisterRoutes wires root, login, and assets
// so the mux is not empty.
// VALIDATES: route registration produces working endpoints.
// PREVENTS: empty mux returning 404 for all requests (the original bug).
func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()

	authHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	assetsHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	RegisterRoutes(mux, authHandler, assetsHandler)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"root redirects to /show/", "GET", "/", http.StatusFound},
		{"assets served", "GET", "/assets/style.css", http.StatusOK},
		{"show path hits auth", "GET", "/show/bgp", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestParseURL_ShowPath verifies verb-first URL parsing for /show/ paths.
// VALIDATES: URL scheme verb-first parsing -- show prefix produces TierView with correct segments.
func TestParseURL_ShowPath(t *testing.T) {
	r := httptest.NewRequest("GET", "/show/bgp/peer/192.168.1.1/", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierView, parsed.Tier)
	assert.Equal(t, "show", parsed.Verb)
	assert.Equal(t, []string{"bgp", "peer", "192.168.1.1"}, parsed.Path)
}

// TestParseURL_MonitorPath verifies verb-first URL parsing for /monitor/ paths.
// VALIDATES: monitor prefix produces TierView with verb "monitor" and correct path segments.
func TestParseURL_MonitorPath(t *testing.T) {
	r := httptest.NewRequest("GET", "/monitor/bgp/summary", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierView, parsed.Tier)
	assert.Equal(t, "monitor", parsed.Verb)
	assert.Equal(t, []string{"bgp", "summary"}, parsed.Path)
}

// TestParseURL_ConfigEdit verifies /config/edit/<path> parsing.
// VALIDATES: config tier edit verb extracts YANG path after the verb.
func TestParseURL_ConfigEdit(t *testing.T) {
	r := httptest.NewRequest("GET", "/config/edit/bgp/peer/192.168.1.1/", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierConfig, parsed.Tier)
	assert.Equal(t, "edit", parsed.Verb)
	assert.Equal(t, []string{"bgp", "peer", "192.168.1.1"}, parsed.Path)
}

// TestParseURL_ConfigSet verifies /config/set/<path> parsing.
// VALIDATES: config tier set verb extracts YANG path after the verb.
func TestParseURL_ConfigSet(t *testing.T) {
	r := httptest.NewRequest("GET", "/config/set/bgp/peer/192.168.1.1/", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierConfig, parsed.Tier)
	assert.Equal(t, "set", parsed.Verb)
	assert.Equal(t, []string{"bgp", "peer", "192.168.1.1"}, parsed.Path)
}

// TestParseURL_ConfigRenameAccepted verifies /config/rename/<path> parsing.
func TestParseURL_ConfigRenameAccepted(t *testing.T) {
	r := httptest.NewRequest("POST", "/config/rename/bgp/peer/london/", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierConfig, parsed.Tier)
	assert.Equal(t, "rename", parsed.Verb)
	assert.Equal(t, []string{"bgp", "peer", "london"}, parsed.Path)
}

// TestParseURL_ConfigCommit verifies /config/commit parsing with no trailing path.
// VALIDATES: config verbs with no YANG path produce empty Path slice.
func TestParseURL_ConfigCommit(t *testing.T) {
	r := httptest.NewRequest("GET", "/config/commit", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierConfig, parsed.Tier)
	assert.Equal(t, "commit", parsed.Verb)
	assert.Empty(t, parsed.Path)
}

// TestParseURL_AdminPath verifies /admin/<path> parsing.
// VALIDATES: admin prefix produces TierAdmin with all path segments preserved.
func TestParseURL_AdminPath(t *testing.T) {
	r := httptest.NewRequest("POST", "/admin/peer/192.168.1.1/teardown", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierAdmin, parsed.Tier)
	assert.Equal(t, "admin", parsed.Verb)
	assert.Equal(t, []string{"peer", "192.168.1.1", "teardown"}, parsed.Path)
}

// TestParseURL_Root verifies that / returns a redirect-equivalent ParsedURL (show with no path).
// VALIDATES: root URL produces TierView with verb "show" and no path segments.
func TestParseURL_Root(t *testing.T) {
	r := httptest.NewRequest("GET", "/", http.NoBody)

	parsed, err := ParseURL(r)
	require.NoError(t, err)

	assert.Equal(t, TierView, parsed.Tier)
	assert.Equal(t, "show", parsed.Verb)
	assert.Empty(t, parsed.Path)
}

// TestParseURL_InvalidPrefix verifies that unrecognized URL prefixes return an error.
// VALIDATES: unknown prefixes are rejected, not silently routed.
// PREVENTS: arbitrary paths being accepted as valid.
func TestParseURL_InvalidPrefix(t *testing.T) {
	r := httptest.NewRequest("GET", "/invalid/path", http.NoBody)

	_, err := ParseURL(r)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "unknown URL prefix")
}

// TestValidatePathSegments validates path segment security checks.
// VALIDATES: AC-16 (path traversal rejected), null bytes rejected, empty segments rejected, slash-in-segment rejected.
// PREVENTS: directory traversal, null byte injection, malformed YANG paths.
func TestValidatePathSegments(t *testing.T) {
	tests := []struct {
		name      string
		segments  []string
		wantError bool
		errMsg    string
	}{
		// Valid segments.
		{
			name:     "valid BGP peer path",
			segments: []string{"bgp", "peer", "192.168.1.1"},
		},
		{
			name:     "valid BGP router-id",
			segments: []string{"bgp", "router-id"},
		},
		{
			name:     "valid IPv6 address",
			segments: []string{"2001:db8::1"},
		},
		{
			name:     "empty slice is valid",
			segments: []string{},
		},
		{
			name:     "nil slice is valid",
			segments: nil,
		},

		// Invalid segments.
		{
			name:      "path traversal",
			segments:  []string{".."},
			wantError: true,
			errMsg:    "path traversal",
		},
		{
			name:      "empty segment from double slash",
			segments:  []string{"bgp", "", "peer"},
			wantError: true,
			errMsg:    "empty path segment",
		},
		{
			name:      "null byte in segment",
			segments:  []string{"bgp\x00peer"},
			wantError: true,
			errMsg:    "null byte",
		},
		{
			name:      "slash in segment",
			segments:  []string{"bgp/peer"},
			wantError: true,
			errMsg:    "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathSegments(tt.segments)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNegotiateContentType verifies content negotiation logic.
// VALIDATES: AC-5 (?format=json), AC-6 (Accept header), AC-7 (URL wins over header), AC-17 (unknown format ignored).
// PREVENTS: incorrect content type selection, parameter/header priority inversion.
func TestNegotiateContentType(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		accept     string
		wantFormat string
	}{
		{
			name:       "format=json query parameter",
			url:        "/show/bgp?format=json",
			accept:     "",
			wantFormat: "json",
		},
		{
			name:       "Accept application/json header",
			url:        "/show/bgp",
			accept:     "application/json",
			wantFormat: "json",
		},
		{
			name:       "URL format wins over Accept header",
			url:        "/show/bgp?format=json",
			accept:     "text/html",
			wantFormat: "json",
		},
		{
			name:       "no header no param defaults to html",
			url:        "/show/bgp",
			accept:     "",
			wantFormat: "html",
		},
		{
			name:       "unknown format param ignored defaults to html",
			url:        "/show/bgp?format=invalid",
			accept:     "",
			wantFormat: "html",
		},
		{
			name:       "Accept text/html with json lower quality picks html",
			url:        "/show/bgp",
			accept:     "text/html, application/json;q=0.9",
			wantFormat: "html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, http.NoBody)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}

			got := NegotiateContentType(r)
			assert.Equal(t, tt.wantFormat, got)
		})
	}
}
