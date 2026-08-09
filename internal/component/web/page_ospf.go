// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- OSPF web page shell + SSE script.
// Related: handler_ospf.go -- the handlers that call ospfPageHTML
//
// The OSPF neighbor/database pages are a dependency-light HTML shell: a heading, a <pre>
// showing the current snapshot JSON, and an EventSource that replaces the <pre> contents
// on each SSE push. All dynamic values are HTML/JS-escaped so a hostile snapshot or path
// cannot inject markup (no XSS via rendered state).

package web

import "errors"

// errOSPFDispatchUnavailable is returned when the OSPF web handlers have no
// CommandDispatcher wired (the engine command path is unavailable).
var errOSPFDispatchUnavailable = errors.New("ospf: command dispatch not available")
