// Related: golden_test.go -- the TEMPLATE capture whose fixtures this compares
// Related: handler_golden_test.go -- the HANDLER capture whose fixtures this compares

package lg

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// lgPortHandlers explains each response whose content changed on purpose. Every
// other difference is a finding.
//
// The three below are one defect the port replaced the producer of.
// renderSearchError set Error and rendered the search page, which held the
// error banner inside a branch that only ran when there were routes. A bad
// filter therefore answered 200 with an empty results panel. searchPage
// (search.templ) renders that panel when there are routes OR an error, so the
// banner reaches the operator. Recorded in plan/journal/silent-fall-through.md.
const lgPortSearchBanner = "the search error banner reached no rendered byte before the port"

// The two below are one live outage the port left standing, and phase 5b
// closed. Each graph-mode button carried an onclick that removed .active from
// its siblings and set it on itself. setSecurityHeaders (server.go) answers
// `default-src 'self'` with no script-src beside it, so default-src is what a
// browser applies to script and it refuses an inline handler. The pressed
// button therefore never looked pressed, in any browser.
//
// assets/graph-mode.js carries the class change now. Its listener sits on the
// document, so it survives the HTMX swap that replaces the result panel and its
// buttons.
const (
	// lgPortGraphScript covers every page that carries the looking-glass
	// chrome. pageLayout (layout.templ) loads the new script beside the three
	// already there, so each such page gains one element.
	lgPortGraphScript = "pageLayout loads the graph-mode script, which default-src 'self' allows"
	// lgPortGraphHandler covers the two buttons themselves.
	lgPortGraphHandler = "the graph-mode buttons dropped an onclick no browser ran"
)

// The eight below are one deliberate change: pageLayout stopped loading the
// SSE extension on every page. It loads the assets of the page it wraps
// instead, and only the peers page opens a stream. The sets are derived from
// the markup each page reaches (internal/le/web/assets/webassets.go, page_assets.go).
const lgPortPageAssets = "the head loads what this page needs, and this page opens no SSE stream"

// The seven below are the htmx 4 cutover (spec-web-htmx4-cutover). htmx 2 is
// deleted from the tree, so the peers page subscribes to its stream the way
// htmx 4 spells it and loads the extension htmx 4 publishes.
const (
	// lgPortSSEAttribute covers the peers table body itself. htmx 4 removed
	// hx-ext -- an extension is global once its script loads -- and its SSE
	// extension reads hx-sse:connect.
	lgPortSSEAttribute = "the peers tbody names hx-sse:connect, and hx-ext went with htmx 2"
	// lgPortSSEAsset covers every page whose head loads the extension. htmx 4
	// ships it inside the core npm package as hx-sse.min.js, where htmx 2
	// published htmx-ext-sse separately as sse.js.
	lgPortSSEAsset = "the head loads hx-sse.min.js, which is htmx 4's SSE extension"
)

// The one below is a fix the port left standing. routeCountsAvailable
// (handler_api.go) answered on key PRESENCE, so a count key holding a value
// getNum cannot read as a number reported the counts as available while
// publishing four zeros. mockDispatch (server_test.go) sends its counts as
// strings, so the captured response claimed a real source over a fabricated
// zero -- the one thing routes_counts_available exists to prevent
// (docs/architecture/api/birdwatcher-compat.md Section 7.2).
const lgPortCountsReadable = "routes_counts_available answers on a readable count, not a present key"

// The one below is a defect the port left standing and 2026-09-20 repaired.
// handleAPIStatus dispatched `bgp status`, which no command tree has ever
// declared, so the dispatch failed on every request and the endpoint answered
// 200 over it. The captured bytes held an empty last_reconfig and a version
// mockDispatch had invented for that command. Four declared commands answer it
// now, and a failed dispatch is a refusal
// (plan/journal/zero-value-as-valid-answer.md, 2026-09-17).
const lgPortStatusCommands = "the status endpoint reads four commands the daemon serves, in place of one it does not"

// The two below are one owner decision, 2026-10-08: the looking glass shows
// what a peer sent Ze and what Ze sent it, each in its own field. Every per-peer
// query named no bgp-rib scope, so the plugin answered its default,
// `sent-received`, and the routes Ze sent a peer were listed and counted as
// learned from it (plan/journal/counter-counts-the-wrong-packets.md). The peer
// page now reads the `received` scope and counts the `sent` one beside it, and
// the fake answers each scope apart (ribfake_test.go), so routes/export/{name}
// answers the rows Ze sent where it used to echo the received ones.
const (
	// lgPortPeerCounts covers the peer page: a received and a sent count, side
	// by side.
	lgPortPeerCounts = "the peer page counts the routes the peer sent Ze beside the routes Ze sent it"
	// lgPortExportScope covers routes/export/{name}.
	lgPortExportScope = "routes/export answers the sent scope, which the fake now tells apart from the received"
)

// The two below are one deliberate change: the search fields match what the
// help page promises (spec-lg-search-matches-what-the-pages-promise). A prefix
// with its length is an exact lookup and a bare address lists every covering
// route, and the AS path field takes AS numbers separated by spaces with an
// optional leading ^.
const (
	// lgPortSearchHelp covers the help page, which describes both fields as
	// they now behave.
	lgPortSearchHelp = "the help page describes the exact and covering prefix lookups and the AS path run with its optional ^"
	// lgPortPrefixRefusal covers the search page's refusal of an invalid
	// prefix. prefixRefusal (prefix_lookup.go) names the value, why it is
	// refused and a fix, in place of the fixed "invalid prefix format" text.
	// The route-detail refusals carry the same text but are newer than the
	// pre-port capture, so they have no pre-port bytes to differ from.
	lgPortPrefixRefusal = "an invalid prefix is refused with an operator error that names the value and a fix"
)

// The nine below are one spec, spec-lg-peer-history-origin-search-and-named-nodes,
// and each names what one of its features put on a page that the pre-port page
// did not hold. A capture is given only the reasons its own bytes show.
const (
	// lgPortPeerHistoryColumns covers every capture of the peers table with
	// its header: peersColumns (view.go) gained two entries.
	lgPortPeerHistoryColumns = "the peers table has a Last Change and a Drops column"
	// lgPortPeerHistoryCells covers every capture holding a peer row. The row
	// gained the two cells of those columns, empty when the engine sent no
	// value, and its description cell gained a class.
	lgPortPeerHistoryCells = "each peer row holds a state-changed and a connections-dropped cell, and its description cell carries the peer-description class"
	// lgPortPeerHeaderHistory covers the peer page whose header names a peer
	// with history. peerRoutesContent (peer_routes.templ) writes each span
	// only when the engine sent its value.
	lgPortPeerHeaderHistory = "the peer page header shows the session's last change and its drops, and its last error when it has one"
	// lgPortRouteAge covers every capture holding a route table: both tables
	// end on an Age header and each row on a route-age cell, empty when the
	// engine sent no age.
	lgPortRouteAge = "the route table ends on an Age column"
	// lgPortRouteDetailSpan covers the route detail row, which spans the
	// columns of the table it opens in.
	lgPortRouteDetailSpan = "the route detail row spans nine columns, the route table's count with Age"
	// lgPortTraceButton covers the result panel of a search that names a
	// prefix.
	lgPortTraceButton = "the result panel offers a third graph button, Trace"
	// lgPortASPathTitle covers every capture holding the search form.
	lgPortASPathTitle = "the AS path input states its form in a title, the $ after the last AS number included"
	// lgPortFeatureHelp covers the help page.
	lgPortFeatureHelp = "the help page describes the Last Change and Drops columns, the $ end anchor of an AS path, the route age and the trace graph"
	// lgPortAPIAgeAvailable covers every API answer that lists routes.
	// transformRoutes (handler_api.go) writes age_available beside age, false
	// for a row the engine sent no age for, so a 0 is never read as measured.
	lgPortAPIAgeAvailable = "each route carries age_available beside age, false where the engine sent no age"
)

var lgPortTemplates = map[string]string{
	"layout--peers.html":  lgPortGraphScript + ", " + lgPortSSEAsset,
	"layout--search.html": lgPortGraphScript + ", " + lgPortPageAssets,

	"help.html": lgPortSearchHelp + ", " + lgPortFeatureHelp,

	"peers--empty.html":           lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns,
	"peers--full.html":            lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns + ", " + lgPortPeerHistoryCells,
	"peers_content--empty.html":   lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns,
	"peers_content--full.html":    lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns + ", " + lgPortPeerHistoryCells,
	"peers_table_body--full.html": lgPortPeerHistoryCells,

	"peer_routes--empty.html":           lgPortPeerCounts,
	"peer_routes--routes.html":          lgPortPeerCounts + ", " + lgPortPeerHeaderHistory + ", " + lgPortRouteAge,
	"peer_routes--summary.html":         lgPortPeerCounts,
	"peer_routes_content--empty.html":   lgPortPeerCounts,
	"peer_routes_content--routes.html":  lgPortPeerCounts + ", " + lgPortPeerHeaderHistory + ", " + lgPortRouteAge,
	"peer_routes_content--summary.html": lgPortPeerCounts,

	"route_detail--found.html":   lgPortRouteDetailSpan,
	"route_detail--missing.html": lgPortRouteDetailSpan,

	"route_results--no-prefix.html": lgPortRouteAge,
	"route_results--routes.html":    lgPortGraphHandler + ", " + lgPortTraceButton + ", " + lgPortRouteAge,

	"search--blank.html":       lgPortASPathTitle,
	"search--filled.html":      lgPortGraphHandler + ", " + lgPortASPathTitle + ", " + lgPortTraceButton + ", " + lgPortRouteAge,
	"search_form--blank.html":  lgPortASPathTitle,
	"search_form--filled.html": lgPortASPathTitle,
}

var lgPortHandlers = map[string]string{
	"api-protocols-bgp.txt":   lgPortCountsReadable,
	"api-routes-bmp.txt":      lgPortAPIAgeAvailable,
	"api-routes-export.txt":   lgPortExportScope + ", " + lgPortAPIAgeAvailable,
	"api-routes-peer.txt":     lgPortAPIAgeAvailable,
	"api-routes-prefix.txt":   lgPortAPIAgeAvailable,
	"api-routes-protocol.txt": lgPortAPIAgeAvailable,
	"api-routes-search.txt":   lgPortAPIAgeAvailable,
	"api-status.txt":          lgPortStatusCommands,

	"ui-search-empty.txt":   lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortASPathTitle,
	"ui-search-invalid.txt": lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortPrefixRefusal + ", " + lgPortASPathTitle,
	"ui-search-result.txt":  lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortASPathTitle,

	"gated-peers-authorized.txt": lgPortGraphScript + ", " + lgPortSSEAsset + ", " + lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns + ", " + lgPortPeerHistoryCells,
	"ui-help.txt":                lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortSearchHelp + ", " + lgPortFeatureHelp,
	"ui-peer-routes.txt":         lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortPeerCounts + ", " + lgPortPeerHeaderHistory + ", " + lgPortRouteAge,
	"ui-peers.txt":               lgPortGraphScript + ", " + lgPortSSEAsset + ", " + lgPortSSEAttribute + ", " + lgPortPeerHistoryColumns + ", " + lgPortPeerHistoryCells,
	"ui-route-detail.txt":        lgPortRouteDetailSpan,
	"ui-search-form.txt":         lgPortGraphScript + ", " + lgPortPageAssets + ", " + lgPortASPathTitle,
}

// TestLGTemplPortFidelity compares every captured unit against the bytes it
// held before the templ port.
//
// It is the evidence for AC-2 of spec-web-templ-migration over the lg
// half. Phase 2 ran that comparison by hand and no caller kept it runnable.
//
// The lg port restructured no unit: every fixture the pre-port capture held has
// the same path today.
//
// VALIDATES: no unit of the looking glass renders different content after the
// port.
// PREVENTS: a re-baselined fixture hiding a change an operator would see.
func TestLGTemplPortFidelity(t *testing.T) {
	ref := golden.PortRef()

	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "golden"), golden.PortMarkup, lgPortTemplates)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", lgHandlerFixtures), golden.PortResponse, lgPortHandlers)
}
