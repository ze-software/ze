// Related: internal/test/markupcheck -- the scans this calls
// Related: view_test.go -- the sibling guard, over what a component TAKES

package lg

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// lgDrawingReason is why the two graph builders keep their markup in Go.
//
// They are not documents. Every attribute either one writes is a coordinate
// computeLayout and computeNextHopLayout produced. The port therefore buys
// neither of the two things this migration exists to buy.
//
// It buys no type safety: renderGraphSVG reads layout.Positions[n.ASN].X in Go
// today, so a renamed field is ALREADY a compile error. The blank-panel failure
// that motivates the spec needs an unchecked name, and there is none here.
//
// It buys no escaping: the label and the tooltip are the only values either
// builder interpolates, and both go through template.HTMLEscapeString.
//
// It costs bytes and legibility. templ rewrites every self-closing element,
// measured 2026-08-15 on a probe. <rect x="20" y="20" width="81" height="40"/>
// becomes <rect ...></rect>, and a graph draws three such elements per node and
// two per edge. The response is image/svg+xml, which golden.AssertPortFidelity
// compares byte for byte rather than normalizing. Each coordinate attribute
// would also become an intText call inside the markup.
//
// graphEmpty (graph_empty.templ) is the counter-example that keeps this
// narrow. It is an SVG too, and it IS a document: two fixed elements and one
// message. It is ported.
const lgDrawingReason = "a generated drawing: every attribute is a coordinate a layout pass computed"

// lgMarkupExempt names each Go file in this package allowed to build markup.
var lgMarkupExempt = map[string]string{
	"layout.go":         lgDrawingReason,
	"layout_nexthop.go": lgDrawingReason,
}

// lgMarkupFloors is what a pass of TestNoGoFileBuildsMarkup must clear.
//
// Literals is a floor rather than the exact count, so adding a file does not
// red it. The walk read 703 string literals on 2026-08-15.
//
// Exempt is EXACT, and it is what makes the PREVENTS claim below true.
// markupcheck.Findings reports an exemption that exempts nothing, so a RETIRED
// drawing is caught. It is blind to a THIRD entry written over a file that does
// build markup, which is the edit that widens the table.
var lgMarkupFloors = markupcheck.Floors{Literals: 350, Exempt: 2}

// lgTemplFileFloor is the least number of .templ sources the CSP scan and the
// asset scan must read. The package holds 9.
const lgTemplFileFloor = 6

// lgAssetRefFloor is the least number of /lg/assets/ paths the templ sources
// must name. They name 6 on 2026-08-15: the stylesheet, three scripts, the
// graph-mode script and the logo.
const lgAssetRefFloor = 5

// TestNoGoFileBuildsMarkup requires every HTML tag in this package to live in a
// .templ file, except the two drawings named above.
//
// It is the evidence for AC-7 of plan/spec-web-templ-migration.md over the lg
// half.
//
// VALIDATES: no markup is built in Go here beyond the two named drawings.
// PREVENTS: the exemption widening. A third file that starts building markup
// is a finding. Either entry above is a finding once its builder is ported. A
// third ENTRY is a finding too, because lgMarkupFloors fixes the table size.
func TestNoGoFileBuildsMarkup(t *testing.T) {
	markupcheck.AssertNoMarkup(t, ".", lgMarkupExempt, lgMarkupFloors)
}

// TestTemplatesAvoidInlineScriptAndStyle requires every .templ in this package
// to stay compatible with the header the looking glass sends.
//
// setSecurityHeaders (server.go) answers `default-src 'self'` with NO
// script-src beside it, so default-src is what a browser applies to script. It
// refuses an inline handler and an inline script block.
//
// The header does allow an inline style, through `style-src 'self'
// 'unsafe-inline'`. The scan refuses one anyway, so both rendering packages
// hold the same rule and the header can be tightened without a hunt.
//
// THE PACKAGE HAD NO SUCH CHECK, and it shipped the failure. route_table.templ
// carried an onclick on each graph-mode button, so the pressed button never
// gained its .active class in any browser. The web package's sibling test
// caught nothing here, because it walks its own directory.
//
// VALIDATES: every markup source in this package uses external scripts and
// styles and delegated event handlers.
// PREVENTS: an inline handler shipping again, dead on arrival and silent.
func TestTemplatesAvoidInlineScriptAndStyle(t *testing.T) {
	markupcheck.AssertNoInlineScriptOrStyle(t, ".", lgTemplFileFloor)
}

// TestTemplAssetsResolve requires every asset a .templ names to exist in the
// filesystem the server mounts at /lg/assets/.
//
// It resolves against the SUB-filesystem registerRoutes builds, not the embed
// root, so it answers the question a browser asks.
//
// VALIDATES: each src and href under /lg/assets/ resolves to a served file.
// PREVENTS: a 404 that only a browser sees. Renaming assets/graph-mode.js
// would leave the graph-mode buttons inert again, with every gate green.
func TestTemplAssetsResolve(t *testing.T) {
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("sub-FS for the served assets: %v", err)
	}

	markupcheck.AssertAssetsResolve(t, ".", "/lg/assets/", assets, lgAssetRefFloor)
}

// graphModeClass is the one token the buttons and their script must agree on.
const graphModeClass = "graph-mode-btn"

// TestGraphModeScriptTargetsTheButtons requires the delegated handler to select
// the class the rendered buttons carry.
//
// This is the join no other guard sees. TestTemplatesAvoidInlineScriptAndStyle
// proves the buttons carry no onclick. TestTemplAssetsResolve proves the script
// is served. Neither reads both sides, so renaming the class on one side leaves
// the buttons inert again and every gate green.
//
// VALIDATES: pressing a graph-mode button reaches a handler with no inline
// attribute. The page loads the script, the script selects the class, and the
// class is on each button.
// PREVENTS: the outage this phase closed coming back through a rename.
func TestGraphModeScriptTargetsTheButtons(t *testing.T) {
	panel, err := renderToString(routeResults(searchView{
		Prefix: "203.0.113.0/24",
		Routes: lgRouteRows(), Count: 2,
	}))
	if err != nil {
		t.Fatalf("render routeResults: %v", err)
	}

	if !strings.Contains(panel, graphModeClass) {
		t.Errorf("the rendered buttons carry no %q class, so no delegated handler can find them", graphModeClass)
	}

	if strings.Contains(panel, "onclick") {
		t.Errorf("the rendered buttons carry an onclick, which default-src 'self' refuses")
	}

	page, err := renderToString(pageLayout(layoutView{Title: "Search"}, routeResults(searchView{})))
	if err != nil {
		t.Fatalf("render pageLayout: %v", err)
	}

	if !strings.Contains(page, "/lg/assets/graph-mode.js") {
		t.Errorf("the page loads no graph-mode script, so the buttons reach no handler")
	}

	script, err := assetsFS.ReadFile("assets/graph-mode.js")
	if err != nil {
		t.Fatalf("read the served graph-mode script: %v", err)
	}

	if !strings.Contains(string(script), graphModeClass) {
		t.Errorf("the script selects no %q, so it answers no button", graphModeClass)
	}

	if !strings.Contains(string(script), "active") {
		t.Errorf("the script sets no active class, so the pressed button still looks unpressed")
	}
}
