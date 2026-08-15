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

var lgPortHandlers = map[string]string{
	"ui-search-empty.txt":   lgPortSearchBanner,
	"ui-search-invalid.txt": lgPortSearchBanner,
	"ui-search-result.txt":  lgPortSearchBanner,
}

// TestLGTemplPortFidelity compares every captured unit against the bytes it
// held before the templ port.
//
// It is the evidence for AC-2 of plan/spec-web-templ-migration.md over the lg
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

	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "golden"), golden.PortMarkup, nil)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", lgHandlerFixtures), golden.PortResponse, lgPortHandlers)
}
