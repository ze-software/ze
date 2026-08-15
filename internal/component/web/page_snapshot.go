// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- shared read-only snapshot page shell.
// Related: page_isis.go, page_ospf.go -- the IS-IS/OSPF view shells that wrap it.
// Related: page_snapshot.templ -- the markup this fills
// Related: assets/snapshot-live.js -- the script that reads the data attributes below
//
// The IS-IS and OSPF neighbor/database pages are the same dependency-light HTML shell,
// differing only in the title, stream path, event name, and the pre element id. This is
// the one shared renderer both wrap.

package web

import (
	"html/template"
)

// snapshotPageData is what snapshotPage renders: a read-only view of one
// command's JSON, refreshed by the SSE stream at StreamPath.
//
// StreamPath and EventName reach the browser as data attributes rather than as
// JavaScript. assets/snapshot-live.js reads them, so the page carries no inline
// script and needs none: every response owes script-src 'self'
// (setSecurityHeaders, auth.go), and a browser refuses an inline script under
// that policy. Neither value is JavaScript-escaped for the same reason. Each is
// an HTML attribute now, and templ escapes it once.
type snapshotPageData struct {
	Title      string
	StreamPath string
	EventName  string
	DataID     string
	Initial    string
}

// snapshotPageHTML renders the live-view shell: a heading, a pre element
// showing the initial snapshot, and the data attributes the live-view script
// subscribes with.
func snapshotPageHTML(renderer *Renderer, title, streamPath, eventName, initialJSON, dataID string) template.HTML {
	return renderer.renderComponent("snapshot_page", snapshotPage(snapshotPageData{
		Title:      title,
		StreamPath: streamPath,
		EventName:  eventName,
		DataID:     dataID,
		Initial:    initialJSON,
	}))
}
