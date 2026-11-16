// Design: docs/architecture/system-architecture.md -- a role selected by an environment key before command dispatch

package earlyrole

import "testing"

// Keys of this file. No program sets them, so the developer's environment
// cannot select a role in these tests.
const (
	testKeyOwned   = "ZE_TEST_EARLY_ROLE_OWNED"
	testKeyOther   = "ZE_TEST_EARLY_ROLE_OTHER"
	testKeyUnowned = "ZE_TEST_EARLY_ROLE_UNOWNED"
)

// registerCounted registers a role under key that counts its runs and answers
// code. The caller restores the registered roles with Snapshot.
func registerCounted(t *testing.T, key string, code int) *int {
	t.Helper()
	runs := new(int)
	Register(key, func() int {
		*runs++
		return code
	})
	return runs
}

// TestEarlyRoleRunsWhenItsKeyIsSet proves the positive path: the role whose
// key the environment holds runs once and its result is the exit code.
// Method: register a counting role, set its key, dispatch.
func TestEarlyRoleRunsWhenItsKeyIsSet(t *testing.T) {
	t.Cleanup(Snapshot())
	runs := registerCounted(t, testKeyOwned, 7)
	t.Setenv(testKeyOwned, "3")

	handled, code := Dispatch()

	if !handled {
		t.Fatal("Dispatch answered not handled with the role's key in the environment")
	}
	if code != 7 {
		t.Fatalf("Dispatch answered exit code %d, want the role's 7", code)
	}
	if *runs != 1 {
		t.Fatalf("the role ran %d times, want 1", *runs)
	}
}

// TestEarlyRoleKeyIsSelectedByPresence proves an empty value still selects
// the role, so a damaged marker is judged by the role and never falls through
// to an ordinary command line.
func TestEarlyRoleKeyIsSelectedByPresence(t *testing.T) {
	t.Cleanup(Snapshot())
	runs := registerCounted(t, testKeyOwned, 0)
	t.Setenv(testKeyOwned, "")

	handled, _ := Dispatch()

	if !handled {
		t.Fatal("Dispatch answered not handled for a key present with an empty value")
	}
	if *runs != 1 {
		t.Fatalf("the role ran %d times, want 1", *runs)
	}
}

// TestEarlyRoleDoesNotRunWithoutItsKey proves ordinary startup is untouched:
// a registered role whose key is absent does not run.
func TestEarlyRoleDoesNotRunWithoutItsKey(t *testing.T) {
	t.Cleanup(Snapshot())
	runs := registerCounted(t, testKeyOwned, 7)

	handled, code := Dispatch()

	if handled {
		t.Fatalf("Dispatch answered handled (code %d) with no key in the environment", code)
	}
	if *runs != 0 {
		t.Fatalf("the role ran %d times with its key absent", *runs)
	}
}

// TestEarlyRoleKeyWithNoOwnerRefuses proves a key no role registered answers
// "not handled": a build without the owner of a key starts as it always did,
// and no registered role runs in its place.
func TestEarlyRoleKeyWithNoOwnerRefuses(t *testing.T) {
	t.Cleanup(Snapshot())
	runs := registerCounted(t, testKeyOwned, 7)
	t.Setenv(testKeyUnowned, "3")

	handled, code := Dispatch()

	if handled {
		t.Fatalf("Dispatch answered handled (code %d) for a key no role registered", code)
	}
	if *runs != 0 {
		t.Fatalf("another key's role ran %d times", *runs)
	}
}

// TestEarlyRoleRefusesTwoKeysInOneEnvironment proves two selected roles are
// refused before either runs.
func TestEarlyRoleRefusesTwoKeysInOneEnvironment(t *testing.T) {
	t.Cleanup(Snapshot())
	first := registerCounted(t, testKeyOwned, 0)
	second := registerCounted(t, testKeyOther, 0)
	t.Setenv(testKeyOwned, "3")
	t.Setenv(testKeyOther, "4")

	handled, code := Dispatch()

	if !handled {
		t.Fatal("Dispatch answered not handled with two roles selected")
	}
	if code != 2 {
		t.Fatalf("Dispatch answered exit code %d, want the refusal's 2", code)
	}
	if *first != 0 {
		t.Fatalf("the first role ran %d times before the refusal", *first)
	}
	if *second != 0 {
		t.Fatalf("the second role ran %d times before the refusal", *second)
	}
}

// TestRegisterRefusesASecondOwnerOfOneKey proves the registry holds one role
// for one key: a second registration is a Ze defect and panics.
func TestRegisterRefusesASecondOwnerOfOneKey(t *testing.T) {
	t.Cleanup(Snapshot())
	registerCounted(t, testKeyOwned, 0)

	defer func() {
		if recover() == nil {
			t.Fatal("a second registration of one key did not panic")
		}
	}()
	Register(testKeyOwned, func() int { return 0 })
}
