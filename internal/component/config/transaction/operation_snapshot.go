// Design: docs/architecture/config/apply-ordering.md -- isolated registry fixtures

package transaction

import "maps"

// OperationRegistrySnapshot retains registrations for a nonparallel test.
// It copies registry entries, not state captured by a decomposer's closure.
// Its zero value represents an empty registry.
type OperationRegistrySnapshot struct {
	decomposers map[string]OperationDecomposer
	rules       map[string]ConstraintRule
	settlement  map[string]SettlementRule
}

// SnapshotOperationRegistry saves the registry before a test adds fixtures.
// The caller must not run in parallel with another registry-mutating test.
func SnapshotOperationRegistry() OperationRegistrySnapshot {
	operationRegistry.RLock()
	defer operationRegistry.RUnlock()
	return OperationRegistrySnapshot{
		decomposers: maps.Clone(operationRegistry.decomposers),
		rules:       maps.Clone(operationRegistry.rules),
		settlement:  maps.Clone(operationRegistry.settlement),
	}
}

// RestoreOperationRegistry removes a test's registrations after its users stop.
// The caller must serialize the entire save, use and restore interval. Copies
// keep the snapshot reusable without later registrations changing its contents.
func RestoreOperationRegistry(snapshot OperationRegistrySnapshot) {
	operationRegistry.Lock()
	defer operationRegistry.Unlock()
	operationRegistry.decomposers = make(map[string]OperationDecomposer, len(snapshot.decomposers))
	maps.Copy(operationRegistry.decomposers, snapshot.decomposers)
	operationRegistry.rules = make(map[string]ConstraintRule, len(snapshot.rules))
	maps.Copy(operationRegistry.rules, snapshot.rules)
	operationRegistry.settlement = make(map[string]SettlementRule, len(snapshot.settlement))
	maps.Copy(operationRegistry.settlement, snapshot.settlement)
}
