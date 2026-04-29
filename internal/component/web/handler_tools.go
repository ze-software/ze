// Design: docs/architecture/web-interface.md -- V2 workbench related-tool execution
// Related: related_resolver.go -- placeholder substitution against the working tree
// Related: handler_admin.go -- CommandDispatcher type and existing operational dispatch
// Related: ../config/related.go -- RelatedTool descriptor and parser
//
// Spec: plan/spec-web-2-operator-workbench.md (Phase 3 -- secure run endpoint).
//
// The handler accepts only a tool id and a context path from the browser.
// It resolves the descriptor server-side, walks the user's working tree to
// substitute placeholders, validates the resolved value, and dispatches
// through the standard CommandDispatcher (same authz path as CLI/admin).
// Output is ANSI-stripped, capped at 4 MiB, and HTML-escaped before
// rendering as an overlay fragment.

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// Output buffering and rendering caps. The spec's Boundary Tests row defines
// these explicitly; centralized here so tests can reference the same values.
const (
	relatedOverlayInlineBytes = 128 * 1024      // first 128 KiB rendered inline
	relatedOverlayMaxBufBytes = 4 * 1024 * 1024 // 4 MiB total before truncation
)

// ToolOverlayState is the rendered overlay variant.
type ToolOverlayState int

const (
	// ToolOverlayResult means the command ran and the output is ready.
	ToolOverlayResult ToolOverlayState = iota
	// ToolOverlayError means the command ran but the dispatcher returned an
	// error, or earlier validation failed.
	ToolOverlayError
	// ToolOverlayConfirm means the descriptor required confirmation and
	// the operator hasn't confirmed yet.
	ToolOverlayConfirm
)

// ToolOverlayData is the template payload for one overlay instance. Each
// overlay renders into its own DOM node identified by ID so multiple
// overlays can coexist (Spec Tool Overlay region row, AC-25).
type ToolOverlayData struct {
	ID             string
	State          ToolOverlayState
	Title          string
	Command        string // Resolved command shown for transparency.
	ToolID         string
	ContextPath    string
	OutputInline   template.HTML // First 128 KiB, escaped.
	OutputOverflow template.HTML // Bytes beyond 128 KiB, escaped (rendered inside <details>).
	HasOverflow    bool
	Truncated      bool
	ErrorMessage   string
	ConfirmPrompt  string
}

// HandleRelatedToolRun returns the POST handler for /tools/related/run.
// Callers wrap it in the same authentication middleware as the rest of the
// workbench routes; the handler reads the username from the request
// context (set by AuthMiddleware) and rejects requests without one.
//
// `tree` is the committed working tree used as a fallback. `mgr` provides
// the per-user editor session tree when available; pass nil in tests that
// inject a tree directly.
func HandleRelatedToolRun(renderer *Renderer, schema *config.Schema, tree *config.Tree, mgr *EditorManager, dispatch CommandDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// CSRF defense in depth: SameSite=Strict on the session cookie is the
		// primary gate, but destructive related tools (Spec D8) merit a
		// belt-and-braces Origin/Referer check. Browsers send Origin on
		// cross-origin POSTs; if it does not match the request Host the
		// request is rejected before any dispatch. Same-origin POSTs from
		// the legitimate UI either omit Origin (legacy) or send a matching
		// value; both are accepted. See spec Security Review Checklist
		// "CSRF posture" row. Detail (internal host names, accepted set)
		// is logged server-side; the client only sees `forbidden`.
		if err := checkSameOrigin(r); err != nil {
			serverLogger.Warn("workbench tool POST rejected", "user", username, "remote", r.RemoteAddr, "error", err)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		toolID := strings.TrimSpace(r.PostFormValue("tool_id"))
		if toolID == "" {
			renderToolError(w, renderer, "missing tool_id", http.StatusBadRequest)
			return
		}

		rawCtxPath := strings.Trim(r.PostFormValue("context_path"), "/")
		var contextPath []string
		if rawCtxPath != "" {
			contextPath = strings.Split(rawCtxPath, "/")
		}
		if err := ValidatePathSegments(contextPath); err != nil {
			renderToolError(w, renderer, "invalid context path: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Walk the schema to the context node and find the matching descriptor.
		tool, err := lookupRelatedTool(schema, contextPath, toolID)
		if err != nil {
			renderToolError(w, renderer, err.Error(), http.StatusNotFound)
			return
		}

		// Confirm-required tools return a confirmation overlay on the first
		// POST and only dispatch once the operator submits confirm=true.
		confirmed := strings.EqualFold(strings.TrimSpace(r.PostFormValue("confirm")), "true")
		if tool.Confirm != "" && !confirmed {
			data := ToolOverlayData{
				ID:            overlayID(toolID, contextPath),
				State:         ToolOverlayConfirm,
				Title:         tool.Label,
				ToolID:        toolID,
				ContextPath:   strings.Join(contextPath, "/"),
				ConfirmPrompt: tool.Confirm,
			}
			renderToolOverlay(w, renderer, data, http.StatusOK)
			return
		}

		// Resolve placeholders against the user's working tree.
		viewTree := tree
		if mgr != nil {
			if userTree := mgr.Tree(username); userTree != nil {
				viewTree = userTree
			}
		}
		resolver := NewRelatedResolver(schema, viewTree)
		res, err := resolver.Resolve(tool, contextPath)
		if err != nil {
			data := errorOverlay(toolID, contextPath, tool, err.Error())
			renderToolOverlay(w, renderer, data, http.StatusOK)
			return
		}
		if res.Disabled {
			data := errorOverlay(toolID, contextPath, tool, "tool disabled: "+res.DisabledReason)
			renderToolOverlay(w, renderer, data, http.StatusOK)
			return
		}

		// Dispatch through the standard pipeline. Authz, accounting, and
		// peer-selector extraction live there; this handler only constructs
		// the trusted command and identity.
		output, dispatchErr := dispatch(res.Command, username, r.RemoteAddr)
		if dispatchErr != nil {
			data := errorOverlay(toolID, contextPath, tool, dispatchErr.Error())
			data.Command = res.Command
			renderToolOverlay(w, renderer, data, http.StatusOK)
			return
		}

		// Strip ANSI, cap at 4 MiB, split inline / overflow.
		cleaned, truncated := normalizeOutput(output)
		data := ToolOverlayData{
			ID:          overlayID(toolID, contextPath),
			State:       ToolOverlayResult,
			Title:       tool.Label,
			Command:     res.Command,
			ToolID:      toolID,
			ContextPath: strings.Join(contextPath, "/"),
			Truncated:   truncated,
		}
		assignOverlayBody(&data, cleaned)
		renderToolOverlay(w, renderer, data, http.StatusOK)
	}
}

// lookupRelatedTool walks the schema to the context node and returns the
// declared descriptor with the requested tool id.
func lookupRelatedTool(schema *config.Schema, contextPath []string, toolID string) (*config.RelatedTool, error) {
	node, err := walkSchema(schema, contextPath)
	if err != nil {
		return nil, fmt.Errorf("invalid context path: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("no schema node at %q", strings.Join(contextPath, "/"))
	}
	var pool []*config.RelatedTool
	switch n := node.(type) {
	case *config.ContainerNode:
		pool = n.Related
	case *config.ListNode:
		pool = n.Related
	case *config.LeafNode:
		pool = n.Related
	}
	for _, t := range pool {
		if t.ID == toolID {
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown tool id %q at %s", toolID, strings.Join(contextPath, "/"))
}

// errorOverlay builds a populated ToolOverlayData for the error state.
func errorOverlay(toolID string, contextPath []string, tool *config.RelatedTool, msg string) ToolOverlayData {
	title := toolID
	if tool != nil {
		title = tool.Label
	}
	return ToolOverlayData{
		ID:           overlayID(toolID, contextPath),
		State:        ToolOverlayError,
		Title:        title,
		ToolID:       toolID,
		ContextPath:  strings.Join(contextPath, "/"),
		ErrorMessage: msg,
	}
}

// renderToolOverlay renders the overlay template to the response. Status
// is 200 for both success and tool-level error states (the overlay carries
// its own visual error treatment); 4xx/5xx are reserved for protocol
// errors that prevent overlay rendering at all.
func renderToolOverlay(w http.ResponseWriter, renderer *Renderer, data ToolOverlayData, status int) {
	html := renderer.RenderFragment("tool_overlay", data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(html)); err != nil {
		return
	}
}

// RequireSameOrigin rejects authenticated mutating requests whose Origin or
// Referer does not match the request host. It is intended to sit inside
// AuthMiddleware so GetUsernameFromRequest is already populated; requests that
// have not reached authentication yet are left to the next handler.
func RequireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if isMutatingMethod(r.Method) && username != "" {
			if err := checkSameOrigin(r); err != nil {
				serverLogger.Warn("web mutation rejected", "user", username, "remote", r.RemoteAddr, "error", err)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// renderToolError writes a minimal error overlay for protocol-level
// failures (missing tool_id, invalid path, unknown tool) where the schema
// lookup itself failed and we have no descriptor to title the overlay.
func renderToolError(w http.ResponseWriter, renderer *Renderer, msg string, status int) {
	data := ToolOverlayData{
		ID:           "overlay-error",
		State:        ToolOverlayError,
		Title:        "Tool error",
		ErrorMessage: msg,
	}
	renderToolOverlay(w, renderer, data, status)
}

// overlayID builds a stable, DOM-safe id for one overlay instance. The id
// embeds the tool id and the row's context-path segments so multiple
// overlays for the same tool but different rows do not collide. Output is
// restricted to `[a-zA-Z0-9_-]` -- enough to disambiguate the inputs the
// resolver actually accepts (YANG identifier characters), and narrow
// enough that the test contract (`isOverlayIDChar`) matches the
// implementation 1:1.
func overlayID(toolID string, contextPath []string) string {
	parts := make([]string, 0, 2+len(contextPath))
	parts = append(parts, "overlay", asciiSafeID(toolID))
	for _, seg := range contextPath {
		parts = append(parts, asciiSafeID(seg))
	}
	return strings.Join(parts, "-")
}

// asciiSafeID returns s with each non-ASCII-alphanumeric, non-`-`, non-`_`
// rune replaced by `-`. Acts as the single point of truth for DOM-id
// safety; tests assert against the same character class.
//
// Collision safety: distinct inputs can collapse to the same id when one
// uses `/` (replaced with `-`) and the other has a literal `-` at the
// same position. Tool ids are restricted to `[a-zA-Z0-9_-]` by the
// `ze:related` parser and context-path segments are restricted to YANG
// identifier characters by `ValidatePathSegments`, so `/` never appears
// inside a segment and the only `-` survivors come from inputs that
// already contained a `-`. Distinct overlay-id inputs therefore produce
// distinct overlay ids in practice; if a future input domain loosens
// the character set, revisit this contract.
func asciiSafeID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// assignOverlayBody splits cleaned output into the inline preview and the
// overflow that lives behind a `Show full output` affordance. Both halves
// are HTML-escaped here so the template can mark them safe. The split is
// rounded back to the previous valid UTF-8 rune boundary so a multi-byte
// rune that straddles the inline cap is rendered fully in the overflow,
// not partially in both halves.
func assignOverlayBody(data *ToolOverlayData, cleaned string) {
	if len(cleaned) <= relatedOverlayInlineBytes {
		data.OutputInline = template.HTML(template.HTMLEscapeString(cleaned)) //nolint:gosec // already escaped
		return
	}
	cut := utf8RuneBoundary(cleaned, relatedOverlayInlineBytes)
	inline := cleaned[:cut]
	overflow := cleaned[cut:]
	data.OutputInline = template.HTML(template.HTMLEscapeString(inline))     //nolint:gosec // already escaped
	data.OutputOverflow = template.HTML(template.HTMLEscapeString(overflow)) //nolint:gosec // already escaped
	data.HasOverflow = true
}

// normalizeOutput strips ANSI control sequences and other terminal control
// characters and truncates the buffer to relatedOverlayMaxBufBytes. Returns
// the cleaned string and a flag indicating whether truncation occurred.
// Truncation is rounded back to the previous UTF-8 rune boundary so the
// caller never observes a partial multi-byte sequence at the buffer edge.
func normalizeOutput(s string) (string, bool) {
	truncated := false
	if len(s) > relatedOverlayMaxBufBytes {
		s = s[:utf8RuneBoundary(s, relatedOverlayMaxBufBytes)]
		truncated = true
	}
	return ansiStrip(stripC0Controls(s)), truncated
}

// utf8RuneBoundary returns the largest index <= max at which s can be split
// without breaking a multi-byte UTF-8 sequence. When max already lands on a
// rune start (or one byte past the end), it is returned unchanged. Walks at
// most utf8.UTFMax bytes back, so cost is O(1).
//
// Fallback: if the bytes immediately before max are all UTF-8 continuation
// bytes (no rune start within utf8.UTFMax of max), the input was already
// malformed UTF-8. We return max unchanged so the byte-cap invariant
// holds; the resulting slice still ends with invalid bytes, but the
// caller renders through `template.HTMLEscapeString` and the browser
// shows a replacement glyph -- safer than scanning the entire buffer
// looking for a rune start that may never exist.
func utf8RuneBoundary(s string, max int) int {
	if max >= len(s) {
		return len(s)
	}
	for i := max; i > 0 && i > max-utf8.UTFMax; i-- {
		if utf8.RuneStart(s[i]) {
			return i
		}
	}
	return max
}

// stripC0Controls drops C0 control bytes (0x00-0x1F and 0x7F) from the
// string, except for tab (0x09), newline (0x0A), and carriage return (0x0D)
// which are routinely present in command output and render fine. ESC bytes
// (0x1B) are kept here -- the ANSI strip below handles full ESC sequences
// and removes any stray ESCs after.
func stripC0Controls(s string) string {
	if !needsC0Strip(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '\t' || c == '\n' || c == '\r' || c == 0x1b:
			b.WriteByte(c)
		case c < 0x20 || c == 0x7f:
			// drop
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// needsC0Strip returns true when at least one byte in s is a C0 control
// character that stripC0Controls would remove. Lets the fast path (no
// strip needed) skip the rebuild allocation.
func needsC0Strip(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c == '\t' || c == '\n' || c == '\r' || c == 0x1b {
			continue
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// ansiCSI matches Control Sequence Introducer (CSI) and Operating System
// Command (OSC) sequences along with simple two-char ESC sequences. The
// regex deliberately does not normalise printable text, only control
// patterns; legitimate output passes through unchanged.
// CSI: \x1b [ <params> <intermediates> <final>. Final is one byte in 0x40-0x7E.
// OSC: \x1b ] ... ST (ST is BEL or ESC \).
// Two-char ESC: \x1b followed by one byte in 0x40-0x7E.
//
//nolint:gocritic // explicit byte ranges are spec-driven, not human-readable typos
var ansiCSI = regexp.MustCompile(`\x1b(\[[0-9;?]*[!-/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-~])`)

// checkSameOrigin validates that an authenticated POST request came from
// the same origin as the server. Browsers attach an `Origin` header to
// cross-origin POSTs; if present and not matching any of the server's
// accepted host names, the request is treated as a CSRF attempt and
// rejected. Falls back to `Referer` when `Origin` is absent.
//
// Accepted hosts: `r.Host` plus every value of `X-Forwarded-Host` (a
// reverse proxy sets this to the public host the browser saw, so the
// browser's Origin matches it but not the internal listen address).
//
// Threat model: this function only runs after `GetUsernameFromRequest`
// has confirmed an authenticated session, and SameSite=Strict on the
// session cookie prevents cross-site POSTs from carrying the operator's
// cookie at all. Trusting `X-Forwarded-Host` is therefore safe: an
// unauthenticated cross-site request never reaches this check, and a
// privileged direct caller can already act on the authenticated
// session. The Origin/Referer match is defense in depth against the
// narrow case where a future browser softens SameSite=Strict.
//
// Requests with neither Origin nor Referer are accepted -- legacy
// clients and `curl` users do not set them, and the dispatcher's authz
// layer is the primary access gate.
func checkSameOrigin(r *http.Request) error {
	// HTTP/1.1 requires Host on every request; net/http populates r.Host
	// from the request line or the Host header. We never see an empty
	// r.Host in practice, so no defensive bypass here.
	const maxAcceptedHosts = 16
	accepted := []string{r.Host}
forwardedLoop:
	for _, fh := range r.Header.Values("X-Forwarded-Host") {
		// Header may carry a single host or a comma-separated chain
		// "outer-proxy, inner-proxy"; accept every entry up to the cap.
		// The cap keeps the slice bounded so a malicious or buggy proxy
		// pumping thousands of hosts cannot grow the per-request
		// allocation without limit.
		for h := range strings.SplitSeq(fh, ",") {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			if len(accepted) >= maxAcceptedHosts {
				break forwardedLoop
			}
			accepted = append(accepted, h)
		}
	}

	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return fmt.Errorf("malformed Origin header")
		}
		if !hostMatchesAny(u.Host, accepted) {
			return fmt.Errorf("origin %q does not match host %q (accepted %v)", u.Host, r.Host, accepted)
		}
		return nil
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" {
			return fmt.Errorf("malformed Referer header")
		}
		if !hostMatchesAny(u.Host, accepted) {
			return fmt.Errorf("referer %q does not match host %q (accepted %v)", u.Host, r.Host, accepted)
		}
	}
	return nil
}

// hostMatchesAny reports whether `got` matches any of the accepted hosts
// using case-insensitive comparison (RFC 3986: scheme + host are
// case-insensitive).
func hostMatchesAny(got string, accepted []string) bool {
	for _, a := range accepted {
		if strings.EqualFold(a, got) {
			return true
		}
	}
	return false
}

// ansiStrip removes terminal control sequences. Bare ESC characters with
// no recognized continuation are dropped to avoid leaving stray 0x1b bytes
// in the rendered text.
func ansiStrip(s string) string {
	cleaned := ansiCSI.ReplaceAllString(s, "")
	cleaned = strings.ReplaceAll(cleaned, "\x1b", "")
	return cleaned
}
