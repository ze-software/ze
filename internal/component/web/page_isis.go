// Design: plan/spec-isis-13-cli-diag-interop.md -- IS-IS web page shell + SSE script.
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

import (
	"errors"
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// errISISDispatchUnavailable is returned when the IS-IS web handlers have no
// CommandDispatcher wired (the engine command path is unavailable).
var errISISDispatchUnavailable = errors.New("isis: command dispatch not available")

// isisPageHTML renders the IS-IS view shell. title is the page heading,
// streamPath the SSE endpoint, eventName the SSE event the script listens for,
// and initialJSON the snapshot embedded so the page renders before the first
// push. Every interpolated value is HTML-escaped.
func isisPageHTML(title, streamPath, eventName, initialJSON string) string {
	t := template.HTMLEscapeString(title)
	path := template.JSEscapeString(streamPath)
	evt := template.JSEscapeString(eventName)
	initial := template.HTMLEscapeString(initialJSON)

	var tb textbuf.Buffer
	return tb.
		Str("<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n").
		Str("<title>").Str(t).Str("</title>\n").
		Str("</head><body>\n").
		Str("<h1>").Str(t).Str("</h1>\n").
		Str("<pre id=\"isis-data\">").Str(initial).Str("</pre>\n").
		Str("<script>\n").
		Str("(function(){\n").
		Str("  var es = new EventSource(\"").Str(path).Str("\");\n").
		Str("  es.addEventListener(\"").Str(evt).Str("\", function(e){\n").
		Str("    var el = document.getElementById(\"isis-data\");\n").
		Str("    el.textContent = e.data;\n").
		Str("  });\n").
		Str("  window.addEventListener(\"beforeunload\", function(){ es.close(); });\n").
		Str("})();\n").
		Str("</script>\n").
		Str("</body></html>\n").
		String()
}
