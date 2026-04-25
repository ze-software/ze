package web

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// fakeDispatcher returns a CommandDispatcher that records the command,
// username, and remote address it received and returns the configured
// response/error.
type fakeDispatcher struct {
	command  string
	username string
	remote   string
	response string
	err      error
}

func (f *fakeDispatcher) dispatch() CommandDispatcher {
	return func(command, username, remoteAddr string) (string, error) {
		f.command = command
		f.username = username
		f.remote = remoteAddr
		return f.response, f.err
	}
}

// toolsRequest builds a POST request whose context carries an authenticated
// username, matching what the auth middleware would set on a real session.
// Distinct from cli_test.go's authedRequest so its (method, target, body)
// signature isn't confused with this handler's tighter contract.
//
// exercise alternate routes if registration changes in a follow-up.
//
//nolint:unparam // target kept as a parameter so individual tests can
func toolsRequest(target string, form url.Values, username string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = "10.0.0.5:54321"
	if username != "" {
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUsername, username))
	}
	return r
}

// TestHandleRelatedToolRun_DispatchesResolvedCommand verifies the handler
// resolves the tool's command template against the user's working tree and
// dispatches the substituted command, threading the authenticated username
// and remote address through the dispatcher signature.
//
// VALIDATES: AC-5 (command resolved server-side, dispatched with caller
// identity from trusted request context).
// PREVENTS: A future regression that drops username/remote, which would
// break authz attribution and accounting.
func TestHandleRelatedToolRun_DispatchesResolvedCommand(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{response: "OK"}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
	assert.Equal(t, "alice", disp.username)
	assert.Contains(t, disp.remote, "10.0.0.5")
}

// TestHandleRelatedToolRun_DoesNotTrustCommandFormValue verifies that a
// `command` field smuggled in the form body is ignored. The browser must
// only be able to reference tool ids and context paths; raw command text
// from the wire never reaches the dispatcher.
//
// VALIDATES: AC-8 (browser cannot forge raw commands), Spec D2.
// PREVENTS: Command injection through a forged form field.
func TestHandleRelatedToolRun_DoesNotTrustCommandFormValue(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{response: "OK"}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
		// Smuggled raw command -- handler MUST ignore it.
		"command": {"rm -rf /"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command, "command must come from YANG, not the form")
	assert.NotContains(t, disp.command, "rm -rf")
}

// TestHandleRelatedToolRun_UnknownTool verifies the handler returns a
// client-side error overlay (4xx) for an unknown tool id; no dispatch is
// attempted.
//
// VALIDATES: Defensive routing -- only declared tools dispatch.
// PREVENTS: Operators / attackers triggering unknown tool ids and getting
// confusing error states or a 500.
func TestHandleRelatedToolRun_UnknownTool(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"does-not-exist"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	assert.GreaterOrEqual(t, rec.Code, 400)
	assert.Less(t, rec.Code, 500)
	assert.Empty(t, disp.command, "no dispatch should happen for unknown tool")
}

// TestHandleRelatedToolRun_ConfirmRequired verifies a tool with `confirm`
// metadata returns a confirmation overlay on the first POST and only
// dispatches once the operator submits the confirm flag.
//
// VALIDATES: AC-10 (mutating tools require confirmation), Spec D8.
// PREVENTS: A misclick on a destructive tool tearing down a session
// without an explicit operator confirmation.
func TestHandleRelatedToolRun_ConfirmRequired(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{response: "torn down"}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	// First POST: no confirmation flag -> handler returns confirmation
	// overlay, no dispatch.
	form := url.Values{
		"tool_id":      {"peer-teardown"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, disp.command, "first POST must not dispatch")
	assert.Contains(t, strings.ToLower(rec.Body.String()), "confirm", "response must surface confirmation prompt")

	// Second POST with confirm=true -> dispatch.
	form.Set("confirm", "true")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, toolsRequest("/tools/related/run", form, "alice"))

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "peer 10.0.0.1 teardown", disp.command)
}

// TestHandleRelatedToolRun_StripsANSI verifies that ANSI escape sequences
// in the dispatcher's output are stripped before reaching the overlay.
//
// VALIDATES: AC-28 (ANSI sequences stripped server-side).
// PREVENTS: Terminal control sequences arriving in the browser, which
// could be used for cursor manipulation or visual confusion.
func TestHandleRelatedToolRun_StripsANSI(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{response: "\x1b[31mred text\x1b[0m end"}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	body := rec.Body.String()
	assert.NotContains(t, body, "\x1b", "raw ANSI escape must not reach overlay")
	assert.Contains(t, body, "red text")
	assert.Contains(t, body, "end")
}

// TestHandleRelatedToolRun_TruncatesAtBufferLimit verifies that output
// exceeding the 4 MiB buffer cap is truncated server-side and the overlay
// surfaces the truncation notice.
//
// VALIDATES: AC-27 (truncation at the 4 MiB cap).
// PREVENTS: A pathological command flooding the browser with megabytes of
// output and exhausting the response.
func TestHandleRelatedToolRun_TruncatesAtBufferLimit(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	huge := strings.Repeat("x", 5*1024*1024) // 5 MiB
	disp := &fakeDispatcher{response: huge}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	body := rec.Body.String()
	assert.Less(t, len(body), 5*1024*1024, "response must not contain the entire 5 MiB payload")
	assert.Contains(t, strings.ToLower(body), "truncat", "response must surface the truncation notice")
}

// TestHandleRelatedToolRun_DispatchError verifies that a dispatcher error
// surfaces in the overlay's error state instead of as an HTTP 500.
//
// VALIDATES: AC-7 (failed commands render an error overlay; the page does
// not navigate away).
func TestHandleRelatedToolRun_DispatchError(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{err: errors.New("authorization denied")}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, "alice"))

	assert.NotEqual(t, http.StatusInternalServerError, rec.Code, "errors must not surface as 500")
	body := rec.Body.String()
	assert.Contains(t, body, "authorization denied")
}

// TestHandleRelatedToolRun_RequiresAuthentication verifies that an
// unauthenticated request (no username in context) is rejected.
func TestHandleRelatedToolRun_RequiresAuthentication(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	// No username on the context.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, toolsRequest("/tools/related/run", form, ""))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, disp.command)
}

// csrfHeaders captures the four request properties that drive the
// same-origin gate. Empty fields mean "do not set"; non-empty `host`
// overrides the default `r.Host` set by `toolsRequest`.
type csrfHeaders struct {
	host, origin, referer, xForwardedHost string
}

// runCSRFCase exercises HandleRelatedToolRun with a single CSRF-relevant
// header configuration and returns the response recorder plus the
// dispatcher-recorded command (empty when no dispatch occurred). This
// removes the six-line setup boilerplate that every CSRF-gate test
// otherwise repeats.
func runCSRFCase(t *testing.T, h csrfHeaders) (*httptest.ResponseRecorder, *fakeDispatcher) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	disp := &fakeDispatcher{response: "OK"}
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, disp.dispatch())

	form := url.Values{
		"tool_id":      {"peer-detail"},
		"context_path": {"bgp/peer/thomas"},
	}
	req := toolsRequest("/tools/related/run", form, "alice")
	if h.host != "" {
		req.Host = h.host
	}
	// `toolsRequest` does not set Origin/Referer/X-Forwarded-Host, so
	// only the explicit overrides matter; Del before Set is harmless.
	req.Header.Del("Origin")
	if h.origin != "" {
		req.Header.Set("Origin", h.origin)
	}
	if h.referer != "" {
		req.Header.Set("Referer", h.referer)
	}
	if h.xForwardedHost != "" {
		req.Header.Set("X-Forwarded-Host", h.xForwardedHost)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, disp
}

// TestHandleRelatedToolRun_RejectsCrossOriginPOST verifies that a POST
// whose Origin header does not match the request Host is rejected before
// dispatch. SameSite=Strict on the session cookie is the primary CSRF
// gate; this Origin check is the spec's recommended defense in depth for
// destructive related tools (Spec Security Review Checklist "CSRF
// posture" row).
//
// VALIDATES: A forged cross-origin POST cannot trigger an authenticated
// related-tool dispatch.
// PREVENTS: A misconfigured browser or downgraded SameSite policy
// allowing a cross-site script to run authenticated workbench actions.
func TestHandleRelatedToolRun_RejectsCrossOriginPOST(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:   "ze.example.com",
		origin: "https://attacker.example.com",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, disp.command, "no dispatch on cross-origin POST")
}

// TestHandleRelatedToolRun_AcceptsSameOriginPOST verifies the matching-
// Origin path dispatches normally. Without this case the check above
// could be implemented as "always reject" and pass.
func TestHandleRelatedToolRun_AcceptsSameOriginPOST(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:   "ze.example.com",
		origin: "https://ze.example.com",
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
}

// TestHandleRelatedToolRun_AcceptsXForwardedHost verifies that a request
// reaching Ze through a reverse proxy validates against the public host
// the browser saw (`X-Forwarded-Host`), not just the internal listen
// address.
//
// VALIDATES: Reverse-proxied operators are not locked out by the CSRF gate.
// PREVENTS: A naive Origin == Host check breaking production deployments.
func TestHandleRelatedToolRun_AcceptsXForwardedHost(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:           "internal-router:3443",
		xForwardedHost: "router.example.com",
		origin:         "https://router.example.com",
	})
	assert.Equal(t, http.StatusOK, rec.Code, "X-Forwarded-Host must extend the accepted-host set")
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
}

// TestHandleRelatedToolRun_AcceptsRefererFallback verifies the same-origin
// gate falls back to `Referer` when `Origin` is absent. Some legacy fetch
// flavors and image POSTs do not set Origin; the gate must still accept a
// matching Referer rather than reject the request outright.
func TestHandleRelatedToolRun_AcceptsRefererFallback(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:    "ze.example.com",
		referer: "https://ze.example.com/show/bgp/peer/",
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
}

// TestHandleRelatedToolRun_RejectsCrossRefererFallback verifies the same
// fallback rejects when Referer is foreign and no Origin is present.
func TestHandleRelatedToolRun_RejectsCrossRefererFallback(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:    "ze.example.com",
		referer: "https://attacker.example.com/",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, disp.command)
}

// TestHandleRelatedToolRun_AcceptsXForwardedHostChain verifies the
// chain-of-proxies form `outer-proxy, inner-proxy` is split correctly
// and every entry contributes to the accepted-host set.
//
// VALIDATES: comma-separated `X-Forwarded-Host` chain handling.
// PREVENTS: A regression that swaps `strings.SplitSeq` back to passing
// the raw header value through unchanged, which would only accept the
// concatenated form.
func TestHandleRelatedToolRun_AcceptsXForwardedHostChain(t *testing.T) {
	// Two proxies: outer (router.example.com) is what the browser saw;
	// inner (cdn.example.com) sits between the outer proxy and Ze.
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:           "internal-router:3443",
		xForwardedHost: "router.example.com, cdn.example.com",
		origin:         "https://router.example.com",
	})
	assert.Equal(t, http.StatusOK, rec.Code, "outer host in chain must satisfy the same-origin gate")
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
}

// TestHandleRelatedToolRun_RejectsXForwardedHostMismatch verifies that
// having `X-Forwarded-Host` set does NOT relax the gate. An attacker who
// can reach the inner Ze host directly can spoof X-Forwarded-Host, but
// their Origin still must match the spoofed value.
func TestHandleRelatedToolRun_RejectsXForwardedHostMismatch(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:           "internal-router:3443",
		xForwardedHost: "router.example.com",
		// Origin advertises a third party; neither r.Host nor the proxy
		// host matches.
		origin: "https://attacker.example.com",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, disp.command)
}

// TestHandleRelatedToolRun_RejectsMalformedOrigin verifies that an
// `Origin` header that does not parse to a usable URL is rejected.
// `null` is what sandboxed browsers send for opaque origins; it parses
// but yields an empty Host, which the handler rejects.
func TestHandleRelatedToolRun_RejectsMalformedOrigin(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:   "ze.example.com",
		origin: "null",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, disp.command)
}

// TestHandleRelatedToolRun_ResponseBodyDoesNotLeakInternalHost verifies
// the rejection response body is generic and does NOT include the
// internal listen address or the accepted-host set. Detail goes to the
// server log; clients only see `forbidden`.
//
// VALIDATES: Information-disclosure mitigation -- error-body has no
// internal host names.
func TestHandleRelatedToolRun_ResponseBodyDoesNotLeakInternalHost(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host:   "internal-router:3443",
		origin: "https://attacker.example.com",
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "internal-router", "internal listen address must not leak")
	assert.NotContains(t, body, "3443", "internal port must not leak")
	assert.NotContains(t, body, "accepted", "accepted-host detail belongs in the server log")
	// Sanity: the dispatcher must not have run.
	assert.Empty(t, disp.command)
}

// TestHandleRelatedToolRun_AcceptsNoOriginNoReferer verifies the spec
// contract that requests with neither `Origin` nor `Referer` are
// accepted (legacy clients, `curl`, scripts). The dispatcher's authz
// gate is the primary access control; CSRF is irrelevant when the
// request was not initiated by a browser session at all.
//
// VALIDATES: Same-origin gate's "no headers = accept" contract.
// PREVENTS: A regression that defaulted to reject and silently broke
// every JSON / curl client.
func TestHandleRelatedToolRun_AcceptsNoOriginNoReferer(t *testing.T) {
	rec, disp := runCSRFCase(t, csrfHeaders{
		host: "ze.example.com",
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "peer 10.0.0.1 detail", disp.command)
}

// testRenderer returns a renderer for handler tests.
func testRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := NewRenderer()
	require.NoError(t, err)
	return r
}

// TestToolOverlay_RenderSuccess verifies the success-state overlay shows
// the resolved command, the inline output, the close button, and a
// stable DOM id derived from tool id + context path.
//
// VALIDATES: Spec TDD `TestToolOverlay_RenderSuccess` row.
// PREVENTS: A render regression that drops the close affordance or the
// command transparency line.
func TestToolOverlay_RenderSuccess(t *testing.T) {
	r := testRenderer(t)
	data := ToolOverlayData{
		ID:           "overlay-peer-detail-bgp-peer-thomas",
		State:        ToolOverlayResult,
		Title:        "Peer Detail",
		Command:      "peer 10.0.0.1 detail",
		ToolID:       "peer-detail",
		ContextPath:  "bgp/peer/thomas",
		OutputInline: "session established",
	}
	html := string(r.RenderFragment("tool_overlay", data))

	assert.Contains(t, html, `id="overlay-peer-detail-bgp-peer-thomas"`)
	assert.Contains(t, html, "Peer Detail")
	assert.Contains(t, html, "peer 10.0.0.1 detail")
	assert.Contains(t, html, "session established")
	assert.Contains(t, html, "tool-overlay-close")
	assert.Contains(t, html, "Rerun")
}

// TestToolOverlay_RenderError verifies the error-state overlay carries the
// error styling class and surfaces the error message.
//
// VALIDATES: Spec TDD `TestToolOverlay_RenderError` row.
// PREVENTS: An error overlay that visually looks like a successful one.
func TestToolOverlay_RenderError(t *testing.T) {
	r := testRenderer(t)
	data := ToolOverlayData{
		ID:           "overlay-peer-detail-bgp-peer-x",
		State:        ToolOverlayError,
		Title:        "Peer Detail",
		ErrorMessage: "authorization denied",
	}
	html := string(r.RenderFragment("tool_overlay", data))

	assert.Contains(t, html, "tool-overlay--error")
	assert.Contains(t, html, "authorization denied")
}

// TestToolOverlay_ShowFullOutput verifies that output between 128 KiB and
// the 4 MiB cap is split into an inline preview and a "Show full output"
// disclosure that holds the remainder. The overlay must NOT re-dispatch
// the command -- the full bytes ride in the same response.
//
// VALIDATES: AC-26 (`Show full output` expands without re-dispatch).
// PREVENTS: A regression that drops the overflow tail or replaces the
// disclosure with a re-fetch button (Spec Failure Routing row).
func TestToolOverlay_ShowFullOutput(t *testing.T) {
	r := testRenderer(t)
	inline := strings.Repeat("a", relatedOverlayInlineBytes)
	overflow := strings.Repeat("b", 1024)
	data := ToolOverlayData{
		ID:             "overlay-x",
		State:          ToolOverlayResult,
		Title:          "X",
		ToolID:         "x",
		ContextPath:    "p",
		OutputInline:   template.HTML(inline),
		OutputOverflow: template.HTML(overflow),
		HasOverflow:    true,
	}
	html := string(r.RenderFragment("tool_overlay", data))

	assert.Contains(t, html, "Show full output")
	assert.Contains(t, html, "<details")
	assert.Contains(t, html, "tool-overlay-output-overflow")
	// The expansion target must be inline -- not a fresh hx-post.
	assert.NotContains(t, html, "hx-get=", "show-full-output must not refetch")
}

// TestToolOverlay_TruncationNotice verifies the overlay surfaces a clear
// truncation notice when the dispatcher's output exceeded the 4 MiB cap.
//
// VALIDATES: AC-27 (truncation notice).
func TestToolOverlay_TruncationNotice(t *testing.T) {
	r := testRenderer(t)
	data := ToolOverlayData{
		ID:           "overlay-x",
		State:        ToolOverlayResult,
		Title:        "X",
		ToolID:       "x",
		ContextPath:  "p",
		OutputInline: "first chunk",
		HasOverflow:  true,
		Truncated:    true,
	}
	html := string(r.RenderFragment("tool_overlay", data))

	assert.Contains(t, strings.ToLower(html), "truncat")
}

// TestToolOverlay_MultipleInstancesUniqueIDs verifies that two overlays
// rendered for different (tool_id, context_path) combinations end up with
// distinct DOM ids. Pinning multiple overlays simultaneously requires
// uniqueness so close handlers and OOB swaps target only one instance.
//
// VALIDATES: AC-25 (multi-overlay DOM uniqueness).
// PREVENTS: Closing one overlay accidentally removing another.
func TestToolOverlay_MultipleInstancesUniqueIDs(t *testing.T) {
	a := overlayID("peer-detail", []string{"bgp", "peer", "thomas"})
	b := overlayID("peer-detail", []string{"bgp", "peer", "alex"})
	c := overlayID("peer-statistics", []string{"bgp", "peer", "thomas"})

	assert.NotEqual(t, a, b, "different rows must produce different overlay ids")
	assert.NotEqual(t, a, c, "different tools on the same row must produce different overlay ids")
	assert.True(t, strings.HasPrefix(a, "overlay-"), "overlay id must use the overlay- prefix")
	for _, id := range []string{a, b, c} {
		// IDs must be safe to drop into HTML id attributes (alphanumerics,
		// dash, underscore only).
		for _, ch := range id {
			assert.True(t, isOverlayIDChar(ch), "overlay id contains unsafe char %q", ch)
		}
	}
}

func isOverlayIDChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	return r == '-' || r == '_'
}
