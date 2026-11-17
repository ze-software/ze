// Design: docs/architecture/api/process-protocol.md -- registered selected refresh.
package server

import "testing"

// This root can run before the contract implementation: absent registration is
// a behavioral failure, independent of the new DTOs or reactor implementation.
func TestSelectedRefreshOperationRegistered(t *testing.T) {
	operation := lookupEngineOp("ze-plugin-engine:select-refresh")
	if operation == nil {
		t.Fatal("selected refresh operation is not registered")
	}
	if operation.native == nil {
		t.Fatal("selected refresh has no native adapter")
	}
	if operation.handle == nil {
		t.Fatal("selected refresh has no public RPC adapter")
	}
	if operation.cancellationScope == nil {
		t.Fatal("selected refresh has no authenticated cancellation scope")
	}
}
