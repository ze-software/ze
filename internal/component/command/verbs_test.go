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
		"show":    RoleRead,
		"monitor": RoleRead,
		"resolve": RoleRead,
		"set":     RoleMutation,
		"delete":  RoleMutation,
		"clear":   RoleAction,
		"request": RoleAction,
		"commit":  RoleAction,
		"update":  RoleAction,
		"cache":   RoleAction,
		"create":  RoleAction,
		"debug":   RoleAction,
		"send":    RoleAction,
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

// TestVerbConstantsAreTheRegistryKeys asserts that every exported verb spelling
// is a key of Verbs and that Verbs holds no key without one. The constants are
// what other packages reference instead of writing the word again, so a verb
// added to the map without a constant would leave those surfaces with nothing
// to reference and a copy would grow back (ai/rules/principles.md).
func TestVerbConstantsAreTheRegistryKeys(t *testing.T) {
	constants := []string{
		VerbShow, VerbMonitor, VerbResolve,
		VerbSet, VerbDelete,
		VerbClear, VerbRequest, VerbCommit, VerbUpdate, VerbCache,
		VerbCreate, VerbSend, VerbDebug,
	}
	held := make(map[string]bool, len(constants))
	for _, verb := range constants {
		if !IsVerb(verb) {
			t.Errorf("constant %q is not a key of Verbs", verb)
		}
		held[verb] = true
	}
	for verb := range Verbs {
		if !held[verb] {
			t.Errorf("verb %q has no exported constant, so a caller must spell it again", verb)
		}
	}
}

// TestRoleUnspecifiedForNonVerb asserts that a token Verbs does not hold reads
// back as RoleUnspecified rather than as the first role of the enum. A zero
// value that reads as a legitimate role would classify every unknown first word
// as a read (ai/rules/principles.md).
func TestRoleUnspecifiedForNonVerb(t *testing.T) {
	for _, tok := range []string{"", "peer", "validate", "SHOW", "interface"} {
		if got := Verbs[tok]; got != RoleUnspecified {
			t.Errorf("Verbs[%q] = %d, want RoleUnspecified", tok, got)
		}
		if IsReadOnlyVerb(tok) {
			t.Errorf("IsReadOnlyVerb(%q) = true; it is not a canonical verb", tok)
		}
	}
}

// TestIsReadOnlyVerbTracksTheRegistry asserts that the read-only answer is the
// registry's role and nothing else. Written against Verbs rather than a list of
// words, so giving a verb a new role, or adding one, moves this test with it.
func TestIsReadOnlyVerbTracksTheRegistry(t *testing.T) {
	for verb, role := range Verbs {
		want := role == RoleRead
		if got := IsReadOnlyVerb(verb); got != want {
			t.Errorf("IsReadOnlyVerb(%q) = %v, want %v (role %d)", verb, got, want, role)
		}
	}
}

// TestNoVerbCarriesRoleUnspecified asserts the invariant IsVerb rests on: every
// entry of Verbs names a role. Without it a verb added with the zero role would
// be reported as not a verb by IsVerb and as a verb by a range over Verbs, and
// the two readers would disagree with no line deleted (ai/rules/principles.md).
func TestNoVerbCarriesRoleUnspecified(t *testing.T) {
	for verb, role := range Verbs {
		if role == RoleUnspecified {
			t.Errorf("verb %q carries RoleUnspecified: IsVerb would refuse it", verb)
		}
	}
}
