// Related: view.go -- the structs this test requires every component to take

package lg

import (
	"testing"

	"github.com/ze-software/ze/internal/test/templcheck"
)

// lgComponentCount is the number of templ components the looking glass
// declares. The check below would pass over an empty set, so it counts what it
// inspected and fails when the walk finds nothing.
const lgComponentCount = 15

// TestLGViewDataIsTyped reads the generated components and requires every
// parameter to be a type the compiler checks a field name against.
//
// VALIDATES: AC-8. No map reaches a templ component, so every field the markup
// reads is resolved by the compiler.
// PREVENTS: the failure html/template allowed. ExecuteTemplate on a
// map[string]any returns no error for a key the markup misspells. It renders
// an empty value and reports success. A renamed field therefore produced a
// blank panel and nothing in the log.
//
// The rules live in internal/test/templcheck, because phase 3 of
// plan/spec-web-templ-migration.md applies the same guard to the web package.
// A named type is resolved through the package's own declarations, so
// `type viewData map[string]any` is refused as the map it is.
//
// test-relax: the AST walk that used to sit here moved to
// internal/test/templcheck, which refuses strictly more than it did. The
// assertions are in templcheck_test.go, where a fixture proves each escape is
// refused by name. Coverage is replaced and widened, not dropped.
func TestLGViewDataIsTyped(t *testing.T) {
	templcheck.AssertTyped(t, ".", lgComponentCount)
}
