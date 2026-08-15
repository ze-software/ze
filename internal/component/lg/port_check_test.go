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
// the markup each page reaches (scripts/codegen/web_assets.go, page_assets.go).
const lgPortPageAssets = "the head loads what this page needs, and this page opens no SSE stream"

var lgPortTemplates = map[string]string{
	"layout--peers.html":  lgPortGraphScript,
	"layout--search.html": lgPortGraphScript + ", " + lgPortPageAssets,

	"route_results--routes.html": lgPortGraphHandler,
	"search--filled.html":        lgPortGraphHandler,
}

var lgPortHandlers = map[string]string{
	"ui-search-empty.txt":   lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets,
	"ui-search-invalid.txt": lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets,
	"ui-search-result.txt":  lgPortSearchBanner + ", " + lgPortGraphScript + ", " + lgPortPageAssets,

	"gated-peers-authorized.txt": lgPortGraphScript,
	"ui-help.txt":                lgPortGraphScript + ", " + lgPortPageAssets,
	"ui-peer-routes.txt":         lgPortGraphScript + ", " + lgPortPageAssets,
	"ui-peers.txt":               lgPortGraphScript,
	"ui-search-form.txt":         lgPortGraphScript + ", " + lgPortPageAssets,
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
