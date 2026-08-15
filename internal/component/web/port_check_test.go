// Related: golden_test.go -- the TEMPLATE capture whose fixtures this compares
// Related: handler_golden_test.go -- the HANDLER capture whose fixtures this compares

package web

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// webPortTemplates explains each template fixture that does not match its
// pre-port bytes. Every other difference is a finding.
//
// html/template has no children, so a wrapper is written as a start template
// and an end template, with the content between the two calls. A templ
// component takes its children, so the pair became one component.
//
// The two shapes cannot be compared by path. The pre-port capture rendered the
// start and the end as two units with nothing between them. The ported fixture
// renders the whole wrapper around its children, and covers strictly more.
//
// The wrapper's own frame is still compared. component/detail--fields.html
// renders ten fields through it and has a pre-port counterpart. That fixture
// carries the `class="ze-field" id="field-hold-time"` frame on both sides.
const webPortWrapperPair = "the start and end pair became fieldWrapper, one component taking its children"

var webPortTemplates = map[string]string{
	"input/field_wrapper_start--bare.html":      webPortWrapperPair,
	"input/field_wrapper_start--annotated.html": webPortWrapperPair,
	"input/field_wrapper_end.html":              webPortWrapperPair,
}

// webPortHandlers explains the ONE response whose content changed on purpose.
// It is AC-5, recorded against A-2 in plan/spec-web-templ-migration.md.
//
// handleDashboardEventsPage (page_dashboard.go) ran each cell through
// template.HTMLEscapeString, and handed the result to markup that escapes
// again. An operator therefore read the JSON payload of an event as &#34;
// rather than as a quote. The pre-port fixture holds that defect. The hand
// escape is deleted, and templ escapes the value once.
var webPortHandlers = map[string]string{
	"nav-show-events.txt": "AC-5: the event payload was escaped twice before the port",
}

// TestWebTemplPortFidelity compares every captured unit against the bytes it
// held before the templ port.
//
// It is the evidence for AC-2 of plan/spec-web-templ-migration.md, and it takes
// the ref as a parameter so a reader can run the same comparison. The
// comparison behind it was hand-run once. That left the acceptance criterion on
// a measurement nobody can repeat.
//
// VALIDATES: no unit of the web UI renders different content after the port.
// PREVENTS: a re-baselined fixture hiding a change an operator would see. Once
// a fixture is recaptured, the golden check compares the port against itself.
func TestWebTemplPortFidelity(t *testing.T) {
	ref := golden.PortRef()

	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "golden"), golden.PortMarkup, webPortTemplates)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", webHandlerFixtures), golden.PortResponse, webPortHandlers)
	golden.AssertPortFidelity(t, ref, filepath.Join("testdata", "markup"), golden.PortMarkup, nil)
}
