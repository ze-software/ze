// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS web page shell + SSE script.
// Related: handler_isis.go -- the handlers that call isisPageHTML
//
// The IS-IS neighbor/database pages are a dependency-light HTML shell: a heading,
// a <pre> showing the current snapshot JSON, and an EventSource that replaces the
// <pre> contents on each SSE push. Keeping the page self-contained (no template
// dependency, no shared layout coupling) matches the read-only nature of the
// view and keeps the IS-IS surface removable with the component. All dynamic
// values are HTML-escaped (template.HTMLEscapeString) so a hostile snapshot or
// path cannot inject markup (security review: no XSS via rendered state).

package web

import "errors"

// errISISDispatchUnavailable is returned when the IS-IS web handlers have no
// CommandDispatcher wired (the engine command path is unavailable).
var errISISDispatchUnavailable = errors.New("isis: command dispatch not available")
