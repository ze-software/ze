// Related: view.go -- the values every component below renders
// Related: render.go -- the Renderer that calls these components

package web

import (
	"testing"

	"github.com/ze-software/ze/internal/test/templcheck"
)

// webComponentCount is the number of templ components the web package declares.
// The check below would pass over an empty set, so it counts what it inspected
// and fails when the walk finds nothing. Raise it with each component added.
const webComponentCount = 93

// TestWebViewDataIsTyped reads the generated components and requires every
// parameter to be a type the compiler checks a field name against.
//
// VALIDATES: AC-8 of spec-web-templ-migration for the web package. No
// map reaches a templ component, so every field the markup reads is resolved by
// the compiler.
// PREVENTS: the failure html/template allowed. ExecuteTemplate on a
// map[string]any returns no error for a key the markup misspells. It renders an
// empty value and reports success, so a renamed field produced a blank panel
// and nothing in the log.
//
// A STRUCT THAT WRAPS A MAP DOES NOT SATISFY IT. templcheck walks struct
// fields, embedded ones included, and refuses the map it reaches. That wrapper
// was the cheapest port of the map[string]any the l2tp handlers build. It would
// have defeated the guard one dereference in.
//
// Those maps are still the JSON response body and reach no component.
// view_l2tp.go carries the view models, and handler_l2tp.go builds both shapes
// from one snapshot.
func TestWebViewDataIsTyped(t *testing.T) {
	templcheck.AssertTyped(t, ".", webComponentCount)
}
