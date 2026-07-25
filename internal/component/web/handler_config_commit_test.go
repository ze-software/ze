package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/audit"
)

// hostileRedirectHeaders are request-header values an attacker would supply to
// turn a "navigate back one level" redirect into an off-origin one. Every entry
// must resolve to a same-origin path.
var hostileRedirectHeaders = []struct {
	name   string
	header string
}{
	{"protocol relative", "//evil.example/a/b"},
	{"protocol relative trailing slash", "//evil.example/a/b/"},
	{"backslash escape", "/\\evil.example/a/b"},
	{"triple slash", "///evil.example/a/b"},
	{"scheme with no path", "https://evil.example"},
	{"scheme with port and no path", "https://evil.example:8443"},
	{"empty", ""},
	{"bare slash", "/"},
	{"single segment", "/config"},
}

// VALIDATES: AC-10 -- parentFromCurrentURL never returns an off-origin redirect target.
// PREVENTS: An attacker-supplied HX-Current-URL or Referer header steering the
// post-discard redirect to another origin (a "//evil.example/a/b" header value
// used to survive every branch and come back out as "//evil.example/a/").
func TestParentFromCurrentURLRejectsProtocolRelative(t *testing.T) {
	for _, tc := range hostileRedirectHeaders {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/config/discard/", http.NoBody)
			req.Header.Set("HX-Current-URL", tc.header)

			assertSameOriginPath(t, parentFromCurrentURL(req))
		})
	}
}

// VALIDATES: AC-10 -- parentFromCurrentURL sanitizes the Referer fallback too.
// PREVENTS: Closing the HX-Current-URL hole while leaving the Referer path open.
func TestParentFromCurrentURLSanitizesReferer(t *testing.T) {
	for _, tc := range hostileRedirectHeaders {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/config/discard/", http.NoBody)
			req.Header.Set("Referer", tc.header)

			assertSameOriginPath(t, parentFromCurrentURL(req))
		})
	}
}

// VALIDATES: AC-10 -- a legitimate HX-Current-URL still navigates back one level.
// PREVENTS: The sanitizer flattening every redirect to "/" and breaking navigation.
func TestParentFromCurrentURLKeepsLegitimateParent(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"full url nested path", "https://ze.example/config/edit/bgp/peer/", "/config/edit/bgp/"},
		{"path only", "/config/edit/bgp/peer/", "/config/edit/bgp/"},
		{"no trailing slash", "/config/edit/bgp/peer", "/config/edit/bgp/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/config/discard/", http.NoBody)
			req.Header.Set("HX-Current-URL", tc.header)

			assert.Equal(t, tc.want, parentFromCurrentURL(req))
		})
	}
}

// VALIDATES: AC-10 -- POST /config/discard/ with a hostile HX-Current-URL redirects
// same-origin, driven from the handler entry point rather than the helper.
// PREVENTS: A future caller reaching htmxRedirect with an unsanitized target
// (ai/rules/fail-closed-guards.md: test the guard from its entry point).
func TestConfigDiscardRedirectIsSameOrigin(t *testing.T) {
	for _, tc := range hostileRedirectHeaders {
		t.Run(tc.name, func(t *testing.T) {
			handler, mgr := discardHandlerWithPendingChange(t)

			req := postConfigRequest(t, "/config/discard/", url.Values{}, "alice")
			req.Header.Set("HX-Current-URL", tc.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, 0, mgr.ChangeCount("alice"), "discard must still take effect")
			assertSameOriginPath(t, rec.Header().Get("Location"))
		})
	}
}

// VALIDATES: AC-10 -- the HTMX branch (HX-Redirect header) is sanitized as well.
// PREVENTS: Fixing only the http.Redirect branch and leaving HX-Redirect open;
// htmx performs a client-side navigation from that header, so it redirects too.
func TestConfigDiscardHXRedirectIsSameOrigin(t *testing.T) {
	for _, tc := range hostileRedirectHeaders {
		t.Run(tc.name, func(t *testing.T) {
			handler, _ := discardHandlerWithPendingChange(t)

			req := postConfigRequest(t, "/config/discard/", url.Values{}, "alice")
			req.Header.Set("HX-Request", htmxRequestTrue)
			req.Header.Set("HX-Current-URL", tc.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assertSameOriginPath(t, rec.Header().Get("HX-Redirect"))
		})
	}
}

// discardHandlerWithPendingChange builds a discard handler over a manager that
// already holds one pending change for "alice", so the handler reaches its
// redirect instead of erroring out early.
func discardHandlerWithPendingChange(t *testing.T) (http.Handler, *EditorManager) {
	t.Helper()

	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "9.9.9.9"))
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	return HandleConfigDiscardWithAuthorizerAndAudit(mgr, adminWebAuthorizer(), recorder), mgr
}

// assertSameOriginPath fails unless target is a path a browser resolves against
// the current origin: it must start with a single "/" and must not be
// scheme-relative ("//host"), backslash-escaped ("/\host"), or absolute.
func assertSameOriginPath(t *testing.T, target string) {
	t.Helper()

	require.NotEmpty(t, target, "redirect target must never be empty")
	assert.True(t, strings.HasPrefix(target, "/"), "target %q must be a rooted path", target)
	assert.False(t, strings.HasPrefix(target, "//"), "target %q is scheme-relative (off-origin)", target)
	assert.False(t, strings.HasPrefix(target, "/\\"), "target %q is backslash-escaped (off-origin)", target)
	assert.NotContains(t, target, "://", "target %q carries a scheme (off-origin)", target)
}

// VALIDATES: backToRefererOrShow, the other request-derived redirect producer,
// obeys the same same-origin guard.
// PREVENTS: The guard's own doc claiming "every redirect target ... MUST pass
// this" while a sibling producer one file over kept an inline check that
// rejected "//host" but not "/\host", which several browsers normalize to
// "//host".
func TestBackToRefererOrShowIsSameOrigin(t *testing.T) {
	for _, tc := range hostileRedirectHeaders {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/config/set/", http.NoBody)
			req.Header.Set("Referer", tc.header)

			assertSameOriginPath(t, backToRefererOrShow(req))
		})
	}
}

// VALIDATES: a legitimate Referer still returns its path.
// PREVENTS: The shared guard flattening every post-form redirect to the fallback.
func TestBackToRefererOrShowKeepsLegitimatePath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/config/set/", http.NoBody)
	req.Header.Set("Referer", "https://ze.example/show/bgp/peer/")

	if got := backToRefererOrShow(req); got != "/show/bgp/peer/" {
		t.Errorf("backToRefererOrShow = %q, want %q", got, "/show/bgp/peer/")
	}
}
