// VALIDATES: the canonical CLI verb vocabulary (Verbs) and each verb's role.
// PREVENTS: silent drift of the verb set that the grammar gate and the plugin
// registration gate both derive from (AC-1); a stray verb or a wrong role would
// let a non-verb-first command pass the gate.

package command

import "testing"

// TestVerbRegistryCanonical pins the canonical verb vocabulary and each verb's
// role. The gate and the plugin registration check both derive from Verbs, so a
// change here is a deliberate vocabulary decision (AC-1).
func TestVerbRegistryCanonical(t *testing.T) {
	want := map[string]verbRole{
		"show":    VerbRead,
		"monitor": VerbRead,
		"resolve": VerbRead,
		"set":     VerbMutation,
		"delete":  VerbMutation,
		"clear":   VerbAction,
		"request": VerbAction,
		"commit":  VerbAction,
		"update":  VerbAction,
		"cache":   VerbAction,
		"create":  VerbAction,
		"debug":   VerbAction,
		"send":    VerbAction,
	}
	if len(Verbs) != len(want) {
		t.Fatalf("Verbs has %d entries, want %d: %v", len(Verbs), len(want), VerbList())
	}
	for v, role := range want {
		got, ok := Verbs[v]
		if !ok {
			t.Errorf("missing canonical verb %q", v)
			continue
		}
		if got != role {
			t.Errorf("verb %q role = %d, want %d", v, got, role)
		}
	}
}

func TestIsVerb(t *testing.T) {
	for _, v := range []string{"show", "set", "delete", "request"} {
		if !IsVerb(v) {
			t.Errorf("IsVerb(%q) = false, want true", v)
		}
	}
	if !IsVerb("create") {
		t.Error("IsVerb(create) = false; create is a runtime-lifecycle verb")
	}
	// Noun-first / non-verb first tokens must be rejected.
	for _, v := range []string{"metrics", "config", "peer", "interface", "", "SHOW"} {
		if IsVerb(v) {
			t.Errorf("IsVerb(%q) = true, want false", v)
		}
	}
}

// TestVerbListSortedAndComplete asserts VerbList is sorted and covers every verb,
// so error messages never drift from the registry.
func TestVerbListSortedAndComplete(t *testing.T) {
	list := VerbList()
	if len(list) != len(Verbs) {
		t.Fatalf("VerbList len %d != Verbs len %d", len(list), len(Verbs))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1] >= list[i] {
			t.Errorf("VerbList not sorted at %d: %q >= %q", i, list[i-1], list[i])
		}
	}
}
