// Design: docs/research/l2tpv2-ze-integration.md -- regular-file ownership model

//go:build linux

package kernel

// This private helper never calls the public native binder. Its explicit
// regular-file fixture cannot produce a kernel PPP grant or authorize a syscall.
func bindCapturedModelNativeOperations(unit *CreatedUnit, ops sessionOps) {
	done := make(chan struct{})
	close(done)
	unit.mu.Lock()
	unit.nativeOperations = &nativeUnitOperationsBinding{done: done, ownership: nativeUnitOperationsOwned{
		operations: sessionTestMutationOperations{backend: unit.backend, identity: unit.identity, ops: &ops},
	}}
	unit.mu.Unlock()
}
