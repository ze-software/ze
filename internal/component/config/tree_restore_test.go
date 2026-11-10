package config

import (
	"slices"
	"testing"
)

// TestRestoreValueCopiesTheCommittedLeaf proves RestoreValue makes one leaf
// what the source tree holds and leaves every other leaf alone. It serves the
// editor's `discard <path>`, which must drop the pending change to that leaf
// and nothing else.
//
// Method: a committed tree and a working copy diverge on a changed leaf, a
// deleted deactivated leaf, an added leaf and a leaf-list; each is restored in
// turn and read back, together with a leaf no restore names.
func TestRestoreValueCopiesTheCommittedLeaf(t *testing.T) {
	committed := NewTree()
	committed.Set("changed", "1")
	committed.Set("deleted", "2")
	committed.SetLeafInactive("deleted", true)
	committed.Set("untouched", "u")
	committed.SetMemberStates("members", []MemberState{{Value: "x"}, {Value: "y", Inactive: true}})

	working := committed.Clone()
	working.Set("changed", "9")
	working.Delete("deleted")
	working.ClearLeafInactive("deleted")
	working.Set("added", "3")
	working.Set("untouched", "pending")
	working.SetMemberStates("members", []MemberState{{Value: "z"}})

	working.RestoreValue("changed", committed)
	if got, _ := working.Get("changed"); got != "1" {
		t.Errorf("changed = %q, want the committed 1", got)
	}

	working.RestoreValue("deleted", committed)
	if got, ok := working.Get("deleted"); !ok || got != "2" {
		t.Errorf("deleted = %q (present %v), want the committed 2 back", got, ok)
	}
	if !working.IsLeafInactive("deleted") {
		t.Error("deleted came back active, want its committed deactivation")
	}

	working.RestoreValue("added", committed)
	if got, ok := working.Get("added"); ok {
		t.Errorf("added = %q, want it gone: the committed tree does not hold it", got)
	}
	if slices.Contains(working.Values(), "added") {
		t.Error("added is still in the leaf order")
	}

	working.RestoreValue("members", committed)
	if got, want := working.GetMultiValuesState("members"), committed.GetMultiValuesState("members"); !slices.Equal(got, want) {
		t.Errorf("members = %v, want the committed %v", got, want)
	}

	if got, _ := working.Get("untouched"); got != "pending" {
		t.Errorf("untouched = %q, want its pending value kept: no restore named it", got)
	}

	working.RestoreValue("changed", nil)
	if _, ok := working.Get("changed"); ok {
		t.Error("a nil source must remove the leaf")
	}
}
