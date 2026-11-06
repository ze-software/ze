// Design: docs/architecture/plugin/feature-gates.md -- range assignments remain declaration writes.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedMutationQueriesRetainRangeWrites distinguishes a range-local
// binding from assignment to the same existing package declaration. Both policy
// queries exercise the actual committed-source mutation reader.
func TestCommittedMutationQueriesRetainRangeWrites(t *testing.T) {
	root := committedFixture(t)
	const directory = "internal/plugins/example"
	const name = directory + "/range_query_controls.go"
	const positive = `package example
var CallbackRange func()
func rangeWrite() {
 for _, local := range []func(){func(){}} { _ = local }
}
`
	const negative = `package example
var CallbackRange func()
func rangeWrite() {
 for _, CallbackRange = range []func(){func(){}} {}
}
`
	write(t, root, name, positive)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "range declaration control")
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	reader := snapshot.evaluator()
	source := mutationQueryGlobal(t, snapshot, directory, "CallbackRange")
	if err := reader.immutable(source); err != nil {
		t.Fatalf("range-local definition contaminated the package control: %v", err)
	}
	if err := reader.callableImmutable(source); err != nil {
		t.Fatalf("range-local definition contaminated the callable control: %v", err)
	}
	write(t, root, name, negative)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "range writes existing declaration")
	fresh, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	changed := fresh.evaluator()
	target := mutationQueryGlobal(t, fresh, directory, "CallbackRange")
	for _, scenario := range []struct {
		name string
		read func(sourceExpr) error
	}{
		{"declaration", changed.immutable},
		{"callable", changed.callableImmutable},
	} {
		failure := scenario.read(target)
		if failure == nil {
			t.Errorf("%s policy certified an existing declaration written by range assignment", scenario.name)
			continue
		}
		if !strings.Contains(failure.Error(), "mutated") {
			t.Errorf("%s policy lost its actual range mutation cause: %v", scenario.name, failure)
		}
	}
}

// TestCommittedMutationQueriesRetainRangeKeysAndShadows keeps same-spelled
// fresh range bindings separate while requiring an existing key target to count
// as a write even when the loop body is empty.
func TestCommittedMutationQueriesRetainRangeKeysAndShadows(t *testing.T) {
	root := committedFixture(t)
	const directory = "internal/plugins/example"
	const name = directory + "/range_key_controls.go"
	const positive = `package example
var CallbackRange func()
var KeyRange string
func rangeControl() {
 for _, CallbackRange := range []func(){func(){}} { _ = CallbackRange }
 for KeyRange := range map[string]int{"first": 1} { _ = KeyRange }
}
`
	write(t, root, name, positive)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "range shadow controls")
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	reader := snapshot.evaluator()
	for _, binding := range []string{"CallbackRange", "KeyRange"} {
		source := mutationQueryGlobal(t, snapshot, directory, binding)
		if err := reader.immutable(source); err != nil {
			t.Fatalf("fresh same-spelled %s range binding contaminated its package declaration: %v", binding, err)
		}
		if err := reader.callableImmutable(source); err != nil {
			t.Fatalf("fresh same-spelled %s range binding contaminated callable ownership: %v", binding, err)
		}
	}
	negative := strings.Replace(positive, "for KeyRange := range", "for KeyRange = range", 1)
	write(t, root, name, negative)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "range writes existing key")
	fresh, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	changed := fresh.evaluator()
	callback := mutationQueryGlobal(t, fresh, directory, "CallbackRange")
	if err := changed.immutable(callback); err != nil {
		t.Fatalf("unchanged callback shadow lost its positive control: %v", err)
	}
	target := mutationQueryGlobal(t, fresh, directory, "KeyRange")
	for _, scenario := range []struct {
		name string
		read func(sourceExpr) error
	}{
		{"declaration", changed.immutable},
		{"callable", changed.callableImmutable},
	} {
		failure := scenario.read(target)
		if failure == nil {
			t.Errorf("%s policy certified an existing range key target", scenario.name)
			continue
		}
		if !strings.Contains(failure.Error(), "mutated") {
			t.Errorf("%s policy lost its actual key mutation cause: %v", scenario.name, failure)
		}
	}
}
