// VALIDATES: the full verification population runs `le arch refuse-by-default
// check`, beside the other change-set arch gates, in both full and fast modes.
// PREVENTS: the refuse-by-default check existing as a command no verify run
// calls, so new code reintroduces the family with every gate green.
package verifyengine

import "testing"

// TestVerifyStagesIncludeRefuseByDefault holds that the stage is in the full
// population exactly once, with its action, after arch compound-guard, and
// that the fast population keeps it: it reads changed lines, not the tree.
func TestVerifyStagesIncludeRefuseByDefault(t *testing.T) {
	const want = "arch refuse-by-default/check"
	for _, mode := range []string{Mode, FastMode} {
		stages := StagesForMode(mode)
		found := -1
		guard := -1
		for index, current := range stages {
			switch current.Identity.Name {
			case want:
				if found >= 0 {
					t.Fatalf("%s: %s appears twice", mode, want)
				}
				found = index
			case "arch compound-guard/check":
				guard = index
			}
		}
		if found < 0 {
			t.Fatalf("%s population holds no %s", mode, want)
		}
		if guard < 0 || found != guard+1 {
			t.Errorf("%s: %s at %d, want right after arch compound-guard at %d", mode, want, found, guard)
		}
	}
}
