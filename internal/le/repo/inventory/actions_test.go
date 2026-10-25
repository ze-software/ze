package repoinventory

import "testing"

// TestInventoryIsDeclaredInItsTable proves the area publishes its one action,
// and that the table reads a line naming it: a stray word after the verb is
// refused with the usage code before any tree is read.
//
// VALIDATES: AC-14 and D-P3-bare (the bare line still runs the report, pinned by
// TestAnswerAnswersTheInventory) and D-P3-stray (a stray word exits 2).
// PREVENTS: a second parser beside the table.
func TestInventoryIsDeclaredInItsTable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 || list.Actions[0].Verb != reportVerb {
		t.Fatalf("repo inventory declares %+v, want the one report action", list.Actions)
	}
	for _, line := range [][]string{{reportVerb, "extra"}, {"extra"}} {
		payload, code := Answer(line)
		if code != 2 {
			t.Errorf("Answer(%q) = %d, want 2", line, code)
		}
		if payload != nil {
			t.Errorf("Answer(%q) answered a payload: %v", line, payload)
		}
	}
}
