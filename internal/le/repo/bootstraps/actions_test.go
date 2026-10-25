package repobootstraps

import "testing"

// TestBootstrapsIsDeclaredInItsTable proves the gate publishes its one action,
// and that a stray word is refused with the usage code before le is built.
//
// VALIDATES: AC-14 and D-P3-bare (bare and `check` are one gate) and
// D-P3-stray (a stray word exits 2).
// PREVENTS: a second parser beside the table.
func TestBootstrapsIsDeclaredInItsTable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 || list.Actions[0].Verb != checkVerb {
		t.Fatalf("repo bootstraps declares %+v, want the one check action", list.Actions)
	}
	if !actions.Holds(checkVerb) {
		t.Error("the check verb is not held, so a bare line and `check` would differ")
	}
	for _, line := range [][]string{{checkVerb, "extra"}, {"extra"}} {
		payload, code := Answer(line)
		if code != 2 {
			t.Errorf("Answer(%q) = %d, want 2", line, code)
		}
		if payload != nil {
			t.Errorf("Answer(%q) answered a payload: %v", line, payload)
		}
	}
}
