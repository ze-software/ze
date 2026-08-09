// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- shared read-only snapshot page shell.
// Related: page_isis.go, page_ospf.go -- the IS-IS/OSPF view shells that wrap it.
//
// The IS-IS and OSPF neighbor/database pages are the same dependency-light HTML shell,
// differing only in the title, stream path, event name, and the <pre> element id. This is
// the one shared renderer both wrap. Every interpolated value is HTML/JS-escaped so a
// hostile snapshot or path cannot inject markup (no XSS via rendered state).

package web

import (
	"html/template"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// snapshotPageHTML renders the live-view shell: a heading, a <pre id="<dataID>"> showing
// the initial snapshot, and an EventSource that replaces the <pre> contents on each SSE
// push of eventName from streamPath.
func snapshotPageHTML(title, streamPath, eventName, initialJSON, dataID string) string {
	t := template.HTMLEscapeString(title)
	path := template.JSEscapeString(streamPath)
	evt := template.JSEscapeString(eventName)
	initial := template.HTMLEscapeString(initialJSON)
	id := template.HTMLEscapeString(dataID)
	idJS := template.JSEscapeString(dataID)

	var tb textbuf.Buffer
	return tb.
		Str("<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n").
		Str("<title>").Str(t).Str("</title>\n").
		Str("</head><body>\n").
		Str("<h1>").Str(t).Str("</h1>\n").
		Str("<pre id=\"").Str(id).Str("\">").Str(initial).Str("</pre>\n").
		Str("<script>\n").
		Str("(function(){\n").
		Str("  var es = new EventSource(\"").Str(path).Str("\");\n").
		Str("  es.addEventListener(\"").Str(evt).Str("\", function(e){\n").
		Str("    var el = document.getElementById(\"").Str(idJS).Str("\");\n").
		Str("    el.textContent = e.data;\n").
		Str("  });\n").
		Str("  window.addEventListener(\"beforeunload\", function(){ es.close(); });\n").
		Str("})();\n").
		Str("</script>\n").
		Str("</body></html>\n").
		String()
}
