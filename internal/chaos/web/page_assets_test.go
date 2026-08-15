// Related: page_assets.go -- the generated set the captured head renders
// Related: render.go -- writeLayout, the one shell of the chaos dashboard
// Related: golden_test.go -- the capture this reads

package web

import (
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// chaosCapturedPageFloor is the least number of whole pages the captures hold.
// They hold two on 2026-08-15: the dashboard with a control channel and
// without. Every other fixture is a fragment served into one of them.
const chaosCapturedPageFloor = 2

// TestChaosPageImportsCoverRenderedAttributes requires the captured dashboard to
// load the asset of every htmx attribute it renders.
//
// The dashboard writes its markup in Go, so no component graph exists to walk.
// scripts/codegen/web_assets.go therefore gives its one page every asset the
// package renders, and this reads the captured bytes back to prove the set is
// enough.
//
// VALIDATES: every htmx attribute the dashboard renders has its asset in the
// head writeLayout wrote.
// PREVENTS: a head that stops loading the SSE extension. The dashboard would
// render, the event stream would never open, and the page would sit still with
// every gate green.
func TestChaosPageImportsCoverRenderedAttributes(t *testing.T) {
	findings, pages, err := markupcheck.HeadCoverageFindings("testdata/golden", "/assets/")
	if err != nil {
		t.Fatalf("scan testdata/golden for captured pages: %v", err)
	}

	if short := markupcheck.Shortfall("captured pages", pages, chaosCapturedPageFloor); short != "" {
		t.Errorf("scan testdata/golden %s", short)
	}

	for _, finding := range findings {
		t.Error(finding)
	}
}
