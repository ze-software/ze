// Related: page_assets.go -- the generated sets the captured heads render
// Related: layout.templ -- the one shell, whose head loads v.Page's assets
// Related: handler_golden_test.go -- the capture this reads

package lg

import (
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// lgCapturedPageFloor is the least number of whole pages the captures hold.
// They hold 11 on 2026-08-15: two layout fixtures and nine handler responses.
// A capture directory that stops holding pages would leave this check reading
// nothing while staying green.
const lgCapturedPageFloor = 8

// TestLGPageImportsCoverRenderedAttributes requires every captured looking-glass
// page to load the asset of every htmx attribute it renders.
//
// One layout serves every page here, and its head loads the set of the page the
// handler names rather than the union of every page. That makes a wrong or
// missing layoutView.Page a live failure: the head loads a set the body does
// not match. The bytes stay valid HTML and the handler still answers 200, so
// only this comparison sees it.
//
// It is the half that reads the captures. internal/le/webassets/webassets.go walks
// the sources instead, and over-approximates. A page loading more than it
// renders is therefore not a finding here.
//
// VALIDATES: every htmx attribute a captured page renders has its asset in that
// page's own head.
// PREVENTS: the peers page losing the SSE extension. Its table would render,
// the stream would never open, and every gate would stay green.
func TestLGPageImportsCoverRenderedAttributes(t *testing.T) {
	findings, pages, err := markupcheck.HeadCoverageFindings("testdata", "/lg/assets/")
	if err != nil {
		t.Fatalf("scan testdata for captured pages: %v", err)
	}

	if short := markupcheck.Shortfall("captured pages", pages, lgCapturedPageFloor); short != "" {
		t.Errorf("scan testdata %s", short)
	}

	for _, finding := range findings {
		t.Error(finding)
	}
}
