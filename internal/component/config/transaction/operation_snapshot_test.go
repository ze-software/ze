// Design: docs/architecture/config/apply-ordering.md -- isolated registry fixtures

package transaction

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// A restored registry retains earlier components, removes fixture closures and
// remains reusable after subsequent registrations. This test must not parallelize.
func TestOperationRegistrySnapshot(t *testing.T) {
	original := SnapshotOperationRegistry()
	t.Cleanup(func() { RestoreOperationRegistry(original) })
	RestoreOperationRegistry(OperationRegistrySnapshot{})
	registerSnapshotFixtures(t, "retained")
	saved := SnapshotOperationRegistry()
	for range 2 {
		registerSnapshotFixtures(t, "temporary")
		RestoreOperationRegistry(saved)
		if _, ok := OperationDecomposerFor("temporary"); ok {
			t.Fatal("fixture decomposer survived restore")
		}
		fn, ok := OperationDecomposerFor("retained")
		if !ok {
			t.Fatal("earlier decomposer disappeared")
		}
		ops, err := fn(context.Background(), DecomposeRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(ops) != 1 {
			t.Fatalf("retained callback returned %v", ops)
		}
		if ops[0].ID != "retained" {
			t.Fatalf("wrong retained callback: %v", ops)
		}
		current := SnapshotOperationRegistry()
		if !reflect.DeepEqual(current.rules, saved.rules) {
			t.Fatal("constraint rules were not restored")
		}
		if !reflect.DeepEqual(current.settlement, saved.settlement) {
			t.Fatal("settlement rules were not restored")
		}
	}
}

func registerSnapshotFixtures(t *testing.T, id string) {
	t.Helper()
	if err := RegisterOperationDecomposer(id, func(context.Context, DecomposeRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{ID: id}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	selector := OperationSelector{Type: testOpAddInterface}
	if err := RegisterConstraintRule(ConstraintRule{ID: id, Before: selector, After: selector}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSettlementRule(SettlementRule{
		ID: id, Operation: selector, Timeout: time.Second,
		Readiness: ConfigOperationReadiness{Namespace: id, EventType: "ready"},
	}); err != nil {
		t.Fatal(err)
	}
}
