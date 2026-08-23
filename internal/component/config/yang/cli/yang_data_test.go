package cli

import "testing"

// TestDataTreeAnswersRows pins the shape the command DECLARES against the shape
// it actually answers. It was declared `doc` first, on the reading that a tree
// is one document; formatTreeJSON emits a top-level array, so its rows are the
// top-level nodes. A declaration that disagrees with the answer publishes a
// refusal the product does not make.
func TestDataTreeAnswersRows(t *testing.T) {
	payload, code := dataTree(nil)
	if code != 0 {
		t.Fatalf("dataTree exited %d", code)
	}
	rows, ok := payload.([]any)
	if !ok {
		t.Fatalf("dataTree answers %T, want a list of top-level nodes", payload)
	}
	if len(rows) == 0 {
		t.Fatal("dataTree answered no nodes")
	}
	if _, isRecord := rows[0].(map[string]any); !isRecord {
		t.Errorf("a tree row is %T, want a record", rows[0])
	}
}

// TestDataTreeFilterNarrows proves the flags reach the builder. Without this
// the flag words would be accepted and silently ignored, which is the shape of
// defect this whole spec exists to end.
func TestDataTreeFilterNarrows(t *testing.T) {
	all, code := dataTree(nil)
	if code != 0 {
		t.Fatalf("dataTree exited %d", code)
	}
	commands, code := dataTree([]string{flagCommands})
	if code != 0 {
		t.Fatalf("dataTree --commands exited %d", code)
	}
	allRows, _ := all.([]any)
	cmdRows, _ := commands.([]any)
	if len(cmdRows) == 0 {
		t.Fatal("--commands answered nothing")
	}
	if len(cmdRows) >= len(allRows) {
		t.Errorf("--commands kept %d of %d nodes; the filter did not narrow", len(cmdRows), len(allRows))
	}
}

// TestDataCompletionAnswersCollisions covers the second converted command.
func TestDataCompletionAnswersCollisions(t *testing.T) {
	payload, code := dataCompletion(nil)
	if code != 0 {
		t.Fatalf("dataCompletion exited %d", code)
	}
	envelope, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("dataCompletion answers %T, want an envelope", payload)
	}
	if _, has := envelope["collisions"]; !has {
		t.Errorf("the answer carries no collisions key: %v", envelope)
	}
}
