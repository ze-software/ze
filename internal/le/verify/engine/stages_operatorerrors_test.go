// VALIDATES: the full and fast verification populations run `le arch
// error-wrap check` and `le arch operator-error check` once each, right after
// arch refuse-by-default, as full judgements (Whole) because both read the
// unpushed lines from refs rather than the published scope
// (spec-operator-errors-name-the-fix, AC-4).
// PREVENTS: either check existing as a command no verify run calls, so new
// code reintroduces a doubled prefix or a refusal with no fix with every gate
// green.
package verifyengine

import "testing"

// TestVerifyStagesIncludeOperatorErrorChecks holds AC-4's stage list.
func TestVerifyStagesIncludeOperatorErrorChecks(t *testing.T) {
	const (
		wrap     = "arch error-wrap/check"
		operator = "arch operator-error/check"
		previous = "arch refuse-by-default/check"
	)
	for _, mode := range []string{Mode, FastMode} {
		positions := map[string][]int{}
		whole := map[string]bool{}
		for index, current := range StagesForMode(mode) {
			positions[current.Identity.Name] = append(positions[current.Identity.Name], index)
			whole[current.Identity.Name] = current.Whole
		}
		for _, name := range []string{wrap, operator, previous} {
			if len(positions[name]) != 1 {
				t.Fatalf("%s population holds %s %d time(s), want once", mode, name, len(positions[name]))
			}
		}
		if positions[wrap][0] != positions[previous][0]+1 {
			t.Errorf("%s: %s at %d, want right after %s at %d", mode, wrap, positions[wrap][0], previous, positions[previous][0])
		}
		if positions[operator][0] != positions[wrap][0]+1 {
			t.Errorf("%s: %s at %d, want right after %s at %d", mode, operator, positions[operator][0], wrap, positions[wrap][0])
		}
		for _, name := range []string{wrap, operator} {
			if !whole[name] {
				t.Errorf("%s: %s is not declared Whole, but it reads changed lines from refs, never the published scope", mode, name)
			}
		}
	}
}
