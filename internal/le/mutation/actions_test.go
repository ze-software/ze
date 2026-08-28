package mutation

import "testing"

func TestMutationActionsExposeTwoNativeWrites(t *testing.T) {
	list := Actions()
	if list.Area != "mutation" || len(list.Actions) != 2 {
		t.Fatalf("mutation actions = %#v", list)
	}
	if got := list.Actions[0]; got.Verb != "combine" || !got.Writes {
		t.Fatalf("combine action = %#v", got)
	}
	if got := list.Actions[1]; got.Verb != "record-history" || !got.Writes {
		t.Fatalf("record-history action = %#v", got)
	}
	if Subs() != "combine (writes) | record-history (writes)" {
		t.Fatalf("mutation subs = %q", Subs())
	}
}
